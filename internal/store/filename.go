package store

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxNameBytes is the longest file name kept, matching common filesystems.
const MaxNameBytes = 255

// fallbackName is used when nothing usable is left after sanitising.
const fallbackName = "file"

// invisible lists runes that can disguise a name (bidirectional overrides,
// zero-width marks, byte order marks) and are always removed.
var invisible = map[rune]bool{
	'\u200B': true, '\u200C': true, '\u200E': true, '\u200F': true,
	'\u202A': true, '\u202B': true, '\u202C': true, '\u202D': true, '\u202E': true,
	'\u2066': true, '\u2067': true, '\u2068': true, '\u2069': true,
	'\u061C': true, '\uFEFF': true,
}

// windowsReserved are device names Windows refuses as file names.
var windowsReserved = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
	"COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
	"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// SanitizeName turns an untrusted, client-supplied file name into one that is
// safe to show, to send in a download header and to extract from a ZIP on any
// operating system. Names are never used as paths on the server's disk.
func SanitizeName(name string) string {
	name = strings.ToValidUTF8(name, "\uFFFD")
	// Keep only the last path segment, whichever separator was used.
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		switch {
		case invisible[r]:
			continue
		case unicode.IsSpace(r):
			b.WriteRune(' ')
		case unicode.IsControl(r):
			continue
		case strings.ContainsRune(`<>:"|?*`, r):
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	name = strings.Trim(b.String(), " .")
	if name == "" {
		return fallbackName
	}
	stem := name
	if i := strings.IndexByte(stem, '.'); i >= 0 {
		stem = stem[:i]
	}
	if windowsReserved[strings.ToUpper(strings.TrimSpace(stem))] {
		name = "_" + name
	}
	return truncateName(name, MaxNameBytes)
}

// truncateName shortens name to at most max bytes, keeping the extension
// when it is reasonably short and never splitting a UTF-8 sequence.
func truncateName(name string, max int) string {
	if len(name) <= max {
		return name
	}
	ext := ""
	if i := strings.LastIndexByte(name, '.'); i > 0 && len(name)-i <= 16 {
		ext = name[i:]
		name = name[:i]
	}
	limit := max - len(ext)
	cut := 0
	for i := range name {
		if i > limit {
			break
		}
		cut = i
	}
	if len(name) <= limit {
		cut = len(name)
	}
	for cut > 0 && !utf8.RuneStart(name[cut]) {
		cut--
	}
	out := strings.TrimRight(name[:cut], " .") + ext
	if out == ext {
		return truncateName(fallbackName+ext, max)
	}
	return out
}
