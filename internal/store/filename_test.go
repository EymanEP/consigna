package store

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func TestSanitizeName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"report.pdf", "report.pdf"},
		{"../../etc/passwd", "passwd"},
		{`C:\Users\me\photo.jpg`, "photo.jpg"},
		{"..", "file"},
		{".", "file"},
		{"", "file"},
		{"   ", "file"},
		{".hidden", "hidden"},
		{"name.", "name"},
		{"a<b>c:d\"e|f?g*h.txt", "a_b_c_d_e_f_g_h.txt"},
		{"tab\there.txt", "tab here.txt"},
		{"new\nline.txt", "new line.txt"},
		{"nul\x00byte.txt", "nulbyte.txt"},
		{"evil\u202Egnp.exe", "evilgnp.exe"},
		{"zero\u200Bwidth.txt", "zerowidth.txt"},
		{"CON.txt", "_CON.txt"},
		{"con", "_con"},
		{"Screen Recording 2026-09-20 at 14-30.mp4", "Screen Recording 2026-09-20 at 14-30.mp4"},
		{"café ☕.png", "café ☕.png"},
		{"bad\xffutf8.txt", "bad\uFFFDutf8.txt"},
	}
	for _, c := range cases {
		if got := SanitizeName(c.in); got != c.want {
			t.Errorf("SanitizeName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSanitizeNameTruncatesKeepingExtension(t *testing.T) {
	long := strings.Repeat("é", 300) + ".tar.gz"
	got := SanitizeName(long)
	if len(got) > MaxNameBytes {
		t.Fatalf("len = %d, want <= %d", len(got), MaxNameBytes)
	}
	if !strings.HasSuffix(got, ".gz") {
		t.Fatalf("extension lost: %q", got)
	}
	if !utf8.ValidString(got) {
		t.Fatalf("invalid UTF-8: %q", got)
	}
}

func FuzzSanitizeName(f *testing.F) {
	for _, s := range []string{"a.txt", "../x", "\u202E", "CON", strings.Repeat("x", 400), "\x00"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		out := SanitizeName(in)
		if out == "" || out == "." || out == ".." {
			t.Fatalf("unsafe result %q", out)
		}
		if len(out) > MaxNameBytes {
			t.Fatalf("too long: %d", len(out))
		}
		if !utf8.ValidString(out) {
			t.Fatalf("invalid UTF-8 %q", out)
		}
		if strings.ContainsAny(out, `/\<>:"|?*`) {
			t.Fatalf("forbidden character in %q", out)
		}
		for _, r := range out {
			if unicode.IsControl(r) || invisible[r] {
				t.Fatalf("control/invisible rune %U in %q", r, out)
			}
		}
		if SanitizeName(out) != out {
			t.Fatalf("not idempotent: %q -> %q", out, SanitizeName(out))
		}
	})
}
