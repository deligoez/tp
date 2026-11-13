package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidate_WarnsOnATaskWithoutTouchesUnderParallel: with run_max_parallel
// raised at the project layer, tp validate warns on an open task without
// touches and still passes.
func TestValidate_WarnsOnATaskWithoutTouchesUnderParallel(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	stdout, stderr, code := runTP(t, dir, "validate")
	require.Equal(t, 0, code, stderr)
	assert.NotContains(t, stdout, `"touches"`, "nothing to warn about at run_max_parallel 1")

	_, stderr, code = runTP(t, dir, "set", "--workflow", "--project", "run_max_parallel=2")
	require.Equal(t, 0, code, stderr)
	stdout, stderr, code = runTP(t, dir, "validate")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, "task seed: no touches, so it never runs beside another task (run_max_parallel 2)")
}
