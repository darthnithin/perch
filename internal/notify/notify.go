// Package notify turns reconciled events into bounded, opt-in notifications.
package notify

import (
	"context"
	"fmt"
	"os/exec"
	"perch/internal/app"
	"perch/internal/domain"
	"runtime"
	"sort"
	"strings"
)

type Message struct{ Title, Body string }
type Sender interface {
	Send(context.Context, Message) error
}
type Desktop struct{}

// Arguments are data to a fixed script, never AppleScript source. No sound is
// requested; display is controlled by the user's macOS notification settings.
const desktopScript = `on run argv
 display notification (item 2 of argv) with title (item 1 of argv)
end run`

func (Desktop) Send(ctx context.Context, m Message) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("desktop notifications require macOS")
	}
	return desktopCommand(ctx, m).Run()
}

func desktopCommand(ctx context.Context, m Message) *exec.Cmd {
	return exec.CommandContext(ctx, "/usr/bin/osascript", "-e", desktopScript, "--", m.Title, m.Body)
}

func label(s string, limit int) string {
	r := []rune(domain.Label(s))
	if len(r) > limit {
		return string(r[:limit]) + "…"
	}
	return string(r)
}

// Messages groups array tasks and caps catch-up delivery to three banners. A
// large burst becomes one summary, with details left in the History view.
func Messages(events []app.Event) []Message {
	groups := map[string][]app.Event{}
	seen := map[string]bool{}
	for _, e := range events {
		if seen[e.Key] {
			continue
		}
		seen[e.Key] = true
		switch e.Kind {
		case "finished", "failed", "cancelled", "time warning":
		default:
			continue
		}
		key := e.Kind + "/job/" + e.JobID
		if e.ArrayID > 0 {
			key = fmt.Sprintf("%s/array/%d", e.Kind, e.ArrayID)
		}
		groups[key] = append(groups[key], e)
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) > 3 {
		counts := map[string]int64{}
		for _, g := range groups {
			for _, e := range g {
				counts[e.Kind] += max(1, e.TaskCount)
			}
		}
		var parts []string
		for _, kind := range []string{"failed", "cancelled", "finished", "time warning"} {
			if n := counts[kind]; n > 0 {
				parts = append(parts, fmt.Sprintf("%d %s", n, kind))
			}
		}
		return []Message{{Title: "Perch · workload updates", Body: strings.Join(parts, ", ") + ". Open Perch for details."}}
	}
	var out []Message
	for _, k := range keys {
		g := groups[k]
		e := g[0]
		title := "Perch · job " + e.Kind
		body := fmt.Sprintf("%s · %s · %s", label(e.JobID, 40), label(e.Name, 80), label(e.State, 40))
		if e.Kind == "time warning" {
			title = "Perch · approaching time limit"
			body = fmt.Sprintf("%s · %s · near the time limit at last refresh", label(e.JobID, 40), label(e.Name, 80))
			if e.WarningThreshold > 0 {
				body = fmt.Sprintf("%s · %s · %s remaining at last refresh", label(e.JobID, 40), label(e.Name, 80), domain.Duration(e.Remaining))
			}
		}
		if e.ArrayID > 0 {
			title = "Perch · array update"
			var tasks int64
			for _, event := range g {
				tasks += max(1, event.TaskCount)
			}
			noun, outcome := "task", e.Kind
			if tasks != 1 {
				noun = "tasks"
			}
			if e.Kind == "time warning" {
				outcome = "near the time limit"
			}
			body = fmt.Sprintf("Array %d · %d %s %s", e.ArrayID, tasks, noun, outcome)
		}
		out = append(out, Message{title, body})
	}
	return out
}
