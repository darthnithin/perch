package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"perch/internal/domain"
)

func TestReportReturnsToPartitionAfterRefreshAndReflow(t *testing.T) {
	for _, width := range []int{48, 80, 140} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			m := demoModel(80, 18)
			defer m.Close()
			parts := make([]domain.Partition, 12)
			for i := range parts {
				parts[i] = domain.Partition{Name: fmt.Sprintf("partition-%02d", i), State: "UP", CPUs: 48, GRES: strings.Repeat("gpu:model:4 ", 8)}
			}
			m.store.Snapshot.Partitions = parts
			press(m, '3')
			find := func() int {
				for i, line := range panelLines(m.panel.GetContent()) {
					if strings.HasPrefix(line, "partition-06 ") {
						return i
					}
				}
				t.Fatal("missing partition")
				return 0
			}
			m.panel.SetYOffset(find() + 3) // allocation row; its value will change
			c := m.action("n")
			m.Update(c())
			m.Update(tea.WindowSizeMsg{Width: width, Height: 18})
			for i := range parts {
				parts[i].AllocatedCPUs = 17
				parts[i].NodeCount = 4
			}
			m.store.Snapshot.Partitions = append([]domain.Partition{{Name: "new-partition", State: "UP"}}, parts...)
			// Refresh and a second report must keep the original return bookmark.
			m.Update(m.action("r")())
			m.Update(command(m, "/priority")())
			m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
			if m.tab != 2 || m.panel.YOffset() != find()+3 {
				t.Fatalf("return lost section: tab %d offset %d want %d", m.tab, m.panel.YOffset(), find()+3)
			}
		})
	}
}

func TestReportReturnsToConnectionScroll(t *testing.T) {
	m := demoModel(48, 18)
	defer m.Close()
	press(m, '4')
	m.panel.SetYOffset(4)
	want := m.panel.YOffset()
	m.Update(command(m, "/software")())
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.tab != 3 || m.panel.YOffset() != want {
		t.Fatalf("connection return: tab %d row %d want %d", m.tab, m.panel.YOffset(), want)
	}
}

func TestReportReturnClampsRemovedContent(t *testing.T) {
	m := demoModel(48, 18)
	defer m.Close()
	press(m, '3')
	m.panel.GotoBottom()
	if m.panel.YOffset() == 0 {
		t.Fatal("test requires a scrolled panel")
	}
	m.Update(m.action("n")())
	m.store.Snapshot.Partitions = nil
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.tab != 2 || m.panel.YOffset() != 0 {
		t.Fatal("removed content left an invalid scroll position")
	}
}

func TestPanelBookmarkFallbacks(t *testing.T) {
	for _, tc := range []struct {
		old, next string
		row, want int
	}{
		{"header\na\nb", "new\nheader\na\nb", 0, 0},
		{"header\na\nb", "unrelated", 2, 2},
		{"header\nrepeat\nrepeat", "new\nheader\nrepeat\nrepeat", 2, 3},
		{"header   up\n━━━\nold count", "new\nheader up\n───\nnew count", 2, 3},
	} {
		if got := bookmarkPanel(tc.old, tc.row).offset(tc.next); got != tc.want {
			t.Errorf("%q → %q: %d want %d", tc.old, tc.next, got, tc.want)
		}
	}
}
