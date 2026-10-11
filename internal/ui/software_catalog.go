package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/query"
)

type softwareCatalog struct {
	request  query.Request
	items    []query.Software
	index    int
	selected string
	at       time.Time
}

type catalogLayout struct{ header, stride, visible, start int }

func (m *Model) catalogReady() bool {
	return m.view == "software" && m.softwareRequest.Command == "software" && m.catalog != nil && !m.detailBusy && m.detailError == ""
}

func (m *Model) catalogGeometry() catalogLayout {
	l := m.layout()
	g := catalogLayout{header: 2, stride: 1}
	if l.inner-4 < 68 {
		g.stride = 2
	}
	if l.bodyHeight-2 < 7 {
		g.header = 1
	}
	g.visible = max(1, (l.bodyHeight-2-g.header-1)/g.stride)
	g.start = max(0, m.catalog.index-g.visible+1)
	return g
}

func (m *Model) selectModule(index int) {
	c := m.catalog
	c.index = max(0, min(len(c.items)-1, index))
	if len(c.items) > 0 {
		c.selected = c.items[c.index].Name
	}
}

func (m *Model) catalogKey(key string) (tea.Cmd, bool) {
	if !m.catalogReady() || m.help || m.palette {
		return nil, false
	}
	c, g := m.catalog, m.catalogGeometry()
	switch key {
	case "down", "j":
		m.selectModule(c.index + 1)
	case "up", "k":
		m.selectModule(c.index - 1)
	case "pgdown":
		m.selectModule(c.index + g.visible)
	case "pgup":
		m.selectModule(c.index - g.visible)
	case "home":
		m.selectModule(0)
	case "end":
		m.selectModule(len(c.items) - 1)
	case "enter":
		if len(c.items) > 0 {
			return m.showSoftware("module", c.items[c.index].Name), true
		}
	default:
		return nil, false
	}
	return nil, true
}

func (m *Model) catalogContent(width, height int) string {
	c, g := m.catalog, m.catalogGeometry()
	if len(c.items) == 0 {
		return "No matching modules.\n\nTry a broader /software search."
	}
	noun := "modules"
	if len(c.items) == 1 {
		noun = "module"
	}
	rows := []string{muted.Render(fmt.Sprintf("%d %s · login environment", len(c.items), noun))}
	if g.header == 2 {
		rows = append(rows, "")
	}
	for i := g.start; i < min(len(c.items), g.start+g.visible); i++ {
		item := c.items[i]
		status := "prerequisites needed"
		color := yellow
		if item.Available {
			status, color = "available", green
		}
		name := "  " + ansi.Truncate(item.Name, width-2, "…")
		if g.stride == 1 {
			name = "  " + fit(ansi.Truncate(item.Name, width-25, "…"), width-25) + "  " + color.Render(status)
		}
		if i == c.index {
			name = selected.Render(fit("› "+ansi.Strip(name[2:]), width))
		}
		rows = append(rows, name)
		if g.stride == 2 {
			rows = append(rows, "  "+color.Render(status))
		}
	}
	for len(rows) < height-1 {
		rows = append(rows, "")
	}
	rows = append(rows, muted.Render(fmt.Sprintf("%d–%d of %d %s", g.start+1, min(len(c.items), g.start+g.visible), len(c.items), noun)))
	return strings.Join(rows, "\n")
}
