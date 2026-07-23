package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

// Backoff schedule for write-lock acquisition retries: starts at lockBackoffStep
// and doubles each attempt up to lockBackoffMax (§12.1).
const (
	lockBackoffStep = 10 * time.Millisecond
	lockBackoffMax  = 50 * time.Millisecond
)

// LockTimeoutError signals that write-lock acquisition retried with backoff
// until lock_timeout_seconds elapsed without succeeding (§12.2). It is the only
// error WithFileLock returns for contention, so the CLI layer can map it to
// exit 4 (STATE) with a hint naming the lock path and the elapsed wait.
type LockTimeoutError struct {
	LockPath string
	Elapsed  time.Duration
}

func (e *LockTimeoutError) Error() string {
	return fmt.Sprintf("timed out waiting for lock %s after %s", e.LockPath, e.Elapsed.Round(time.Millisecond))
}

// Hint is the actionable hint surfaced to the agent alongside exit 4.
func (e *LockTimeoutError) Hint() string {
	return fmt.Sprintf("another tp process holds %s; wait for it to finish, or raise lock_timeout_seconds (range 1-60)", e.LockPath)
}

// WithFileLock acquires an exclusive flock for the given target path, runs fn,
// then releases. The lock file lives under .tp/locks/ (git-ignored) rather than
// beside the target, so a transient or stale lock never litters the working
// tree (§5.3). A stale sibling lock left by an earlier tp version (<path>.lock)
// is removed on acquisition.
//
// Acquisition retries with backoff until the effective lock_timeout_seconds
// elapses, so two concurrent writes both succeed within the timeout (§12.1).
// The timeout resolves at read time like the other workflow fields: task-file
// override > project config (.tp/config.json) > built-in default (5s).
func WithFileLock(path string, fn func() error) error {
	return WithFileLockTimeout(path, effectiveLockTimeoutSeconds(path), fn)
}

// WithFileLockTimeout is the retry-with-backoff core: it loops TryLock with a
// short, capped backoff until timeoutSeconds elapse, then returns a
// *LockTimeoutError. Callers that already hold the resolved timeout (or want a
// fixed one, as in tests) bypass the config-resolve path.
func WithFileLockTimeout(path string, timeoutSeconds int, fn func() error) error {
	lockPath := LockFilePath(path)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return fmt.Errorf("create lock dir: %w", err)
	}

	if sibling := path + ".lock"; sibling != lockPath {
		if _, err := os.Stat(sibling); err == nil {
			_ = os.Remove(sibling)
		}
	}

	fl := flock.New(lockPath)

	start := time.Now()
	timeout := time.Duration(timeoutSeconds) * time.Second
	backoff := lockBackoffStep
	for {
		locked, err := fl.TryLock()
		if err != nil {
			return fmt.Errorf("acquire lock: %w", err)
		}
		if locked {
			break
		}
		if elapsed := time.Since(start); elapsed >= timeout {
			return &LockTimeoutError{LockPath: lockPath, Elapsed: elapsed}
		}
		time.Sleep(backoff)
		if backoff < lockBackoffMax {
			backoff *= 2
			if backoff > lockBackoffMax {
				backoff = lockBackoffMax
			}
		}
	}
	defer func() {
		_ = fl.Unlock()
		_ = os.Remove(lockPath)
	}()

	return fn()
}

// effectiveLockTimeoutSeconds resolves the effective lock_timeout_seconds for a
// lock target via the standard workflow resolution chain. A task-file target
// layers its own override over the project config; any other target (project
// config, local.json, review state) resolves to the project config or the
// built-in default (5s). Best-effort: an unreadable file or missing .tp/
// contributes no override, so the default applies.
func effectiveLockTimeoutSeconds(path string) int {
	wf := EffectiveWorkflowForTaskFile(path)
	return wf.EffectiveLockTimeoutSeconds()
}

// LockFilePath returns the centralized, git-ignored lock path under .tp/locks/
// for a given target path. The name pairs the target's base name with a short
// hash of its absolute path, so it is human-readable while staying unique.
func LockFilePath(target string) string {
	tpDir := ProjectConfigDir(filepath.Dir(target))
	abs := target
	if a, err := filepath.Abs(target); err == nil {
		abs = a
	}
	sum := sha256.Sum256([]byte(abs))
	name := filepath.Base(target) + "-" + hex.EncodeToString(sum[:4]) + ".lock"
	return filepath.Join(tpDir, "locks", name)
}
