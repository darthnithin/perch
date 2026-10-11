package nautilus

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

type metricFixtureReader struct {
	logFixtureReader
	metricCalls int
}

func (r *metricFixtureReader) MetricsVersion(context.Context) (string, error) {
	if r.mode == "no-api" {
		return "", queryError(NotFound, "private server data")
	}
	return "v1beta1", nil
}
func (r *metricFixtureReader) ReadPodMetrics(ctx context.Context, req MetricsRequest) ([]byte, error) {
	r.metricCalls++
	if r.mode == "denied" {
		return nil, queryError(Forbidden, "private server data")
	}
	if r.mode == "no-sample" {
		return nil, queryError(NotFound, "private server data")
	}
	if r.mode == "oversize" {
		return []byte(strings.Repeat("x", maxMetricsBytes+1)), nil
	}
	data, err := r.demoReader.ReadPodMetrics(ctx, req)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	json.Unmarshal(data, &raw)
	meta := raw["metadata"].(map[string]any)
	entries := raw["containers"].([]any)
	entry := entries[0].(map[string]any)
	for _, candidate := range entries {
		if candidate.(map[string]any)["name"] == "compute" {
			entry = candidate.(map[string]any)
		}
	}
	usage := entry["usage"].(map[string]any)
	switch r.mode {
	case "zero":
		usage["cpu"] = "0"
		delete(usage, "memory")
	case "stale":
		raw["timestamp"] = time.Now().Add(-10 * time.Minute)
	case "future":
		raw["timestamp"] = time.Now().Add(time.Minute)
	case "wrong-namespace":
		meta["namespace"] = "other"
	case "metric-uid":
		meta["uid"] = "old-pod"
	case "old-sample":
		raw["timestamp"] = "2000-01-01T00:00:00Z"
	case "wrong-version":
		raw["apiVersion"] = "v1"
	case "unknown-container":
		entry["name"] = "absent"
	case "duplicate":
		raw["containers"] = append(entries, entry)
	case "negative":
		usage["cpu"] = "-1"
	case "exponent":
		usage["cpu"] = "1e999999999"
	case "bad-window":
		raw["window"] = "-1s"
	case "empty":
		raw["containers"] = []any{}
	}
	return json.Marshal(raw)
}

func TestMetricsIdentityQuantitiesAndAvailability(t *testing.T) {
	for _, mode := range []string{"ok", "zero", "empty", "stale", "future", "wrong-namespace", "metric-uid", "old-sample", "wrong-version", "unknown-container", "duplicate", "negative", "exponent", "bad-window", "oversize", "denied", "no-api", "no-sample", "replaced", "restart", "gone", "wrong-scope", "wrong-uid"} {
		t.Run(mode, func(t *testing.T) {
			c := NewDemoClient()
			pod, err := c.GetPod(context.Background(), "retrying-replacement")
			if err != nil {
				t.Fatal(err)
			}
			r := &metricFixtureReader{logFixtureReader: logFixtureReader{mode: mode}}
			c.reader = r
			ref := pod.Ref
			if mode == "wrong-scope" {
				ref.Namespace = "other"
			}
			if mode == "wrong-uid" {
				ref.UID = "old"
			}
			out, err := c.ReadMetrics(context.Background(), ref)
			switch mode {
			case "ok", "zero", "empty", "stale":
				if err != nil {
					t.Fatal(err)
				}
				if out.Pod.UID != pod.UID || out.Window != "30s" || out.Stale != (mode == "stale") {
					t.Fatal(out)
				}
				if mode == "empty" {
					if len(out.Containers) != 0 {
						t.Fatal(out)
					}
					return
				}
				if mode == "zero" {
					if *out.Containers[0].CPUCores != "0" || out.Containers[0].MemoryBytes != nil {
						t.Fatal(out)
					}
					return
				}
				if *out.Containers[0].CPUCores != "0.215" || *out.Containers[0].MemoryBytes != "153092096" {
					t.Fatal(out.Containers)
				}
			default:
				if err == nil || len(out.Containers) != 0 || strings.Contains(err.Error(), "private") {
					t.Fatal(out, err)
				}
				if (mode == "wrong-scope" || mode == "wrong-uid" || mode == "no-api") && r.metricCalls != 0 {
					t.Fatal("unauthorized read")
				}
				if mode == "no-api" {
					checkQueryError(t, err, Unavailable)
				}
				if mode == "no-sample" {
					checkQueryError(t, err, NotFound)
				}
			}
		})
	}
}

func TestMetricsContainerWindowAttribution(t *testing.T) {
	c := NewDemoClient()
	pod, _ := c.GetPod(context.Background(), "retrying-replacement")
	now := time.Now()
	start := now.Add(-15 * time.Second)
	for i := range pod.Containers {
		if pod.Containers[i].State.Phase == "Running" {
			pod.Containers[i].State.Started = &start
		}
	}
	data, _ := demoReader{}.ReadPodMetrics(context.Background(), MetricsRequest{pod.Ref, "v1"})
	sample, err := parseMetrics(data, pod, "v1", now)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range sample.Containers {
		if c.Attribution != "spans-container-start" {
			t.Fatal(c)
		}
	}
}

func TestMetricsTLSVersionDiscoveryAndBoundedScope(t *testing.T) {
	for _, version := range []string{"v1", "v1beta1"} {
		t.Run(version, func(t *testing.T) {
			var paths []string
			s := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				if r.Header.Get("Authorization") != "Bearer "+testBearer {
					t.Error("missing credentials")
				}
				if r.URL.Path == "/apis/metrics.k8s.io" {
					json.NewEncoder(w).Encode(map[string]any{"kind": "APIGroup", "apiVersion": "v1", "name": "metrics.k8s.io", "versions": []map[string]string{{"version": version, "groupVersion": "metrics.k8s.io/" + version}, {"version": "v1beta1", "groupVersion": "metrics.k8s.io/v1beta1"}}})
					return
				}
				w.Write([]byte("{}"))
			})
			r := httpsReader(t, serverConfig(s))
			got, err := r.MetricsVersion(context.Background())
			if err != nil || got != version {
				t.Fatal(got, err)
			}
			ref := Ref{Scope: fixtureScope, Kind: "Pod", Name: "sample", UID: "uid-1"}
			_, err = r.ReadPodMetrics(context.Background(), MetricsRequest{ref, got})
			if err != nil {
				t.Fatal(err)
			}
			if len(paths) != 2 || paths[1] != "/apis/metrics.k8s.io/"+version+"/namespaces/"+fixtureScope.Namespace+"/pods/sample" {
				t.Fatal(paths)
			}
			ref.Namespace = "other"
			_, err = r.ReadPodMetrics(context.Background(), MetricsRequest{ref, got})
			checkQueryError(t, err, Invalid)
			if len(paths) != 2 {
				t.Fatal("wrong namespace reached server")
			}
		})
	}
}

func TestMetricsHTTPFailureBoundaries(t *testing.T) {
	for _, mode := range []string{"missing", "forbidden", "unsupported", "malformed", "oversize"} {
		t.Run(mode, func(t *testing.T) {
			s := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "missing":
					w.WriteHeader(404)
					w.Write([]byte("private-server-output"))
				case "forbidden":
					w.WriteHeader(403)
					w.Write([]byte("private-server-output"))
				case "unsupported":
					w.Write([]byte(`{"kind":"APIGroup","apiVersion":"v1","name":"metrics.k8s.io","versions":[{"version":"v2","groupVersion":"metrics.k8s.io/v2"}]}`))
				case "malformed":
					w.Write([]byte(`{"kind":"private-server-output"}`))
				case "oversize":
					w.Header().Set("Content-Length", "1048577")
					w.WriteHeader(200)
				}
			})
			r := httpsReader(t, serverConfig(s))
			var err error
			if mode == "oversize" {
				_, err = r.ReadPodMetrics(context.Background(), MetricsRequest{Ref{Scope: fixtureScope, Kind: "Pod", Name: "sample", UID: "uid"}, "v1"})
			} else {
				_, err = r.MetricsVersion(context.Background())
			}
			if err == nil || strings.Contains(err.Error(), "private-server-output") {
				t.Fatal(err)
			}
		})
	}
}
