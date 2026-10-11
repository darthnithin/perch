package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"perch/internal/preferences"
	"perch/internal/ui"
)

func chooseDestination(options ui.Options) (string, error) {
	if options.Host != "" {
		var v preferences.Values
		if err := v.Set("host", options.Host); err != nil {
			return "", err
		}
	}
	m := ui.NewSetup(options)
	defer m.Close()
	defer func() {
		if reset := m.PointerReset(); reset != "" {
			fmt.Fprint(os.Stdout, reset)
		}
	}()
	_, err := tea.NewProgram(m).Run()
	if err != nil {
		return "", err
	}
	return m.Host, nil
}
