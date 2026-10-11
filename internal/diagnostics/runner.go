package diagnostics

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"perch/internal/transport"
)

// Runner preserves the underlying execution result even if diagnostics fails.
// Capture failure must never make an accepted submission look safe to retry.
type Runner struct {
	Base  transport.Runner
	Store *Store
}

func (r *Runner) Run(ctx context.Context, args []string) (transport.Result, error) {
	result, err := r.Base.Run(ctx, args)
	r.capture(args, result, err, false)
	return result, err
}
func (r *Runner) Batch(ctx context.Context, commands [][]string) ([]transport.Result, error) {
	results, err := r.Base.Batch(ctx, commands)
	for i, result := range results {
		if i < len(commands) {
			r.capture(commands[i], result, nil, false)
		}
	}
	if err != nil {
		r.capture(nil, transport.Result{ExitCode: -1}, err, false)
	}
	return results, err
}
func (r *Runner) RunInput(ctx context.Context, args []string, input string) (transport.Result, error) {
	b, ok := r.Base.(interface {
		RunInput(context.Context, []string, string) (transport.Result, error)
	})
	if !ok {
		return transport.Result{}, fmt.Errorf("runner does not support submission input")
	}
	result, err := b.RunInput(ctx, args, input)
	r.capture(args, result, err, true) // Never capture stdin or submission output.
	return result, err
}
func (r *Runner) LoginCommand() *exec.Cmd {
	if b, ok := r.Base.(interface{ LoginCommand() *exec.Cmd }); ok {
		return b.LoginCommand()
	}
	return nil
}

// Foreground session contents bypass diagnostics, just like interactive login.
func (r *Runner) InteractiveCommand(args []string) (*exec.Cmd, error) {
	if b, ok := r.Base.(interface {
		InteractiveCommand([]string) (*exec.Cmd, error)
	}); ok {
		return b.InteractiveCommand(args)
	}
	return nil, fmt.Errorf("runner does not support foreground sessions")
}
func operation(args []string) string {
	if len(args) == 0 {
		return "batch"
	}
	switch args[0] {
	case "squeue", "sacct", "sinfo", "scontrol", "sstat", "sprio", "sshare", "sbatch", "id":
		return args[0]
	default:
		return "other" // No user text, paths, argv, shell scripts or error strings.
	}
}
func schedulerJSON(args []string) bool {
	if len(args) == 0 || (args[0] != "squeue" && args[0] != "sacct" && args[0] != "sinfo" && args[0] != "sstat") {
		return false
	}
	found := false
	for _, arg := range args[1:] {
		if arg == "--batch-script" || strings.HasPrefix(arg, "--batch-script=") || arg == "-B" {
			return false
		}
		if arg == "--json" || strings.HasPrefix(arg, "--json=") {
			found = true
		}
	}
	return found
}
func (r *Runner) capture(args []string, result transport.Result, err error, input bool) {
	e := Event{Operation: operation(args), DurationMS: float64(result.Duration) / 1e6, ExitCode: result.ExitCode, StdoutBytes: len(result.Stdout), StderrBytes: len(result.Stderr)}
	for _, arg := range args {
		parser, ok := strings.CutPrefix(arg, "--json=")
		if ok && len(parser) <= 24 && parser != "" && strings.Trim(parser, "v.0123456789") == "" {
			e.Parser = parser
		}
	}
	if err != nil {
		e.ErrorKind = "other"
		var classified *transport.Error
		if errors.As(err, &classified) {
			switch classified.Kind {
			case transport.Authentication, transport.HostKey, transport.Network, transport.Timeout, transport.CommandFailed, transport.OutputLimit, transport.Protocol:
				e.ErrorKind = string(classified.Kind)
			}
		}
	} else if result.ExitCode != 0 {
		e.ErrorKind = "command"
	}
	if r.Store.raw && !input && schedulerJSON(args) {
		e.RawCaptured = true
		n := min(len(result.Stdout), RawLimit)
		e.Stdout = []byte(result.Stdout[:n])
		m := min(len(result.Stderr), RawLimit-n)
		e.Stderr = []byte(result.Stderr[:m])
		e.RawTruncated = n < len(result.Stdout) || m < len(result.Stderr)
	}
	r.Store.record(e)
}
