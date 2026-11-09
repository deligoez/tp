package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEpicLint_InfoFindingIsNoWarning: a partial-parallel finding the
// operator chose (R's explicit entry leaves the earlier Q out) has severity
// info, which counts as neither an error nor a warning and exits 0.
func TestEpicLint_InfoFindingIsNoWarning(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{
  "version": 1,
  "milestones": [["P", "Q", "R"]],
  "opens_after": {"R": ["P"]},
  "epics": [
    {"id": "p", "title": "P", "milestone": "P", "root": true},
    {"id": "q", "title": "Q", "milestone": "Q", "root": true},
    {"id": "r", "title": "R", "milestone": "R", "depends_on": [{"id": "q", "why": "r renders the records q stores in the shared table"}]}
  ]
}
`)
	out, stderr, code := runEpicLint(t, dir)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, 0, out.Errors)
	assert.Equal(t, 0, out.Warnings)
	require.Len(t, out.Findings, 1, "%+v", out.Findings)
	assert.Equal(t, "partial-parallel", out.Findings[0].Rule)
	assert.Equal(t, "info", out.Findings[0].Severity)
}
