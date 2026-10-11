package transport

import (
	"errors"
	"fmt"
	"github.com/charmbracelet/x/term"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

var ErrForegroundInterrupted = errors.New("session interrupted")

// The SSH child owns the foreground TTY. Normal Ctrl-C is carried through its
// raw terminal to the compute-node shell; process termination must still stop
// our child instead of abandoning a shell step in the background.
func RunForeground(command *exec.Cmd, limit time.Duration) (resultErr error) {
	// A killed foreground program may leave raw/no-echo settings behind. Restore
	// the released terminal before Bubble Tea reacquires it; otherwise its next
	// saved state would be that damaged child state, including on final exit.
	if terminal, ok := command.Stdin.(interface{ Fd() uintptr }); ok && term.IsTerminal(terminal.Fd()) {
		fd := terminal.Fd()
		state, err := term.GetState(fd)
		if err != nil {
			return fmt.Errorf("save terminal state: %w", err)
		}
		defer func() {
			if err := term.Restore(fd, state); err != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("restore terminal state: %w", err))
			}
		}()
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	if err := command.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	deadline := time.NewTimer(limit)
	defer deadline.Stop()
	result := ErrForegroundInterrupted
	select {
	case err := <-done:
		return err
	case <-signals:
	case <-deadline.C:
		result = fmt.Errorf("shell time limit reached")
	}
	_ = command.Process.Signal(syscall.SIGTERM)
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		_ = command.Process.Kill()
		<-done
	}
	return result
}
