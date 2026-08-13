package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// streakRound describes one recorded audit round in a role-streak fixture.
// The zero value is not useful: rolesHash must be set for the round to
// contribute rows at all, since §2.1 decides the empty-hash cause first.
type streakRound struct {
	rolesHash string // stored roles_hash; "" instantiates §2.1's empty-hash cause
	file      string // stored `file` entry; "" instantiates the empty-file-entry cause
	rows      string // NDJSON written under file
	deleted   bool   // file is named on the entry but never written (the missing-file cause)
}

// streakFixture writes a spec and one recorded round file per entry into a
// fresh state directory, returning the spec path and the round index in
// oldest-first order — the order ReviewState.AuditRounds stores.
func streakFixture(t *testing.T, rounds ...streakRound) (string, []ReviewRound) {
	t.Helper()
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# spec\n"), 0o600))
	stateDir := ReviewStateDir(specPath)
	require.NoError(t, os.MkdirAll(stateDir, 0o750))

	entries := make([]ReviewRound, 0, len(rounds))
	for i, r := range rounds {
		if r.file != "" && !r.deleted {
			require.NoError(t, os.WriteFile(filepath.Join(stateDir, r.file), []byte(r.rows), 0o600))
		}
		entries = append(entries, ReviewRound{Round: i + 1, File: r.file, RolesHash: r.rolesHash})
	}
	return specPath, entries
}

// auditRows joins row literals into the NDJSON body of a round file.
func auditRows(lines ...string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// auditRow renders one recorded audit row.
func auditRow(role, status string) string {
	b, err := json.Marshal(map[string]any{"role": role, "status": status})
	if err != nil {
		panic(err)
	}
	return string(b)
}

// byRoleStreak indexes a returned array by role id, asserting each id appears
// once.
func byRoleStreak(t *testing.T, streaks []RoleStreak) map[string]RoleStreak {
	t.Helper()
	out := make(map[string]RoleStreak, len(streaks))
	for _, s := range streaks {
		_, dup := out[s.Role]
		require.False(t, dup, "role %q appears twice in role_streaks", s.Role)
		out[s.Role] = s
	}
	return out
}

// streakRoleOrder is the emitted order of role ids.
func streakRoleOrder(streaks []RoleStreak) []string {
	ids := make([]string, 0, len(streaks))
	for _, s := range streaks {
		ids = append(ids, s.Role)
	}
	return ids
}

// Test 1 — per-role streaks over three rounds: spec-coverage all-PASS in every
// one, a second role holding a FAIL in the latest. The entry shape is pinned in
// the same test, since role_streaks is an agent-facing payload and the field
// names are half of its contract.
func TestComputeAuditRoleStreaks_PerRoleStreaks(t *testing.T) {
	specPath, rounds := streakFixture(t,
		streakRound{rolesHash: "h", file: "r1.ndjson", rows: auditRows(auditRow("spec-coverage", "PASS"))},
		streakRound{rolesHash: "h", file: "r2.ndjson", rows: auditRows(auditRow("spec-coverage", "PASS"))},
		streakRound{rolesHash: "h", file: "r3.ndjson", rows: auditRows(
			auditRow("spec-coverage", "PASS"),
			auditRow("go-safety", "FAIL"),
		)},
	)

	var streaks []RoleStreak
	var latestRows []map[string]any
	notices := captureAuditRoundNotices(t, func() {
		streaks, latestRows = ComputeAuditRoleStreaks(specPath, rounds)
	})
	assert.Empty(t, notices, "every round contributes rows, so no advisory fires")

	assert.Equal(t, []RoleStreak{
		{Role: "spec-coverage", ConsecutiveClean: 3, Open: 0},
		{Role: "go-safety", ConsecutiveClean: 0, Open: 1},
	}, streaks)

	data, err := json.Marshal(streaks[1])
	require.NoError(t, err)
	assert.JSONEq(t, `{"role":"go-safety","consecutive_clean":0,"open":1}`, string(data))

	// The latest round's rows come back with the streaks so §2.4's per-row
	// facts about that round need no second read of the same file.
	require.Len(t, latestRows, 2)
	assert.Equal(t, "spec-coverage", AuditRowRole(latestRows[0]))
}

// Test 2 — a round absent a role ends that role's streak rather than being
// skipped. This is the discriminating case against an implementation that
// walks past an unmeasured round: skipping round 2 would report 3.
func TestComputeAuditRoleStreaks_AbsentRoundEndsTheStreak(t *testing.T) {
	specPath, rounds := streakFixture(t,
		streakRound{rolesHash: "h", file: "r1.ndjson", rows: auditRows(auditRow("lens", "PASS"))},
		streakRound{rolesHash: "h", file: "r2.ndjson", rows: auditRows(auditRow("other", "PASS"))},
		streakRound{rolesHash: "h", file: "r3.ndjson", rows: auditRows(auditRow("lens", "PASS"))},
	)

	streaks, _ := ComputeAuditRoleStreaks(specPath, rounds)
	byRole := byRoleStreak(t, streaks)
	require.Contains(t, byRole, "lens")
	assert.Equal(t, 1, byRole["lens"].ConsecutiveClean,
		"round 2 measured no `lens` row, so it ends the streak instead of being skipped")
	assert.NotEqual(t, 3, byRole["lens"].ConsecutiveClean)
}

// Test 3 — role_streaks covers only the latest round's roles: a role present in
// round 1 and absent from round 2 does not appear in the array at round 2.
func TestComputeAuditRoleStreaks_CoversOnlyTheLatestRoundsRoles(t *testing.T) {
	specPath, rounds := streakFixture(t,
		streakRound{rolesHash: "h", file: "r1.ndjson", rows: auditRows(
			auditRow("gone", "PASS"),
			auditRow("keeper", "PASS"),
		)},
		streakRound{rolesHash: "h", file: "r2.ndjson", rows: auditRows(auditRow("keeper", "PASS"))},
	)

	streaks, _ := ComputeAuditRoleStreaks(specPath, rounds)
	assert.Equal(t, []string{"keeper"}, streakRoleOrder(streaks),
		"the panel is the latest round's, not every role ever seen")
	assert.Equal(t, 2, streaks[0].ConsecutiveClean)
}

// Test 4 — ordering: spec-coverage first, then ascending byte order. The ids
// are chosen so a plain alphabetical sort would not put spec-coverage first,
// and "Go-Safety" leads with an uppercase byte whose position separates byte
// order (uppercase before lowercase) from a case-insensitive sort, which would
// place it after "ax-contract".
func TestComputeAuditRoleStreaks_OrderingIsSpecCoverageThenByteOrder(t *testing.T) {
	specPath, rounds := streakFixture(t,
		streakRound{rolesHash: "h", file: "r1.ndjson", rows: auditRows(
			auditRow("ax-contract", "PASS"),
			auditRow("spec-coverage", "PASS"),
			auditRow("Go-Safety", "PASS"),
		)},
	)

	streaks, _ := ComputeAuditRoleStreaks(specPath, rounds)
	assert.Equal(t, []string{"spec-coverage", "Go-Safety", "ax-contract"}, streakRoleOrder(streaks))
}

// Test 12 — a corpus change between recorded rounds truncates the streak. Over
// four rounds in which spec-coverage is all-PASS, rounds 1-2 carrying a
// different stored roles_hash from rounds 3-4 stop the walk at round 2, so the
// streak is 2 rather than 4.
func TestComputeAuditRoleStreaks_CorpusChangeBetweenRoundsEndsTheStreak(t *testing.T) {
	body := auditRows(auditRow("spec-coverage", "PASS"))
	specPath, rounds := streakFixture(t,
		streakRound{rolesHash: "old", file: "r1.ndjson", rows: body},
		streakRound{rolesHash: "old", file: "r2.ndjson", rows: body},
		streakRound{rolesHash: "new", file: "r3.ndjson", rows: body},
		streakRound{rolesHash: "new", file: "r4.ndjson", rows: body},
	)

	var streaks []RoleStreak
	notices := captureAuditRoundNotices(t, func() {
		streaks, _ = ComputeAuditRoleStreaks(specPath, rounds)
	})

	byRole := byRoleStreak(t, streaks)
	require.Contains(t, byRole, "spec-coverage")
	assert.Equal(t, 2, byRole["spec-coverage"].ConsecutiveClean)
	assert.NotEqual(t, 4, byRole["spec-coverage"].ConsecutiveClean)
	assert.Contains(t, notices, "round 2 was recorded under a different auditor corpus; skipping its rows")
	assert.NotContains(t, notices, "round 1 ", "the walk stops on round 2 and never reaches round 1")
}

// Test 12, second half — an earlier round carrying an empty stored hash beside
// rounds carrying a non-empty one also ends the streak, and says so with the
// no-corpus-hash wording rather than the different-corpus one: tp does not know
// which panel that round used, and naming it a different corpus would report a
// change that may never have happened.
func TestComputeAuditRoleStreaks_EmptyStoredHashOnAnEarlierRoundEndsTheStreak(t *testing.T) {
	body := auditRows(auditRow("spec-coverage", "PASS"))
	specPath, rounds := streakFixture(t,
		streakRound{rolesHash: "", file: "r1.ndjson", rows: body},
		streakRound{rolesHash: "h", file: "r2.ndjson", rows: body},
		streakRound{rolesHash: "h", file: "r3.ndjson", rows: body},
	)

	var streaks []RoleStreak
	notices := captureAuditRoundNotices(t, func() {
		streaks, _ = ComputeAuditRoleStreaks(specPath, rounds)
	})

	byRole := byRoleStreak(t, streaks)
	assert.Equal(t, 2, byRole["spec-coverage"].ConsecutiveClean)
	assert.Contains(t, notices, "round 1 has no auditor-corpus hash; skipping its rows")
	assert.NotContains(t, notices, "different auditor corpus")
}

// Test 13 — an empty stored hash on the latest round ends the walk immediately:
// that round contributes no rows, so role_streaks is empty and no earlier round
// is read. (spec_coverage_clean_rounds is the `null` half of the same test and
// belongs to the field that reports it.)
func TestComputeAuditRoleStreaks_EmptyStoredHashOnTheLatestRoundEndsTheWalk(t *testing.T) {
	body := auditRows(auditRow("spec-coverage", "PASS"))
	specPath, rounds := streakFixture(t,
		streakRound{rolesHash: "h", file: "r1.ndjson", rows: body},
		streakRound{rolesHash: "h", file: "r2.ndjson", rows: body},
		streakRound{rolesHash: "", file: "r3.ndjson", rows: body},
	)

	var streaks []RoleStreak
	var latestRows []map[string]any
	notices := captureAuditRoundNotices(t, func() {
		streaks, latestRows = ComputeAuditRoleStreaks(specPath, rounds)
	})

	assert.Empty(t, streaks)
	assert.Nil(t, latestRows, "the latest round contributed no rows")
	assert.Contains(t, notices, "round 3 has no auditor-corpus hash; skipping its rows")
	assert.NotContains(t, notices, "round 2 ", "the walk never reaches an earlier round")
}

// Test 16 — the walk stops at a streak-closing round. Over five rounds where
// round 2's file is deleted and round 4 holds a non-PASS row for every role
// present in round 5, every streak closes at round 4, so round 2 is never read
// — asserted by the absence of its advisory. An implementation walking the
// whole recorded history emits it and fails.
func TestComputeAuditRoleStreaks_WalkStopsAtAStreakClosingRound(t *testing.T) {
	clean := auditRows(auditRow("spec-coverage", "PASS"), auditRow("lens", "PASS"))
	specPath, rounds := streakFixture(t,
		streakRound{rolesHash: "h", file: "r1.ndjson", rows: clean},
		streakRound{rolesHash: "h", file: "r2.ndjson", deleted: true},
		streakRound{rolesHash: "h", file: "r3.ndjson", rows: clean},
		streakRound{rolesHash: "h", file: "r4.ndjson", rows: auditRows(
			auditRow("spec-coverage", "FAIL"),
			auditRow("lens", "FAIL"),
		)},
		streakRound{rolesHash: "h", file: "r5.ndjson", rows: clean},
	)

	var streaks []RoleStreak
	notices := captureAuditRoundNotices(t, func() {
		streaks, _ = ComputeAuditRoleStreaks(specPath, rounds)
	})

	assert.Equal(t, []RoleStreak{
		{Role: "spec-coverage", ConsecutiveClean: 1, Open: 0},
		{Role: "lens", ConsecutiveClean: 1, Open: 0},
	}, streaks)
	assert.Empty(t, notices, "round 2 is below the round that closed every streak and is never read")
}

// Test 17 — role_streaks is an emitted empty array, never null, in every state
// where no role appears in the latest recorded round. The contributes-no-rows
// state is instantiated by three of §2.1's four causes here; the fourth reaches
// the latest round through its own empty stored hash, which is the fixture
// TestComputeAuditRoleStreaks_EmptyStoredHashOnTheLatestRoundEndsTheWalk
// builds. The non-unmarshalable-line case is the discriminating one against an
// implementation that routes that cause through the advisory but computes the
// streaks from a reader keeping the file's surviving rows.
func TestComputeAuditRoleStreaks_EmptyArrayStates(t *testing.T) {
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
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			specPath, rounds := streakFixture(t, tc.rounds...)
			var streaks []RoleStreak
			captureAuditRoundNotices(t, func() {
				streaks, _ = ComputeAuditRoleStreaks(specPath, rounds)
			})

			assert.Empty(t, streaks)
			data, err := json.Marshal(streaks)
			require.NoError(t, err)
			assert.Equal(t, "[]", string(data),
				"an emitted empty array, never a null from a nil slice")
		})
	}
}
