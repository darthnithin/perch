package nautilus

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const validationManifest = `{"apiVersion":"batch/v1","kind":"Job","metadata":{"name":"analysis","namespace":"research"},"spec":{"backoffLimit":0}}`

func TestJobValidationSeparatesRejectionFromUnverified(t *testing.T) {
	for _, tc := range []struct {
		kind         ErrorKind
		status, hint string
	}{
		{JobRejected, "rejected", "correct the manifest"},
		{JobConflict, "rejected", "metadata.name"},
		{Forbidden, "rejected", "can create Jobs"},
		{Unauthenticated, "unverified", "--login"},
		{TLSFailed, "unverified", "certificate authority"},
		{Cancelled, "unverified", "cancelled check"},
		{NotFound, "unverified", "batch/v1"},
		{TimedOut, "unverified", "retry this dry-run"},
		{ReadFailed, "unverified", "retry this dry-run"},
		{InvalidResponse, "unverified", "retry this dry-run"},
		{Unavailable, "unverified", "retry this dry-run"},
	} {
		t.Run(string(tc.kind), func(t *testing.T) {
			status, hint := jobValidationOutcome(false, queryError(tc.kind, "server-secret"))
			if status != tc.status || !strings.Contains(hint, tc.hint) || strings.Contains(hint, "server-secret") {
				t.Fatal(status, hint)
			}
		})
	}
	if status, _ := jobValidationOutcome(false, nil); status != "unverified" {
		t.Fatal("missing evidence became an admission decision", status)
	}
	if status, _ := jobValidationOutcome(true, nil); status != "accepted" {
		t.Fatal(status)
	}
}

func TestJobValidationLocalBoundary(t *testing.T) {
	for _, data := range []string{"", "{}", validationManifest + "{}", strings.Replace(validationManifest, `"Job"`, `"Pod"`, 1), strings.Replace(validationManifest, `"research"`, `"other"`, 1), strings.Replace(validationManifest, `"name":"analysis"`, `"name":"analysis","name":"other"`, 1), strings.Replace(validationManifest, `"name":"analysis"`, `"generateName":"analysis-"`, 1), strings.Repeat(" ", MaxJobManifestBytes+1)} {
		if _, err := InspectJobManifest([]byte(data), "research"); err == nil {
			t.Fatal("accepted invalid request")
		}
	}
	if name, err := InspectJobManifest([]byte(validationManifest), "research"); err != nil || name != "analysis" {
		t.Fatal(name, err)
	}
}

type untrustedJobValidator struct{}

func (untrustedJobValidator) ReadPage(context.Context, ListRequest) ([]byte, error) {
	return nil, queryError(Forbidden, "server-secret")
}
func (untrustedJobValidator) ValidateJob(context.Context, Scope, []byte) error {
	return queryError(JobRejected, "server-secret")
}
func TestJobValidationRedactsTransportMessages(t *testing.T) {
	c, _ := NewClient(fixtureScope, untrustedJobValidator{})
	result, err := c.ValidateJob(context.Background(), []byte(validationManifest))
	data, _ := json.Marshal(result)
	if err != nil || result.Admitted || result.Status != "rejected" || result.NextStep == "" || result.Error.Kind != JobRejected || strings.Contains(string(data), "server-secret") {
		t.Fatal(result, err)
	}
}

func TestJobValidationHTTPSNeverPersistsAndKeepsQuotaSeparate(t *testing.T) {
	var posts, reads atomic.Int32
	s := tlsServer(t, func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer "+testBearer {
			t.Error("missing bound credentials")
		}
		switch req.Method {
		case "POST":
			posts.Add(1)
			if req.URL.RequestURI() != "/gateway/apis/batch/v1/namespaces/research/jobs?dryRun=All&fieldValidation=Strict" || req.Header.Get("Content-Type") != "application/json" {
				t.Error("unbounded write route", req.URL)
			}
			data, _ := io.ReadAll(req.Body)
			if string(data) != validationManifest {
				t.Error("manifest changed")
			}
			w.WriteHeader(201)
			w.Write(data)
		case "GET":
			reads.Add(1)
			if req.URL.Path != "/gateway/api/v1/namespaces/research/resourcequotas" {
				t.Error("unexpected read")
			}
			w.WriteHeader(403)
			w.Write([]byte("server-secret"))
		default:
			t.Error("unexpected operation")
		}
	})
	cfg := serverConfig(s)
	cfg.Server += "/gateway"
	r := httpsReader(t, cfg)
	c, _ := NewClient(fixtureScope, r)
	result, err := c.ValidateJob(context.Background(), []byte(validationManifest))
	if err != nil || !result.Admitted || result.Status != "accepted" || result.NextStep == "" || result.Error != nil || result.Quotas.OK() || result.Quotas.Error.Kind != Forbidden || len(result.ManifestSHA256) != 64 || posts.Load() != 1 || reads.Load() != 1 {
		t.Fatal(result, err)
	}
	encoded, _ := json.Marshal(result)
	if strings.Contains(string(encoded), "server-secret") || strings.Contains(string(encoded), testBearer) {
		t.Fatal("secret escaped")
	}
	before := posts.Load()
	if err := r.ValidateJob(context.Background(), Scope{Cluster: fixtureScope.Cluster, Namespace: "other"}, []byte(validationManifest)); err == nil || posts.Load() != before {
		t.Fatal("cross-scope request")
	}
	r.Close()
	if err := r.ValidateJob(context.Background(), fixtureScope, []byte(validationManifest)); err == nil || posts.Load() != before {
		t.Fatal("closed connection used")
	}
}

func TestJobValidationErrorsNeverRetryOrFollowRedirect(t *testing.T) {
	for _, code := range []int{400, 401, 403, 404, 409, 422, 429, 500, 302, 307, 308} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			var calls atomic.Int32
			s := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Location", "/persisted?server-secret")
				w.WriteHeader(code)
				w.Write([]byte("server-secret"))
			})
			r := httpsReader(t, serverConfig(s))
			err := r.ValidateJob(context.Background(), fixtureScope, []byte(validationManifest))
			if err == nil || calls.Load() != 1 || strings.Contains(err.Error(), "server-secret") {
				t.Fatal(err, calls.Load())
			}
		})
	}
}

func TestJobValidationRejectsWrongIdentityAndExpiredCredentials(t *testing.T) {
	for _, body := range []string{`{}`, strings.Replace(validationManifest, `"analysis"`, `"replacement"`, 1), strings.Replace(validationManifest, `"research"`, `"other"`, 1), validationManifest + validationManifest, strings.Repeat("x", MaxPageBytes+1)} {
		s := tlsServer(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(201); io.WriteString(w, body) })
		r := httpsReader(t, serverConfig(s))
		if err := r.ValidateJob(context.Background(), fixtureScope, []byte(validationManifest)); err == nil {
			t.Fatal("invalid response admitted")
		}
	}
	var calls atomic.Int32
	s := tlsServer(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1) })
	r := httpsReader(t, serverConfig(s))
	r.expires = time.Now().Add(-time.Second)
	checkQueryError(t, r.ValidateJob(context.Background(), fixtureScope, []byte(validationManifest)), Unauthenticated)
	if calls.Load() != 0 {
		t.Fatal("expired credentials used")
	}
}
