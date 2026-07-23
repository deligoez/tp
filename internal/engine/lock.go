package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

// WithFileLock acquires an exclusive flock for the given target path, runs fn,
// then releases. The lock file lives under .tp/locks/ (git-ignored) rather than
// beside the target, so a transient or stale lock never litters the working
// tree (§5.3). A stale sibling lock left by an earlier tp version (<path>.lock)
// is removed on acquisition.
func WithFileLock(path string, fn func() error) error {
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

	locked, err := fl.TryLock()
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("task file is locked by another process")
	}
	defer func() {
		_ = fl.Unlock()
		_ = os.Remove(lockPath)
	}()

	return fn()
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
