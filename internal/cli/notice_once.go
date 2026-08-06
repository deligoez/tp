package cli

import (
	"strings"
	"sync"

	"github.com/deligoez/tp/internal/output"
)

// noticeDetailCap bounds the variable part of one advisory. An underlying tool
// can be arbitrarily loud — git answers a malformed invocation with a full
// usage dump, hundreds of lines long — and since these advisories travel the
// Notice channel, which JSON mode does NOT suppress, an uncapped detail costs
// the agent driving tp more context than the payload it annotates.
const noticeDetailCap = 200

var (
	noticedMu   sync.Mutex
	noticedKeys = map[string]bool{}
)

// noticeOnce routes an advisory to output.Notice, suppressing exact repeats of
// key within one process. Several audit helpers probe the same condition more
// than once — one git invocation per diff range, one task-file read per caller
// — and one condition should cost the reader one line, not one per attempt.
func noticeOnce(key, msg string) {
	noticedMu.Lock()
	seen := noticedKeys[key]
	noticedKeys[key] = true
	noticedMu.Unlock()
	if seen {
		return
	}
	output.Notice(msg)
}

// firstLineCapped reduces an unbounded, possibly multi-line detail to the one
// bounded line an advisory can afford.
func firstLineCapped(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	if len(s) > noticeDetailCap {
		s = s[:noticeDetailCap] + "..."
	}
	return s
}
