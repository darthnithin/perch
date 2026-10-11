package nautilus

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testBearer = "synthetic-secret-not-a-real-token"

func tlsServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	s := httptest.NewUnstartedServer(handler)
	s.Config.ErrorLog = log.New(io.Discard, "", 0)
	s.StartTLS()
	t.Cleanup(s.Close)
	return s
}
func serverConfig(s *httptest.Server) HTTPSConfig {
	return HTTPSConfig{Server: s.URL, CertificateAuthority: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: s.Certificate().Raw}), BearerToken: testBearer}
}
func httpsReader(t *testing.T, cfg HTTPSConfig) *HTTPSReader {
	t.Helper()
	r, err := NewHTTPSReader(fixtureScope, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	return r
}
func pageRequest() ListRequest { return ListRequest{Scope: fixtureScope, Resource: Jobs, Limit: 250} }
func checkQueryError(t *testing.T, err error, kind ErrorKind) {
	t.Helper()
	var qe *QueryError
	if !errors.As(err, &qe) || qe.Kind != kind {
		t.Fatalf("error=%v, want %s", err, kind)
	}
	data, _ := json.Marshal(err)
	for _, value := range []string{err.Error(), fmt.Sprintf("%+v %#v", err, err), string(data)} {
		for _, secret := range []string{testBearer, "server-secret", "proxy-secret"} {
			if strings.Contains(value, secret) {
				t.Fatal("error exposed confidential data")
			}
		}
	}
}

func TestHTTPSListsUseBoundScopeAuthAndOpaquePagination(t *testing.T) {
	token := "opaque/+=&namespace=other?token=server-secret"
	first := fixturePage(t, Jobs, 0, 3, "jobs-version", token)
	last := fixturePage(t, Jobs, 3, 8, "jobs-version", "")
	pods := fixture(t, "pods")
	var calls atomic.Int32
	s := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer "+testBearer || r.Header.Get("Accept") != "application/json" || r.Header.Get("Accept-Encoding") != "identity" {
			t.Error("incorrect read-only request headers")
		}
		q := r.URL.Query()
		if q.Get("limit") != "250" {
			t.Error("lost page limit")
		}
		switch r.URL.Path {
		case "/gateway/apis/batch/v1/namespaces/research/jobs":
			switch q.Get("continue") {
			case "":
				if len(q) != 1 {
					t.Error("extra query fields")
				}
				w.Write(first)
			case token:
				if len(q) != 2 {
					t.Error("continuation changed query")
				}
				w.Write(last)
			default:
				t.Error("altered continuation")
				w.WriteHeader(400)
			}
		case "/gateway/api/v1/namespaces/research/resourcequotas":
			w.Write(fixture(t, "resourcequotas"))
		case "/gateway/api/v1/namespaces/research/pods":
			w.Write(pods)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	})
	cfg := serverConfig(s)
	cfg.Server += "/gateway/"
	reader := httpsReader(t, cfg)
	c := clientFor(t, reader)
	got := c.Refresh(context.Background())
	if !got.Jobs.OK() || !got.Pods.OK() || !got.Quotas.OK() || len(got.Jobs.Collection.Items) != 8 || calls.Load() != 4 {
		t.Fatalf("incomplete HTTP refresh: %+v", got)
	}
	// Both cluster and namespace are frozen; valid but different scopes do not
	// cause a network request or silently borrow this session's credentials.
	for _, scope := range []Scope{{Cluster: "another", Namespace: "research"}, {Cluster: fixtureScope.Cluster, Namespace: "other"}} {
		req := pageRequest()
		req.Scope = scope
		_, err := reader.ReadPage(context.Background(), req)
		checkQueryError(t, err, Invalid)
	}
	if calls.Load() != 4 {
		t.Fatal("scope mismatch contacted API")
	}
}

func TestHTTPSRejectsInvalidConfigurationWithoutLeakingValues(t *testing.T) {
	s := tlsServer(t, func(w http.ResponseWriter, r *http.Request) { t.Error("invalid configuration contacted server") })
	changes := []func(*HTTPSConfig){
		func(c *HTTPSConfig) { c.Server = "http://localhost" },
		func(c *HTTPSConfig) { c.Server = "https://u:proxy-secret@localhost" },
		func(c *HTTPSConfig) { c.Server += "?token=server-secret" },
		func(c *HTTPSConfig) { c.Server += "#server-secret" },
		func(c *HTTPSConfig) { c.Server += "/a/../b" },
		func(c *HTTPSConfig) { c.Server += "/a//b" },
		func(c *HTTPSConfig) { c.Server += "//" },
		func(c *HTTPSConfig) { c.Server += "/%00" },
		func(c *HTTPSConfig) { c.Server += "/%2e%2e/b" },
		func(c *HTTPSConfig) { c.Server = "https://localhost:0" },
		func(c *HTTPSConfig) { c.Server = "https://localhost:99999" },
		func(c *HTTPSConfig) { c.Server = "https:///api" },
		func(c *HTTPSConfig) { c.BearerToken = "" },
		func(c *HTTPSConfig) { c.BearerToken = "a\r\nAuthorization: server-secret" },
		func(c *HTTPSConfig) { c.BearerToken = "has space" },
		func(c *HTTPSConfig) { c.BearerToken = strings.Repeat("a", maxBearerBytes+1) },
		func(c *HTTPSConfig) { c.CertificateAuthority = []byte("server-secret") },
		func(c *HTTPSConfig) { c.CertificateAuthority = make([]byte, maxTLSMaterialBytes+1) },
		func(c *HTTPSConfig) { c.ClientKey = []byte("server-secret") },
		func(c *HTTPSConfig) { c.BearerToken = ""; c.ClientCertificate = []byte("server-secret") },
	}
	for i, change := range changes {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			cfg := serverConfig(s)
			change(&cfg)
			r, err := NewHTTPSReader(fixtureScope, cfg)
			if r != nil {
				r.Close()
				t.Fatal("invalid config accepted")
			}
			checkQueryError(t, err, Invalid)
		})
	}
	cfg := serverConfig(s)
	cfg.ExpiresAt = time.Now().Add(-time.Second)
	_, err := NewHTTPSReader(fixtureScope, cfg)
	checkQueryError(t, err, Unauthenticated)
}

func TestHTTPSEnforcesServerTrustAndName(t *testing.T) {
	var calls atomic.Int32
	s := tlsServer(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1) })
	for _, name := range []string{"untrusted", "wrong-name"} {
		t.Run(name, func(t *testing.T) {
			cfg := serverConfig(s)
			if name == "untrusted" {
				cfg.CertificateAuthority = nil
			} else {
				cfg.Server = strings.Replace(cfg.Server, "127.0.0.1", "localhost", 1)
			}
			r := httpsReader(t, cfg)
			_, err := r.ReadPage(context.Background(), pageRequest())
			checkQueryError(t, err, TLSFailed)
			result := clientFor(t, r).ListJobs(context.Background())
			if result.Collection != nil || result.Error == nil || result.Error.Kind != TLSFailed {
				t.Fatal("list lost TLS failure")
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatal("credentials sent before certificate verification")
	}
}

func TestHTTPSRedirectsNeverForwardCredentials(t *testing.T) {
	var sinkCalls atomic.Int32
	sink := tlsServer(t, func(w http.ResponseWriter, r *http.Request) { sinkCalls.Add(1) })
	for _, code := range []int{301, 302, 303, 307, 308} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			var originCalls atomic.Int32
			origin := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
				originCalls.Add(1)
				http.Redirect(w, r, sink.URL+"/capture?server-secret", code)
			})
			r := httpsReader(t, serverConfig(origin))
			data, err := r.ReadPage(context.Background(), pageRequest())
			checkQueryError(t, err, InvalidResponse)
			if len(data) != 0 || originCalls.Load() != 1 || sinkCalls.Load() != 0 {
				t.Fatal("redirect followed or body exposed")
			}
		})
	}
}

func TestHTTPSStatusAndPartialRefresh(t *testing.T) {
	for code, kind := range map[int]ErrorKind{401: Unauthenticated, 403: Forbidden, 410: Expired, 429: Unavailable, 500: Unavailable, 502: Unavailable, 503: Unavailable, 504: Unavailable, 404: NotFound, 204: ReadFailed} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			s := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
				io.WriteString(w, "server-secret "+testBearer)
			})
			r := httpsReader(t, serverConfig(s))
			data, err := r.ReadPage(context.Background(), pageRequest())
			checkQueryError(t, err, kind)
			if len(data) != 0 {
				t.Fatal("error body returned")
			}
		})
	}
	jobs := fixture(t, "jobs")
	s := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/pods") {
			w.WriteHeader(403)
			io.WriteString(w, "server-secret")
			return
		}
		w.Write(jobs)
	})
	got := clientFor(t, httpsReader(t, serverConfig(s))).Refresh(context.Background())
	if !got.Jobs.OK() || got.Pods.Collection != nil || got.Pods.Error == nil || got.Pods.Error.Kind != Forbidden {
		t.Fatal("partial success erased or invented data")
	}
}

func TestHTTPSBoundsPageBytesAndRejectsUnexpectedEncoding(t *testing.T) {
	for _, mode := range []string{"length", "chunked", "encoding", "exact-limit"} {
		t.Run(mode, func(t *testing.T) {
			s := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "length":
					w.Header().Set("Content-Length", fmt.Sprint(MaxPageBytes+1))
					w.WriteHeader(200)
				case "chunked":
					w.(http.Flusher).Flush()
					w.Write(bytes.Repeat([]byte("x"), MaxPageBytes+1))
				case "exact-limit":
					w.Write(bytes.Repeat([]byte("x"), MaxPageBytes))
				case "encoding":
					w.Header().Set("Content-Encoding", "gzip")
					io.WriteString(w, "server-secret")
				}
			})
			r := httpsReader(t, serverConfig(s))
			data, err := r.ReadPage(context.Background(), pageRequest())
			if mode == "exact-limit" {
				if err != nil || len(data) != MaxPageBytes {
					t.Fatal("exact bound rejected")
				}
				return
			}
			kind := LimitExceeded
			if mode == "encoding" {
				kind = InvalidResponse
			}
			checkQueryError(t, err, kind)
			if len(data) != 0 {
				t.Fatal("partial response returned")
			}
		})
	}
}

func TestHTTPSCancelTimeoutAndCloseReleaseRequests(t *testing.T) {
	for _, mode := range []string{"cancel", "timeout", "close", "body-cancel"} {
		t.Run(mode, func(t *testing.T) {
			entered := make(chan struct{})
			stopped := make(chan struct{})
			s := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
				if mode == "body-cancel" {
					w.(http.Flusher).Flush()
				}
				close(entered)
				<-r.Context().Done()
				close(stopped)
			})
			r := httpsReader(t, serverConfig(s))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "timeout" {
				r.timeout = 150 * time.Millisecond
			}
			result := make(chan error, 1)
			go func() { _, err := r.ReadPage(ctx, pageRequest()); result <- err }()
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("request did not arrive")
			}
			want := Cancelled
			switch mode {
			case "cancel", "body-cancel":
				cancel()
			case "close":
				r.Close()
			case "timeout":
				want = TimedOut
			}
			select {
			case err := <-result:
				checkQueryError(t, err, want)
			case <-time.After(3 * time.Second):
				t.Fatal("request did not stop")
			}
			select {
			case <-stopped:
			case <-time.After(3 * time.Second):
				t.Fatal("server request remained active")
			}
			if mode == "close" {
				_, err := r.ReadPage(context.Background(), pageRequest())
				checkQueryError(t, err, Cancelled)
			}
		})
	}
}

func TestHTTPSCredentialExpiryAndFormatting(t *testing.T) {
	var calls atomic.Int32
	s := tlsServer(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1) })
	cfg := serverConfig(s)
	r := httpsReader(t, cfg)
	for _, value := range []any{cfg, &cfg, r} {
		encoded, _ := json.Marshal(value)
		text := fmt.Sprintf("%v %+v %#v %s", value, value, value, encoded)
		if strings.Contains(text, testBearer) || strings.Contains(text, s.URL) || strings.Contains(text, "CERTIFICATE") {
			t.Fatal("configuration formatting exposed connection material")
		}
	}
	r.expires = time.Now().Add(-time.Second)
	_, err := r.ReadPage(context.Background(), pageRequest())
	checkQueryError(t, err, Unauthenticated)
	if calls.Load() != 0 {
		t.Fatal("expired credential sent to server")
	}
}

func TestHTTPSClientCertificateAuthentication(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	root := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test client CA"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	rootDER, err := x509.CreateCertificate(rand.Reader, root, root, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	root, err = x509.ParseCertificate(rootDER)
	if err != nil {
		t.Fatal(err)
	}
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "perch-test"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	leafDER, err := x509.CreateCertificate(rand.Reader, leaf, root, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(root)
	var calls atomic.Int32
	s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "" || len(r.TLS.VerifiedChains) == 0 || r.TLS.PeerCertificates[0].Subject.CommonName != "perch-test" {
			t.Error("client certificate not verified")
		}
		io.WriteString(w, `{"kind":"JobList","apiVersion":"batch/v1","metadata":{"resourceVersion":"1"},"items":[]}`)
	}))
	s.Config.ErrorLog = log.New(io.Discard, "", 0)
	s.TLS = &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots, MinVersion: tls.VersionTLS12}
	s.StartTLS()
	defer s.Close()
	cfg := serverConfig(s)
	cfg.BearerToken = ""
	cfg.ClientCertificate = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	cfg.ClientKey = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	r := httpsReader(t, cfg)
	got := clientFor(t, r).ListJobs(context.Background())
	if !got.OK() || len(got.Collection.Items) != 0 || calls.Load() != 1 {
		t.Fatal("mTLS list failed", got.Error)
	}
	if !r.expires.Equal(leaf.NotAfter.Truncate(time.Second)) {
		t.Fatal("client certificate expiration not retained")
	}
}

func TestHTTPSDiscardsTruncatedBodyAndOversizeHeaders(t *testing.T) {
	for _, mode := range []string{"truncated", "headers"} {
		t.Run(mode, func(t *testing.T) {
			s := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
				if mode == "headers" {
					w.Header().Set("X-Details", strings.Repeat("server-secret", 6000))
				} else {
					w.Header().Set("Content-Length", "9999")
				}
				io.WriteString(w, "server-secret")
			})
			r := httpsReader(t, serverConfig(s))
			data, err := r.ReadPage(context.Background(), pageRequest())
			checkQueryError(t, err, Unavailable)
			if len(data) != 0 {
				t.Fatal("partial/error response escaped")
			}
		})
	}
}

func TestHTTPSCloseCancelsBothRefreshResources(t *testing.T) {
	entered := make(chan struct{}, 2)
	s := tlsServer(t, func(w http.ResponseWriter, r *http.Request) { entered <- struct{}{}; <-r.Context().Done() })
	reader := httpsReader(t, serverConfig(s))
	client := clientFor(t, reader)
	done := make(chan Refresh, 1)
	go func() { done <- client.Refresh(context.Background()) }()
	for range 2 {
		select {
		case <-entered:
		case <-time.After(3 * time.Second):
			t.Fatal("concurrent query did not start")
		}
	}
	reader.Close()
	select {
	case r := <-done:
		if r.Jobs.Collection != nil || r.Pods.Collection != nil || r.Jobs.Error == nil || r.Pods.Error == nil || r.Jobs.Error.Kind != Cancelled || r.Pods.Error.Kind != Cancelled {
			t.Fatal("close left a success or unclassified query")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("closing session left refresh active")
	}
}
