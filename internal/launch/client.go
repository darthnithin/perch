package launch

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"perch/internal/domain"
	"perch/internal/transport"
)

type Runner interface {
	Run(context.Context, []string) (transport.Result, error)
	RunInput(context.Context, []string, string) (transport.Result, error)
}
type Client struct {
	Runner      Runner
	Journal     *Journal
	Destination string
}
type Check struct {
	At      time.Time `json:"at"`
	Message string    `json:"message"`
}
type Receipt struct {
	At      time.Time `json:"at"`
	JobID   string    `json:"job_id"`
	Cluster string    `json:"cluster,omitempty"`
	Token   string    `json:"submission_token"`
}

// Remove SBATCH_* settings inherited from login startup, so reviewed resource
// arguments cannot be supplemented by hidden array/dependency/hold settings.
const sbatchShell = `umask 077; for key in ${!SBATCH_@}; do unset "$key" || exit 70; done; exec sbatch "$@"`

func batchArgs(p Plan, mode string) []string {
	return append([]string{"bash", "-lc", sbatchShell, "perch-sbatch", mode}, p.Arguments...)
}
func (c Client) Check(ctx context.Context, p Plan) (Check, error) {
	// Rebuild from the spec; callers cannot smuggle arbitrary sbatch flags/script.
	p, e := Build(p.Spec)
	if e != nil {
		return Check{}, e
	}
	r, e := c.Runner.Run(ctx, []string{"bash", "-c", `test -d "$1" && test -x "$1" && test -w "$1"`, "perch-workdir", p.Spec.WorkDir})
	if e != nil || r.ExitCode != 0 {
		return Check{}, fmt.Errorf("cluster working directory is unavailable or not writable: %s", p.Spec.WorkDir)
	}
	r, e = c.Runner.RunInput(ctx, batchArgs(p, "--test-only"), p.Script)
	if e != nil || r.ExitCode != 0 {
		return Check{}, fmt.Errorf("scheduler validation failed: %s", resultError(r, e))
	}
	return Check{At: time.Now(), Message: strings.TrimSpace(domain.Clean(r.Stdout + "\n" + r.Stderr))}, nil
}
func resultError(r transport.Result, e error) string {
	msg := strings.TrimSpace(domain.Label(r.Stderr))
	if len(msg) > 400 {
		msg = msg[:400] + "…"
	}
	if msg != "" {
		return msg
	}
	if e != nil {
		return e.Error()
	}
	return fmt.Sprintf("exit %d", r.ExitCode)
}

var jobResponse = regexp.MustCompile(`^([1-9][0-9]*)(?:;([A-Za-z0-9_.-]+))?$`)

// Submit is an explicit one-shot mutation. It never retries: a lost response can
// mean Slurm accepted the job. The token is attached as a scheduler comment.
func (c Client) Submit(ctx context.Context, p Plan) (Receipt, error) {
	p, e := Build(p.Spec)
	if e != nil {
		return Receipt{}, e
	}
	var b [12]byte
	if _, e = rand.Read(b[:]); e != nil {
		return Receipt{}, e
	}
	receipt := Receipt{At: time.Now(), Token: "perch-" + hex.EncodeToString(b[:])}
	entry := Submission{Destination: c.Destination, Name: p.Spec.Name, Started: receipt.At, Receipt: receipt}
	if c.Journal != nil {
		if e = c.Journal.Save(ctx, entry); e != nil {
			return Receipt{}, fmt.Errorf("not submitted: cannot save recovery record: %w", e)
		}
	}
	args := append(batchArgs(p, "--parsable"), "--comment="+receipt.Token)
	r, e := c.Runner.RunInput(ctx, args, p.Script)
	if e != nil || r.ExitCode != 0 {
		return receipt, fmt.Errorf("submission not confirmed (%s). Check your queue/accounting before retrying; scheduler comment: %s", resultError(r, e), receipt.Token)
	}
	match := jobResponse.FindStringSubmatch(strings.TrimSpace(r.Stdout))
	if match == nil {
		return receipt, fmt.Errorf("submission returned an unrecognized response; check your queue before retrying (comment %s)", receipt.Token)
	}
	receipt.JobID, receipt.Cluster = match[1], match[2]
	receipt.At = time.Now()
	if c.Journal != nil {
		entry.Receipt = receipt
		// Preserve an accepted reply even if the network deadline just expired.
		saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		if e = c.Journal.Save(saveCtx, entry); e != nil {
			return receipt, fmt.Errorf("job %s was accepted but its local receipt could not be updated: %w; do not resubmit; comment %s", receipt.JobID, e, receipt.Token)
		}
	}
	return receipt, nil
}
