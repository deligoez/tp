package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// EpicStaleRevisionError reports a graph write whose --if-revision named a
// revision the file no longer has (spec/1.4.0.md §2); nothing was written.
type EpicStaleRevisionError struct {
	Want, Have string
}

func (e *EpicStaleRevisionError) Error() string {
	have := e.Have
	if have == "" {
		have = "none: the graph file is absent"
	}
	return fmt.Sprintf("the graph file changed since revision %q was read: its revision is %s", e.Want, have)
}

// EpicGraphRevision returns the revision of tpDir/epics.json: the lowercase
// hex SHA-256 of its bytes, or the empty string when the file is absent.
func EpicGraphRevision(tpDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(tpDir, EpicGraphFile))
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// checkEpicRevision returns an *EpicStaleRevisionError when want is set and
// differs from the file's current revision. Callers hold the file's lock.
func checkEpicRevision(tpDir string, want *string) error {
	if want == nil {
		return nil
	}
	have, err := EpicGraphRevision(tpDir)
	if err != nil {
		return err
	}
	if have != *want {
		return &EpicStaleRevisionError{Want: *want, Have: have}
	}
	return nil
}
