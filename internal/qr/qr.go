// Package qr renders QR codes as SVG for the web UI and as text for the
// terminal.
package qr

import (
	"fmt"
	"strings"

	"rsc.io/qr"
)

// quietZone is the blank border, in modules, scanners need around a code.
const quietZone = 4

// SVG renders text as a QR code with dark modules on a light background,
// which every scanner reads reliably.
func SVG(text, dark, light string) (string, error) {
	code, err := qr.Encode(text, qr.M)
	if err != nil {
		return "", fmt.Errorf("qr: %w", err)
	}
	size := code.Size + 2*quietZone
	var path strings.Builder
	for y := range code.Size {
		for x := 0; x < code.Size; x++ {
			if !code.Black(x, y) {
				continue
			}
			start := x
			for x+1 < code.Size && code.Black(x+1, y) {
				x++
			}
			fmt.Fprintf(&path, "M%d %dh%dv1h-%dz", start+quietZone, y+quietZone, x-start+1, x-start+1)
		}
	}
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges">`+
		`<rect width="%d" height="%d" fill="%s"/><path fill="%s" d="%s"/></svg>`,
		size, size, size, size, light, dark, path.String()), nil
}

// Terminal renders text as a QR code using half-block characters, two rows of
// modules per line. It assumes a dark terminal background: light modules are
// printed as filled blocks.
func Terminal(text string) (string, error) {
	code, err := qr.Encode(text, qr.L)
	if err != nil {
		return "", fmt.Errorf("qr: %w", err)
	}
	light := func(x, y int) bool {
		x -= quietZone / 2
		y -= quietZone / 2
		if x < 0 || y < 0 || x >= code.Size || y >= code.Size {
			return true
		}
		return !code.Black(x, y)
	}
	size := code.Size + quietZone
	var b strings.Builder
	for y := 0; y < size; y += 2 {
		b.WriteString("  ")
		for x := range size {
			top, bottom := light(x, y), y+1 < size && light(x, y+1)
			if y+1 >= size {
				bottom = false
			}
			switch {
			case top && bottom:
				b.WriteRune('█')
			case top:
				b.WriteRune('▀')
			case bottom:
				b.WriteRune('▄')
			default:
				b.WriteRune(' ')
			}
		}
		b.WriteByte('\n')
	}
	return b.String(), nil
}
