package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEffectiveLockTimeoutSeconds_RangeBoundaries pins §12.1's rule that a
// lock_timeout_seconds outside 1-60 falls back to 5.
//
// It exists because mutation testing found four surviving mutants on that one
// condition: flipping `< 1` to `<= 1` or `> 60` to `>= 60` changed nothing any
// test could see. The audit had checked the rule — with 999, which is far
// enough outside that it passes under either spelling of the bound. A value
// that far out cannot tell an inclusive bound from an exclusive one; only 1 and
// 60 themselves can, and nothing exercised them.
func TestEffectiveLockTimeoutSeconds_RangeBoundaries(t *testing.T) {
	for _, tc := range []struct {
		configured int
		want       int
		why        string
	}{
		{configured: 1, want: 1, why: "1 is inside the range: the lower bound is inclusive"},
		{configured: 60, want: 60, why: "60 is inside the range: the upper bound is inclusive"},
		{configured: 0, want: 5, why: "0 is below the range"},
		{configured: 61, want: 5, why: "61 is above the range"},
		{configured: -1, want: 5, why: "a negative value is below the range"},
		{configured: 999, want: 5, why: "far outside behaves like just outside"},
	} {
		dir := t.TempDir()
		writeProjectLockTimeout(t, dir, tc.configured)

		// ProjectWorkflowOverride discovers .tp/ from the working directory,
		// not from the task file's directory, so the fixture only takes effect
		// once we are inside it.
		t.Chdir(dir)

		got := effectiveLockTimeoutSeconds(filepath.Join(dir, "spec.tasks.json"))
		assert.Equal(t, tc.want, got, "lock_timeout_seconds=%d: %s", tc.configured, tc.why)
	}
}

// writeProjectLockTimeout writes a .tp/config.json carrying only the timeout,
// so the resolved value comes from the project layer and nothing else.
func writeProjectLockTimeout(t *testing.T, dir string, seconds int) {
	t.Helper()

	tpDir := filepath.Join(dir, ".tp")
	require.NoError(t, os.MkdirAll(tpDir, 0o755))

	cfg := map[string]any{
		"workflow": map[string]any{"lock_timeout_seconds": seconds},
	}
	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(tpDir, "config.json"), data, 0o600))
}
