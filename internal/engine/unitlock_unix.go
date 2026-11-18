//go:build unix

package engine

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// lockUnitFile opens and exclusively flocks a unit's lock file. The caller
// hands the file to the child and closes its own copy without unlocking.
func lockUnitFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600) //nolint:gosec // the path is derived from the cycle key and a task id
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errUnitLockBusy
		}
		return nil, err
	}
	return f, nil
}

// unitLockHeld reports whether some process holds a unit's lock. A file that
// cannot be opened is a lock nobody holds.
func unitLockHeld(path string) bool {
	f, err := os.Open(path) //nolint:gosec // the path is derived from the cycle key and a task id
	if err != nil {
		return false
	}
	defer f.Close()
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil
}
