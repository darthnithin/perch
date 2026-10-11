package ui

import (
	"strings"

	"perch/assets/bird"
)

var tabLabels = []string{"1  Jobs", "2  History", "3  Partitions", "4  Connection"}

// Drawing and pointer hit testing share the same cell geometry.
type workspaceLayout struct {
	inner, bodyTop, bodyHeight, jobsWidth, detailWidth, tabGap int
	chip                                                       []string
	tabs                                                       []string
}

func (m *Model) layout() workspaceLayout {
	l := workspaceLayout{inner: m.width - 4, bodyTop: 4, bodyHeight: m.height - 12, tabGap: 4}
	if m.compactTextReading() || m.view == "warnings" || m.view == "cancel" || m.palette && m.paletteJob != nil || m.launchEditingProblem() {
		l.bodyHeight = m.height - 8
	}
	if m.view == "settings" && m.height == 18 {
		l.bodyHeight++ // Borrow the status spacer so all five controls fit.
	}
	l.tabs = append([]string(nil), tabLabels...)
	if l.inner < 70 {
		l.tabGap = 2
	}
	if l.inner < 58 {
		l.tabGap = 1
		for i, label := range l.tabs {
			l.tabs[i] = strings.ReplaceAll(label, "  ", " ")
		}
	}
	if m.width >= 96 && m.height >= 36 && !m.options.NoColor && m.tab == 0 && !m.help && !m.palette && m.view == "details" {
		l.chip = bird.OctantFrame(m.chipState(), m.frame, m.options.ReducedMotion)
		if len(l.chip) > 1 {
			l.bodyHeight -= len(l.chip) - 2
			l.bodyTop += len(l.chip) - 2
		}
	}
	l.jobsWidth = l.inner
	if l.inner >= 92 && !m.emptyWorkspace() {
		l.detailWidth = max(36, l.inner*43/100)
		l.jobsWidth -= l.detailWidth + 1
	}
	return l
}

type jobListLayout struct {
	compact, roomy                    bool
	header, rowHeight, visible, start int
}

type paletteLayout struct {
	x, y, width, height, rowWidth, visible, start int
}

func (m *Model) paletteGeometry() paletteLayout {
	l := m.layout()
	g := paletteLayout{x: 2, y: l.bodyTop, width: l.inner, height: l.bodyHeight}
	if m.paletteJob != nil {
		g.width = min(g.width, 70)
		g.height = min(g.height, len(jobCommands)+5)
		g.x += (l.inner - g.width) / 2
		g.y += (l.bodyHeight - g.height) / 3
	}
	g.rowWidth = min(60, g.width-8)
	g.visible = max(1, g.height-5)
	g.start = max(0, m.paletteIndex-g.visible+1)
	return g
}

func jobGeometry(width, height, selection int) jobListLayout {
	l := jobListLayout{compact: width < 58, header: 3, rowHeight: 1}
	l.roomy = !l.compact && height >= 14
	if l.compact {
		l.header, l.rowHeight = 2, 2
		if height < 5 {
			l.header = 1
		}
	}
	if l.roomy {
		l.header, l.rowHeight = 5, 2
	}
	l.visible = max(1, (height-l.header-1)/l.rowHeight)
	l.start = max(0, selection-l.visible+1)
	return l
}
