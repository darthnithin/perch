package domain

import "strings"

// Clean removes terminal commands, C0/C1 controls and bidi overrides from
// remote text. It is deliberately conservative; logs are data, not a terminal.
func Clean(s string) string {
	var out strings.Builder
	r := []rune(s)
	for i := 0; i < len(r); i++ {
		c := r[i]
		if c == 0x1b {
			if i+1 >= len(r) {
				break
			}
			i++
			switch r[i] {
			case '[':
				for i+1 < len(r) {
					i++
					if r[i] >= 0x40 && r[i] <= 0x7e {
						break
					}
				}
			case ']', 'P', '^', '_', 'X':
				for i+1 < len(r) {
					i++
					if r[i] == 7 || r[i] == 0x9c {
						break
					}
					if r[i] == 0x1b && i+1 < len(r) && r[i+1] == '\\' {
						i++
						break
					}
				}
			default: // A single escape sequence is discarded.
			}
			continue
		}
		if c == 0x9b {
			for i+1 < len(r) {
				i++
				if r[i] >= 0x40 && r[i] <= 0x7e {
					break
				}
			}
			continue
		}
		if c == 0x9d || c == 0x90 {
			for i+1 < len(r) {
				i++
				if r[i] == 7 || r[i] == 0x9c {
					break
				}
				if r[i] == 0x1b && i+1 < len(r) && r[i+1] == '\\' {
					i++
					break
				}
			}
			continue
		}
		if c == '\n' || c == '\t' {
			out.WriteRune(c)
			continue
		}
		if c < 32 || c >= 127 && c <= 159 || c >= 0x202a && c <= 0x202e || c >= 0x2066 && c <= 0x2069 {
			continue
		}
		out.WriteRune(c)
	}
	return strings.ToValidUTF8(out.String(), "�")
}
func Label(s string) string { return strings.Join(strings.Fields(Clean(s)), " ") }

type Diagnosis struct {
	Summary, Evidence string
	Confirmed         bool
}

func Diagnose(j Job, log string) Diagnosis {
	switch j.State {
	case "OUT_OF_MEMORY":
		return Diagnosis{"Memory limit exceeded", "Slurm reported OUT_OF_MEMORY", true}
	case "TIMEOUT":
		return Diagnosis{"Time limit reached", "Slurm reported TIMEOUT", true}
	case "NODE_FAIL":
		return Diagnosis{"Compute node failed", "Slurm reported NODE_FAIL", true}
	case "BOOT_FAIL":
		return Diagnosis{"Compute node failed to boot", "Slurm reported BOOT_FAIL", true}
	case "PREEMPTED":
		return Diagnosis{"Job was preempted", "Slurm reported PREEMPTED", true}
	case "DEADLINE":
		return Diagnosis{"Latest allowed start reached", "Slurm reported DEADLINE: insufficient time to finish before the requested deadline", true}
	case "CANCELLED":
		return Diagnosis{"Job was cancelled", "Slurm reported CANCELLED", true}
	}
	if !j.Failed() {
		return Diagnosis{}
	}
	if hint := LogHint(log); hint.Summary != "" {
		return hint
	}
	return Diagnosis{"Job failed", "See scheduler exit status and output", true}
}

// LogHint identifies visible output patterns, never a job outcome or a confirmed
// cause. Callers must separately establish failure before presenting a diagnosis.
func LogHint(log string) Diagnosis {
	lower := strings.ToLower(Clean(log))
	for _, pattern := range []struct{ needle, summary string }{{"cuda out of memory", "Possible GPU memory exhaustion"}, {"no space left on device", "Possible storage exhaustion"}, {"disk quota exceeded", "Possible quota exhaustion"}, {"traceback (most recent call last)", "Python exception found in output"}, {"segmentation fault", "Segmentation fault found in output"}} {
		if strings.Contains(lower, pattern.needle) {
			return Diagnosis{pattern.summary, "Log tail contains: " + pattern.needle, false}
		}
	}
	return Diagnosis{}
}
