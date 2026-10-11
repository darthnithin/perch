package nautilus

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

const maxMetricsBytes = 1 << 20

type MetricsRequest struct {
	Pod     Ref
	Version string
}

func (r MetricsRequest) Path() (string, error) {
	_, err := (PodRequest{r.Pod.Scope, r.Pod.Name}).Path()
	if err != nil || r.Pod.Kind != "Pod" || !safeIdentity(r.Pod.UID) || (r.Version != "v1" && r.Version != "v1beta1") {
		return "", queryError(Invalid, "Select a Pod for resource metrics")
	}
	return "/apis/metrics.k8s.io/" + r.Version + "/namespaces/" + r.Pod.Namespace + "/pods/" + r.Pod.Name, nil
}

type MetricsReader interface {
	MetricsVersion(context.Context) (string, error)
	ReadPodMetrics(context.Context, MetricsRequest) ([]byte, error)
}

func metricsError(err error, discovery bool) error {
	var q *QueryError
	if errors.As(err, &q) && q.Kind == NotFound {
		if discovery {
			return queryError(Unavailable, "Pod metrics API is not available on this cluster")
		}
		return queryError(NotFound, "No metrics sample is available for this Pod yet")
	}
	return readError(err)
}

func (r *HTTPSReader) MetricsVersion(ctx context.Context) (string, error) {
	data, err := r.readBounded(ctx, "/apis/metrics.k8s.io", "application/json", 64<<10)
	if err != nil {
		return "", readError(err)
	}
	var group struct {
		Kind, APIVersion, Name string
		Versions               []struct{ GroupVersion, Version string }
	}
	if validateJSON(data) != nil || json.Unmarshal(data, &group) != nil || group.Kind != "APIGroup" || group.APIVersion != "v1" || group.Name != "metrics.k8s.io" {
		return "", queryError(InvalidResponse, "Invalid metrics API discovery response")
	}
	for _, wanted := range []string{"v1", "v1beta1"} {
		for _, v := range group.Versions {
			if v.Version == wanted && v.GroupVersion == "metrics.k8s.io/"+wanted {
				return wanted, nil
			}
		}
	}
	return "", queryError(Unavailable, "No supported Pod metrics API is available")
}

func (r *HTTPSReader) ReadPodMetrics(ctx context.Context, request MetricsRequest) ([]byte, error) {
	if request.Pod.Scope != r.scope {
		return nil, queryError(Invalid, "Metrics request does not match this connection")
	}
	route, err := request.Path()
	if err != nil {
		return nil, err
	}
	return r.readBounded(ctx, route, "application/json", maxMetricsBytes)
}

// Decimal strings preserve exact resource quantities without float rounding.
// Nil means unreported, never zero. Values are not allocation or peak metrics.
type ContainerMetrics struct {
	Name        string  `json:"name"`
	Role        string  `json:"role"`
	CPUCores    *string `json:"cpu_cores"`
	MemoryBytes *string `json:"memory_working_set_bytes"`
	Attribution string  `json:"attribution"`
}

type MetricsSnapshot struct {
	Pod        Ref                `json:"pod"`
	APIVersion string             `json:"api_version"`
	ObservedAt time.Time          `json:"observed_at"`
	Timestamp  time.Time          `json:"timestamp"`
	Window     string             `json:"window"`
	Stale      bool               `json:"stale"`
	Containers []ContainerMetrics `json:"containers"`
}

func metricDecimal(s string) (*string, error) {
	q, err := quotaQuantity(s)
	if err != nil {
		return nil, err
	}
	v := q.AsDec().String()
	if strings.Contains(v, ".") {
		v = strings.TrimRight(strings.TrimRight(v, "0"), ".")
	}
	return &v, nil
}

func parseMetrics(data []byte, pod Pod, version string, observed time.Time) (MetricsSnapshot, error) {
	invalid := func() (MetricsSnapshot, error) {
		return MetricsSnapshot{}, queryError(InvalidResponse, "Invalid Pod metrics response")
	}
	var raw struct {
		Kind, APIVersion string
		Metadata         struct{ Name, Namespace, UID string }
		Timestamp        time.Time
		Window           string
		Containers       []struct {
			Name  string
			Usage map[string]string
		}
	}
	if len(data) > maxMetricsBytes || validateJSON(data) != nil || json.Unmarshal(data, &raw) != nil {
		return invalid()
	}
	window, err := time.ParseDuration(raw.Window)
	if err != nil || window <= 0 || window > 24*time.Hour || raw.Timestamp.IsZero() || raw.Timestamp.After(observed.Add(5*time.Second)) || raw.Kind != "PodMetrics" || raw.APIVersion != "metrics.k8s.io/"+version || raw.Metadata.Name != pod.Name || raw.Metadata.Namespace != pod.Namespace || (raw.Metadata.UID != "" && raw.Metadata.UID != pod.UID) || len(raw.Containers) > 1024 {
		return invalid()
	}
	start := raw.Timestamp.Add(-window)
	if pod.Created == nil {
		return MetricsSnapshot{}, queryError(InvalidResponse, "Pod creation time is unavailable; cannot verify this sample")
	}
	if start.Before(*pod.Created) {
		return MetricsSnapshot{}, queryError(InvalidResponse, "Metrics sample predates this Pod; retry after a new sample")
	}
	out := MetricsSnapshot{Pod: pod.Ref, APIVersion: raw.APIVersion, ObservedAt: observed, Timestamp: raw.Timestamp, Window: raw.Window, Stale: observed.Sub(raw.Timestamp) > max(2*window, 2*time.Minute), Containers: []ContainerMetrics{}}
	seen := map[string]bool{}
	for _, v := range raw.Containers {
		container, ok := findContainer(pod, v.Name)
		if !ok || seen[v.Name] {
			return invalid()
		}
		seen[v.Name] = true
		entry := ContainerMetrics{Name: v.Name, Role: container.Role, Attribution: "instance-unverified"}
		if container.State.Phase == "Running" && container.State.Started != nil {
			entry.Attribution = "current-instance"
			if start.Before(*container.State.Started) {
				entry.Attribution = "spans-container-start"
				if raw.Timestamp.Before(*container.State.Started) {
					entry.Attribution = "predates-current-instance"
				}
			}
		}
		for _, field := range []struct {
			name string
			dst  **string
		}{{"cpu", &entry.CPUCores}, {"memory", &entry.MemoryBytes}} {
			if value, ok := v.Usage[field.name]; ok {
				*field.dst, err = metricDecimal(value)
				if err != nil {
					return invalid()
				}
			}
		}
		out.Containers = append(out.Containers, entry)
	}
	rank := map[string]int{"app": 0, "sidecar": 1, "init": 2, "ephemeral": 3}
	sort.SliceStable(out.Containers, func(i, j int) bool {
		a, b := out.Containers[i], out.Containers[j]
		if rank[a.Role] != rank[b.Role] {
			return rank[a.Role] < rank[b.Role]
		}
		return a.Name < b.Name
	})
	return out, nil
}

// Metrics are fetched on demand; they add no requests to namespace refreshes.
// A name-addressed metrics cache is bracketed by UID and container-instance reads.
func (c *Client) ReadMetrics(parent context.Context, pod Ref) (MetricsSnapshot, error) {
	if _, err := (MetricsRequest{pod, "v1"}).Path(); err != nil || pod.Scope != c.scope {
		return MetricsSnapshot{}, queryError(Invalid, "Metrics request does not match this workspace")
	}
	r, ok := c.reader.(MetricsReader)
	if !ok {
		return MetricsSnapshot{}, queryError(Unavailable, "This connection does not support Pod metrics")
	}
	ctx, cancel := context.WithTimeout(parent, 25*time.Second)
	defer cancel()
	before, err := c.GetPod(ctx, pod.Name)
	if err != nil {
		return MetricsSnapshot{}, err
	}
	if before.UID != pod.UID {
		return MetricsSnapshot{}, queryError(Invalid, "Pod was replaced; refresh and select it again")
	}
	version, err := r.MetricsVersion(ctx)
	if err != nil {
		return MetricsSnapshot{}, metricsError(err, true)
	}
	request := MetricsRequest{pod, version}
	if _, err = request.Path(); err != nil {
		return MetricsSnapshot{}, err
	}
	data, err := r.ReadPodMetrics(ctx, request)
	if err != nil {
		return MetricsSnapshot{}, metricsError(err, false)
	}
	result, err := parseMetrics(data, before, version, time.Now())
	if err != nil {
		return MetricsSnapshot{}, err
	}
	after, err := c.GetPod(ctx, pod.Name)
	if err != nil {
		return MetricsSnapshot{}, err
	}
	if before.UID != after.UID {
		return MetricsSnapshot{}, queryError(Invalid, "Pod changed while metrics were read; refresh and try again")
	}
	for _, sample := range result.Containers {
		a, _ := findContainer(before, sample.Name)
		b, ok := findContainer(after, sample.Name)
		if !ok || (a.Restarts == nil) != (b.Restarts == nil) || (a.Restarts != nil && *a.Restarts != *b.Restarts) || a.State.Phase != b.State.Phase || (a.State.Started == nil) != (b.State.Started == nil) || (a.State.Started != nil && !a.State.Started.Equal(*b.State.Started)) {
			return MetricsSnapshot{}, queryError(Invalid, "Container changed while metrics were read; try again")
		}
	}
	if err := ctx.Err(); err != nil {
		return MetricsSnapshot{}, readError(err)
	}
	return result, nil
}
