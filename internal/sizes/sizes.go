// Package sizes parses and formats byte sizes using decimal (SI) units,
// matching how macOS, iOS and most file managers report sizes.
package sizes

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Decimal byte units.
const (
	KB int64 = 1000
	MB       = 1000 * KB
	GB       = 1000 * MB
	TB       = 1000 * GB
)

var units = []struct {
	suffix string
	factor int64
}{
	{"TB", TB}, {"GB", GB}, {"MB", MB}, {"KB", KB}, {"B", 1},
}

// ErrInvalid is returned when a size string cannot be parsed.
var ErrInvalid = errors.New("invalid size")

// Parse converts strings such as "10GB", "1.5 GB", "500mb" or "1024" into a
// number of bytes. A bare number is interpreted as bytes.
func Parse(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return 0, fmt.Errorf("%w: empty", ErrInvalid)
	}
	factor := int64(1)
	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			factor = u.factor
			s = strings.TrimSpace(strings.TrimSuffix(s, u.suffix))
			break
		}
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return 0, fmt.Errorf("%w: %q", ErrInvalid, s)
	}
	bytes := v * float64(factor)
	if bytes > math.MaxInt64/2 {
		return 0, fmt.Errorf("%w: too large", ErrInvalid)
	}
	return int64(math.Round(bytes)), nil
}

// Format renders a byte count for humans, e.g. 1_200_000_000 -> "1.2 GB".
func Format(n int64) string {
	if n < KB {
		return fmt.Sprintf("%d B", n)
	}
	for _, u := range units {
		if n >= u.factor {
			v := float64(n) / float64(u.factor)
			if v >= 100 {
				return fmt.Sprintf("%.0f %s", v, u.suffix)
			}
			return strings.TrimSuffix(strings.TrimSuffix(fmt.Sprintf("%.1f", v), "0"), ".") + " " + u.suffix
		}
	}
	return fmt.Sprintf("%d B", n)
}
