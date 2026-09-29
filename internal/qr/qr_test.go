package qr

import (
	"strings"
	"testing"
)

func TestSVG(t *testing.T) {
	svg, err := SVG("http://192.168.1.47:7431/t/9fq2xk", "#04070d", "#e8f7ff")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<svg", "viewBox", `fill="#04070d"`, `fill="#e8f7ff"`, "<path"} {
		if !strings.Contains(svg, want) {
			t.Fatalf("svg missing %q", want)
		}
	}
}

func TestTerminal(t *testing.T) {
	out, err := Terminal("http://192.168.1.47:7431/t/9fq2xk")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 10 {
		t.Fatalf("too few lines: %d", len(lines))
	}
	width := len([]rune(lines[0]))
	for _, l := range lines {
		if len([]rune(l)) != width {
			t.Fatal("ragged output")
		}
	}
}
