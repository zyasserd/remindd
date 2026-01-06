package duration

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Parse parses Go durations plus integer day/week suffixes:
// - Go: 30m, 1h, 2h45m
// - Days: 7d (7*24h)
// - Weeks: 3w (3*7d)
func Parse(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty duration")
	}

	// day/week suffixes
	if strings.HasSuffix(s, "d") || strings.HasSuffix(s, "w") {
		unit := s[len(s)-1:]
		nStr := s[:len(s)-1]
		if nStr == "" {
			return 0, fmt.Errorf("missing number before %q", unit)
		}
		n, err := strconv.ParseInt(nStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid %s value: %w", unit, err)
		}
		if n <= 0 {
			return 0, errors.New("duration must be > 0")
		}
		d := time.Duration(n) * 24 * time.Hour
		if unit == "w" {
			d *= 7
		}
		return d, nil
	}

	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	if d <= 0 {
		return 0, errors.New("duration must be > 0")
	}
	return d, nil
}
