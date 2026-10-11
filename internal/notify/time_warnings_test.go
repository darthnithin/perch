package notify

import (
	"perch/internal/app"
	"strings"
	"testing"
	"time"
)

func TestTimeWarningUsesRecordedRemainingTime(t *testing.T) {
	e := app.Event{Key: "custom", Kind: "time warning", JobID: "17", Name: "training", WarningThreshold: 15 * time.Minute, Remaining: 12 * time.Minute}
	messages := Messages([]app.Event{e})
	if len(messages) != 1 || !strings.Contains(messages[0].Body, "12m 00s remaining at last refresh") || strings.Contains(messages[0].Body, "5 minutes") {
		t.Fatal(messages)
	}
	e.WarningThreshold = 0
	if m := Messages([]app.Event{e}); strings.Contains(m[0].Body, "5 minutes") || strings.Contains(m[0].Body, "12m") {
		t.Fatal("missing evidence fabricated duration", m)
	}
}
