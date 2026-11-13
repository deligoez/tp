// Package dashboard is tp dashboard's server: the page, its script and its
// styles embedded in the binary, served over the Go standard library alone
// (spec/1.8.0.md §1).
package dashboard

import (
	"embed"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"strings"
)

// page holds the files the server serves, embedded so the binary needs
// nothing on disk beside the repository it reads.
//
//go:embed page
var page embed.FS

// Host is the only address the dashboard binds.
const Host = "127.0.0.1"

// Handler serves the embedded page, and api's read models under /api/, to a
// request whose Host is the bound address and port, or localhost with that
// port, and whose method is GET. Every other request is refused, so the
// server exposes no write endpoint and a rebound DNS name reaches nothing. No
// response carries a CORS header.
func Handler(port int, api API) http.Handler {
	sub, err := fs.Sub(page, "page")
	if err != nil {
		// The embed directive names page, so the sub-tree always exists.
		panic(err)
	}
	files := http.FileServerFS(sub)
	endpoints := apiHandlers(api)
	live := newHub(func() []Payload { return api.Live() }, Tick)
	portText := strconv.Itoa(port)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowedHost(r.Host, portText) {
			http.Error(w, "forbidden: the Host is not this dashboard's address", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed: the dashboard answers GET only", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path == "/api/events" {
			live.serveEvents(w, r)
			return
		}
		if handle, ok := endpoints[r.URL.Path]; ok {
			serveAPI(w, r, handle)
			return
		}
		if strings.HasPrefix(r.URL.Path, vendorPrefix) {
			serveVendor(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})
}

// acceptsGzip reports whether an Accept-Encoding header names gzip with a
// quality above zero: of its parameters, named in any case, only q counts.
func acceptsGzip(header string) bool {
	for part := range strings.SplitSeq(header, ",") {
		coding, params, _ := strings.Cut(part, ";")
		if !strings.EqualFold(strings.TrimSpace(coding), "gzip") {
			continue
		}
		for param := range strings.SplitSeq(params, ";") {
			name, value, _ := strings.Cut(param, "=")
			if strings.EqualFold(strings.TrimSpace(name), "q") {
				v, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
				return err == nil && v > 0
			}
		}
		return true
	}
	return false
}

// allowedHost reports whether host names the bound address, or localhost,
// with port.
func allowedHost(host, port string) bool {
	name, p, err := net.SplitHostPort(host)
	if err != nil || p != port {
		return false
	}
	return name == Host || strings.EqualFold(name, "localhost")
}
