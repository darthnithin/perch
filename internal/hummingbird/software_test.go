package hummingbird

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"perch/internal/transport"
)

func TestLiveCatalogFixturesAndHierarchy(t *testing.T) {
	spider, e := os.ReadFile("../../testdata/hummingbird/module-spider.txt")
	if e != nil {
		t.Fatal(e)
	}
	avail, e := os.ReadFile("../../testdata/hummingbird/module-avail.txt")
	if e != nil {
		t.Fatal(e)
	}
	runner := &fakeRunner{results: []transport.Result{{Stdout: string(spider)}, {Stdout: string(avail)}}}
	c := Client{Runner: runner}
	list, e := c.Software(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if len(list) < 200 {
		t.Fatalf("catalog unexpectedly small: %d", len(list))
	}
	found := map[string]bool{}
	for _, item := range list {
		found[item.Name] = item.Available
	}
	if !found["cuda/12.8.1"] {
		t.Fatal("available CUDA lost")
	}
	if available, ok := found["openmpi5/5.0.10"]; !ok || available {
		t.Fatal("hierarchical MPI module incorrectly marked available", available, ok)
	}
	for _, args := range runner.commands {
		if !strings.Contains(strings.Join(args, " "), "module --redirect -t") {
			t.Fatal("unexpected command", args)
		}
	}
}
func TestCatalogFailsOnErrorsAndMalformedOutput(t *testing.T) {
	for _, input := range []string{"Lmod warning: catalog unavailable", "welcome to server", "tool/1.0;touch", "tool/$(cmd)"} {
		if _, e := parseModules(input); e == nil {
			t.Fatalf("accepted %q", input)
		}
	}
	r := &fakeRunner{results: []transport.Result{{Stdout: "cuda/12.8.1"}, {ExitCode: 1, Stderr: "permission denied"}}}
	if _, e := (&Client{Runner: r}).Software(context.Background()); e == nil {
		t.Fatal("availability failure hidden")
	}
	r.err = errors.New("offline")
	if _, e := (&Client{Runner: r}).Software(context.Background()); e == nil {
		t.Fatal("transport failure hidden")
	}
}
func TestModuleValidationSanitizationAndArguments(t *testing.T) {
	r := &fakeRunner{results: []transport.Result{{Stdout: "Details\x1b]52;c;bad\a\nload gnu14 first"}}}
	c := Client{Runner: r}
	for _, name := range []string{"", "-x", "a;echo bad", "../../etc/passwd", "cuda/*", "x\ny", "x/$(id)"} {
		if _, e := c.Module(context.Background(), name); e == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	if len(r.commands) != 0 {
		t.Fatal("invalid module reached runner")
	}
	got, e := c.Module(context.Background(), "cuda/12.8.1")
	if e != nil {
		t.Fatal(e)
	}
	if got.Details != "Details\nload gnu14 first" || got.LoadCommand != "module load cuda/12.8.1" {
		t.Fatal(got)
	}
	args := r.commands[0]
	if args[len(args)-1] != "cuda/12.8.1" || strings.Contains(args[2], "cuda/12.8.1") {
		t.Fatal("module name was interpolated into shell", args)
	}
	r.results[0] = transport.Result{ExitCode: 1}
	if _, e := c.Module(context.Background(), "missing/1"); e == nil {
		t.Fatal("missing module succeeded")
	}
}
