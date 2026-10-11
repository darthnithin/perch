package nautilus

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxTLSMaterialBytes = 1 << 20
	maxBearerBytes      = 32 << 10
	pageReadTimeout     = 10 * time.Second
)

// HTTPSConfig is an in-memory credential snapshot from an explicit connection
// flow. It never loads files, runs an exec plugin or renews credentials. An
// expired session requires reconnecting with a new reader. Do not persist it.
// A supplied CA replaces system roots; absent CA uses the system trust store.
// Certificate and key are PEM. Bearer and client-certificate auth are mutually
// exclusive. Credentials and endpoint are excluded from JSON and formatting.
type HTTPSConfig struct {
	Server               string    `json:"-"`
	CertificateAuthority []byte    `json:"-"`
	ClientCertificate    []byte    `json:"-"`
	ClientKey            []byte    `json:"-"`
	BearerToken          string    `json:"-"`
	ExpiresAt            time.Time `json:"-"`
}

func (HTTPSConfig) String() string     { return "nautilus.HTTPSConfig{redacted}" }
func (c HTTPSConfig) GoString() string { return c.String() }

// HTTPSReader binds both cluster and namespace to a verified TLS endpoint. It
// implements PageReader without a subprocess or credential-refresh callback.
// Concurrent reads are supported. Close cancels in-flight requests and releases
// idle sockets; subsequent reads fail without contacting the server.
type HTTPSReader struct {
	scope     Scope
	base      url.URL
	client    *http.Client
	transport *http.Transport
	bearer    string
	expires   time.Time
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
	timeout   time.Duration
}

func (*HTTPSReader) String() string     { return "nautilus.HTTPSReader{redacted}" }
func (r *HTTPSReader) GoString() string { return r.String() }

func NewHTTPSReader(scope Scope, cfg HTTPSConfig) (*HTTPSReader, error) {
	invalid := func() (*HTTPSReader, error) {
		return nil, queryError(Invalid, "Invalid namespace HTTPS connection configuration")
	}
	if !validScope(scope) || len(cfg.Server) > 4096 || len(cfg.CertificateAuthority) > maxTLSMaterialBytes || len(cfg.ClientCertificate) > maxTLSMaterialBytes || len(cfg.ClientKey) > maxTLSMaterialBytes || len(cfg.BearerToken) > maxBearerBytes {
		return invalid()
	}
	base, err := url.Parse(cfg.Server)
	if err != nil || base.Scheme != "https" || base.Opaque != "" || base.Hostname() == "" || base.User != nil || base.RawQuery != "" || base.ForceQuery || base.Fragment != "" || base.RawFragment != "" || base.RawPath != "" {
		return invalid()
	}
	if port := base.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return invalid()
		}
	}
	// Preserve an optional reverse-proxy path prefix, never a query/fragment or
	// dot-segment traversal. Every resource path comes from a typed request.
	if strings.Contains(base.Path, "//") || strings.IndexFunc(base.Path, func(r rune) bool { return r < 32 || r == 127 }) >= 0 {
		return invalid()
	}
	base.Path = strings.TrimSuffix(base.Path, "/")
	if base.Path != "" && path.Clean(base.Path) != base.Path {
		return invalid()
	}
	hasCert := len(cfg.ClientCertificate) > 0 || len(cfg.ClientKey) > 0
	if hasCert == (cfg.BearerToken != "") {
		return invalid()
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if len(cfg.CertificateAuthority) > 0 {
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(cfg.CertificateAuthority) {
			return invalid()
		}
		tlsConfig.RootCAs = roots
	}
	expires := cfg.ExpiresAt
	if hasCert {
		cert, err := tls.X509KeyPair(cfg.ClientCertificate, cfg.ClientKey)
		if err != nil {
			return invalid()
		}
		leaf, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return invalid()
		}
		if time.Now().Before(leaf.NotBefore) {
			return nil, readError(queryError(Unauthenticated, ""))
		}
		if expires.IsZero() || leaf.NotAfter.Before(expires) {
			expires = leaf.NotAfter
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	} else {
		// Tokens are opaque printable ASCII without spaces. Reject header injection
		// locally; never depend on an HTTP error that could include credential data.
		for _, ch := range cfg.BearerToken {
			if ch < 33 || ch > 126 {
				return invalid()
			}
		}
	}
	if !expires.IsZero() && !time.Now().Before(expires) {
		return nil, readError(queryError(Unauthenticated, ""))
	}
	// A dedicated transport avoids DefaultTransport hooks, implicit credential
	// plugins and cookies. Normal HTTPS proxy configuration is respected.
	transport := &http.Transport{
		Proxy:                  http.ProxyFromEnvironment,
		DialContext:            (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSClientConfig:        tlsConfig,
		TLSHandshakeTimeout:    8 * time.Second,
		ResponseHeaderTimeout:  8 * time.Second,
		MaxResponseHeaderBytes: 64 << 10,
		MaxIdleConns:           2, MaxIdleConnsPerHost: 2, MaxConnsPerHost: 2,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
	}
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	ctx, cancel := context.WithCancel(context.Background())
	return &HTTPSReader{scope: scope, base: *base, client: client, transport: transport, bearer: cfg.BearerToken, expires: expires, ctx: ctx, cancel: cancel, timeout: pageReadTimeout}, nil
}

func (r *HTTPSReader) Close() {
	r.closeOnce.Do(func() { r.cancel(); r.transport.CloseIdleConnections() })
}

func (r *HTTPSReader) ReadPage(parent context.Context, request ListRequest) ([]byte, error) {
	if request.Scope != r.scope {
		return nil, queryError(Invalid, "Namespace request does not match this connection")
	}
	route, err := request.Path()
	if err != nil {
		return nil, err
	}
	return r.readBounded(parent, route, "application/json", MaxPageBytes)
}

// Routes are constructed by the typed request methods, never supplied by users.
func (r *HTTPSReader) readBounded(parent context.Context, route, accept string, byteLimit int64) ([]byte, error) {
	return r.requestBounded(parent, http.MethodGet, route, accept, nil, byteLimit)
}

// Only typed operations call this helper. Non-GET callers must provide their
// fixed non-persisting endpoint; it is never a user-supplied URL or method.
func (r *HTTPSReader) requestBounded(parent context.Context, method, route, accept string, body []byte, byteLimit int64) ([]byte, error) {
	if err := r.ctx.Err(); err != nil {
		return nil, readError(err)
	}
	if !r.expires.IsZero() && !time.Now().Before(r.expires) {
		return nil, readError(queryError(Unauthenticated, ""))
	}
	ctx, cancel := context.WithTimeout(parent, r.timeout)
	stop := context.AfterFunc(r.ctx, cancel)
	defer func() { stop(); cancel() }()
	if err := ctx.Err(); err != nil {
		return nil, readError(err)
	}
	endpoint := r.base
	resourcePath, query, _ := strings.Cut(route, "?")
	endpoint.Path += resourcePath
	endpoint.RawQuery = query
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, queryError(Invalid, "Invalid namespace HTTPS request")
	}
	req.Header.Set("Accept", accept)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("User-Agent", "perch-nautilus")
	if r.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+r.bearer)
	}
	response, err := r.client.Do(req)
	if err != nil {
		return nil, httpsReadError(ctx, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK && !(method == http.MethodPost && response.StatusCode == http.StatusCreated) {
		if method == http.MethodPost {
			switch response.StatusCode {
			case http.StatusConflict:
				return nil, readError(queryError(JobConflict, ""))
			case http.StatusBadRequest, http.StatusUnprocessableEntity:
				return nil, readError(queryError(JobRejected, ""))
			}
		}
		return nil, httpStatusError(response.StatusCode)
	}
	if encoding := response.Header.Get("Content-Encoding"); encoding != "" && encoding != "identity" {
		return nil, queryError(InvalidResponse, "Unsupported namespace response encoding")
	}
	if response.ContentLength > byteLimit {
		return nil, readError(queryError(LimitExceeded, ""))
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, byteLimit+1))
	if err != nil {
		return nil, httpsReadError(ctx, err)
	}
	if err := ctx.Err(); err != nil {
		return nil, readError(err)
	}
	if int64(len(data)) > byteLimit {
		return nil, readError(queryError(LimitExceeded, ""))
	}
	return data, nil
}

func httpsReadError(ctx context.Context, err error) *QueryError {
	if ctx.Err() != nil {
		return readError(ctx.Err())
	}
	var verification *tls.CertificateVerificationError
	if errors.As(err, &verification) {
		return queryError(TLSFailed, "Namespace TLS certificate verification failed")
	}
	var network net.Error
	if errors.As(err, &network) && network.Timeout() {
		return readError(context.DeadlineExceeded)
	}
	// Never attach a URL error: it can contain server-controlled locations or
	// proxy credentials. The classification is sufficient for reconnect UX.
	return readError(queryError(Unavailable, ""))
}
