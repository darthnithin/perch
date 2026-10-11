// Compare border emission paths in the user's terminal without changing its
// font/profile or connecting to a cluster. --tui adds Bubble Tea's renderer.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func sample() string {
	const w = 24
	peach := lipgloss.NewStyle().Foreground(lipgloss.Color("#E6AD8F"))
	native := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#E6AD8F")).Width(w - 2).Height(6).Render(" Lip Gloss")
	plain := "╭" + strings.Repeat("─", w-2) + "╮\n"
	split := peach.Render("╭"+strings.Repeat("─", w-2)+"╮") + "\n"
	for i := 0; i < 6; i++ {
		plain += "│" + strings.Repeat(" ", w-2) + "│\n"
		split += peach.Render("│") + strings.Repeat(" ", w-2) + peach.Render("│") + "\n"
	}
	plain += "╰" + strings.Repeat("─", w-2) + "╯"
	split += peach.Render("╰" + strings.Repeat("─", w-2) + "╯")
	bg := "\x1b[48;2;28;27;26m"
	painted := bg + strings.ReplaceAll(split, "\x1b[m", "\x1b[m"+bg) + "\x1b[m"
	return "Border comparison · same Unicode characters in every column\n\n" +
		lipgloss.JoinHorizontal(lipgloss.Top, "Plain Unicode\n"+plain, "  ", "Lip Gloss border\n"+native, "  ", "Styled edges\n"+split, "  ", "Edges + background\n"+painted) +
		"\n\nCompare continuity of vertical lines and rounded corners.\nWith --tui: q exits. No terminal profile settings are changed.\n"
}

type model struct{}

func (model) Init() tea.Cmd { return nil }
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && (k.String() == "q" || k.String() == "ctrl+c" || k.String() == "esc") {
		return m, tea.Quit
	}
	return m, nil
}
func (model) View() tea.View { v := tea.NewView(sample()); v.AltScreen = true; return v }
func main() {
	tui := flag.Bool("tui", false, "compare through Bubble Tea's alternate-screen renderer")
	flag.Parse()
	if *tui {
		if _, err := tea.NewProgram(model{}).Run(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	fmt.Print(sample())
}
