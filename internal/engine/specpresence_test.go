package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSpecCoverageUnproven: a round leaves spec-coverage unproven when the
// role graded fewer items than its emission asked, or when it was recorded
// without an emission under 1.20.0 (spec/backlog/role-presence.md §2.2,
// §2.3). A code role's cut list, a round asked nothing and an old round
// carrying neither field are left as they were.
func TestSpecCoverageUnproven(t *testing.T) {
	t.Parallel()
	sc := func(graded, pool int) *ReviewRound {
		return &ReviewRound{Graded: []RoleGraded{{Role: RoleSpecCoverage, Graded: graded, Pool: pool}}}
	}
	assert.False(t, SpecCoverageUnproven(sc(4, 4)), "every asked item graded")
	assert.True(t, SpecCoverageUnproven(sc(3, 4)), "one asked item without a row")
	assert.True(t, SpecCoverageUnproven(sc(0, 4)), "spec-coverage wrote no rows")
	assert.False(t, SpecCoverageUnproven(sc(0, 0)), "a round asked nothing is clean as today")
	assert.False(t, SpecCoverageUnproven(&ReviewRound{Graded: []RoleGraded{{Role: "security", Graded: 3, Pool: 14}}}),
		"a code role's list is a cut by design")
	assert.True(t, SpecCoverageUnproven(&ReviewRound{Unasked: true}), "recorded without its emission")
	assert.False(t, SpecCoverageUnproven(&ReviewRound{}), "a round from before 1.20.0 is not re-judged")
}

// TestAuditRoundClean_UnprovenSpecCoverageIsNotClean: a round stamped clean
// whose spec-coverage did not answer every asked item is not clean, so it
// cannot count toward convergence.
func TestAuditRoundClean_UnprovenSpecCoverageIsNotClean(t *testing.T) {
	t.Parallel()
	spec := filepath.Join(t.TempDir(), "spec.md")
	require.NoError(t, os.WriteFile(spec, []byte("# S\n"), 0o600))
	partial := ReviewRound{Clean: true, Graded: []RoleGraded{{Role: RoleSpecCoverage, Graded: 3, Pool: 4}}}
	assert.False(t, AuditRoundClean(spec, &partial))
	full := ReviewRound{Clean: true, Graded: []RoleGraded{{Role: RoleSpecCoverage, Graded: 4, Pool: 4}}}
	assert.True(t, AuditRoundClean(spec, &full))
}

// TestComputeAuditRoleStreaks_UnprovenSpecCoverageBreaksItsStreak: a latest
// round whose spec-coverage rows are all PASS but short of what it was asked
// starts no streak, and such an earlier round ends one.
func TestComputeAuditRoleStreaks_UnprovenSpecCoverageBreaksItsStreak(t *testing.T) {
	t.Parallel()
	pass := auditRows(auditRow(RoleSpecCoverage, "PASS"), auditRow("security", "PASS"))
	spec, rounds := streakFixture(t,
		streakRound{rolesHash: "h", file: "audit-round-1.ndjson", rows: pass},
		streakRound{rolesHash: "h", file: "audit-round-2.ndjson", rows: pass},
		streakRound{rolesHash: "h", file: "audit-round-3.ndjson", rows: pass},
	)
	short := []RoleGraded{{Role: RoleSpecCoverage, Graded: 1, Pool: 2}}

	rounds[2].Graded = short
	streaks, _ := ComputeAuditRoleStreaks(spec, rounds)
	got := byRoleStreak(t, streaks)
	assert.Equal(t, 0, got[RoleSpecCoverage].ConsecutiveClean, "the latest round left spec-coverage unproven")
	assert.Equal(t, 3, got["security"].ConsecutiveClean, "other roles are not touched")

	rounds[2].Graded = nil
	rounds[1].Graded = short
	streaks, _ = ComputeAuditRoleStreaks(spec, rounds)
	assert.Equal(t, 1, byRoleStreak(t, streaks)[RoleSpecCoverage].ConsecutiveClean, "an unproven earlier round ends the streak")
}
