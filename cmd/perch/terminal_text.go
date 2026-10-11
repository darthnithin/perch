package main

import (
	"io"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
)

func writeTerminalText(out io.Writer, text string) error {
	if terminal, ok := out.(interface{ Fd() uintptr }); ok && term.IsTerminal(terminal.Fd()) {
		if width, _, err := term.GetSize(terminal.Fd()); err == nil && width > 0 {
			text = ansi.Wrap(text, width, "")
		}
	}
	_, err := io.WriteString(out, text)
	return err
}
