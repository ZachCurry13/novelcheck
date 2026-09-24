// Package web embeds the NovelCheck front-end (HTML, Tailwind CSS, vanilla
// JS, PWA manifest and service worker) into the Go binary.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:static
var files embed.FS

// FS returns the static site rooted at web/static.
func FS() fs.FS {
	sub, err := fs.Sub(files, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
