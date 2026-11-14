package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGradeAuditRoles_CountsDistinctFileChecksOfAnyVerdict pins decision 4's
// N: the distinct emitted file_check items a role returned a row for,
// whatever the verdict, so a file read and found clean counts as graded.
// Another role's rows, a spec item's row and a repeated id add nothing, and
// neither does an id the round never emitted, however well it is formed: N
// never exceeds the items the prompt listed.
func TestGradeAuditRoles_CountsDistinctFileChecksOfAnyVerdict(t *testing.T) {
	a, b, c := "file-security-a-go.0000000000000001", "file-security-b-go.0000000000000002", "file-security-c-go.0000000000000003"
	pools := []AuditRolePool{
		{Role: "security", Pool: 14, Items: []string{a, b, c}},
		{Role: "quality", Pool: 14, Items: []string{"file-quality-a-go.0000000000000001"}},
	}
	rows := []map[string]any{
		{"role": "security", "item_id": a, "status": "PASS"},
		{"role": "security", "item_id": b, "status": "FAIL"},
		{"role": "security", "item_id": b, "status": "PARTIAL"},
		{"role": "security", "item_id": c, "status": "PARTIAL"},
		{"role": "security", "item_id": "file-security-bogus-go.00000000000000ff", "status": "PASS"},
		{"role": "security", "item_id": "file-security-x-go.0000000000000009", "status": "PASS"},
		{"role": "security", "item_id": "list-2-3", "status": "PASS"},
		{"role": "spec-coverage", "item_id": a, "status": "PASS"},
	}
	assert.Equal(t, []RoleGraded{{Role: "security", Graded: 3, Pool: 14}, {Role: "quality", Graded: 0, Pool: 14}},
		GradeAuditRoles(pools, rows), "pool order, never-emitted ids ignored, and a role that returned nothing graded 0")
}

// TestGradeAuditRoles_ARolelessRowCountsForTheOnePoolAskingItsID: a row that
// names no role is graded for the one pool whose items hold its id, so an
// all-PASS spec-coverage file written without role keys is not read as
// unanswered; an id two pools asked stays unattributed.
func TestGradeAuditRoles_ARolelessRowCountsForTheOnePoolAskingItsID(t *testing.T) {
	pools := []AuditRolePool{
		{Role: RoleSpecCoverage, Pool: 3, Items: []string{"item-1", "item-2", "both"}},
		{Role: "security", Pool: 2, Items: []string{"file-a", "both"}},
	}
	rows := []map[string]any{
		{"item_id": "item-1", "status": "PASS"},
		{"item_id": "file-a", "status": "PASS"},
		{"item_id": "both", "status": "PASS"},
	}
	assert.Equal(t, []RoleGraded{{Role: RoleSpecCoverage, Graded: 1, Pool: 3}, {Role: "security", Graded: 1, Pool: 2}},
		GradeAuditRoles(pools, rows), "each roleless id counts for its one asking pool, and an id two pools asked for neither")
}

// TestAuditPools_RoundTripAndRemovedWithTheInbox: the pools an emission
// writes are read back at record, live in the audit inbox, and leave with
// the round's role files.
func TestAuditPools_RoundTripAndRemovedWithTheInbox(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(spec, []byte("# S\n"), 0o600))

	_, found, err := ReadAuditPools(spec, 2)
	require.NoError(t, err)
	assert.False(t, found, "no emission, no pools")

	want := []AuditRolePool{{Role: "security", Pool: 46}}
	require.NoError(t, WriteAuditPools(spec, 2, want))
	path := AuditPoolsPath(spec, 2)
	assert.Equal(t, filepath.Join(ReviewStateDir(spec), "audit-inbox", "round-2.pools.json"), path)
	got, found, err := ReadAuditPools(spec, 2)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, want, got)

	require.NoError(t, RemoveRoundInbox(spec, PhaseAudit, 2))
	assert.NoFileExists(t, path)
	assert.NoDirExists(t, filepath.Dir(path), "the emptied inbox goes too")
}

// TestAuditPools_IsRoundBookkeeping: the dirty-state classifier reads the
// pools file as round bookkeeping of its round.
func TestAuditPools_IsRoundBookkeeping(t *testing.T) {
	assert.Equal(t, "3", roundRefFromName("spec/.tp-review/x/audit-inbox/round-3.pools.json"))
}
