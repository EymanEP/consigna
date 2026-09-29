package sizes

import (
	"errors"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"0", 0},
		{"1024", 1024},
		{"10GB", 10 * GB},
		{"10 gb", 10 * GB},
		{"1.5GB", 1_500_000_000},
		{"500MB", 500 * MB},
		{"4kb", 4 * KB},
		{"2TB", 2 * TB},
		{"12B", 12},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if err != nil {
			t.Fatalf("Parse(%q) error: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("Parse(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	for _, in := range []string{"", "GB", "-1GB", "ten", "NaN", "Inf", "1e30TB"} {
		if _, err := Parse(in); !errors.Is(err, ErrInvalid) {
			t.Errorf("Parse(%q) error = %v, want ErrInvalid", in, err)
		}
	}
}

func TestFormat(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{999, "999 B"},
		{1000, "1 KB"},
		{310 * KB, "310 KB"},
		{1_400_000, "1.4 MB"},
		{842 * MB, "842 MB"},
		{1_200_000_000, "1.2 GB"},
		{10 * GB, "10 GB"},
	}
	for _, c := range cases {
		if got := Format(c.in); got != c.want {
			t.Errorf("Format(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
