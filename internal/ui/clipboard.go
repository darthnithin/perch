package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"perch/internal/hummingbird"
)

type copiedMsg struct {
	spec    string
	subject string
	err     error
}

func clipboardCommand(ctx context.Context) (*exec.Cmd, error) {
	if runtime.GOOS == "darwin" {
		if p, e := exec.LookPath("pbcopy"); e == nil {
			return exec.CommandContext(ctx, p), nil
		}
	}
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if p, e := exec.LookPath("wl-copy"); e == nil {
			return exec.CommandContext(ctx, p), nil
		}
	}
	if os.Getenv("DISPLAY") != "" {
		if p, e := exec.LookPath("xclip"); e == nil {
			return exec.CommandContext(ctx, p, "-selection", "clipboard"), nil
		}
	}
	return nil, fmt.Errorf("no local clipboard helper is available")
}
func (m *Model) copyFailed() tea.Cmd {
	if m.cachedStore != nil {
		m.setNotice("Refresh before copying cached array outcomes.")
		return nil
	}
	if m.clipboardCancel != nil {
		m.setNotice("Copying failed tasks…")
		return nil
	}
	j, ok := m.selected()
	if !ok || j.ArrayID == 0 {
		m.setNotice("Select an array task first.")
		return nil
	}
	spec := ""
	for _, array := range m.arraySummaries() {
		if array.ID == j.ArrayID {
			if array.SummaryError != "" {
				m.setNotice("Cannot copy failed tasks: " + array.SummaryError)
				return nil
			}
			spec = array.FailedSpec()
			break
		}
	}
	if spec == "" {
		m.setNotice("No confirmed failed task IDs are available for this array.")
		return nil
	}
	return m.copyText(spec, "failed tasks")
}

func (m *Model) copyModule() tea.Cmd {
	if m.clipboardCancel != nil {
		m.setNotice("Clipboard copy in progress…")
		return nil
	}
	info := m.moduleInfo
	if m.detailBusy || m.detailError != "" || info == nil || info.Name != m.softwareRequest.ID || !hummingbird.ValidModuleName(info.Name) || info.LoadCommand != "module load "+info.Name {
		m.setNotice("Read module details before copying its load command.")
		return nil
	}
	return m.copyText(info.LoadCommand, "module command")
}

func (m *Model) copyText(spec, subject string) tea.Cmd {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	m.clipboardCancel = cancel
	m.setNotice("Copying " + subject + "…")
	return func() tea.Msg {
		defer cancel()
		cmd, e := clipboardCommand(ctx)
		if e == nil {
			cmd.Stdin = strings.NewReader(spec)
			e = cmd.Run()
		}
		return copiedMsg{spec: spec, subject: subject, err: e}
	}
}
