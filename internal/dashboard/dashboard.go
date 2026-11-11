// Package dashboard is tp dashboard's server: the page, its script and its
// styles embedded in the binary, served over the Go standard library alone
// (spec/1.8.0.md §1).
package dashboard

import (
	"embed"
	"io/fs"
	"net/http"
)

// page holds the files the server serves, embedded so the binary needs
// nothing on disk beside the repository it reads.
//
//go:embed page
var page embed.FS

// Handler serves the embedded page at the root path.
func Handler() http.Handler {
	sub, err := fs.Sub(page, "page")
	if err != nil {
		// The embed directive names page, so the sub-tree always exists.
		panic(err)
	}
	return http.FileServerFS(sub)
}
