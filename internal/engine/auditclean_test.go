package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// auditRoundWith builds a round of rows: one PASS row carrying the
// `severity: null` that every one of the 2,913 recorded PASS rows in tp's own
// corpus carries (§2), plus the single non-PASS row under test. Every fixture
// below differs only in that row, so an assertion that moves is an assertion
// about the severity that row carries and nothing else.
func auditRoundWith(nonPass map[string]any) []map[string]any {
	return []map[string]any{
		{"status": "PASS", "item_id": "item-1", "severity": nil},
		nonPass,
	}
}

// TestAuditRowsClean_InfoIsAdvisory pins §7 row 5: over the engine clean
// predicate, an `info` non-PASS row is advisory — the round is unclean under
// `all` and clean under `blocking`. The mutant that must fail it grades on
// `status` alone, which reports unclean under both.
func TestAuditRowsClean_InfoIsAdvisory(t *testing.T) {
	rows := auditRoundWith(map[string]any{"status": "PARTIAL", "item_id": "item-2", "severity": "info"})

	assert.False(t, AuditRowsClean(rows, AuditConvergeOnAll),
		"under all, the existence of a non-PASS row makes the round unclean")
	assert.True(t, AuditRowsClean(rows, AuditConvergeOnBlocking),
		"under blocking, an info row is advisory and does not block")
}

// TestAuditRowsClean_WarningIsAdvisory pins §7 row 6: the same predicate over a
// `warning` row, asserted separately from info so the mutant that special-cases
// info alone fails here.
func TestAuditRowsClean_WarningIsAdvisory(t *testing.T) {
	rows := auditRoundWith(map[string]any{"status": "PARTIAL", "item_id": "item-2", "severity": "warning"})

	assert.False(t, AuditRowsClean(rows, AuditConvergeOnAll),
		"under all, the existence of a non-PASS row makes the round unclean")
	assert.True(t, AuditRowsClean(rows, AuditConvergeOnBlocking),
		"under blocking, a warning row is advisory and does not block")
}

// TestAuditRowsClean_ErrorBlocksUnderBoth pins §7 row 7: `error` is the audit
// vocabulary's blocking severity, so it blocks under both policy values. The
// mutant that must fail it returns clean whenever the policy is `blocking`.
func TestAuditRowsClean_ErrorBlocksUnderBoth(t *testing.T) {
	rows := auditRoundWith(map[string]any{"status": "FAIL", "item_id": "item-2", "severity": "error"})

	assert.False(t, AuditRowsClean(rows, AuditConvergeOnAll),
		"under all, the existence of a non-PASS row makes the round unclean")
	assert.False(t, AuditRowsClean(rows, AuditConvergeOnBlocking),
		"under blocking, error is the blocking severity")
}

// TestAuditRowsClean_UnusableSeverityBlocks pins §7 row 8: a round whose only
// non-PASS row carries a severity tp cannot grade is unclean under `blocking`.
// Four fixtures, one per unrecognised shape — a string outside the enum, JSON
// null, an absent key, and a value that is not a string. The mutant that must
// fail it grades a row with no usable severity as advisory.
func TestAuditRowsClean_UnusableSeverityBlocks(t *testing.T) {
	cases := []struct {
		name    string
		nonPass map[string]any
	}{
		{"a string outside the enum", map[string]any{"status": "FAIL", "item_id": "item-2", "severity": "moderate"}},
		{"JSON null", map[string]any{"status": "FAIL", "item_id": "item-2", "severity": nil}},
		{"an absent key", map[string]any{"status": "FAIL", "item_id": "item-2"}},
		{"a value that is not a string", map[string]any{"status": "FAIL", "item_id": "item-2", "severity": float64(3)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := auditRoundWith(tc.nonPass)

			assert.False(t, AuditRowsClean(rows, AuditConvergeOnBlocking),
				"a row tp cannot grade is blocking, not ignored")
			assert.False(t, AuditRowsClean(rows, AuditConvergeOnAll),
				"and under all it is unclean for the ordinary reason: it is non-PASS")
		})
	}
}

// TestAuditRowsClean_PassRowsAreNotGraded pins the population the predicate
// reads: its subject is the round's non-PASS rows, so a round holding only PASS
// rows is clean under both values. It is asserted because every recorded PASS
// row carries `severity: null` (§2) — a predicate phrased over the rows of any
// severity would read that null under the fail-closed rule and report every
// all-PASS round unclean under `blocking`.
func TestAuditRowsClean_PassRowsAreNotGraded(t *testing.T) {
	rows := []map[string]any{
		{"status": "PASS", "item_id": "item-1", "severity": nil},
		{"status": "PASS", "item_id": "item-2"},
	}

	assert.True(t, AuditRowsClean(rows, AuditConvergeOnAll), "an all-PASS round is clean under all")
	assert.True(t, AuditRowsClean(rows, AuditConvergeOnBlocking), "an all-PASS round is clean under blocking")
	assert.True(t, AuditRowsClean(nil, AuditConvergeOnBlocking), "a round with no rows has nothing to grade")
}

