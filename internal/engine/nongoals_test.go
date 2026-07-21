package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNonGoals_OutputContractAndNoSpeculativeField: a role file may carry only the
// §3.3 keys — no user-defined finding schema (non-goal 2) and no speculative role
// field (non-goal 5).
func TestNonGoals_OutputContractAndNoSpeculativeField(t *testing.T) {
	// An attempt to declare an output/finding schema is rejected.
	_, err := ParseRoleBytes([]byte(`{"id":"r","title":"T","instructions":"I","output_schema":{"x":1}}`), "r")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown top-level key")

	// A speculative field beyond §3.3 is rejected too.
	_, err = ParseRoleBytes([]byte(`{"id":"r","title":"T","instructions":"I","weight":5}`), "r")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown top-level key")
}

// TestNonGoals_NoCrossProjectCorpus: discovery stops at the git boundary, so a
// .tp/ above the boundary is never read — no cross-project or global corpus
// (non-goal 7).
func TestNonGoals_NoCrossProjectCorpus(t *testing.T) {
	outer := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(outer, ".tp", "reviewers"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(outer, ".tp", "reviewers", "x.json"),
		[]byte(`{"id":"x","title":"T","instructions":"I"}`), 0o600))

	inner := filepath.Join(outer, "project")
	require.NoError(t, os.MkdirAll(filepath.Join(inner, ".git"), 0o755))

	_, populated, err := LoadRoleCorpus(inner, PhaseReviewers)
	require.NoError(t, err)
	assert.False(t, populated, "the outer .tp/ above the git boundary must not be read")
}
