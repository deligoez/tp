package engine

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/deligoez/tp/internal/output"
)

// A unit lock is one of the three things that tell a live child from a dead
// one across a driver's death (liveUnits).
//
// The driver takes an exclusive flock on the unit's lock file, hands the
// descriptor to the child (ExtraFiles) and closes its own copy without
// unlocking: the lock then lives as long as the child's descriptor does. Every
// process the unit starts inherits it too, so a daemon the unit leaves behind
// keeps it held after the unit's child exited; that is why the lock alone is
// not liveness, and the pid of the unit's own child, recorded on its row, is
// checked beside it. A retry that finds its lock so held starts without one and
// says so. Where ExtraFiles is unsupported (Windows) no lock is taken and a
// restarted run cannot tell a live unit.

// errUnitLockBusy is a unit lock some process already holds: one the unit's
// previous attempt left running with the descriptor still open.
var errUnitLockBusy = errors.New("unit lock held")

// unitLockPath is one unit's lock file, beside the cycle's run lock:
// .tp/locks/run-<key>-unit-<id>.lock.
func unitLockPath(taskFile, id string) string {
	return strings.TrimSuffix(RunLockPath(taskFile), ".lock") + "-unit-" + id + ".lock"
}

// unitLockPoll is how often a restarted run looks again at a live unit.
const unitLockPoll = 100 * time.Millisecond

// awaitLiveUnits waits until no unit the dead run left live (d.orphans) still
// runs, each carried into this run's pool first so outside readers see it
// running. It reports false when the signal cut the wait short: the run then
// touches nothing the live units own, and its checkpoint stops it as
// interrupted.
func (d *driver) awaitLiveUnits() (bool, error) {
	if len(d.orphans) == 0 {
		return true, nil
	}
	names := make([]string, 0, len(d.orphans))
	for _, u := range d.orphans {
		if err := d.rec.JoinOrphan(u.ID, u.PID); err != nil {
			return false, driverErrorf(err, "record task %q in the pool", u.ID)
		}
		names = append(names, fmt.Sprintf("%s (pid %d)", u.ID, u.PID))
	}
	output.Notice(fmt.Sprintf("a stopped run's units still run: %s; waiting for them to exit before recovering", strings.Join(names, ", ")))
	for _, u := range d.orphans {
		poolHook("waiting", u.ID)
		for u.alive(d.opts.TaskFile) {
			if d.sig != nil && d.sig.signalled() {
				return false, nil
			}
			time.Sleep(unitLockPoll)
		}
	}
	return true, nil
}

// leaveOrphans takes the waited-for units out of this run's pool once their
// closes are integrated.
func (d *driver) leaveOrphans() error {
	for _, u := range d.orphans {
		if err := d.leavePool(u.ID); err != nil {
			return err
		}
	}
	d.orphans = nil
	return nil
}
