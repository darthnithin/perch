package ui

import (
	"fmt"
	"regexp"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"perch/internal/domain"
)

const maxReaderMatches = 4096

type readerSearch struct {
	row            int
	savedCurrent   int
	savedPlain     string
	input          textinput.Model
	editing        bool
	query, plain   string
	matches        [][]int
	current        int
	truncated      bool
	savedQuery     string
	savedOffset    int
	savedFollowing bool
}

func (m *Model) startReaderSearch() tea.Cmd {
	loaded := m.logLoaded
	if m.view == "script" {
		loaded = m.scriptLoaded && m.detailError == "" && !m.cacheBlocked
	}
	if !m.textReader() || !loaded {
		m.notice = "Read " + m.readerSearchSubject() + " before searching it."
		return nil
	}
	if m.readerSearch == nil {
		input := textinput.New()
		input.Prompt, input.Placeholder, input.CharLimit = "find > ", "Search "+m.readerSearchSubject(), 100
		input.SetStyles(m.input.Styles())
		input.SetVirtualCursor(true)
		m.readerSearch = &readerSearch{input: input, current: -1, plain: m.readerPlainText()}
	}
	s := m.readerSearch
	if s.editing {
		return s.input.Focus()
	}
	m.inputError, m.notice = "", ""
	s.savedQuery, s.savedOffset, s.savedFollowing = s.query, m.viewport.YOffset(), m.logFollowing
	s.savedCurrent, s.savedPlain = s.current, s.plain
	s.editing = true
	s.input.SetValue(s.query)
	s.input.CursorEnd()
	s.input.SetWidth(max(10, m.width-16))
	m.input.Blur()
	if m.view == "logs" {
		m.logFollowing = false
	}
	return s.input.Focus()
}

func (m *Model) readerSearchEditing() bool {
	return m.textReader() && m.readerSearch != nil && m.readerSearch.editing && !m.help && !m.palette
}

func (m *Model) textReader() bool { return m.view == "logs" || m.view == "script" }

func (m *Model) readerSearchSubject() string {
	if m.view == "script" {
		return "submitted script"
	}
	return "retained output"
}

func (m *Model) readerPlainText() string {
	if m.view == "script" {
		return cleanLogText(m.scriptText)
	}
	return cleanLogText(m.logRaw)
}

func (m *Model) readerTextReplaced(previous string) bool {
	plain := m.readerPlainText()
	if m.view == "script" {
		return plain != previous
	}
	return !strings.HasPrefix(plain, previous)
}

func (m *Model) readerSearchUpdate(msg tea.Msg) tea.Cmd {
	s := m.readerSearch
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "enter":
			s.editing = false
			s.input.Blur()
			if s.query == "" {
				m.clearReaderSearch()
			}
			return nil
		case "esc":
			query, offset, following := s.savedQuery, s.savedOffset, s.savedFollowing
			s.query = query
			s.input.SetValue(query)
			s.current = s.savedCurrent
			if m.readerTextReplaced(s.savedPlain) {
				s.current = -1
			}
			s.editing = false
			s.input.Blur()
			if query == "" {
				m.clearReaderSearch()
			} else {
				m.indexReaderSearch(false)
			}
			m.viewport.SetYOffset(offset)
			if m.view == "logs" {
				m.logFollowing = following
			}
			return nil
		}
	}
	if paste, ok := msg.(tea.PasteMsg); ok {
		msg = tea.PasteMsg{Content: appendSearchText("", paste.Content)}
	}
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	query := appendSearchText("", s.input.Value())
	if query != s.input.Value() {
		s.input.SetValue(query)
	}
	if query != s.query {
		s.query = query
		m.indexReaderSearch(true)
	}
	return cmd
}

func (m *Model) clearReaderSearch() {
	m.readerSearch = nil
	m.paintReaderSearch()
}

// Reindex only on an edited query or a new read, never animation frames. A
// replaced script or rotated log cannot inherit an old match identity.
func (m *Model) indexReaderSearch(jump bool) {
	s := m.readerSearch
	plain := m.readerPlainText()
	if m.readerTextReplaced(s.plain) {
		s.current = -1
	}
	s.plain = plain
	s.matches = nil
	s.truncated = false
	if s.query != "" {
		pattern := regexp.MustCompile("(?i)" + regexp.QuoteMeta(s.query))
		s.matches = pattern.FindAllStringIndex(plain, maxReaderMatches+1)
		if len(s.matches) > maxReaderMatches {
			s.matches = s.matches[:maxReaderMatches]
			s.truncated = true
		}
	}
	if jump {
		s.current = 0
	}
	if s.current >= len(s.matches) {
		s.current = -1
	}
	m.paintReaderSearch()
	if jump {
		m.revealReaderMatch()
	}
}

func (m *Model) paintReaderSearch() {
	if m.view == "script" {
		m.displayScriptText()
	} else {
		m.displayLogText()
	}
}

func (m *Model) moveReaderMatch(forward bool) {
	s := m.readerSearch
	if len(s.matches) == 0 {
		return
	}
	if s.current < 0 {
		s.current = 0
		if !forward {
			s.current = len(s.matches) - 1
		}
	} else if forward {
		s.current = (s.current + 1) % len(s.matches)
	} else {
		s.current = (s.current - 1 + len(s.matches)) % len(s.matches)
	}
	if m.view == "logs" {
		m.logFollowing = false
	}
	m.paintReaderSearch()
	m.revealReaderMatch()
}

func (m *Model) revealReaderMatch() {
	s := m.readerSearch
	if s == nil || s.current < 0 || s.current >= len(s.matches) {
		return
	}
	m.viewport.SetYOffset(max(0, s.row-m.viewport.Height()/3))
}

func (m *Model) readerSearchSummary() string {
	s := m.readerSearch
	if s.query == "" {
		return "Search " + m.readerSearchSubject() + " · literal, case-insensitive"
	}
	if len(s.matches) == 0 {
		return "No matches in " + m.readerSearchSubject()
	}
	count := fmt.Sprint(len(s.matches))
	if s.truncated {
		count += "+"
	}
	if s.current < 0 {
		if len(s.matches) == 1 {
			return "1 match · " + m.readerSearchSubject()
		}
		return count + " matches · " + m.readerSearchSubject()
	}
	return fmt.Sprintf("Match %d / %s · %s", s.current+1, count, m.readerSearchSubject())
}

func cleanLogText(raw string) string { return strings.ReplaceAll(domain.Clean(raw), "\t", "    ") }
