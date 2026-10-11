package transport

import "os/exec"

// InteractiveCommand is an explicit foreground action. Callers must supply a
// terminal and restore their renderer around Run. Query deadlines/output capture
// deliberately do not apply to a human-operated shell.
func (s *SSH) InteractiveCommand(args []string) (*exec.Cmd, error) {
	if err := validateArgs(args); err != nil {
		return nil, err
	}
	// OpenSSH uses the first value for each -o setting. Replace RequestTTY=no
	// rather than appending a conflicting override to the query options.
	options := s.options(false)
	options[0] = "-tt"
	for i, option := range options {
		if option == "RequestTTY=no" {
			options[i] = "RequestTTY=force"
		}
	}
	return exec.Command("ssh", append(options, "--", s.Host, QuoteArgs(args))...), nil
}

func (Local) InteractiveCommand(args []string) (*exec.Cmd, error) {
	if err := validateArgs(args); err != nil {
		return nil, err
	}
	return exec.Command(args[0], args[1:]...), nil
}
