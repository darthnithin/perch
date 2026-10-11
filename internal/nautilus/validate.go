package nautilus

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"
)

const MaxJobManifestBytes = 256 << 10

// JobValidation is an admission observation, never a submission receipt.
// Jobs are admitted separately from the Pods their controller creates later.
type JobValidation struct {
	Scope
	Name           string        `json:"name"`
	ObservedAt     time.Time     `json:"observed_at"`
	ManifestSHA256 string        `json:"manifest_sha256"`
	Admitted       bool          `json:"job_dry_run_admitted"`
	Status         string        `json:"status"`
	NextStep       string        `json:"next_step"`
	Error          *QueryError   `json:"error,omitempty"`
	Quotas         Result[Quota] `json:"quota_snapshot"`
	Notes          []string      `json:"notes"`
}

type jobIdentity struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name         string `json:"name"`
		Namespace    string `json:"namespace"`
		GenerateName string `json:"generateName"`
	} `json:"metadata"`
}

// InspectJobManifest checks the local request boundary without reading config or
// contacting a server. The exact JSON is forwarded for strict server validation;
// unknown workload fields are never silently stripped by a typed round trip.
func InspectJobManifest(data []byte, namespace string) (string, error) {
	var job jobIdentity
	if len(data) == 0 || len(data) > MaxJobManifestBytes || !namespacePattern.MatchString(namespace) || validateJSON(data) != nil || json.Unmarshal(data, &job) != nil || job.APIVersion != "batch/v1" || job.Kind != "Job" || job.Metadata.Namespace != namespace || job.Metadata.GenerateName != "" || !validObjectName(job.Metadata.Name) {
		return "", queryError(Invalid, "Provide one named batch/v1 Job JSON (at most 256 KiB) with the explicit destination namespace")
	}
	return job.Metadata.Name, nil
}

// ValidateJob makes exactly one dry-run create request. The route always carries
// dryRun=All and strict validation. No fallback to real creation is possible.
func (r *HTTPSReader) ValidateJob(ctx context.Context, scope Scope, manifest []byte) error {
	if scope != r.scope {
		return queryError(Invalid, "Job namespace does not match this connection")
	}
	name, err := InspectJobManifest(manifest, scope.Namespace)
	if err != nil {
		return err
	}
	route := "/apis/batch/v1/namespaces/" + scope.Namespace + "/jobs?dryRun=All&fieldValidation=Strict"
	data, err := r.requestBounded(ctx, http.MethodPost, route, "application/json", manifest, MaxPageBytes)
	if err != nil {
		return err
	}
	var got jobIdentity
	if validateJSON(data) != nil || json.Unmarshal(data, &got) != nil || got.APIVersion != "batch/v1" || got.Kind != "Job" || got.Metadata.Namespace != scope.Namespace || got.Metadata.Name != name {
		return queryError(InvalidResponse, "Job dry-run returned an unexpected response")
	}
	return nil
}

type jobValidator interface {
	ValidateJob(context.Context, Scope, []byte) error
}

func (c *Client) ValidateJob(ctx context.Context, manifest []byte) (JobValidation, error) {
	name, err := InspectJobManifest(manifest, c.scope.Namespace)
	if err != nil {
		return JobValidation{}, err
	}
	validator, ok := c.reader.(jobValidator)
	if !ok {
		return JobValidation{}, queryError(Unavailable, "This connection does not support server Job validation")
	}
	digest := sha256.Sum256(manifest)
	out := JobValidation{Scope: c.scope, Name: name, ManifestSHA256: hex.EncodeToString(digest[:]), Notes: []string{
		"Server dry-run only: no Job or Pod was created and no workload ran.",
		"Job admission checks your create permission and Job policies. Its Pods are admitted separately later.",
		"Quota is an independent snapshot, not reserved capacity or proof that the Pods will be admitted.",
		"Image pulls, PVC mounting, scheduling and workload completion have not been tested.",
	}}
	err = validator.ValidateJob(ctx, c.scope, manifest)
	if err != nil {
		out.Error = readError(err)
	} else {
		out.Admitted = true
	}
	out.Status, out.NextStep = jobValidationOutcome(out.Admitted, out.Error)
	if ctx.Err() == nil {
		out.Quotas = c.ListQuotas(ctx)
	} else {
		out.Quotas.Error = readError(ctx.Err())
	}
	out.ObservedAt = time.Now()
	return out, nil
}

// Only an explicit API denial is a rejection. A missing, malformed or delayed
// response cannot establish whether admission accepted this dry-run.
func jobValidationOutcome(admitted bool, failure *QueryError) (string, string) {
	if admitted {
		return "accepted", "Review the quota snapshot and Pod requirements before submitting with your cluster tooling. Perch has not submitted this Job."
	}
	if failure != nil {
		switch failure.Kind {
		case JobRejected:
			return "rejected", "Review the Job fields and namespace admission policies, correct the manifest, then validate it again."
		case JobConflict:
			return "rejected", "Choose a new metadata.name for this Job, then validate the updated manifest."
		case Forbidden:
			return "rejected", "Check that the selected context and namespace are correct and that your account can create Jobs there."
		case Unauthenticated:
			return "unverified", "Repeat this validation in your terminal with --login to explicitly sign in."
		case TLSFailed:
			return "unverified", "Check the server address and certificate authority in the selected kubeconfig context."
		case Cancelled:
			return "unverified", "Run validation again when ready. The cancelled check did not establish an admission result."
		case NotFound:
			return "unverified", "Check the selected context, namespace and availability of the batch/v1 Jobs API."
		}
	}
	return "unverified", "Check the connection and retry this dry-run. No automatic retry or workload submission occurred."
}
