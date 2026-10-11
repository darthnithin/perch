package ui

func (m *Model) logHeading(width int) string {
	kind := "Standard output"
	if m.logErr {
		kind = "Standard error"
	}
	state := muted.Render("waiting")
	switch {
	case m.cacheBlocked:
		state = yellow.Render("saved · c reconnect")
	case !m.connected && m.logLoaded:
		state = yellow.Render("stale · c reconnect")
	case m.detailError != "" && m.logLoaded:
		state = yellow.Render("stale · r retry")
	case m.detailError != "":
		state = yellow.Render("read failed · r retry")
	case !m.logLoaded && m.detailBusy:
		state = muted.Render("reading…")
	case m.logLoaded && !m.logFollowing:
		state = yellow.Render("paused · End latest")
	case m.logLoaded:
		state = green.Render("following")
	}
	return joinLR(accent.Render(kind), state, width)
}

// Give short log, script, inspector and priority views four more rows while no input is
// being edited. Rendering and pointer geometry use the same decision.
func (m *Model) compactTextReading() bool {
	reading := m.view == "logs" || m.view == "script" || m.view == "inspect" || m.view == "software" && m.softwareRequest.Command == "priority"
	return reading && m.height <= 24 && !m.input.Focused() && !m.readerSearchEditing() && !m.help && !m.palette
}
