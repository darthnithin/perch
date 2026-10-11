package ui

import "strings"

type emptyAction struct {
	key, label string
	x, y       int
}

// A filtered list or an unobserved queue is not an empty workspace.
func (m *Model) emptyWorkspace() bool {
	return m.tab == 0 && m.view == "details" && !m.help && !m.palette &&
		m.search == "" && !m.searching && !m.visibleStore().Snapshot.At.IsZero() && len(m.jobs()) == 0
}

// Shared row/column positions keep pointer targets attached to visible labels.
func (m *Model) emptyQueueContent(height int) (string, []emptyAction) {
	title := "No active jobs."
	subtitle := "A quiet moment. Plan what comes next."
	if !m.connected || m.lastError != "" {
		title = "No jobs at last refresh."
		subtitle = "Refresh to check the current queue."
		if !m.connected {
			subtitle = "Reconnect to check the current queue."
		}
	}
	if height < 10 {
		actions := []emptyAction{{"n", "New job", 0, 2}, {"2", "History", 12, 2}, {"3", "Partitions", 24, 2}}
		return ink.Bold(true).Render(title) + "\n\n" + emptyActionText(actions[0]) + "   " + emptyActionText(actions[1]) + "   " + emptyActionText(actions[2]), actions
	}
	actions := []emptyAction{{"n", "Prepare a job", 0, 4}, {"2", "Recent history", 0, 7}, {"3", "Cluster partitions", 0, 8}}
	lines := []string{"", ink.Bold(true).Render(title), muted.Render(subtitle), "",
		emptyActionText(actions[0]), muted.Render("Review the command and resources first."), "",
		emptyActionText(actions[1]), emptyActionText(actions[2])}
	return strings.Join(lines, "\n"), actions
}

func emptyActionText(a emptyAction) string {
	return accent.Render(a.key) + " " + ink.Render(a.label)
}
