package engine

import (
	"fmt"

	"github.com/gofrs/flock"
)

// WithFileLock acquires an exclusive lock on path+".lock" and runs fn.
// The lock is released when fn returns.
func WithFileLock(path string, fn func() error) error {
	fl := flock.New(path + ".lock")

	locked, err := fl.TryLock()
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("task file is locked by another process")
	}
	defer func() { _ = fl.Unlock() }()

	return fn()
}
