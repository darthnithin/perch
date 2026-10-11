package ui

import (
	"io"
	"os/exec"
	"perch/internal/transport"
	"time"
)

// Bubble Tea buffers Raw output. Write the reset through Exec's assigned
// terminal writer after renderer suspension, before the child starts.
type pointerExec struct {
	command *exec.Cmd
	reset   string
	output  io.Writer
	limit   time.Duration
}

func (c *pointerExec) SetStdin(r io.Reader) {
	if c.command.Stdin == nil {
		c.command.Stdin = r
	}
}
func (c *pointerExec) SetStdout(w io.Writer) {
	c.output = w
	if c.command.Stdout == nil {
		c.command.Stdout = w
	}
}
func (c *pointerExec) SetStderr(w io.Writer) {
	if c.command.Stderr == nil {
		c.command.Stderr = w
	}
}
func (c *pointerExec) Run() error {
	if c.reset != "" {
		if _, err := io.WriteString(c.output, c.reset); err != nil {
			return err
		}
	}
	if c.limit > 0 {
		return transport.RunForeground(c.command, c.limit)
	}
	return c.command.Run()
}
