package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"perch/internal/query"
)

func (m *Model) showSoftware(command, argument string) tea.Cmd {
	request := query.Request{Command: command, ID: argument}
	if err := query.Validate(request); err != nil {
		m.inputError = err.Error()
		return nil
	}
	m.input.Blur()
	m.input.SetValue("")
	m.inputError = ""
	m.help, m.palette, m.searching = false, false, false
	if m.view != "software" {
		m.reportOrigin = reportReturn{tab: m.tab, search: m.search, selectedID: m.selectedID}
		if m.tab >= 2 {
			m.syncPanel()
			m.reportOrigin.panel = bookmarkPanel(m.panel.GetContent(), m.panel.YOffset())
		}
		m.switchTab(0)
	}
	previousRequest := m.softwareRequest
	if m.view != "software" || previousRequest.Command != "nodes" || command != "nodes" {
		m.nodesGPUOnly = false
	}
	m.nodeRows = nil
	previousCatalog := m.catalog
	if command == "module" && m.catalogReady() {
		m.catalogReturn = m.catalog
	} else if command != "module" || m.view != "software" {
		m.catalogReturn = nil
	}
	m.catalog = nil
	if command == "software" {
		m.catalog = &softwareCatalog{request: request}
		if m.view == "software" && previousRequest == request && previousCatalog != nil {
			m.catalog.selected, m.catalog.index = previousCatalog.selected, previousCatalog.index
		}
	}
	m.view = "software"
	m.softwareRequest = request
	m.invalidateDetail()
	m.detailBusy = true
	m.reportAt = time.Time{}
	m.activity.returning = false
	m.softwareBody = "Looking through the cluster software catalog…"
	if command == "nodes" {
		m.softwareBody = "Reading node allocations…"
	}
	if command == "priority" {
		m.softwareBody = "Reading your pending-job priority factors…"
	}
	m.viewport.SetContent(m.softwareBody)
	seq := m.detailSequence
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	m.detailCancel = cancel
	service := query.Service{Backend: m.client, Demo: m.options.Demo}
	return func() tea.Msg {
		defer cancel()
		result, err := service.Execute(ctx, request)
		var body string
		if err == nil {
			body = softwareContent(result.Data)
			if result.PriorityPolicy != nil {
				body += "\n\n" + ink.Bold(true).Render("Cluster policy") + "\n" + result.PriorityPolicy.Note
				if weight := result.PriorityPolicy.FairshareWeight; weight != nil && *weight != 0 {
					body += fmt.Sprintf("\nConfigured fair-share weight: %d", *weight)
				}
			}
			if command == "priority" || command == "nodes" {
				body += "\n\n" + muted.Render("Observed: "+result.At.Local().Format("Jan 2 15:04:05")+" · on demand")
			}
		}
		modules, _ := result.Data.([]query.Software)
		nodes, _ := result.Data.([]query.Node)
		var module *query.ModuleInfo
		if info, ok := result.Data.(query.ModuleInfo); ok && err == nil {
			module = &info
		}
		return detailMsg{sequence: seq, view: "software", text: body, modules: modules, nodes: nodes, module: module, observed: result.At, err: err}
	}
}

func softwareContent(data any) string {
	switch data := data.(type) {
	case []query.Node:
		return nodesContent(data)
	case []query.Priority:
		return priorityContent(data)
	case []query.Software:
		if len(data) == 0 {
			return "No matching modules. Try a broader /software search."
		}
		label := "modules"
		if len(data) == 1 {
			label = "module"
		}
		rows := []string{ink.Bold(true).Render(fmt.Sprintf("%d %s", len(data), label)), ""}
		for _, module := range data {
			status := muted.Render("prerequisites needed")
			if module.Available {
				status = green.Render("available")
			}
			rows = append(rows, accent.Render(module.Name)+"  "+status)
		}
		rows = append(rows, "", muted.Render("Availability reflects the login environment."), muted.Render("Open /module name/version to see setup instructions."))
		return strings.Join(rows, "\n")
	case query.ModuleInfo:
		return ink.Bold(true).Render("After loading prerequisites") + "\n" + accent.Render(data.LoadCommand) + "\n\n" + data.Details + "\n\n" + muted.Render("Use in your cluster shell or batch script.")
	}
	return "Software details unavailable."
}

func (m *Model) syncSoftware() {
	if m.view != "software" {
		return
	}
	width := max(10, m.width-8)
	if m.softwareBody == m.softwareRenderedText && width == m.softwareRenderedWidth {
		return
	}
	// Word-wrap prose and hard-wrap long paths/tokens before the viewport sees
	// them. Reuse static layout on animation frames and scroll events.
	body := ansi.Wrap(m.softwareBody, width, " ")
	offset := m.viewport.YOffset()
	m.viewport.SetContent(body)
	m.viewport.SetYOffset(offset)
	m.softwareRenderedText, m.softwareRenderedWidth = m.softwareBody, width
}

// Global reports return to the calling list with its filter and identity intact.
type reportReturn struct {
	tab                int
	search, selectedID string
	panel              panelBookmark
}

func (m *Model) returnFromReport() tea.Cmd {
	if m.softwareRequest.Command == "module" && m.catalogReturn != nil {
		m.invalidateDetail()
		m.catalog, m.catalogReturn = m.catalogReturn, nil
		m.softwareRequest, m.reportAt = m.catalog.request, m.catalog.at
		m.softwareBody = softwareContent(m.catalog.items)
		m.syncSoftware()
		return nil
	}
	m.catalog, m.catalogReturn = nil, nil
	origin := m.reportOrigin
	m.switchTab(origin.tab)
	m.search, m.selectedID = origin.search, origin.selectedID
	m.normalizeSelection()
	cmd := m.setView("details")
	if origin.tab >= 2 {
		m.syncPanel()
		m.panel.SetYOffset(origin.panel.offset(m.panel.GetContent()))
	}
	return cmd
}
