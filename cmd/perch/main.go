package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/charmbracelet/x/term"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"os"
	"os/signal"
	"path/filepath"
	"perch/internal/cli"
	"perch/internal/diagnostics"
	"perch/internal/launch"
	"perch/internal/mcpserver"
	"perch/internal/notify"
	"perch/internal/preferences"
	"perch/internal/query"
	"perch/internal/snapshotcache"
	"perch/internal/timewarning"
	"runtime"
	"strings"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"perch/internal/hummingbird"
	"perch/internal/transport"
	"perch/internal/ui"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, "perch:", err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) > 0 && args[0] == "cancel" {
		return runCancellation(args[1:], os.Stdout)
	}
	if len(args) > 0 && args[0] == "shell" {
		return runAllocationShell(args[1:], os.Stdout)
	}
	if len(args) > 0 && args[0] == "nrp" {
		return runNRP(args[1:], os.Stdout)
	}
	if len(args) > 0 && args[0] == "config" {
		return runConfig(args[1:], os.Stdout)
	}
	if len(args) > 0 && args[0] == "run" {
		return runWorkload(args[1:], os.Stdout)
	}
	if len(args) > 0 && args[0] == "receipts" {
		return runReceipts(args[1:], os.Stdout, nil)
	}
	flags := flag.NewFlagSet("perch", flag.ContinueOnError)
	host := flags.String("host", "hb.ucsc.edu", "SSH destination or config alias (for example user@hb.ucsc.edu)")
	debug := flags.Bool("debug", false, "save private bounded operation metadata; see Connection for its path")
	debugRaw := flags.Bool("debug-raw", false, "with --debug, also save raw scheduler JSON (may contain private job metadata)")
	cacheView := flags.Bool("cache", false, "keep a private last-known monitoring view for up to 24 hours (TUI only)")
	demo := flags.Bool("demo", false, "explore Perch offline with synthetic workloads")
	local := flags.Bool("local", false, "run Slurm commands locally, where cluster policy permits")
	static := flags.Bool("reduced-motion", false, "use static Chip artwork")
	noColor := flags.Bool("no-color", os.Getenv("NO_COLOR") != "", "disable terminal colors and pixel artwork")
	warningSpec := flags.String("time-warnings", "5m", "TUI remaining-time warnings: comma-separated whole minutes, or off")
	notificationMode := flags.String("notify", "off", "TUI notifications: off or desktop (macOS, opt-in)")
	noMouse := flags.Bool("no-mouse", false, "disable mouse capture for native terminal text selection")
	flags.Bool("no-config", false, "ignore saved preferences for this invocation")
	asJSON := flags.Bool("json", false, "print structured JSON for CLI commands")
	colorMode := flags.String("color-mode", "auto", "terminal colors: auto, truecolor or 256")
	theme := flags.String("theme", "warm", "color palette: warm or graphite")
	stderr := flags.Bool("stderr", false, "read stderr instead of stdout (logs only)")
	timeout := flags.Duration("timeout", 45*time.Second, "maximum time for a CLI query")
	flags.Usage = func() { fmt.Fprint(flags.Output(), usage); flags.PrintDefaults() }
	ordered, err := interspersed(flags, args)
	if err != nil {
		return err
	}
	if err := flags.Parse(ordered); err != nil {
		return err
	}
	command := "ui"
	positional := flags.Args()
	if len(positional) > 0 {
		command = positional[0]
		positional = positional[1:]
	}
	setupRequested := command == "setup"
	if setupRequested {
		command = "ui"
	}
	if command == "help" {
		flags.Usage()
		return nil
	}
	if command == "version" {
		if len(positional) > 0 {
			return fmt.Errorf("version takes no arguments")
		}
		fmt.Println("perch", version)
		return nil
	}
	if command == "receipts" {
		receiptArgs := append([]string{}, positional...)
		var invalid string
		flags.Visit(func(f *flag.Flag) {
			if f.Name != "host" && f.Name != "local" && f.Name != "json" {
				invalid = f.Name
				return
			}
			receiptArgs = append(receiptArgs, "--"+f.Name+"="+f.Value.String())
		})
		if invalid != "" {
			return fmt.Errorf("--%s is not supported by receipts", invalid)
		}
		return runReceipts(receiptArgs, os.Stdout, nil)
	}
	req := query.Request{Command: command, Stderr: *stderr}
	if command != "ui" && command != "doctor" && command != "mcp" {
		if len(positional) > 1 {
			return fmt.Errorf("%s takes at most one argument", command)
		}
		if len(positional) == 1 {
			req.ID = positional[0]
		}
		if err := query.Validate(req); err != nil {
			return err
		}
	} else {
		if len(positional) > 0 {
			return fmt.Errorf("unexpected argument %q", positional[0])
		}
		if *stderr {
			return fmt.Errorf("--stderr is only supported by logs")
		}
		if command == "mcp" && *asJSON {
			return fmt.Errorf("MCP uses structured protocol messages automatically; omit --json")
		}
		if command == "ui" && *asJSON {
			return fmt.Errorf("--json needs a CLI command, for example perch jobs --json")
		}
	}
	if err := applyPreferences(flags); err != nil {
		return err
	}
	warningPolicy, err := timewarning.Parse(*warningSpec)
	if err != nil {
		return err
	}
	if *cacheView && (command != "ui" || *demo) {
		return fmt.Errorf("--cache requires a live TUI workspace")
	}
	if *colorMode != "auto" && *colorMode != "truecolor" && *colorMode != "256" {
		return fmt.Errorf("--color-mode must be auto, truecolor or 256")
	}
	if *timeout <= 0 {
		return fmt.Errorf("--timeout must be positive")
	}
	if *theme != "warm" && *theme != "graphite" {
		return fmt.Errorf("unknown theme %q; choose warm or graphite", *theme)
	}
	if *demo && *local {
		return fmt.Errorf("choose either --demo or --local")
	}
	if setupRequested && (*demo || *local) {
		return fmt.Errorf("setup chooses an SSH destination; omit --demo and --local")
	}
	if *notificationMode != "off" && *notificationMode != "desktop" {
		return fmt.Errorf("--notify must be off or desktop")
	}
	if *debugRaw && !*debug {
		return fmt.Errorf("--debug-raw requires --debug; raw scheduler responses may contain private job metadata")
	}
	var notifier notify.Sender
	var stateDir string
	if *notificationMode == "desktop" {
		if command != "ui" || *demo {
			return fmt.Errorf("--notify desktop requires a live TUI workspace")
		}
		if runtime.GOOS != "darwin" {
			return fmt.Errorf("desktop notifications require macOS")
		}
		cache, err := os.UserCacheDir()
		if err != nil {
			return fmt.Errorf("notification state directory: %w", err)
		}
		stateDir = filepath.Join(cache, "perch", "notifications")
		notifier = notify.Desktop{}
	}
	var settings *preferences.Store
	if command == "ui" && !*demo && flags.Lookup("no-config").Value.String() != "true" {
		store, err := preferences.DefaultStore()
		if err != nil {
			return err
		}
		settings = &store
	}
	hostChosen := false
	// applyPreferences marks saved values as set too. Both explicit flags and
	// saved destinations bypass automatic setup; an SSH alias is never guessed.
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "host" {
			hostChosen = true
		}
	})
	interactive := term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd())
	if setupRequested || command == "ui" && !*demo && !*local && !hostChosen && interactive {
		if !interactive {
			return fmt.Errorf("setup needs an interactive terminal; use perch config set host user@hb.ucsc.edu")
		}
		initial := *host
		if !hostChosen {
			initial = ""
		}
		chosen, err := chooseDestination(ui.Options{Host: initial, Preferences: settings, Theme: *theme, NoColor: *noColor, NoMouse: *noMouse, ColorMode: *colorMode, PointerStyle: ui.DetectPointerStyle(os.Getenv)})
		if err != nil {
			return err
		}
		if chosen == "" {
			return nil
		}
		*host = chosen
	}
	capture, err := startDiagnostics(*debug, *debugRaw)
	if err != nil {
		return err
	}
	defer finishDiagnostics(capture)
	var diagnosticStatus func() string
	if capture != nil {
		diagnosticStatus = capture.Status
	}
	var runner transport.Runner
	if !*demo {
		if *local {
			runner = transport.Local{}
			*host = "local"
		} else {
			s, e := transport.NewSSH(*host)
			if e != nil {
				return e
			}
			defer s.Close()
			runner = s
		}
	}
	if capture != nil && runner != nil {
		runner = &diagnostics.Runner{Base: runner, Store: capture}
	}
	client := &hummingbird.Client{Runner: runner, Host: *host}
	if command == "mcp" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		server := mcpserver.New(version, mcpserver.Options{Service: query.Service{Backend: client, Demo: *demo}, Connect: client.Identify, Probe: client.Probe, Timeout: *timeout})
		return server.Run(ctx, &mcp.StdioTransport{})
	}
	if command != "ui" && command != "doctor" && command != "mcp" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		ctx, cancel := context.WithTimeout(ctx, *timeout)
		defer cancel()
		if !*demo {
			if err := client.Identify(ctx); err != nil {
				return err
			}
		}
		response, err := (query.Service{Backend: client, Demo: *demo}).Execute(ctx, req)
		if err != nil {
			return err
		}
		return cli.Write(os.Stdout, response, *asJSON)
	}
	if command == "doctor" {
		if *demo {
			return fmt.Errorf("doctor requires a real SSH host or --local")
		}
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		defer cancel()
		report, e := client.Probe(ctx)
		if *asJSON {
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(report); err != nil {
				return err
			}
		} else {
			fmt.Printf("Perch · connection check\n\nHost: %s\nUser: %s\nScheduler: %s\nParser: %s\n\n", report.Host, report.User, report.Version, report.Parser)
			for _, c := range report.Capabilities {
				fmt.Printf("%-22s %-19s %s\n", c.Name, c.Status, c.Detail)
			}
		}
		return e
	}
	journal, err := launch.DefaultJournal()
	if err != nil {
		return err
	}
	var viewCache *snapshotcache.Store
	if *cacheView {
		var err error
		viewCache, err = snapshotcache.DefaultStore()
		if err != nil {
			return err
		}
	}
	model := ui.New(client, ui.Options{TimeWarnings: warningPolicy, Cache: viewCache, ColorMode: *colorMode, PointerStyle: ui.DetectPointerStyle(os.Getenv), Diagnostics: diagnosticStatus, Demo: *demo, ReducedMotion: *static, NoColor: *noColor, NoMouse: *noMouse, Host: *host, Theme: *theme, StateDir: stateDir, Notifier: notifier, Journal: journal, Preferences: settings})
	defer model.Close()
	defer func() {
		if reset := model.PointerReset(); reset != "" {
			fmt.Fprint(os.Stdout, reset)
		}
	}()
	_, err = tea.NewProgram(model).Run()
	if receipt := model.CancellationSummary(); receipt != "" {
		fmt.Fprint(os.Stdout, receipt)
	}
	return err
}

const usage = `Perch · Hummingbird at a glance

Usage:
  perch [flags]                    Open the workspace
  perch setup [flags]              Choose an SSH destination and open the workspace
  perch run [options] -- COMMAND   Preview, validate or explicitly submit a job
  perch shell JOB_ID [--open]      Preview or enter a shell in a running allocation
  perch cancel JOB_ID [--confirm]  Preview or request job/task cancellation
  perch cancel ARRAY_ID --array   Preview cancellation of a whole array
  perch receipts [--host HOST]     Saved local submission receipts (offline)
  perch config [show|set|unset]    Inspect or save local preferences (offline)
  perch jobs [flags]               Current personal queue
  perch history [flags]            Seven days of accounting
  perch job JOB_ID [flags]         Inspect an allocation or array task
  perch logs JOB_ID [--stderr]     Read the last 256 KiB of output
  perch script JOB_ID [flags]      View the submitted script
  perch resources JOB_ID [flags]   Recorded and available live measurements
  perch nodes [flags]             Per-node CPU, memory and GPU allocations
  perch priority [flags]          Your pending-job priority factors by partition
  perch waits [flags]             Your observed queue waits by partition
  perch arrays [flags]             Observed array progress and failed IDs
  perch partitions [flags]         Partition allocation overview
  perch software [SEARCH] [flags] Search installed software modules
  perch module NAME/VERSION       Inspect module prerequisites and load command
  perch doctor [flags]             Check cluster capabilities
  perch mcp [flags]               Serve tools and guides over MCP stdio
  perch nrp [command] [flags]     Experimental namespace-scoped Nautilus queries
  perch version

Use --demo to explore offline, or --host your-cruzid@hb.ucsc.edu.
Add --json for machine-readable output. Flags may appear before or after IDs.
Hummingbird CLI queries never prompt; configure SSH keys or use the TUI.

Flags:
`

// The standard flag parser stops at the first positional argument. Preserve
// flag values while allowing natural commands such as logs 123 --stderr.
func interspersed(set *flag.FlagSet, args []string) ([]string, error) {
	var options, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			positional = append(positional, a)
			continue
		}
		options = append(options, a)
		name := strings.TrimLeft(a, "-")
		if strings.Contains(name, "=") {
			continue
		}
		f := set.Lookup(name)
		if f == nil {
			continue
		} // Parse supplies the standard unknown-flag/help error.
		if b, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && b.IsBoolFlag() {
			continue
		}
		i++
		if i >= len(args) {
			return nil, fmt.Errorf("flag needs an argument: %s", a)
		}
		options = append(options, args[i])
	}
	return append(append(options, "--"), positional...), nil
}
