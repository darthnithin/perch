package nautilus

import (
	"strconv"
	"strings"
)

func validIndex(s string) bool {
	_, err := indexNumber(s)
	return err == nil
}

func indexNumber(s string) (int64, error) {
	// ParseUint excludes signs; explicit digit validation also excludes leading
	// plus and whitespace. API indices are decimal nonnegative int32 values.
	if s == "" || strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return 0, strconv.ErrSyntax
	}
	return strconv.ParseInt(s, 10, 32)
}

func validIndexes(s string, completions *int64) bool {
	if s == "" {
		return true
	}
	previous := int64(-1)
	for _, part := range strings.Split(s, ",") {
		a, b, ranged := strings.Cut(part, "-")
		start, err := indexNumber(a)
		if err != nil || start <= previous {
			return false
		}
		end := start
		if ranged {
			end, err = indexNumber(b)
		}
		if err != nil || end < start || completions != nil && end >= *completions {
			return false
		}
		previous = end
	}
	return true
}
