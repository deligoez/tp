package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The row every carry test re-records: one security FAIL on code.go.
const carryFailRow = `{"item_id":"file-security-code","status":"FAIL","role":"security",` +
	`"evidence_file":"code.go","category":"security","severity":"error","notes":""}` + "\n"

// carryRepo is a git repo whose code.go was committed on 2020-01-01 (every
// test commit is dated then), with audit round 1 written directly: its one
// row is failRow carrying the given resolved block, and the round was recorded
// at recordedAt. A 2021 recordedAt leaves code.go unchanged since the round; a
// 2019 one sees the 2020 commit as a change.
func carryRepo(t *testing.T, recordedAt, failRow, resolved string) string {
	t.Helper()
	dir, _ := newAuditRepo(t)
	commitFile(t, dir, "code.go", "add code")
	state := `{"spec":"spec.md","review_rounds":[],"audit_rounds":[` +
		`{"round":1,"findings":1,"clean":false,"recorded_at":"` + recordedAt + `",` +
		`"file":"audit-round-1.ndjson","spec_hash":"sha256:x","id_scheme":"slug","converge_on":"all"}]}`
	row := strings.TrimSuffix(failRow, "}\n") + `,"resolved":` + resolved + "}\n"
	writeRecordedAuditRound(t, dir, state, row)
	return dir
}

const carryWontfix = `{"status":"wontfix","evidence":"vendored code, out of scope","resolved_at":"2020-06-01T00:00:00Z"}`

// carriedRow returns the one row of a recorded audit round file.
func carriedRow(t *testing.T, dir string, round int) map[string]any {
	t.Helper()
	name := filepath.Join(dir, ".tp-review", "spec", fmt.Sprintf("audit-round-%d.ndjson", round))
	data, err := os.ReadFile(name)
	require.NoError(t, err)
	var row map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(string(data))), &row))
	return row
}

// TestAuditCarry_AnUnchangedAcceptanceIsCarriedIntoTheNextRound: a FAIL
// accepted wontfix with evidence in round 1, re-FAILed in round 2 with its
// evidence_file untouched since, is carried: round 2's row holds the same
// disposition stamped carried_from 1, the round is clean, and the payload
// counts it. Without the carry an acceptance lasts exactly one round.
func TestAuditCarry_AnUnchangedAcceptanceIsCarriedIntoTheNextRound(t *testing.T) {
	t.Parallel()
	dir := carryRepo(t, "2021-01-01T00:00:00Z", carryFailRow, carryWontfix)

	out, stderr, code := auditRecord(t, dir, carryFailRow)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, true, out["clean"], "the carried acceptance leaves the round clean")
	assert.Equal(t, float64(1), out["carried"], "the payload counts the carried disposition")

	resolved, ok := carriedRow(t, dir, 2)["resolved"].(map[string]any)
	require.True(t, ok, "round 2's row carries the disposition in its recorded file")
	assert.Equal(t, "wontfix", resolved["status"])
	assert.Equal(t, "vendored code, out of scope", resolved["evidence"])
	assert.Equal(t, "2020-06-01T00:00:00Z", resolved["resolved_at"], "the disposition is copied verbatim")
	assert.Equal(t, float64(1), resolved["carried_from"], "carried_from names the round it was decided in")
}

// TestAuditCarry_AnEditToTheEvidenceFileBreaksTheCarry: the same acceptance
// with code.go committed after round 1 was recorded is not carried — the
// acceptance was made against code that has since changed, so the auditor's
// FAIL stands and the round is unclean.
func TestAuditCarry_AnEditToTheEvidenceFileBreaksTheCarry(t *testing.T) {
	t.Parallel()
	dir := carryRepo(t, "2019-01-01T00:00:00Z", carryFailRow, carryWontfix)

	out, stderr, code := auditRecord(t, dir, carryFailRow)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, false, out["clean"], "a FAIL on a changed file keeps the round unclean")
	assert.Equal(t, float64(0), out["carried"], "nothing is carried, and carried is reported at zero")
	assert.NotContains(t, carriedRow(t, dir, 2), "resolved", "the recorded row carries no disposition")
}

// TestAuditCarry_AFixedRowIsNeverCarried: a fixed disposition claims a repair
// the next round has to verify, so it is never carried, even on an unchanged
// file.
func TestAuditCarry_AFixedRowIsNeverCarried(t *testing.T) {
	t.Parallel()
	dir := carryRepo(t, "2021-01-01T00:00:00Z", carryFailRow,
		`{"status":"fixed","evidence":"abc1234 escapes the token","resolved_at":"2020-06-01T00:00:00Z"}`)

	out, stderr, code := auditRecord(t, dir, carryFailRow)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, false, out["clean"], "a re-FAILed fixed row keeps the round unclean")
	assert.Equal(t, float64(0), out["carried"])
	assert.NotContains(t, carriedRow(t, dir, 2), "resolved", "a fixed disposition is not carried")
}

// TestAuditCarry_TheCarryIsNotFencedUnattended: the carry re-applies the
// operator's earlier acceptance and writes no new one, so under TP_UNATTENDED
// it still carries and --record exits 0 — the accept-finding fence belongs to
// --resolve, where an acceptance is made.
func TestAuditCarry_TheCarryIsNotFencedUnattended(t *testing.T) {
	t.Parallel()
	dir := carryRepo(t, "2021-01-01T00:00:00Z", carryFailRow, carryWontfix)
	f := filepath.Join(dir, "results.ndjson")
	require.NoError(t, os.WriteFile(f, []byte(carryFailRow), 0o600))

	stdout, stderr, code := runTPFence(t, dir, true, "audit", "spec.md", "--record", f)
	require.Equal(t, 0, code, "the carry is not fenced under TP_UNATTENDED: %s", stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, float64(1), out["carried"])
	assert.Equal(t, true, out["clean"])
}

// TestAuditCarry_TheCarryChainsAndNamesTheRoundFirstDecided: a carried
// acceptance is itself carried into the round after, while the file stays
// unchanged, and carried_from keeps naming round 1 — the round the operator
// decided it in, as ground's carry does — rather than the round before.
func TestAuditCarry_TheCarryChainsAndNamesTheRoundFirstDecided(t *testing.T) {
	t.Parallel()
	dir := carryRepo(t, "2021-01-01T00:00:00Z", carryFailRow, carryWontfix)

	_, stderr, code := auditRecord(t, dir, carryFailRow)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	out, stderr, code := auditRecord(t, dir, carryFailRow)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, float64(1), out["carried"], "round 3 inherits the acceptance round 2 carried")
	assert.Equal(t, true, out["clean"])

	resolved, ok := carriedRow(t, dir, 3)["resolved"].(map[string]any)
	require.True(t, ok, "round 3's row carries the disposition")
	assert.Equal(t, "wontfix", resolved["status"])
	assert.Equal(t, float64(1), resolved["carried_from"], "carried_from names the round first decided in")
}

// TestAuditCarry_ARowWithNoFileCarriesUntilReDecided: an accepted row with no
// evidence_file has no file a commit could change — the Prior Round block
// shows it no changed_since — so its acceptance carries even when the round
// it was decided in is older than every commit in the repo.
func TestAuditCarry_ARowWithNoFileCarriesUntilReDecided(t *testing.T) {
	t.Parallel()
	const specRow = `{"item_id":"list-0-2","status":"FAIL","role":"spec-coverage",` +
		`"category":"correctness","severity":"error","notes":""}` + "\n"
	dir := carryRepo(t, "2019-01-01T00:00:00Z", specRow, carryWontfix)

	out, stderr, code := auditRecord(t, dir, specRow)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, float64(1), out["carried"], "a row with no file to change is carried")
	assert.Equal(t, true, out["clean"])
}
