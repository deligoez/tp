package cli

import (
	"fmt"
	"os"

	"github.com/deligoez/tp/internal/engine"
)

// unexplainedChangeCount returns the number of uncommitted working-tree changes
// that are neither covered by the keep-list nor tp's own bookkeeping write of
// taskFilePath. Excluding the task file keeps the close's own state update from
// counting as an unexplained change, so the count reflects only what the agent
// left behind (§8.3).
func unexplainedChangeCount(taskFilePath string) int {
	raw := engine.WorktreeChanges(".")
	if len(raw) == 0 {
		return 0
	}
	// Classify against the keep-list; a malformed keep pattern is fail-safe here —
	// every change stays unexplained (over-report) rather than being swallowed.
	changes := raw
	if classified, err := engine.ClassifyPaths(engine.LoadKeepList("."), raw); err == nil {
		changes = classified.Changes
	}
	// Exclude tp's own artifacts for this close: the task file it just wrote and
	// the flock file still held while this runs (both normalize to the same base
	// as git status output). If the path cannot be normalized, fall back to it as
	// given rather than dropping the exclusion.
	skip, err := engine.NormalizeKeepPath(".", taskFilePath)
	if err != nil {
		skip = taskFilePath
	}
	skipLock := skip + ".lock"
	n := 0
	for _, c := range changes {
		if c == skip || c == skipLock {
			continue
		}
		n++
	}
	return n
}

// warnUnexplainedChanges prints the §8.3 one-line stderr warning when count > 0,
// after a successful close. It never changes the exit code, and tp neither
// commits nor discards the change; a keep-listed change is not counted and so
// produces no warning.
func warnUnexplainedChanges(count int) {
	if count > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d uncommitted change(s) not on the keep-list remain after close; commit them or record them with tp keep\n", count)
	}
}
