// Package web embeds the built user interface into the binary.
//
// The UI is built by Vite into dist/app (see vite.config.ts). A binary built
// without it still runs and serves a short notice instead.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// FS returns the built UI rooted at index.html, or nil if it was not built.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist/app")
	if err != nil {
		return nil
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil
	}
	return sub
}
