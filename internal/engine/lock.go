package engine

import (
	"fmt"
	"os"

	"github.com/gofrs/flock"
)

func WithFileLock(path string, fn func() error) error {
	lockPath := path + ".lock"
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
