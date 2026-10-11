package nautilusui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/nautilus"
)

func TestQuotaViewPreservesScopeAndStaleEvidence(t *testing.T) {
	for _, size := range [][2]int{{140, 40}, {80, 24}, {48, 18}} {
		m := ready(t, size[0], size[1])
		key(m, "3")
		text := ansi.Strip(m.panelText())
		for _, want := range []string{"compute", "NVIDIA GPU requests", "4 / 4 · at limit", "PriorityClass In (interactive)", "Configured limit: 2Ti", "unknown / 20", "Scoped quotas overlap"} {
			if !strings.Contains(text, want) {
				t.Fatal(size, "missing", want, text)
			}
		}
		lines := strings.Split(m.View().Content, "\n")
		if size[1] == 18 && !strings.Contains(ansi.Strip(m.View().Content), "at limit") {
			t.Fatal("compact quota hides all usage")
		}
		if len(lines) > size[1] {
			t.Fatal("height overflow", size)
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("width overflow", size, line)
			}
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
		if !m.panel.AtBottom() {
			t.Fatal("quota panel cannot scroll")
		}
		old := m.quotas
		result := m.client.Refresh(context.Background())
		result.Quotas = nautilus.Result[nautilus.Quota]{Error: &nautilus.QueryError{Kind: nautilus.Forbidden, Message: "Access denied"}}
		m.refresh()
		m.Update(refreshMsg{m.sequence, result})
		text = ansi.Strip(m.panelText())
		if m.quotas != old || !strings.Contains(text, "Stale quota snapshot") || !strings.Contains(text, "at limit") || m.authRequired {
			t.Fatal("lost retained quota/incorrect auth", text)
		}
		if m.nextRefresh.Sub(m.now) <= 15*time.Second {
			t.Fatal("missing backoff")
		}
		result.Quotas = nautilus.Result[nautilus.Quota]{Collection: &nautilus.Collection[nautilus.Quota]{Items: []nautilus.Quota{}, ObservedAt: time.Now()}}
		m.refresh()
		m.Update(refreshMsg{m.sequence, result})
		if !strings.Contains(m.panelText(), "No ResourceQuota objects reported") || strings.Contains(m.panelText(), "at limit") {
			t.Fatal("empty success retained removed quota")
		}
	}
}

func TestQuotaAuthFailureAndCommandReturn(t *testing.T) {
	m := ready(t, 80, 24)
	m.input.SetValue("/find retrying")
	m.submit()
	m.input.SetValue("/quotas")
	m.submit()
	if m.tab != 2 || m.filters[0] != "retrying" {
		t.Fatal("quota command lost list state")
	}
	result := m.client.Refresh(context.Background())
	result.Quotas = nautilus.Result[nautilus.Quota]{Error: &nautilus.QueryError{Kind: nautilus.Unauthenticated, Message: "Sign in again"}}
	m.refresh()
	m.Update(refreshMsg{m.sequence, result})
	if !m.authRequired {
		t.Fatal("quota 401 did not suspend background reads")
	}
	seq := m.sequence
	m.Update(clockMsg(m.now.Add(time.Hour)))
	if seq != m.sequence {
		t.Fatal("unexpected background auth retry")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.tab != 0 || m.filters[0] != "retrying" {
		t.Fatal("return lost filtered list")
	}
}
