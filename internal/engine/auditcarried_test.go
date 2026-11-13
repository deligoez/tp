package engine

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The rows of spec/backlog/audit-carried-fail-blocks.md's tests: one item of
// one role, graded FAIL, PASS, fixed, accepted or advisory.
const (
	auditFail      = `{"role":"r","item_id":"task-t1","status":"FAIL","severity":"error","notes":"t1 is not done"}`
	auditPass      = `{"role":"r","item_id":"task-t1","status":"PASS","severity":null}`
	auditFixed     = `{"role":"r","item_id":"task-t1","status":"FAIL","severity":"error","resolved":{"status":"fixed","evidence":"abc1234"}}`
	auditAccepted  = `{"role":"r","item_id":"task-t1","status":"FAIL","severity":"error","resolved":{"status":"wontfix","evidence":"operator accepted"}}`
	auditWarn      = `{"role":"r","item_id":"task-t1","status":"PARTIAL","severity":"warning"}`
	auditListFail  = `{"role":"spec-coverage","item_id":"list-1-1","status":"FAIL","severity":"error"}`
	auditListPass  = `{"role":"spec-coverage","item_id":"list-1-1","status":"PASS","severity":null}`
	auditOtherPass = `{"role":"r","item_id":"task-t2","status":"PASS","severity":null}`
)

// auditState writes one audit round file per element of files and returns the
// state holding them, each round under spec hash "h" and policy convergeOn.
func auditState(t *testing.T, specPath, convergeOn string, files ...[]string) *ReviewState {
	t.Helper()
	rounds := loopRounds(t, specPath, files...)
	for i := range rounds {
		rounds[i].ConvergeOn = convergeOn
	}
	return &ReviewState{AuditRounds: rounds}
}

func auditSpec(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "spec.md")
}

// TestAuditCarried_EmptyRoundsLeaveAFailBlocking is the BUGS.md repro at the
// engine: a FAIL of round 1 that no later round re-grades blocks.
func TestAuditCarried_EmptyRoundsLeaveAFailBlocking(t *testing.T) {
	t.Parallel()
	spec := auditSpec(t)
	st := auditState(t, spec, AuditConvergeOnAll, []string{auditFail}, []string{}, []string{})

	rows := AuditCarriedBlocking(spec, st)
	require.Len(t, rows, 1)
	assert.Equal(t, 1, rows[0].Round)
	assert.Equal(t, 0, rows[0].Index)
	assert.Equal(t, CarriedStateUnresolved, rows[0].State())
	assert.Equal(t, "error", rows[0].Severity)
	assert.True(t, rows[0].Earlier)
}

// TestAuditCarried_ALaterPassClearsIt: a later round's PASS under the same
// (role, item_id) is the re-grade that clears it.
func TestAuditCarried_ALaterPassClearsIt(t *testing.T) {
	t.Parallel()
	spec := auditSpec(t)
	st := auditState(t, spec, AuditConvergeOnAll, []string{auditFail}, []string{}, []string{auditPass})
	assert.Empty(t, AuditCarriedBlocking(spec, st))
}

// TestAuditCarried_APassOnAnotherItemClearsNothing: a PASS clears only its own
// key.
func TestAuditCarried_APassOnAnotherItemClearsNothing(t *testing.T) {
	t.Parallel()
	spec := auditSpec(t)
	st := auditState(t, spec, AuditConvergeOnAll, []string{auditFail}, []string{auditOtherPass})
	assert.Len(t, AuditCarriedBlocking(spec, st), 1)
}

// TestAuditCarried_FixedAloneClearsNothing: a row marked fixed waits for a
// re-grade; an empty round after it is not one.
func TestAuditCarried_FixedAloneClearsNothing(t *testing.T) {
	t.Parallel()
	spec := auditSpec(t)
	st := auditState(t, spec, AuditConvergeOnAll, []string{auditFixed}, []string{}, []string{})

	rows := AuditCarriedBlocking(spec, st)
	require.Len(t, rows, 1)
	assert.Equal(t, CarriedStateFixedUnverified, rows[0].State())
}

// TestAuditCarried_AnAcceptanceDoesNotBlock: wontfix with evidence takes the
// row out of the blocking set.
func TestAuditCarried_AnAcceptanceDoesNotBlock(t *testing.T) {
	t.Parallel()
	spec := auditSpec(t)
	st := auditState(t, spec, AuditConvergeOnAll, []string{auditAccepted}, []string{})
	assert.Empty(t, AuditCarriedBlocking(spec, st))
}

// TestAuditCarried_APositionalIdClearsOnlyOverTheSameText: list-1-1 is a
// position in the spec, so a PASS over a different spec text may name another
// item and does not clear it; over the same text it does. A task id names its
// item whatever the text.
func TestAuditCarried_APositionalIdClearsOnlyOverTheSameText(t *testing.T) {
	t.Parallel()
	spec := auditSpec(t)
	st := auditState(t, spec, AuditConvergeOnAll, []string{auditListFail, auditFail}, []string{auditListPass, auditPass})
	st.AuditRounds[1].SpecHash = "h2"

	rows := AuditCarriedBlocking(spec, st)
	require.Len(t, rows, 1, "the task id clears over a new text, the list id does not")
	assert.Equal(t, "list-1-1", rows[0].Row["item_id"])

	st.AuditRounds[1].SpecHash = "h"
	assert.Empty(t, AuditCarriedBlocking(spec, st), "over the same text both clear")
}

// TestAuditCarried_APositionalIdClearsUnlessItsItemMoved: a spec repaired
// between two rounds changes the hash. The PASS at list-0-1 still clears the
// row when the later snapshot holds the old item's text at the same id, or
// nowhere (the item itself was repaired); when the old text now sits at
// another id, the item moved and the PASS graded another one, so it clears
// nothing.
func TestAuditCarried_APositionalIdClearsUnlessItsItemMoved(t *testing.T) {
	t.Parallel()
	fail := `{"role":"spec-coverage","item_id":"list-0-1","status":"FAIL","severity":"error"}`
	pass := `{"role":"spec-coverage","item_id":"list-0-1","status":"PASS","severity":null}`
	for _, tc := range []struct {
		later   string
		cleared bool
	}{
		{"# S\n\n## 1. Rules\n\nAn added line.\n\n1. Every row is checked.\n", true},
		{"# S\n\n## 1. Rules\n\n1. Every row is checked and counted.\n", true},
		{"# S\n\n## 1. Rules\n\n1. A new first rule.\n2. Every row is checked.\n", false},
	} {
		spec := auditSpec(t)
		st := auditState(t, spec, AuditConvergeOnAll, []string{fail}, []string{pass})
		st.AuditRounds[1].SpecHash = "h2"
		require.NoError(t, WriteSnapshotAtomic(spec, PhaseAudit, 1, []byte("# S\n\n## 1. Rules\n\n1. Every row is checked.\n")))
		require.NoError(t, WriteSnapshotAtomic(spec, PhaseAudit, 2, []byte(tc.later)))
		assert.Equal(t, tc.cleared, len(AuditCarriedBlocking(spec, st)) == 0, "later text %q", tc.later)
	}
}

// TestAuditCarried_ALowerReGradeKeepsTheHighestSeverity: a FAIL error re-graded
// PARTIAL warning with no disposition is still graded error, so the blocking
// policy does not let it go.
func TestAuditCarried_ALowerReGradeKeepsTheHighestSeverity(t *testing.T) {
	t.Parallel()
	spec := auditSpec(t)
	st := auditState(t, spec, AuditConvergeOnBlocking, []string{auditFail}, []string{auditWarn})

	rows := AuditCarriedBlocking(spec, st)
	require.Len(t, rows, 1)
	assert.Equal(t, 2, rows[0].Round, "the latest row is carried")
	assert.Equal(t, "error", rows[0].Severity, "graded at the highest since the last resolution")
}

// TestAuditCarried_AnAdvisoryRowBlocksOnlyUnderAll: a warning blocks under all
// and not under blocking, the latest round's policy.
func TestAuditCarried_AnAdvisoryRowBlocksOnlyUnderAll(t *testing.T) {
	t.Parallel()
	spec := auditSpec(t)
	st := auditState(t, spec, AuditConvergeOnAll, []string{auditWarn}, []string{})
	assert.Len(t, AuditCarriedBlocking(spec, st), 1)

	st.AuditRounds[1].ConvergeOn = AuditConvergeOnBlocking
	assert.Empty(t, AuditCarriedBlocking(spec, st))
}

// TestAuditCarried_TheWorstRowOfARoundCounts: --merge keeps disagreeing rows
// under one key, and the FAIL beside a PASS in one round is not cleared by it.
func TestAuditCarried_TheWorstRowOfARoundCounts(t *testing.T) {
	t.Parallel()
	spec := auditSpec(t)
	st := auditState(t, spec, AuditConvergeOnAll, []string{auditPass, auditFail}, []string{})

	rows := AuditCarriedBlocking(spec, st)
	require.Len(t, rows, 1)
	assert.Equal(t, 1, rows[0].Index, "the FAIL row, not the PASS before it")
}

// TestAuditCarried_AnEpochCutsIt: a row at or below the newest epoch's audit
// cut is not carried.
func TestAuditCarried_AnEpochCutsIt(t *testing.T) {
	t.Parallel()
	spec := auditSpec(t)
	st := auditState(t, spec, AuditConvergeOnAll, []string{auditFail}, []string{})
	st.Epochs = []Epoch{{AfterAuditRound: 1}}
	assert.Empty(t, AuditCarriedBlocking(spec, st))
}
