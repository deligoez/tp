package dashboard

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"
)

// vendorPrefix is the path the vendored scripts are served under. Each is
// stored only gzip-compressed, as page/vendor/<name>.gz, and served as
// <name> alone: the stored .gz, the licence and the provenance note beside
// it are embedded for whoever reads the binary's sources, not served.
const vendorPrefix = "/vendor/"

// vendorCache is how long a browser keeps a vendored script: its name
// holds its version, so a new version is a new name.
const vendorCache = "public, max-age=31536000, immutable"

// vendorScript is one vendored script: the stored bytes and the script
// decompressed, each with its ETag.
type vendorScript struct {
	gz, plain       []byte
	gzTag, plainTag string
}

// vendorScripts reads every vendored script once, keyed by the name it is
// served as.
var vendorScripts = sync.OnceValues(func() (map[string]*vendorScript, error) {
	out := map[string]*vendorScript{}
	names, err := fs.Glob(page, "page/vendor/*.js.gz")
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		gz, err := fs.ReadFile(page, name)
		if err != nil {
			return nil, err
		}
		zr, err := gzip.NewReader(bytes.NewReader(gz))
		if err != nil {
			return nil, err
		}
		plain, err := io.ReadAll(zr)
		if err != nil {
			return nil, err
		}
		out[strings.TrimSuffix(path.Base(name), ".gz")] = &vendorScript{gz: gz, plain: plain, gzTag: etag(gz, "-gz"), plainTag: etag(plain, "")}
	}
	return out, nil
})

// etag is a strong ETag for b: its sha256's first 16 hex digits and suffix.
func etag(b []byte, suffix string) string {
	sum := sha256.Sum256(b)
	return `"` + hex.EncodeToString(sum[:8]) + suffix + `"`
}

// serveVendor answers a path under vendorPrefix: a vendored script, as its
// stored bytes with Content-Encoding gzip to a client that accepts gzip and
// decompressed to any other, cached as immutable and revalidated by its
// ETag; anything else under the prefix is not found.
func serveVendor(w http.ResponseWriter, r *http.Request) {
	scripts, err := vendorScripts()
	if err != nil {
		// The embedded files are checked by TestVendor_DeckGLIsTheUpstreamFileWithItsLicence.
		http.Error(w, "the vendored scripts do not read: "+err.Error(), http.StatusInternalServerError)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, vendorPrefix)
	s, ok := scripts[name]
	if !ok {
		http.NotFound(w, r)
		return
	}
	h := w.Header()
	h.Set("Vary", "Accept-Encoding")
	h.Set("Cache-Control", vendorCache)
	body, tag := s.plain, s.plainTag
	if acceptsGzip(r.Header.Get("Accept-Encoding")) {
		body, tag = s.gz, s.gzTag
		h.Set("Content-Encoding", "gzip")
	}
	h.Set("ETag", tag)
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(body))
}
