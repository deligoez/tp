package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These close the gaps the v1.18.0 mutation run left in the audit carried
// set (spec/1.18.0-measurements.md, "Mutation").

const auditInfo = `{"role":"r","item_id":"task-t1","status":"PARTIAL","severity":"info"}`

// TestAuditCarried_TheWorstRowOfARoundIsCarried: of one round's rows under
// one key the most severe is carried, so a warning beside an error does not
// let the error through under the blocking policy.
func TestAuditCarried_TheWorstRowOfARoundIsCarried(t *testing.T) {
	t.Parallel()
	spec := auditSpec(t)
	st := auditState(t, spec, AuditConvergeOnBlocking, []string{auditWarn, auditFail})
	rows := AuditCarriedBlocking(spec, st)
	require.Len(t, rows, 1)
	assert.Equal(t, "error", rows[0].Severity)
}

// TestAuditCarried_SeverityIsTheHighestSinceCleared: a warning followed by an
// info row under the same key is carried as a warning.
func TestAuditCarried_SeverityIsTheHighestSinceCleared(t *testing.T) {
	t.Parallel()
	spec := auditSpec(t)
	st := auditState(t, spec, AuditConvergeOnAll, []string{auditWarn}, []string{auditInfo})
	rows, _ := AuditCarriedSet(spec, st)
	require.Len(t, rows, 1)
	assert.Equal(t, "warning", rows[0].Severity)
}

// TestAuditCarried_ASingleRoundsPolicyApplies: with one round recorded, its
// own audit_converge_on decides, so a warning under blocking holds nothing.
func TestAuditCarried_ASingleRoundsPolicyApplies(t *testing.T) {
	t.Parallel()
	spec := auditSpec(t)
	st := auditState(t, spec, AuditConvergeOnBlocking, []string{auditWarn})
	assert.Empty(t, AuditCarriedBlocking(spec, st))
}

// TestAuditCarried_ALatestRoundRowIsNotEarlier: only a row graded before the
// latest round is framed as coming from an earlier one.
func TestAuditCarried_ALatestRoundRowIsNotEarlier(t *testing.T) {
	t.Parallel()
	spec := auditSpec(t)
	st := auditState(t, spec, AuditConvergeOnAll, []string{auditFail})
	rows, _ := AuditCarriedSet(spec, st)
	require.Len(t, rows, 1)
	assert.False(t, rows[0].Earlier)
}

// TestAuditLoopDone_OneCarriedRowHoldsTheCap: at the round cap a single
// carried blocking row still keeps the audit open.
func TestAuditLoopDone_OneCarriedRowHoldsTheCap(t *testing.T) {
	t.Parallel()
	spec := auditSpec(t)
	st := auditState(t, spec, AuditConvergeOnAll, []string{auditFail}, []string{}, []string{})
	d := AuditLoopDone(spec, st, 2, 3, "h")
	assert.False(t, d.Done)
	assert.Len(t, d.CarriedBlocking, 1)
}

// TestAuditCarriedBlockingClause_TwoRowsOneOrphaned: two rows take the plural
// wording, and the one whose role the panel no longer holds is counted.
func TestAuditCarriedBlockingClause_TwoRowsOneOrphaned(t *testing.T) {
	t.Parallel()
	spec := auditSpec(t)
	rows := []CarriedRow{
		{Round: 1, Index: 0, Severity: "error", Row: map[string]any{"role": "spec-coverage", "item_id": "list-1-1"}},
		{Round: 1, Index: 1, Severity: "error", Row: map[string]any{"role": "retired-role", "item_id": "task-t1"}},
	}
	clause := AuditCarriedBlockingClause(rows, spec, false)
	assert.Contains(t, clause, "2 carried blocking audit rows hold the audit open")
	assert.Contains(t, clause, "1 of them belong to a role the audit panel no longer holds")
}
