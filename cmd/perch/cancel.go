package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"perch/internal/cancellation"
	"perch/internal/hummingbird"
	"perch/internal/transport"
)

func runCancellation(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("perch cancel", flag.ContinueOnError)
	flags.SetOutput(out)
	host := flags.String("host", "hb.ucsc.edu", "SSH destination or config alias")
	flags.Bool("no-config", false, "ignore saved preferences")
	local := flags.Bool("local", false, "use local Slurm, where permitted")
	array := flags.Bool("array", false, "explicitly select every remaining task of this parent array")
	confirm := flags.Bool("confirm", false, "send one cancellation request after fresh scope checks")
	asJSON := flags.Bool("json", false, "print a structured preview or request receipt")
	timeout := flags.Duration("timeout", 20*time.Second, "deadline per preview or confirmation; requests are never retried")
	flags.Usage = func() {
		fmt.Fprint(out, `Usage: perch cancel JOB_ID [--host HOST] [--confirm]
       perch cancel ARRAY_TASK [--host HOST] [--confirm]
       perch cancel ARRAY_ID --array [--host HOST] [--confirm]

Preview cancellation of your own current job, one array task, or a whole array.
Nothing is cancelled without --confirm. A numeric array ID requires --array;
use ARRAY_TASK (for example 9100_4) to leave siblings running.

All steps and shells in scope will stop. Dependencies that require success may
remain blocked. Perch does not remove output files. A successful request does
not mean all processes have stopped; inspect jobs and history afterward.
SSH authentication must already work. There is no automatic retry on failure.

`)
		flags.PrintDefaults()
	}
	ordered, err := interspersed(flags, args)
	if err != nil {
		return err
	}
	if err := flags.Parse(ordered); err != nil {
		return err
	}
	if err := applyPreferences(flags); err != nil {
		return err
	}
	if len(flags.Args()) != 1 {
		return fmt.Errorf("provide one cancellation target; see perch cancel --help")
	}
	if *timeout <= 0 || *timeout > time.Minute {
		return fmt.Errorf("cancellation timeout must be greater than zero and at most 1m")
	}
	target := cancellation.Target{ID: flags.Args()[0], Array: *array}
	if err := cancellation.Validate(target); err != nil {
		return err
	}
	var runner transport.Runner
	destination := *host
	if *local {
		runner, destination = transport.Local{}, "local"
	} else {
		remote, err := transport.NewSSH(destination)
		if err != nil {
			return err
		}
		defer remote.Close()
		runner = remote
	}
	client := &hummingbird.Client{Runner: runner, Host: destination}
	parent, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(parent, *timeout)
	plan, err := cancellation.Prepare(ctx, client, target)
	cancel()
	if err != nil {
		return err
	}
	v := plan.View()
	if !*confirm && *asJSON {
		return writeCancellationJSON(out, struct {
			Mode    string               `json:"mode"`
			Preview cancellation.Preview `json:"preview"`
		}{"preview", v})
	}
	if !*asJSON {
		if err := writeTerminalText(out, fmt.Sprintf("Cancel %s · %s\n%s\n\n  Host       %s\n  Owner      %s\n  Cluster    %s\n  Scope      %s\n  Observed   %s\n\nAll steps and shells in scope will stop.\nDependencies requiring success may remain blocked.\nPerch does not remove output files.\n", v.Scope, v.Target.ID, v.Name, v.Host, v.User, v.Cluster, cancellationScope(v), strings.Join(v.States, ", "))); err != nil {
			return err
		}
	}
	if !*confirm {
		return writeTerminalText(out, "\nPreview only. Add --confirm to request cancellation after fresh checks.\n")
	}
	ctx, cancel = context.WithTimeout(parent, *timeout)
	receipt, requestErr := cancellation.Execute(ctx, client, plan)
	cancel()
	if receipt != nil {
		if *asJSON {
			err = writeCancellationJSON(out, receipt)
		} else {
			err = writeTerminalText(out, "\n"+receipt.Detail+"\n")
		}
		if err != nil {
			return fmt.Errorf("request outcome could not be written: %w; inspect queue and history before another request", err)
		}
	}
	return requestErr
}

func cancellationScope(v cancellation.Preview) string {
	switch v.Scope {
	case "array":
		return fmt.Sprintf("Every remaining array task (%d observed)", v.Tasks)
	case "task":
		return "One array task; siblings continue"
	default:
		return "One job allocation and all its steps"
	}
}

func writeCancellationJSON(out io.Writer, value any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
