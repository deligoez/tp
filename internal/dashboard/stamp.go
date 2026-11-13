package dashboard

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"net/http"
	"regexp"
	"strings"
	"sync"
)

// stampOf is a hash of every file in fsys, its path and its bytes, cut to
// 12 hex digits: the stamp the page's own files are asked for with, so
// files of two tp versions never share an address.
func stampOf(fsys fs.FS) string {
	h := sha256.New()
	_ = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		h.Write([]byte(p + "\x00"))
		h.Write(b)
		return nil
	})
	return hex.EncodeToString(h.Sum(nil))[:12]
}

// pageStamp is the embedded page's stamp, hashed once.
var pageStamp = sync.OnceValue(func() string { return stampOf(page) })

// pageRefs are the page's references to its own scripts and styles.
var pageRefs = regexp.MustCompile(`((?:src|href)="[a-z]+\.(?:js|css))"`)

// stampedIndex is index.html with each script and style it loads asked for
// with ?v=stamp and the stamp on <html data-v>, where app.js reads it to
// ask for the words the same way.
var stampedIndex = sync.OnceValues(func() ([]byte, error) {
	b, err := fs.ReadFile(page, "page/index.html")
	if err != nil {
		return nil, err
	}
	stamp := pageStamp()
	out := pageRefs.ReplaceAllString(string(b), `$1?v=`+stamp+`"`)
	out = strings.Replace(out, `<html lang="en">`, `<html lang="en" data-v="`+stamp+`">`, 1)
	return []byte(out), nil
})

// serveIndex answers the page itself, never cached, so a reload after an
// upgrade always reads the new stamp.
func serveIndex(w http.ResponseWriter) {
	b, err := stampedIndex()
	if err != nil {
		http.Error(w, "the page does not read: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(b)
}

// pageCache is the Cache-Control of a page file asked for at r: immutable
// under the current stamp, revalidated otherwise.
func pageCache(r *http.Request) string {
	if r.URL.Query().Get("v") == pageStamp() {
		return vendorCache
	}
	return "no-cache"
}
