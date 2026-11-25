package engine

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// With no hc to run, a land's hc commit says so with ErrHCNotFound, which the
// caller reads as "leave the task file for the agent" rather than a failure.
func TestLandCommitTaskFileHC_NoHCIsErrHCNotFound(t *testing.T) {
	prev := hcBinary
	hcBinary = filepath.Join(t.TempDir(), "no-such-hc")
	t.Cleanup(func() { hcBinary = prev })

	_, err := LandCommitTaskFileHC(t.TempDir(), filepath.Join(t.TempDir(), "spec.tasks.json"), "m")
	assert.True(t, errors.Is(err, ErrHCNotFound), "%v", err)
}
