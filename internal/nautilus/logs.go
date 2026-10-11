package nautilus

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"

	"perch/internal/domain"
)

const MaxLogBytes = 256 << 10
const LogTailLines = 2000
const maxPodBytes = 1 << 20

type PodRequest struct {
	Scope
	Name string
}

func validObjectName(name string) bool {
	if len(name) == 0 || len(name) > 253 {
		return false
	}
	for _, part := range strings.Split(name, ".") {
		if !namespacePattern.MatchString(part) {
			return false
		}
	}
	return true
}

func (r PodRequest) Path() (string, error) {
	if !validScope(r.Scope) || !validObjectName(r.Name) {
		return "", queryError(Invalid, "Invalid namespace Pod request")
	}
	return "/api/v1/namespaces/" + r.Namespace + "/pods/" + r.Name, nil
}

type LogRequest struct {
	Pod       Ref
	Container string
	Previous  bool
}

func (r LogRequest) Path() (string, error) {
	path, err := (PodRequest{r.Pod.Scope, r.Pod.Name}).Path()
	if err != nil || r.Pod.Kind != "Pod" || !safeIdentity(r.Pod.UID) || !namespacePattern.MatchString(r.Container) {
		return "", queryError(Invalid, "Select a Pod and an explicit container for logs")
	}
	query := url.Values{"container": {r.Container}, "previous": {strconv.FormatBool(r.Previous)},
		"follow": {"false"}, "limitBytes": {strconv.Itoa(MaxLogBytes)}, "tailLines": {strconv.Itoa(LogTailLines)}}
	return path + "/log?" + query.Encode(), nil
}

type LogReader interface {
	ReadPod(context.Context, PodRequest) ([]byte, error)
	ReadContainerLog(context.Context, LogRequest) ([]byte, error)
}

func (r *HTTPSReader) ReadPod(ctx context.Context, request PodRequest) ([]byte, error) {
	if request.Scope != r.scope {
		return nil, queryError(Invalid, "Pod request does not match this connection")
	}
	route, err := request.Path()
	if err != nil {
		return nil, err
	}
	return r.readBounded(ctx, route, "application/json", maxPodBytes)
}

func (r *HTTPSReader) ReadContainerLog(ctx context.Context, request LogRequest) ([]byte, error) {
	if request.Pod.Scope != r.scope {
		return nil, queryError(Invalid, "Log request does not match this connection")
	}
	route, err := request.Path()
	if err != nil {
		return nil, err
	}
	return r.readBounded(ctx, route, "text/plain", MaxLogBytes)
}

func parsePod(data []byte, scope Scope) (Pod, error) {
	invalid := func() (Pod, error) { return Pod{}, queryError(InvalidResponse, "Invalid Pod response") }
	if len(data) > maxPodBytes || validateJSON(data) != nil {
		return invalid()
	}
	var raw rawPod
	if json.Unmarshal(data, &raw) != nil || raw.Kind != "Pod" || raw.APIVersion != "v1" {
		return invalid()
	}
	// Reuse the same status/container validation as namespace lists.
	envelope, _ := json.Marshal(struct {
		Kind       string            `json:"kind"`
		APIVersion string            `json:"apiVersion"`
		Metadata   map[string]string `json:"metadata"`
		Items      []json.RawMessage `json:"items"`
	}{"PodList", "v1", map[string]string{"resourceVersion": raw.Metadata.ResourceVersion}, []json.RawMessage{data}})
	page, err := ParsePods(envelope, scope)
	if err != nil || len(page.Items) != 1 {
		return invalid()
	}
	return page.Items[0], nil
}

func (c *Client) GetPod(ctx context.Context, name string) (Pod, error) {
	request := PodRequest{c.scope, name}
	if _, err := request.Path(); err != nil {
		return Pod{}, err
	}
	reader, ok := c.reader.(LogReader)
	if !ok {
		return Pod{}, queryError(Unavailable, "This connection does not support Pod detail reads")
	}
	data, err := reader.ReadPod(ctx, request)
	if err != nil {
		return Pod{}, readError(err)
	}
	pod, err := parsePod(data, c.scope)
	if err != nil {
		return Pod{}, err
	}
	if pod.Name != name {
		return Pod{}, queryError(InvalidResponse, "Pod response did not match the requested name")
	}
	if err := ctx.Err(); err != nil {
		return Pod{}, readError(err)
	}
	return pod, nil
}

type LogSnapshot struct {
	Pod        Ref       `json:"pod"`
	Container  string    `json:"container"`
	Previous   bool      `json:"previous"`
	ObservedAt time.Time `json:"observed_at"`
	Text       string    `json:"text"`
	ByteLimit  int       `json:"byte_limit"`
	TailLines  int       `json:"tail_lines"`
	// This is a bounded observation, never a claim to contain the complete log.
}

func findContainer(pod Pod, name string) (Container, bool) {
	for _, c := range pod.Containers {
		if c.Name == name {
			return c, true
		}
	}
	return Container{}, false
}

// ReadLogs validates Pod UID before and after the name-addressed log request.
// A replacement or observed restart discards the payload instead of attributing
// it to the wrong workload. Nothing is streamed before these checks complete.
func (c *Client) ReadLogs(parent context.Context, request LogRequest) (LogSnapshot, error) {
	if _, err := request.Path(); err != nil {
		return LogSnapshot{}, err
	}
	if request.Pod.Scope != c.scope {
		return LogSnapshot{}, queryError(Invalid, "Log request scope does not match this workspace")
	}
	ctx, cancel := context.WithTimeout(parent, 25*time.Second)
	defer cancel()
	before, err := c.GetPod(ctx, request.Pod.Name)
	if err != nil {
		return LogSnapshot{}, err
	}
	if before.UID != request.Pod.UID {
		return LogSnapshot{}, queryError(Invalid, "Pod was replaced; refresh and select it again")
	}
	container, ok := findContainer(before, request.Container)
	if !ok {
		return LogSnapshot{}, queryError(Invalid, "Container is not part of this Pod")
	}
	reader := c.reader.(LogReader) // GetPod already checks this capability.
	data, err := reader.ReadContainerLog(ctx, request)
	if err != nil {
		return LogSnapshot{}, readError(err)
	}
	if len(data) > MaxLogBytes {
		return LogSnapshot{}, readError(queryError(LimitExceeded, ""))
	}
	after, err := c.GetPod(ctx, request.Pod.Name)
	if err != nil {
		return LogSnapshot{}, err
	}
	next, ok := findContainer(after, request.Container)
	if after.UID != before.UID || !ok {
		return LogSnapshot{}, queryError(Invalid, "Pod changed while logs were read; refresh and try again")
	}
	if (container.Restarts == nil) != (next.Restarts == nil) || container.Restarts != nil && *container.Restarts != *next.Restarts {
		return LogSnapshot{}, queryError(Invalid, "Container restarted while logs were read; try again")
	}
	if err := ctx.Err(); err != nil {
		return LogSnapshot{}, readError(err)
	}
	return LogSnapshot{Pod: request.Pod, Container: request.Container, Previous: request.Previous,
		ObservedAt: time.Now(), Text: domain.Clean(string(data)), ByteLimit: MaxLogBytes, TailLines: LogTailLines}, nil
}
