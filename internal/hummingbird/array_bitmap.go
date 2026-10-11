package hummingbird

import (
	"fmt"
	"strconv"
	"strings"
)

// Accounting exposes array.task as a hexadecimal bitmap in v0.0.44.
// Bit zero is the low bit of the rightmost digit (Slurm bit_unfmt_hexmask).
// Normalize at the adapter boundary so domain and presentation use task ranges.
func accountingTaskRange(value string) (string, error) {
	if !strings.HasPrefix(value, "0x") {
		return value, nil
	}
	digits := value[2:]
	if len(digits) == 0 || len(digits) > 1<<20 {
		return "", fmt.Errorf("invalid or oversized accounting array bitmap")
	}
	var result strings.Builder
	first, last := -1, -1
	flush := func() {
		if first < 0 {
			return
		}
		if result.Len() > 0 {
			result.WriteByte(',')
		}
		result.WriteString(strconv.Itoa(first))
		if first != last {
			result.WriteByte('-')
			result.WriteString(strconv.Itoa(last))
		}
		first = -1
	}
	for i := len(digits) - 1; i >= 0; i-- {
		c := digits[i]
		var n byte
		switch {
		case c >= '0' && c <= '9':
			n = c - '0'
		case c >= 'a' && c <= 'f':
			n = c - 'a' + 10
		case c >= 'A' && c <= 'F':
			n = c - 'A' + 10
		default:
			return "", fmt.Errorf("invalid accounting array bitmap digit")
		}
		for bit := 0; bit < 4; bit++ {
			index := (len(digits)-1-i)*4 + bit
			if n&(1<<bit) != 0 {
				if first < 0 {
					first = index
				}
				last = index
			} else {
				flush()
			}
		}
		if result.Len() > 1<<20 {
			return "", fmt.Errorf("accounting array bitmap is too fragmented")
		}
	}
	flush()
	if result.Len() > 1<<20 {
		return "", fmt.Errorf("accounting array bitmap is too fragmented")
	}
	return result.String(), nil
}
