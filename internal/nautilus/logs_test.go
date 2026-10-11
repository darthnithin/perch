package nautilus

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"perch/internal/domain"
)

type logFixtureReader struct {
	demoReader
	mode        string
	reads, logs int
	previous    bool
}

func (r *logFixtureReader) ReadPod(ctx context.Context, req PodRequest) ([]byte, error) {
	r.reads++
	data, err := r.demoReader.ReadPod(ctx, req)
	if err != nil || r.reads != 2 {
		return data, err
	}
	if r.mode == "gone" {
		return nil, queryError(NotFound, "server-secret")
	}
	var pod map[string]any
	json.Unmarshal(data, &pod)
	if r.mode == "replaced" {
		pod["metadata"].(map[string]any)["uid"] = "new-uid"
	}
	if r.mode == "restart" {
		for _, c := range pod["status"].(map[string]any)["containerStatuses"].([]any) {
			container := c.(map[string]any)
			if container["name"] == "compute" {
				container["restartCount"] = 3
			}
		}
	}
	return json.Marshal(pod)
}
func (r *logFixtureReader) ReadContainerLog(ctx context.Context, req LogRequest) ([]byte, error) {
	r.logs++
	r.previous = req.Previous
	if r.mode == "empty" {
		return nil, nil
	}
	if r.mode == "oversize" {
		return []byte(strings.Repeat("x", MaxLogBytes+1)), nil
	}
	if r.mode == "denied" {
		return nil, queryError(Forbidden, "server-secret")
	}
	return []byte("checkpoint\x1b[2J\n\x1b]52;c;clipboard\aend\n"), nil
}

func TestLogsVerifyUIDAndContainerWithoutLeakingDiscardedOutput(t *testing.T) {
	for _, mode := range []string{"ok", "previous", "empty", "wrong-scope", "wrong-uid", "missing-container", "replaced", "restart", "gone", "oversize", "denied"} {
		t.Run(mode, func(t *testing.T) {
			c := NewDemoClient()
			pod, err := c.GetPod(context.Background(), "retrying-replacement")
			if err != nil {
				t.Fatal(err)
			}
			reader := &logFixtureReader{mode: mode}
			c.reader = reader
			req := LogRequest{Pod: pod.Ref, Container: "compute", Previous: mode == "previous"}
			switch mode {
			case "wrong-scope":
				req.Pod.Namespace = "other"
			case "wrong-uid":
				req.Pod.UID = "old-uid"
			case "missing-container":
				req.Container = "absent"
			}
			out, err := c.ReadLogs(context.Background(), req)
			if mode == "ok" || mode == "previous" || mode == "empty" {
				want := "checkpoint\nend\n"
				if mode == "empty" {
					want = ""
				}
				if err != nil || out.Text != want || out.Pod != pod.Ref || out.Previous != req.Previous || reader.previous != req.Previous || reader.reads != 2 || reader.logs != 1 || out.ByteLimit != MaxLogBytes || out.TailLines != LogTailLines || out.ObservedAt.IsZero() {
					t.Fatalf("invalid log observation: %+v %v", out, err)
				}
			} else {
				if err == nil || out.Text != "" || !out.ObservedAt.IsZero() {
					t.Fatalf("discarded log escaped: %+v %v", out, err)
				}
				if strings.Contains(err.Error(), "server-secret") {
					t.Fatal("reader error escaped")
				}
				if (mode == "wrong-scope" || mode == "wrong-uid" || mode == "missing-container") && reader.logs != 0 {
					t.Fatal("read logs before validating target")
				}
			}
		})
	}
}

func TestLogEndpointsAreBoundedAndAllowlisted(t *testing.T) {
	ref := Ref{fixtureScope, "Pod", "uid", "sample.pod"}
	for _, name := range []string{"../escape", "x/log", "x?follow=true", "%2e%2e", "", strings.Repeat("x", 64), "-x"} {
		bad := ref
		bad.Name = name
		if _, err := (LogRequest{Pod: bad, Container: "compute"}).Path(); err == nil {
			t.Fatal("invalid Pod name accepted", name)
		}
		if _, err := (LogRequest{Pod: ref, Container: name}).Path(); err == nil {
			t.Fatal("invalid container accepted", name)
		}
	}
	var calls atomic.Int32
	s := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/api/v1/namespaces/research/pods/sample.pod/log" || r.Method != "GET" || r.Header.Get("Accept") != "text/plain" || r.Header.Get("Authorization") != "Bearer "+testBearer {
			t.Error("bad log request")
		}
		q := r.URL.Query()
		if len(q) != 5 || q.Get("container") != "compute" || q.Get("follow") != "false" || q.Get("previous") != "true" || q.Get("limitBytes") != "262144" || q.Get("tailLines") != "2000" {
			t.Error("lost bounded explicit log options", q)
		}
		w.Write([]byte("hello\n"))
	})
	r := httpsReader(t, serverConfig(s))
	data, err := r.ReadContainerLog(context.Background(), LogRequest{ref, "compute", true})
	if err != nil || string(data) != "hello\n" {
		t.Fatal(string(data), err)
	}
	ref.Cluster = "other"
	_, err = r.ReadContainerLog(context.Background(), LogRequest{ref, "compute", true})
	checkQueryError(t, err, Invalid)
	if calls.Load() != 1 {
		t.Fatal("cross-cluster request reached the server")
	}
}

func TestLogHTTPRejectsOversizeAndPrivateErrors(t *testing.T) {
	for _, mode := range []string{"length", "chunked", "forbidden", "missing", "unauthorized"} {
		t.Run(mode, func(t *testing.T) {
			s := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "length":
					w.Header().Set("Content-Length", "262145")
				case "chunked":
					w.WriteHeader(200)
					w.(http.Flusher).Flush()
				case "forbidden":
					w.WriteHeader(403)
				case "missing":
					w.WriteHeader(404)
				case "unauthorized":
					w.WriteHeader(401)
				}
				if mode == "length" || mode == "chunked" {
					w.Write([]byte(strings.Repeat("x", MaxLogBytes+1)))
				} else {
					w.Write([]byte("server-secret"))
				}
			})
			r := httpsReader(t, serverConfig(s))
			data, err := r.ReadContainerLog(context.Background(), LogRequest{Ref{fixtureScope, "Pod", "uid", "sample"}, "compute", false})
			kind := map[string]ErrorKind{"length": LimitExceeded, "chunked": LimitExceeded, "forbidden": Forbidden, "missing": NotFound, "unauthorized": Unauthenticated}[mode]
			checkQueryError(t, err, kind)
			if len(data) != 0 {
				t.Fatal("partial/error body escaped")
			}
		})
	}
}

func TestSinglePodValidationAndDemoPreviousLogs(t *testing.T) {
	c := NewDemoClient()
	pod, err := c.GetPod(context.Background(), "retrying-replacement")
	if err != nil {
		t.Fatal(err)
	}
	for _, previous := range []bool{false, true} {
		out, err := c.ReadLogs(context.Background(), LogRequest{pod.Ref, "compute", previous})
		if err != nil || !strings.Contains(out.Text, "[demo]") || out.Text != domain.Clean(out.Text) {
			t.Fatal(out, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.ReadLogs(ctx, LogRequest{pod.Ref, "compute", false}); err == nil {
		t.Fatal("cancelled logs returned data")
	}
	for _, raw := range []string{`{}`, `{"apiVersion":"v1","kind":"Status","code":200}`, `{"kind":"Pod","kind":"Pod"}`, strings.Repeat(" ", maxPodBytes+1)} {
		if _, err := parsePod([]byte(raw), fixtureScope); err == nil {
			t.Fatal("invalid object accepted")
		}
	}
}
