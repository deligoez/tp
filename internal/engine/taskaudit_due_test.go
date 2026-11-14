package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordOf is a recorded task audit holding the batch's members.
func recordOf(seq int, b TaskAuditBatch) TaskAuditRecord {
	return TaskAuditRecord{Seq: seq, Members: b.Members, Files: b.Files(), Roles: b.Roles()}
}

// TestDueTaskAudits_OnlyFullBatchesMidPhase: mid-phase only a full batch is
// due; at the end of the phase the partial one is due too.
func TestDueTaskAudits_OnlyFullBatchesMidPhase(t *testing.T) {
	t.Parallel()
	cs := []TaskAuditCandidate{
		cand("a", "2026-01-01T00:00:00Z", files("a", 10)...),
		cand("b", "2026-01-02T00:00:00Z", files("b", 3)...),
	}
	mid := DueTaskAudits(cs, nil, false)
	require.Equal(t, [][]string{{"a"}}, batchIDs(mid))
	end := DueTaskAudits(cs, nil, true)
	assert.Equal(t, [][]string{{"a"}, {"b"}}, batchIDs(end))
}

// TestDueTaskAudits_ARecordedMemberIsNotPackedAgain: a member a recorded
// task audit holds for the same close owes nothing.
func TestDueTaskAudits_ARecordedMemberIsNotPackedAgain(t *testing.T) {
	t.Parallel()
	cs := []TaskAuditCandidate{
		cand("a", "2026-01-01T00:00:00Z", files("a", 10)...),
		cand("b", "2026-01-02T00:00:00Z", files("b", 3)...),
	}
	first := DueTaskAudits(cs, nil, false)
	require.Len(t, first, 1)
	recs := []TaskAuditRecord{recordOf(1, first[0])}
	assert.Empty(t, DueTaskAudits(cs, recs, false))
	assert.Equal(t, [][]string{{"b"}}, batchIDs(DueTaskAudits(cs, recs, true)))
	assert.True(t, TaskAuditsOwed(cs, recs))
	recs = append(recs, recordOf(2, DueTaskAudits(cs, recs, true)[0]))
	assert.False(t, TaskAuditsOwed(cs, recs))
}

// TestDueTaskAudits_AReClosedTaskOwesAgain: a task reopened and closed again
// is a new close, which no recorded member names.
func TestDueTaskAudits_AReClosedTaskOwesAgain(t *testing.T) {
	t.Parallel()
	cs := []TaskAuditCandidate{cand("a", "2026-01-01T00:00:00Z", files("a", 10)...)}
	recs := []TaskAuditRecord{recordOf(1, DueTaskAudits(cs, nil, false)[0])}
	require.False(t, TaskAuditsOwed(cs, recs))
	cs[0].ClosedAt = "2026-01-05T00:00:00Z"
	assert.True(t, TaskAuditsOwed(cs, recs))
	assert.Equal(t, [][]string{{"a"}}, batchIDs(DueTaskAudits(cs, recs, false)))
}

// TestDueTaskAudits_ARecordedSliceOfALargeTaskIsNotPackedAgain: slices of a
// task over ten files are owed one by one.
func TestDueTaskAudits_ARecordedSliceOfALargeTaskIsNotPackedAgain(t *testing.T) {
	t.Parallel()
	cs := []TaskAuditCandidate{cand("big", "2026-01-01T00:00:00Z", files("big", 15)...)}
	first := DueTaskAudits(cs, nil, false)
	require.Equal(t, [][]string{{"big#0"}}, batchIDs(first))
	recs := []TaskAuditRecord{recordOf(1, first[0])}
	assert.Empty(t, DueTaskAudits(cs, recs, false))
	assert.Equal(t, [][]string{{"big#1"}}, batchIDs(DueTaskAudits(cs, recs, true)))
}

// TestDueTaskAudits_ACandidateOwingNoRoleIsSkipped: a task whose stamped
// roles are no longer configured owes nothing.
func TestDueTaskAudits_ACandidateOwingNoRoleIsSkipped(t *testing.T) {
	t.Parallel()
	c := cand("a", "2026-01-01T00:00:00Z", "a.go")
	c.Roles = nil
	assert.Empty(t, DueTaskAudits([]TaskAuditCandidate{c}, nil, true))
	assert.False(t, TaskAuditsOwed([]TaskAuditCandidate{c}, nil))
}
