package hummingbird

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"perch/internal/domain"
)

type Software struct {
	Name      string `json:"name"`
	Available bool   `json:"available_in_login_environment"`
}
type ModuleInfo struct {
	Name        string `json:"name"`
	Details     string `json:"details"`
	LoadCommand string `json:"load_command"`
}

var moduleName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.+-]*(/[A-Za-z0-9][A-Za-z0-9_.+-]*)*$`)

func ValidModuleName(name string) bool { return len(name) <= 160 && moduleName.MatchString(name) }

// Lmod's terse output contains exact versions, family markers (name/), and
// optional modulepath headings. Reject other text instead of reporting a broken
// command or a shell startup banner as a successful empty software catalog.
func parseModules(text string) ([]string, error) {
	names := map[string]bool{}
	for _, line := range strings.Split(domain.Clean(text), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasSuffix(line, "/") || (strings.HasPrefix(line, "/") && strings.HasSuffix(line, ":")) {
			continue
		}
		if !ValidModuleName(line) {
			return nil, fmt.Errorf("unrecognized Lmod catalog output")
		}
		names[line] = true
	}
	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

func (c *Client) Software(ctx context.Context) ([]Software, error) {
	// module is a shell function initialized by the cluster's login profile.
	// These commands enumerate metadata; they never load software or run a task.
	r, e := c.Runner.Batch(ctx, [][]string{
		{"bash", "-lc", "export LMOD_COLORIZE=no LMOD_PAGER=none; module --redirect -t spider"},
		{"bash", "-lc", "export LMOD_COLORIZE=no LMOD_PAGER=none; module --redirect -t avail"},
	})
	if e != nil {
		return nil, e
	}
	if len(r) != 2 {
		return nil, fmt.Errorf("incomplete software catalog response")
	}
	for _, result := range r {
		if e := commandError(result); e != nil {
			return nil, e
		}
	}
	all, e := parseModules(r[0].Stdout)
	if e != nil {
		return nil, e
	}
	current, e := parseModules(r[1].Stdout)
	if e != nil {
		return nil, e
	}
	available := map[string]bool{}
	for _, name := range current {
		available[name] = true
	}
	out := make([]Software, 0, len(all))
	for _, name := range all {
		out = append(out, Software{Name: name, Available: available[name]})
	}
	return out, nil
}

func (c *Client) Module(ctx context.Context, name string) (ModuleInfo, error) {
	if !ValidModuleName(name) {
		return ModuleInfo{}, fmt.Errorf("use an exact module name or name/version")
	}
	r, e := c.Runner.Run(ctx, []string{"bash", "-lc", `export LMOD_COLORIZE=no LMOD_PAGER=none; module --redirect spider "$1" 2>&1`, "perch-module", name})
	if e != nil {
		return ModuleInfo{}, e
	}
	if e = commandError(r); e != nil {
		return ModuleInfo{}, e
	}
	details := strings.TrimSpace(domain.Clean(r.Stdout))
	if details == "" {
		return ModuleInfo{}, fmt.Errorf("module details unavailable")
	}
	return ModuleInfo{Name: name, Details: details, LoadCommand: "module load " + name}, nil
}
