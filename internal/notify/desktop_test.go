package notify

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestMacBridgeSyntaxWithoutSendingNotification(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS AppleScript compiler")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "/usr/bin/osacompile", "-o", filepath.Join(t.TempDir(), "notification.scpt"), "-")
	command.Stdin = strings.NewReader(desktopScript)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile only: %v: %s", err, output)
	}
	// The same argv transport with a harmless echo handler proves the messages
	// remain literal arguments. The actual notification handler is never run.
	message := Message{Title: "Perch", Body: `\" & do shell script \"not-an-instruction\" & \"`}
	echo := desktopCommand(ctx, message)
	echo.Args[2] = "on run argv\nreturn item 2 of argv\nend run"
	output, err := echo.Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSuffix(string(output), "\n") != message.Body {
		t.Fatalf("changed message: %q", output)
	}
}
