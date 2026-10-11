package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"perch/internal/preferences"
	"strings"
)

func applyPreferences(flags *flag.FlagSet) error {
	if f := flags.Lookup("no-config"); f != nil && f.Value.String() == "true" {
		return nil
	}
	store, err := preferences.DefaultStore()
	if err != nil {
		return err
	}
	values, err := store.Load()
	if err != nil {
		return fmt.Errorf("preferences at %s: %w; inspect with perch config path or bypass with --no-config", store.Path(), err)
	}
	explicit := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	for name, value := range values.Flags() {
		if flags.Lookup(name) != nil && !explicit[name] {
			if err := flags.Set(name, value); err != nil {
				return err
			}
		}
	}
	return nil
}

func runConfig(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("perch config", flag.ContinueOnError)
	flags.SetOutput(out)
	asJSON := flags.Bool("json", false, "show saved overrides as JSON")
	flags.Usage = func() {
		fmt.Fprintln(out, "Usage: perch config [show [--json] | path | set KEY VALUE | unset KEY | reset]\nKeys: host, theme (warm/graphite), color-mode (auto/truecolor/256), reduced-motion (true/false), mouse (true/false), time-warnings (5m or 15m,5m,1m or off)\nFlags override saved preferences. Desktop notification opt-in and submission actions are never saved.")
	}
	ordered, err := interspersed(flags, args)
	if err != nil {
		return err
	}
	if err = flags.Parse(ordered); err != nil {
		return err
	}
	args = flags.Args()
	if len(args) == 0 {
		args = []string{"show"}
	}
	store, err := preferences.DefaultStore()
	if err != nil {
		return err
	}
	if *asJSON && args[0] != "show" {
		return fmt.Errorf("--json is supported by config show only")
	}
	switch args[0] {
	case "path":
		if len(args) != 1 {
			return fmt.Errorf("config path takes no arguments")
		}
		_, err = fmt.Fprintln(out, store.Path())
		return err
	case "show":
		if len(args) != 1 {
			return fmt.Errorf("config show takes no arguments")
		}
		values, err := store.Load()
		if err != nil {
			return fmt.Errorf("%s: %w", store.Path(), err)
		}
		if *asJSON {
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			return enc.Encode(values)
		}
		var text strings.Builder
		fmt.Fprintln(&text, "Preferences · "+store.Path())
		fmt.Fprint(&text, "Command-line flags override these defaults.\n\n")
		saved := values.Flags()
		for _, entry := range []struct{ key, flag, fallback string }{{"host", "host", "hb.ucsc.edu"}, {"theme", "theme", "warm"}, {"color-mode", "color-mode", "auto"}, {"reduced-motion", "reduced-motion", "false"}, {"mouse", "no-mouse", "false"}, {"time-warnings", "time-warnings", "5m"}} {
			value, ok := saved[entry.flag]
			origin := "saved"
			if !ok {
				value, origin = entry.fallback, "built-in"
			}
			if entry.key == "mouse" {
				if value == "true" {
					value = "false"
				} else {
					value = "true"
				}
			}
			fmt.Fprintf(&text, "%-16s %-24s %s\n", entry.key, value, origin)
		}
		_, err = fmt.Fprint(out, text.String())
		return err
	case "set":
		if len(args) != 3 {
			return fmt.Errorf("usage: perch config set KEY VALUE")
		}
		err = store.Edit(context.Background(), args[1], &args[2], false)
	case "unset":
		if len(args) != 2 {
			return fmt.Errorf("usage: perch config unset KEY")
		}
		err = store.Edit(context.Background(), args[1], nil, false)
	case "reset":
		if len(args) != 1 {
			return fmt.Errorf("config reset takes no arguments")
		}
		err = store.Edit(context.Background(), "", nil, true)
	default:
		return fmt.Errorf("unknown config command %q; use perch config --help", args[0])
	}
	if err != nil {
		return fmt.Errorf("could not finish saving preferences at %s: %w; inspect with perch config show", store.Path(), err)
	}
	_, err = fmt.Fprintln(out, "Preferences saved. They apply to the next invocation.")
	return err
}
