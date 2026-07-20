package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetRejectsGateSkippedReason(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)

	_, stderr, code := runTP(t, dir, "set", "t1", "gate_skipped_reason=manual")
	require.Equal(t, 2, code)
	assert.Contains(t, stderr, "managed")
	assert.Contains(t, stderr, "--skip-gate")
}
