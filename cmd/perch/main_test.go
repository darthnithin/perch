package main

import (
	"errors"
	"flag"
	"io"
	"reflect"
	"testing"
)

func TestInterspersedFlags(t *testing.T) {
	for _, args := range [][]string{{"logs", "9100_11", "--stderr", "--host", "ndanday@hb.ucsc.edu"}, {"--host=ndanday@hb.ucsc.edu", "logs", "--stderr", "9100_11"}} {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		host := fs.String("host", "", "")
		stderr := fs.Bool("stderr", false, "")
		ordered, err := interspersed(fs, args)
		if err != nil {
			t.Fatal(err)
		}
		if err = fs.Parse(ordered); err != nil {
			t.Fatal(err)
		}
		if *host != "ndanday@hb.ucsc.edu" || !*stderr || !reflect.DeepEqual(fs.Args(), []string{"logs", "9100_11"}) {
			t.Fatalf("bad parse: %+v %s %v", fs.Args(), *host, *stderr)
		}
	}
}
func TestHelpAndInvalidCommands(t *testing.T) {
	for _, args := range [][]string{{"job"}, {"wat"}, {"logs", "12", "13"}, {"jobs", "--stderr"}, {"--json"}, {"jobs", "--timeout=0"}, {"jobs", "--demo", "--local"}, {"--notify=bogus"}, {"--demo", "--notify=desktop"}, {"jobs", "--notify=desktop"}} {
		if err := run(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	args, _ := interspersed(fs, []string{"jobs", "--help"})
	if err := fs.Parse(args); !errors.Is(err, flag.ErrHelp) {
		t.Fatal(err)
	}
}
