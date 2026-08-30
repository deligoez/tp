package engine

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

// RunLockBusyError signals that another tp run already holds the run-scoped
// lock for this cycle (§3.1.1). It is the only error WithRunLock returns for
// contention, so the CLI layer maps it to exit 4 (STATE) with a hint naming the
// cycle a driver is already working on.
//
// It is deliberately not a LockTimeoutError. The task-file write lock is held
// for the length of one write, so retrying with backoff until
// lock_timeout_seconds is the right answer there — the holder is about to let
// go. A run lock is held for the length of a whole run, which can be hours, so
// a second driver is told to stand down at once rather than parked in a wait
// that would end in the same refusal, and its hint says to wait for or stop the
// run rather than to raise a timeout that cannot help.
type RunLockBusyError struct {
	LockPath string
	Base     string
}

func (e *RunLockBusyError) Error() string {
	return fmt.Sprintf("another tp run holds %s", e.LockPath)
}

// Hint is the actionable hint surfaced to the agent alongside exit 4.
func (e *RunLockBusyError) Hint() string {
	return fmt.Sprintf("a tp run is already driving %s; wait for it to finish or stop it before starting another run over the same task file", e.Base)
}

// RunLockPath returns the run-scoped lock path for a task file:
// .tp/locks/run-<base>.lock, under the same git-ignored .tp/locks/ directory
// the write locks live in (§3.1.1).
//
// The name is the cycle's <base> verbatim rather than the path hash
// LockFilePath uses, because §3.1.1 fixes the path and because run artifacts
// are keyed by base throughout (§3.5): .tp/run-<base>.json,
// .tp/last_failure-<base>.json and .tp/rounds/<base> all name the same cycle,
// and a driver, a --status reader and a hook must agree on it without hashing.
func RunLockPath(taskFile string) string {
	tpDir := ProjectConfigDir(filepath.Dir(taskFile))
	return filepath.Join(tpDir, "locks", "run-"+RunBase(taskFile)+".lock")
}

// RunLockHeld reports whether some process is holding a cycle's run-scoped
// lock right now. It is §3.5's evidence for `tp run --status`: a run state with
// no stop_reason belongs either to a driver still working or to one that died,
// and the lock is the only thing that separates the two.
//
// Probing a flock means trying to take it, so this does take the lock for the
// instant between TryLock and Unlock. That is not the run lock being held —
// nothing runs inside it, and a --status that failed to take it reports rather
// than refuses — but it does mean a tp run starting in that same instant could
// see contention. The window is one syscall pair wide and the alternative,
// writing a pid file beside the lock, would be the second source of truth §3.5
// exists to avoid.
//
// Every failure reads as "not held": a missing .tp/locks directory is a cycle
// no run has ever locked, which is exactly the absence the caller asks about.
// The lock file itself is never created here — a reporting command that made a
// file would be reporting on its own footprint.
func RunLockHeld(taskFile string) bool {
	lockPath := RunLockPath(taskFile)
	if _, err := os.Stat(lockPath); err != nil {
		return false
	}
	fl := flock.New(lockPath)
	locked, err := fl.TryLock()
	if err != nil {
		return false
	}
	if !locked {
		return true
	}
	_ = fl.Unlock()
	return false
}

// WithRunLock acquires the run-scoped lock for a cycle, runs fn — the whole run
// — then releases. A second tp run over the same task file gets a
// *RunLockBusyError while fn is in flight (§3.1.1).
//
// The lock is distinct from the task-file write lock on purpose: that one stays
// the children's, taken and released by each tp write they make, so a child
// unit writes the task file normally while its driver runs. A driver that held
// the write lock for the run would deadlock the first child it spawned.
//
// Acquisition is a single TryLock, not the retry-with-backoff WithFileLock
// does; see RunLockBusyError for why waiting out a run is not worth doing.
func WithRunLock(taskFile string, fn func() error) error {
	lockPath := RunLockPath(taskFile)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return fmt.Errorf("create lock dir: %w", err)
	}
	// The lock file outlives the lock, so whatever creates the directory has to
	// ignore it too — otherwise one run leaves an untracked .tp/locks/ behind
	// and tp resume reports a change the agent cannot clear by committing. Not
	// fatal: the lock still works, and refusing to take it would be a worse
	// trade, but swallowing it silently reproduced the symptom this call exists
	// to prevent.
	if err := EnsureTPGitignore(ProjectConfigDir(filepath.Dir(taskFile))); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write .tp/.gitignore (%v); .tp/locks/ may show up as an untracked change\n", err)
	}

	fl := flock.New(lockPath)
	locked, err := fl.TryLock()
	if err != nil {
		return fmt.Errorf("acquire run lock: %w", err)
	}
	if !locked {
		return &RunLockBusyError{LockPath: lockPath, Base: RunBase(taskFile)}
	}
	// Unlock only — the lock file STAYS, for the reason WithFileLock records:
	// flock is held on an inode, so unlinking the file lets the next waiter
	// lock a different inode at the same path, and two drivers run the same
	// cycle at once. It is a zero-byte marker under the git-ignored
	// .tp/locks/, so keeping it costs nothing the removal was buying.
	defer func() { _ = fl.Unlock() }()

	return fn()
}
