package engine

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// specCoverageOver computes §2.3's field over a fixture's rounds the way a
// caller does: one walk, whose streaks the field is then read from.
func specCoverageOver(t *testing.T, specPath string, rounds []ReviewRound) *int {
	t.Helper()
	var streaks []RoleStreak
	captureAuditRoundNotices(t, func() {
		streaks, _ = ComputeAuditRoleStreaks(specPath, rounds)
	})
	return SpecCoverageCleanRounds(streaks)
}

// marshalSpecCoverageField renders the value as the payload key carries it: a
// field with no omitempty, so nil is an explicit JSON null and 0 is a 0. The
// assertion that the two never collapse is what this helper exists for.
func marshalSpecCoverageField(t *testing.T, v *int) string {
	t.Helper()
	data, err := json.Marshal(struct {
		SpecCoverageCleanRounds *int `json:"spec_coverage_clean_rounds"`
	}{SpecCoverageCleanRounds: v})
	require.NoError(t, err)
	return string(data)
}

// Test 18, first half — null when the latest recorded round holds no
// spec-coverage row even though an earlier round does. This is the
// discriminating case against an implementation that scans the whole history
// for the role: it would report round 1's streak instead of null.
func TestSpecCoverageCleanRounds_NullWhenOnlyAnEarlierRoundHoldsTheRole(t *testing.T) {
	specPath, rounds := streakFixture(t,
		streakRound{rolesHash: "h", file: "r1.ndjson", rows: auditRows(auditRow("spec-coverage", "PASS"))},
		streakRound{rolesHash: "h", file: "r2.ndjson", rows: auditRows(auditRow("go-safety", "PASS"))},
	)

	got := specCoverageOver(t, specPath, rounds)
	assert.Nil(t, got, "the latest round contributes no row attributed to spec-coverage")
	assert.JSONEq(t, `{"spec_coverage_clean_rounds":null}`, marshalSpecCoverageField(t, got))
}

// Test 18, second half — 0 when the latest round holds a spec-coverage row that
// is not PASS: the role was measured and has an open finding, which is a
// different answer from null and must not collapse into it.
func TestSpecCoverageCleanRounds_ZeroWhenTheLatestRoundHoldsAnOpenRow(t *testing.T) {
	specPath, rounds := streakFixture(t,
		streakRound{rolesHash: "h", file: "r1.ndjson", rows: auditRows(auditRow("spec-coverage", "PASS"))},
		streakRound{rolesHash: "h", file: "r2.ndjson", rows: auditRows(auditRow("spec-coverage", "FAIL"))},
	)

	got := specCoverageOver(t, specPath, rounds)
	require.NotNil(t, got, "the role was measured in the latest round, so the answer is 0, not null")
	assert.Equal(t, 0, *got)
	assert.JSONEq(t, `{"spec_coverage_clean_rounds":0}`, marshalSpecCoverageField(t, got))
}

// The two answers of test 18 are rendered differently: an implementation
// treating 0 as "no answer" — an int with a zero sentinel, or an omitempty
// field — collapses them and fails here.
func TestSpecCoverageCleanRounds_NullAndZeroDoNotCollapse(t *testing.T) {
	zero := 0
	assert.NotEqual(t,
		marshalSpecCoverageField(t, nil),
		marshalSpecCoverageField(t, &zero),
		"null and 0 are different answers")
	assert.Contains(t, marshalSpecCoverageField(t, nil), `"spec_coverage_clean_rounds"`,
		"the key is always emitted, never omitted")
}

// A clean latest round carries the streak itself, which is what makes the field
// worth reading at all.
func TestSpecCoverageCleanRounds_CarriesTheStreakOfACleanLatestRound(t *testing.T) {
	body := auditRows(auditRow("spec-coverage", "PASS"))
	specPath, rounds := streakFixture(t,
		streakRound{rolesHash: "h", file: "r1.ndjson", rows: body},
		streakRound{rolesHash: "h", file: "r2.ndjson", rows: body},
		streakRound{rolesHash: "h", file: "r3.ndjson", rows: body},
	)

	got := specCoverageOver(t, specPath, rounds)
	require.NotNil(t, got)
	assert.Equal(t, 3, *got)
}

// Test 12 — the field half. Over four rounds in which spec-coverage is all-PASS,
// rounds 1-2 carrying a different stored roles_hash from rounds 3-4 make the
// reported number 2, not 4.
func TestSpecCoverageCleanRounds_CorpusChangeBetweenRoundsTruncatesTheNumber(t *testing.T) {
	body := auditRows(auditRow("spec-coverage", "PASS"))
	specPath, rounds := streakFixture(t,
		streakRound{rolesHash: "old", file: "r1.ndjson", rows: body},
		streakRound{rolesHash: "old", file: "r2.ndjson", rows: body},
		streakRound{rolesHash: "new", file: "r3.ndjson", rows: body},
		streakRound{rolesHash: "new", file: "r4.ndjson", rows: body},
	)

	got := specCoverageOver(t, specPath, rounds)
	require.NotNil(t, got)
	assert.Equal(t, 2, *got)
	assert.NotEqual(t, 4, *got)
}

// Test 12, second half — the field half. An earlier round carrying an empty
// stored hash beside rounds carrying a non-empty one truncates the number the
// same way.
func TestSpecCoverageCleanRounds_EmptyStoredHashOnAnEarlierRoundTruncatesTheNumber(t *testing.T) {
	body := auditRows(auditRow("spec-coverage", "PASS"))
	specPath, rounds := streakFixture(t,
		streakRound{rolesHash: "", file: "r1.ndjson", rows: body},
		streakRound{rolesHash: "h", file: "r2.ndjson", rows: body},
		streakRound{rolesHash: "h", file: "r3.ndjson", rows: body},
	)

	got := specCoverageOver(t, specPath, rounds)
	require.NotNil(t, got)
	assert.Equal(t, 2, *got)
}

// Test 13 — the field half. An empty stored hash on the latest round makes that
// round contribute no rows, so the answer is null even though the file on disk
// holds spec-coverage rows and every earlier round is clean.
func TestSpecCoverageCleanRounds_NullWhenTheLatestRoundHasAnEmptyStoredHash(t *testing.T) {
	body := auditRows(auditRow("spec-coverage", "PASS"))
	specPath, rounds := streakFixture(t,
		streakRound{rolesHash: "h", file: "r1.ndjson", rows: body},
		streakRound{rolesHash: "h", file: "r2.ndjson", rows: body},
		streakRound{rolesHash: "", file: "r3.ndjson", rows: body},
	)

	got := specCoverageOver(t, specPath, rounds)
	assert.Nil(t, got, "a readable round with an empty stored hash contributes no rows")
	assert.JSONEq(t, `{"spec_coverage_clean_rounds":null}`, marshalSpecCoverageField(t, got))
}

// The remaining states in which the latest round contributes no row attributed
// to spec-coverage all report null too — the same statement in each: this round
// did not measure conformance.
func TestSpecCoverageCleanRounds_NullStates(t *testing.T) {
	clean := auditRows(auditRow("spec-coverage", "PASS"))

	cases := []struct {
		name   string
		rounds []streakRound
	}{
		{"no recorded round at all", nil},
		{"latest round's file is deleted", []streakRound{
			{rolesHash: "h", file: "r1.ndjson", rows: clean},
			{rolesHash: "h", file: "r2.ndjson", deleted: true},
		}},
		{"latest round's file entry is empty", []streakRound{
			{rolesHash: "h", file: "r1.ndjson", rows: clean},
			{rolesHash: "h", file: ""},
		}},
		{"latest round holds a line that does not unmarshal into a map", []streakRound{
			{rolesHash: "h", file: "r1.ndjson", rows: clean},
			{rolesHash: "h", file: "r2.ndjson", rows: clean + "[1,2]\n"},
		}},
		{"latest round recorded zero rows", []streakRound{
			{rolesHash: "h", file: "r1.ndjson", rows: clean},
			{rolesHash: "h", file: "r2.ndjson", rows: ""},
		}},
		{"latest round's every row carries no role", []streakRound{
			{rolesHash: "h", file: "r1.ndjson", rows: clean},
			{rolesHash: "h", file: "r2.ndjson", rows: auditRows(
				`{"status":"PASS"}`,
				`{"role":"   ","status":"FAIL"}`,
				`null`,
			)},
		}},
		{"latest round attributes its rows to a differently-cased id", []streakRound{
			{rolesHash: "h", file: "r1.ndjson", rows: clean},
			{rolesHash: "h", file: "r2.ndjson", rows: auditRows(auditRow("Spec-Coverage", "PASS"))},
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			specPath, rounds := streakFixture(t, tc.rounds...)
			got := specCoverageOver(t, specPath, rounds)
			assert.Nil(t, got)
			assert.JSONEq(t, `{"spec_coverage_clean_rounds":null}`, marshalSpecCoverageField(t, got))
		})
	}
}

// The returned pointer addresses a copy: writing through it cannot reach the
// role_streaks entry the same walk reports.
func TestSpecCoverageCleanRounds_ReturnsACopy(t *testing.T) {
	streaks := []RoleStreak{{Role: RoleSpecCoverage, ConsecutiveClean: 3, Open: 0}}

	got := SpecCoverageCleanRounds(streaks)
	require.NotNil(t, got)
	*got = 99
	assert.Equal(t, 3, streaks[0].ConsecutiveClean)
}
