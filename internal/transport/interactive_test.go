package transport

import (
	"strings"
	"testing"
)

func TestInteractiveSSHForcesTTYWithoutOverridingFirstOptions(t *testing.T) {
	s, err := NewSSH("test.invalid")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	cmd, err := s.InteractiveCommand([]string{"printf", "a'b; literal"})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(cmd.Args, " ")
	for _, expected := range []string{"-tt", "RequestTTY=force", "BatchMode=yes", "ClearAllForwardings=yes", QuoteArgs([]string{"printf", "a'b; literal"})} {
		if !strings.Contains(joined, expected) {
			t.Fatal(cmd.Args)
		}
	}
	if strings.Contains(joined, "RequestTTY=no") || strings.Contains(joined, " -T ") {
		t.Fatal(cmd.Args)
	}
	query := strings.Join(s.options(false), " ")
	if !strings.Contains(query, "RequestTTY=no") || !strings.HasPrefix(query, "-T ") {
		t.Fatal("interactive changed query options", query)
	}
	if _, err = s.InteractiveCommand([]string{"x", "bad\x00"}); err == nil {
		t.Fatal("accepted NUL")
	}
}
