package launch

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type Array struct {
	Expression string `json:"expression"`
	Tasks      int64  `json:"tasks"`
	Concurrent int64  `json:"maximum_concurrent_tasks"`
	Throttled  bool   `json:"throttled"`
}

var throttlePattern = regexp.MustCompile(`^[1-9][0-9]*$`)

var arrayPart = regexp.MustCompile(`^([0-9]+)(?:-([0-9]+)(?::([0-9]+))?)?$`)

// ParseArray deliberately bounds expansion. Actual cluster limits are checked
// by Slurm; neither a valid expression nor its throttle guarantees scheduling.
func ParseArray(value string) (Array, error) {
	if value == "" {
		return Array{Tasks: 1, Concurrent: 1}, nil
	}
	var out Array
	if len(value) > 4096 {
		return out, fmt.Errorf("array expression is too long")
	}
	indices, throttle, limited := strings.Cut(value, "%")
	limit := int64(0)
	if limited {
		if !throttlePattern.MatchString(throttle) {
			return out, fmt.Errorf("array concurrency must be a positive integer, for example 0-99%%4")
		}
		var err error
		limit, err = strconv.ParseInt(throttle, 10, 64)
		if err != nil || limit > 100000 {
			return out, fmt.Errorf("array concurrency must be between 1 and 100000")
		}
	}
	seen := map[int64]bool{}
	var parts []string
	for _, part := range strings.Split(indices, ",") {
		m := arrayPart.FindStringSubmatch(part)
		if m == nil {
			return out, fmt.Errorf("array tasks must be indices/ranges such as 0-99%%4 or 1,3,5-15:2")
		}
		lo, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil || lo > 4294967293 {
			return out, fmt.Errorf("array index is out of range")
		}
		hi, step := lo, int64(1)
		if m[2] != "" {
			hi, err = strconv.ParseInt(m[2], 10, 64)
			if err != nil || hi < lo || hi > 4294967293 {
				return out, fmt.Errorf("array range must have ascending valid indices")
			}
		}
		if m[3] != "" {
			step, err = strconv.ParseInt(m[3], 10, 64)
			if err != nil || step < 1 {
				return out, fmt.Errorf("array step must be positive")
			}
		}
		n := (hi-lo)/step + 1
		if n+int64(len(seen)) > 100000 {
			return out, fmt.Errorf("Perch supports at most 100000 tasks per array; cluster limits may be lower")
		}
		for i := lo; i <= hi; i += step {
			if seen[i] {
				return out, fmt.Errorf("array index %d appears more than once", i)
			}
			seen[i] = true
			if step > hi-i {
				break
			}
		}
		normalized := strconv.FormatInt(lo, 10)
		if hi != lo {
			normalized += "-" + strconv.FormatInt(hi, 10)
			if step != 1 {
				normalized += ":" + strconv.FormatInt(step, 10)
			}
		}
		parts = append(parts, normalized)
	}
	out.Tasks = int64(len(seen))
	out.Concurrent = out.Tasks
	out.Expression = strings.Join(parts, ",")
	out.Throttled = limited
	if limited {
		out.Concurrent = min(limit, out.Tasks)
		out.Expression += "%" + strconv.FormatInt(limit, 10)
	}
	return out, nil
}

var dependencyID = regexp.MustCompile(`^[1-9][0-9]{0,18}(?:_[0-9]{1,10})?$`)

func Dependencies(value string) ([]string, error) {
	if value == "" {
		return nil, nil
	}
	values := strings.Split(value, ",")
	if len(values) > 32 {
		return nil, fmt.Errorf("at most 32 successful-job dependencies are supported")
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, id := range values {
		if !dependencyID.MatchString(id) {
			return nil, fmt.Errorf("dependencies must be comma-separated job IDs or array task IDs, for example 123,456_2")
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out, nil
}
