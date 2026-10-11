package nautilus

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// PageReader is the connection boundary. An implementation must honor context
// cancellation, bound output to MaxPageBytes, and never start interactive login
// during a read. Authentication belongs in an explicit connection flow.
// ReadPage can be called concurrently for Jobs, Pods and ResourceQuotas.
type PageReader interface {
	ReadPage(context.Context, ListRequest) ([]byte, error)
}

type Resource string

const (
	Jobs   Resource = "jobs"
	Pods   Resource = "pods"
	Quotas Resource = "resourcequotas"
)

type ListRequest struct {
	Scope
	Resource Resource
	Limit    int
	Continue string
}

var namespacePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// Path constructs only the namespaced read endpoints used by this adapter.
// A future HTTP or kubectl transport must bind Scope.Cluster to its connection;
// cluster identity is deliberately not encoded as a server-controlled URL.
func (r ListRequest) Path() (string, error) {
	if !validScope(r.Scope) || r.Limit < 1 || r.Limit > 1000 || len(r.Continue) > maxContinueBytes {
		return "", queryError(Invalid, "Invalid namespace list request")
	}
	prefix := "/api/v1"
	switch r.Resource {
	case Jobs:
		prefix = "/apis/batch/v1"
	case Pods, Quotas:
	default:
		return "", queryError(Invalid, "Unsupported namespace resource")
	}
	params := url.Values{"limit": {strconv.Itoa(r.Limit)}}
	if r.Continue != "" {
		params.Set("continue", r.Continue)
	}
	return prefix + "/namespaces/" + r.Namespace + "/" + string(r.Resource) + "?" + params.Encode(), nil
}

func validScope(scope Scope) bool {
	return safeIdentity(scope.Cluster) && namespacePattern.MatchString(scope.Namespace)
}

const (
	maxContinueBytes = 16 << 10
	maxListItems     = 20000
	maxListBytes     = 32 << 20
	maxListPages     = 100
	listPageSize     = 250
	listTimeout      = 20 * time.Second
)

type ErrorKind string

const (
	Invalid         ErrorKind = "invalid-request"
	Cancelled       ErrorKind = "cancelled"
	TimedOut        ErrorKind = "timeout"
	ReadFailed      ErrorKind = "read-failed"
	TLSFailed       ErrorKind = "tls-failed"
	InvalidResponse ErrorKind = "invalid-response"
	LimitExceeded   ErrorKind = "limit-exceeded"
	Expired         ErrorKind = "expired-list"
	Unauthenticated ErrorKind = "authentication-required"
	Forbidden       ErrorKind = "forbidden"
	Unavailable     ErrorKind = "unavailable"
	NotFound        ErrorKind = "not-found"
	JobConflict     ErrorKind = "job-name-conflict"
	JobRejected     ErrorKind = "job-admission-rejected"
)

// QueryError deliberately excludes arbitrary transport or server messages.
// Credential URLs, response bodies and plugin output must not enter diagnostics.
type QueryError struct {
	Kind    ErrorKind `json:"kind"`
	Message string    `json:"message"`
	cause   error
}

func (e *QueryError) Error() string { return e.Message }
func (e *QueryError) Unwrap() error { return e.cause }
func queryError(kind ErrorKind, message string) *QueryError {
	return &QueryError{Kind: kind, Message: message}
}

func readError(err error) *QueryError {
	if errors.Is(err, context.Canceled) {
		return &QueryError{Cancelled, "Namespace query cancelled", context.Canceled}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &QueryError{TimedOut, "Namespace query timed out", context.DeadlineExceeded}
	}
	var known *QueryError
	if errors.As(err, &known) {
		// Preserve classification, never a reader-supplied message or raw cause.
		switch known.Kind {
		case JobConflict:
			return queryError(JobConflict, "A Job with this name already exists; choose a new name")
		case JobRejected:
			return queryError(JobRejected, "Job dry-run was rejected by API validation or admission")
		case Unauthenticated:
			return queryError(Unauthenticated, "Sign in again to read this namespace")
		case Forbidden:
			return queryError(Forbidden, "Access to this namespace resource was denied")
		case NotFound:
			return queryError(NotFound, "Requested namespace resource is no longer available")
		case Expired:
			return queryError(Expired, "List snapshot expired; refresh to start a new list")
		case Unavailable:
			return queryError(Unavailable, "Namespace API is unavailable")
		case TLSFailed:
			return queryError(TLSFailed, "Namespace TLS certificate verification failed")
		case InvalidResponse:
			return queryError(InvalidResponse, "Invalid namespace API response")
		case Invalid:
			return queryError(Invalid, "Invalid namespace request")
		case LimitExceeded:
			return queryError(LimitExceeded, "Namespace response exceeded its byte limit")
		}
	}
	return queryError(ReadFailed, "Unable to read namespace resource")
}

type Collection[T any] struct {
	Items           []T       `json:"items"`
	ResourceVersion string    `json:"resource_version"`
	ObservedAt      time.Time `json:"observed_at"`
}

// Result is either a complete collection or an error, never a partial collection
// presented as success. Failed results must not replace previously shown data.
type Result[T any] struct {
	Collection *Collection[T] `json:"collection,omitempty"`
	Error      *QueryError    `json:"error,omitempty"`
}

func (r Result[T]) OK() bool { return r.Collection != nil && r.Error == nil }

type Refresh struct {
	Scope  Scope         `json:"scope"`
	Jobs   Result[Job]   `json:"jobs"`
	Pods   Result[Pod]   `json:"pods"`
	Quotas Result[Quota] `json:"quotas"`
}

type listLimits struct {
	items, bytes, pages, size int
	timeout                   time.Duration
}

type Client struct {
	scope  Scope
	reader PageReader
	limits listLimits
}

func NewClient(scope Scope, reader PageReader) (*Client, error) {
	if !validScope(scope) || reader == nil {
		return nil, queryError(Invalid, "Cluster, namespace and page reader are required")
	}
	return &Client{scope: scope, reader: reader, limits: listLimits{maxListItems, maxListBytes, maxListPages, listPageSize, listTimeout}}, nil
}

// Refresh reads resources independently. Jobs, Pods and quotas need not have the same
// resource version or observation time: this is not a cross-resource transaction.
func (c *Client) Refresh(ctx context.Context) Refresh {
	r := Refresh{Scope: c.scope}
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); r.Jobs = c.ListJobs(ctx) }()
	go func() { defer wg.Done(); r.Pods = c.ListPods(ctx) }()
	go func() { defer wg.Done(); r.Quotas = c.ListQuotas(ctx) }()
	wg.Wait()
	return r
}

func (c *Client) ListJobs(ctx context.Context) Result[Job] {
	return list(ctx, c, Jobs, ParseJobs, func(j Job) Ref { return j.Ref })
}
func (c *Client) ListPods(ctx context.Context) Result[Pod] {
	return list(ctx, c, Pods, ParsePods, func(p Pod) Ref { return p.Ref })
}

func list[T any](ctx context.Context, c *Client, resource Resource, parse func([]byte, Scope) (Page[T], error), ref func(T) Ref) Result[T] {
	ctx, cancel := context.WithTimeout(ctx, c.limits.timeout)
	defer cancel()
	fail := func(err *QueryError) Result[T] { return Result[T]{Error: err} }
	request := ListRequest{Scope: c.scope, Resource: resource, Limit: c.limits.size}
	collection := &Collection[T]{Items: make([]T, 0)}
	seenTokens := make(map[string]bool)
	seenIDs := make(map[Identity]bool)
	seenNames := make(map[string]bool)
	totalBytes := 0
	for pageNumber := 0; pageNumber < c.limits.pages; pageNumber++ {
		if err := ctx.Err(); err != nil {
			return fail(readError(err))
		}
		data, err := c.reader.ReadPage(ctx, request)
		if ctx.Err() != nil {
			return fail(readError(ctx.Err()))
		}
		if err != nil {
			return fail(readError(err))
		}
		if len(data) > MaxPageBytes || len(data) > c.limits.bytes-totalBytes {
			return fail(queryError(LimitExceeded, "Namespace list exceeded its byte limit"))
		}
		totalBytes += len(data)
		if statusErr := apiStatusError(data); statusErr != nil {
			return fail(statusErr)
		}
		page, err := parse(data, c.scope)
		if err != nil {
			return fail(queryError(InvalidResponse, "Invalid namespace list response"))
		}
		if ctx.Err() != nil {
			return fail(readError(ctx.Err()))
		}
		if len(page.Items) > c.limits.items-len(collection.Items) {
			return fail(queryError(LimitExceeded, "Namespace list exceeded its item limit"))
		}
		if pageNumber == 0 {
			collection.ResourceVersion = page.ResourceVersion
		} else if page.ResourceVersion != collection.ResourceVersion {
			return fail(queryError(InvalidResponse, "Namespace list changed version between pages"))
		}
		for _, value := range page.Items {
			r := ref(value)
			if seenIDs[r.Identity()] || seenNames[r.Name] {
				return fail(queryError(InvalidResponse, "Namespace list contains duplicate object identities or names"))
			}
			seenIDs[r.Identity()], seenNames[r.Name] = true, true
			collection.Items = append(collection.Items, value)
		}
		if page.Continue == "" {
			if page.Remaining != nil && *page.Remaining > 0 {
				return fail(queryError(InvalidResponse, "Namespace list ended before its reported remaining items"))
			}
			collection.ObservedAt = time.Now().UTC()
			return Result[T]{Collection: collection}
		}
		if len(page.Continue) > maxContinueBytes {
			return fail(queryError(LimitExceeded, "Namespace continuation token exceeded its limit"))
		}
		if seenTokens[page.Continue] {
			return fail(queryError(InvalidResponse, "Namespace list repeated a continuation token"))
		}
		seenTokens[page.Continue] = true
		request.Continue = page.Continue
	}
	return fail(queryError(LimitExceeded, fmt.Sprintf("Namespace list exceeded its %d-page limit", c.limits.pages)))
}
