package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/x/term"
	"perch/internal/allocshell"
	"perch/internal/hummingbird"
	"perch/internal/launch"
	"perch/internal/transport"
)

func runAllocationShell(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("perch shell", flag.ContinueOnError)
	flags.SetOutput(out)
	host := flags.String("host", "hb.ucsc.edu", "SSH destination or config alias")
	flags.Bool("no-config", false, "ignore saved preferences")
	local := flags.Bool("local", false, "use local Slurm, where permitted")
	node := flags.String("node", "", "one allocated node (default: first node)")
	cpus := flags.Int("cpus", 1, "CPUs shared with the existing workload")
	memory := flags.String("mem", "64M", "memory shared with the existing workload")
	duration := flags.String("time", "10m", "shell step limit, 1m–24h; cannot extend the parent job")
	open := flags.Bool("open", false, "explicitly open the interactive shell in this terminal")
	jsonOutput := flags.Bool("json", false, "print the read-only preview as JSON")
	timeout := flags.Duration("timeout", 20*time.Second, "deadline for each read-only preflight (not the shell duration)")
	flags.Usage = func() {
		fmt.Fprint(out, `Usage: perch shell JOB_ID [--host HOST] [--node NODE] [--open]

Preview a new shell step in one of your RUNNING allocations or array tasks.
Add --open to enter it. This shares resources with the workload; it does not
attach to the batch process. No GPUs are requested. The shell inherits the
login environment, not variables or modules set inside the batch script.
Use exit or Ctrl-D to leave. The parent allocation continues.
SSH authentication must already work; configure system SSH key/agent access.

`)
		flags.PrintDefaults()
	}
	ordered, err := interspersed(flags, args)
	if err != nil {
		return err
	}
	if err = flags.Parse(ordered); err != nil {
		return err
	}
	if err = applyPreferences(flags); err != nil {
		return err
	}
	if len(flags.Args()) != 1 {
		return fmt.Errorf("provide one job ID or array task; see perch shell --help")
	}
	if *timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if *open && *jsonOutput {
		return fmt.Errorf("--json is for preview; omit it when using --open")
	}
	if *open && (!term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd())) {
		return fmt.Errorf("--open requires an interactive terminal on stdin and stdout")
	}
	mem, err := launch.MemoryMiB(*memory)
	if err != nil {
		return err
	}
	minutes, err := launch.Minutes(*duration)
	if err != nil {
		return err
	}
	if minutes < 1 || minutes > 1440 {
		return fmt.Errorf("shell time limit must be between 1m and 24h")
	}
	options := allocshell.Options{Node: *node, CPUs: *cpus, MemoryMiB: mem, Minutes: int(minutes)}
	id := flags.Args()[0]
	if err = allocshell.Validate(id, options); err != nil {
		return err
	}
	var runner transport.Runner
	var interactive interface {
		InteractiveCommand([]string) (*exec.Cmd, error)
	}
	destination := *host
	if *local {
		runner, interactive, destination = transport.Local{}, transport.Local{}, "local"
	} else {
		remote, err := transport.NewSSH(*host)
		if err != nil {
			return err
		}
		defer remote.Close()
		runner, interactive = remote, remote
	}
	client := &hummingbird.Client{Runner: runner, Host: destination}
	preflight, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(preflight, *timeout)
	plan, err := allocshell.Prepare(ctx, client, id, options)
	cancel()
	if err != nil {
		return err
	}
	if *jsonOutput {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Mode string          `json:"mode"`
			Host string          `json:"host"`
			Plan allocshell.Plan `json:"plan"`
		}{"preview", destination, plan})
	}
	if err = writeShellPreview(out, plan); err != nil {
		return err
	}
	if !*open {
		return writeTerminalText(out, "\nAdd --open to enter. The allocation is checked again before entry.\n")
	}
	ctx, cancel = context.WithTimeout(preflight, *timeout)
	plan, err = allocshell.Revalidate(ctx, client, plan)
	cancel()
	if err != nil {
		return err
	}
	command, err := interactive.InteractiveCommand(plan.Command())
	if err != nil {
		return err
	}
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	if _, err = fmt.Fprint(out, "\nOpening shell · exit or Ctrl-D returns here.\n\n"); err != nil {
		return err
	}
	stop()
	if err = runForegroundShell(command, time.Duration(plan.Options.Minutes)*time.Minute+15*time.Second); err != nil {
		return fmt.Errorf("shell session ended: %w; the parent job was not cancelled by Perch", err)
	}
	_, err = fmt.Fprintln(out, "\nShell closed. Perch has not cancelled the parent job.")
	return err
}

func runForegroundShell(command *exec.Cmd, limit time.Duration) error {
	return transport.RunForeground(command, limit)
}

func writeShellPreview(out io.Writer, p allocshell.Plan) error {
	return writeTerminalText(out, fmt.Sprintf("Shell · %s\n%s\n\n  Node       %s\n  Directory  %s\n  CPUs       %d\n  Memory     %d MiB\n  GPUs       None requested\n  Time limit %dm (parent may end sooner)\n\nShares resources with this running job.\nUses your login environment. Batch-script variables and modules are not copied.\n", p.JobID, p.Name, p.Node, p.WorkDir, p.Options.CPUs, p.Options.MemoryMiB, p.Options.Minutes))
}
