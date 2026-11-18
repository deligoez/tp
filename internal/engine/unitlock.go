package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/deligoez/tp/internal/output"
)

// A unit lock tells a live child from a dead one across a driver's death.
//
// The driver takes an exclusive flock on the unit's lock file, hands the
// descriptor to the child (ExtraFiles) and closes its own copy without
// unlocking: the lock then lives exactly as long as the child's descriptor
// does. A run that starts after its predecessor died finds the lock still held
// while that child runs, and waits for it rather than removing a worktree a
// process is writing in or handing its task out a second time.
//
// The limit is the descriptor's: a grandchild the unit leaves running with it
// open holds the lock, and the wait, until it exits too. Where ExtraFiles is
// unsupported (Windows) no lock is taken and a restarted run cannot tell.

// unitLockPrefix is the shared stem of a cycle's unit lock files, beside its
// run lock: .tp/locks/run-<key>-unit-.
func unitLockPrefix(taskFile string) string {
	return strings.TrimSuffix(RunLockPath(taskFile), ".lock") + "-unit-"
}

// unitLockPath is one unit's lock file.
func unitLockPath(taskFile, id string) string {
	return unitLockPrefix(taskFile) + id + ".lock"
}

// heldUnitLocks lists the ids whose unit lock some process holds, sorted.
func heldUnitLocks(taskFile string) []string {
	prefix := unitLockPrefix(taskFile)
	entries, err := os.ReadDir(filepath.Dir(prefix))
	if err != nil {
		return nil
	}
	stem := filepath.Base(prefix)
	var held []string
	for _, e := range entries {
		id, ok := strings.CutPrefix(e.Name(), stem)
		if !ok || !strings.HasSuffix(id, ".lock") {
			continue
		}
		id = strings.TrimSuffix(id, ".lock")
		if unitLockHeld(unitLockPath(taskFile, id)) {
			held = append(held, id)
		}
	}
	slices.Sort(held)
	return held
}

// unitLockPoll is how often a restarted run looks again at a live unit.
const unitLockPoll = 100 * time.Millisecond

// awaitLiveUnits waits until no unit of a dead run still holds its lock. It
// reports false when the signal cut the wait short: the run then touches
// nothing the live units own, and its checkpoint stops it as interrupted.
func (d *driver) awaitLiveUnits() bool {
	held := heldUnitLocks(d.opts.TaskFile)
	if len(held) > 0 {
		output.Notice(fmt.Sprintf("a stopped run's units still run (%s); waiting for them to exit before recovering", strings.Join(held, ", ")))
	}
	for len(held) > 0 {
		if d.sig != nil && d.sig.signalled() {
			return false
		}
		time.Sleep(unitLockPoll)
		held = heldUnitLocks(d.opts.TaskFile)
	}
	return true
}
