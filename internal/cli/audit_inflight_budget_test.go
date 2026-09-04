package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuditSecondEmissionWithCapIsNotCorruptState: tp audit never calls
// EnsureReviewState, so after the first emission writes a snapshot, state.json
// is legitimately absent — the normal in-flight round that loadAuditSpec and
// loadAuditPriorRound both treat as "no recorded state".
// refuseAuditIfBudgetExhausted used to abort on ANY LoadReviewState error, so
// the SECOND tp audit exited 3 ("state is unusable") whenever audit_max_rounds
// was non-zero, and exited 0 when the cap was 0: a normal in-flight state read
// as corruption, gated on an unrelated knob.
func TestAuditSecondEmissionWithCapIsNotCorruptState(t *testing.T) {
	t.Parallel()
	// setupBudgetProject caps audit_max_rounds at 2, which is enough: the two
	// emissions below record nothing, so the budget is never consumed.
	dir := setupBudgetProject(t, "audit_max_rounds")
	aPath := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(aPath, []byte("package main\n"), 0o600))

	_, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", aPath)
	require.Equal(t, 0, code, "first audit emission: %s", stderr)

	// The first emission wrote snapshot-audit-round-1.md but no state.json.
	_, err := os.Stat(filepath.Join(dir, ".tp-review", "spec", "state.json"))
	require.True(t, os.IsNotExist(err), "emission writes a snapshot, not a state index")

	_, stderr, code = runTP(t, dir, "audit", "spec.md", "--affected-files", aPath)
	assert.Equal(t, 0, code, "an unrecorded in-flight round is not corrupt state: %s", stderr)
	assert.NotContains(t, stderr, "unusable", "a missing state index must not read as corruption")
}
