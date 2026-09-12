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

// carryRoundRel is round 1's file, relative to the repo root every carry
// fixture builds.
const carryRoundRel = ".tp-review/spec/audit-round-1.ndjson"

// commitRoundFile commits round 1's file alone — the record commit the
// changed-since delta is measured from. Every test commit shares one date, so
// the record commit lands in the SAME SECOND as the commits around it.
func commitRoundFile(t *testing.T, dir, path string) {
	t.Helper()
	git(t, dir, "add", path)
	git(t, dir, "commit", "-m", "record audit round 1")
}

// commitEdit rewrites name with content that differs from commitFile's, and
// commits it. commitFile cannot express an edit: it always writes the same
// bytes, so a second call stages nothing.
func commitEdit(t *testing.T, dir, name, msg string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name),
		[]byte("package main\n\n// edited after the record commit\n"), 0o600))
	git(t, dir, "add", name)
	git(t, dir, "commit", "-m", msg)
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

// TestAuditCarry_ACommitInTheRecordSecondDoesNotBreakTheCarry is the reviewer's
// fixture for the second-granularity defect. code.go is committed, then round
// 1's file is committed in the SAME SECOND that recorded_at names. code.go was
// not touched by that record commit, so its acceptance must carry. Measuring
// from the record commit's PARENT says so; asking git for commits "since
// 2020-01-01T00:00:00Z" cannot, because --since is inclusive and catches the
// commit that happens to share the second.
//
// Branch: the round file IS tracked, so the record-commit delta runs.
func TestAuditCarry_ACommitInTheRecordSecondDoesNotBreakTheCarry(t *testing.T) {
	t.Parallel()
	dir := carryRepo(t, "2020-01-01T00:00:00Z", carryFailRow, carryWontfix)
	commitRoundFile(t, dir, carryRoundRel)

	out, stderr, code := auditRecord(t, dir, carryFailRow)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, true, out["clean"], "a commit sharing the record's second is not an edit to code.go")
	assert.Equal(t, float64(1), out["carried"])
	resolved, ok := carriedRow(t, dir, 2)["resolved"].(map[string]any)
	require.True(t, ok, "round 2's row carries the disposition")
	assert.Equal(t, "wontfix", resolved["status"])
}

// TestAuditCarry_AnEditCommittedAfterTheRecordBreaksTheCarry: the evidence file
// is genuinely edited in a commit that lands after the record commit, so it is
// inside the delta and the acceptance does not carry. The record-commit delta
// must not turn every post-record edit into a carry.
//
// Branch: the round file IS tracked, so the record-commit delta runs.
func TestAuditCarry_AnEditCommittedAfterTheRecordBreaksTheCarry(t *testing.T) {
	t.Parallel()
	dir := carryRepo(t, "2020-01-01T00:00:00Z", carryFailRow, carryWontfix)
	commitRoundFile(t, dir, carryRoundRel)
	commitEdit(t, dir, "code.go", "edit the evidence file")

	out, stderr, code := auditRecord(t, dir, carryFailRow)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, false, out["clean"], "a FAIL on a file edited since the record keeps the round unclean")
	assert.Equal(t, float64(0), out["carried"])
	assert.NotContains(t, carriedRow(t, dir, 2), "resolved", "the recorded row carries no disposition")
}

// TestAuditCarry_AnUncommittedRoundFileFallsBackToRecordedAt: a project that
// never commits its round state has no record commit to diff from, so the carry
// falls back to --since <recorded_at> and behaves exactly as it did before.
// Both directions are checked, because the fallback has to keep answering both.
//
// Branch: the round file is NOT tracked, so the recorded_at fallback runs.
func TestAuditCarry_AnUncommittedRoundFileFallsBackToRecordedAt(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		recordedAt string
		carried    float64
	}{
		{"recorded after the only commit", "2021-01-01T00:00:00Z", 1},
		{"recorded before the only commit", "2019-01-01T00:00:00Z", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := carryRepo(t, tc.recordedAt, carryFailRow, carryWontfix)
			require.Empty(t, gitOut(t, dir, "log", "--diff-filter=A", "--format=%H", "--", carryRoundRel),
				"the fixture leaves the round file untracked, so the fallback is what runs")

			out, stderr, code := auditRecord(t, dir, carryFailRow)
			require.Equal(t, 0, code, "stderr: %s", stderr)
			assert.Equal(t, tc.carried, out["carried"])
		})
	}
}

// TestAuditCarry_TheRecordCommitIsFoundAcrossARename: round 1's file is
// committed at one path, code.go is edited and committed after that record, and
// only then is the round file moved. --follow makes the FIRST add the record, so
// the edit falls inside the delta and the carry breaks. Without --follow the
// rename would look like the record, the edit would fall outside the delta, and
// an acceptance would carry over code nobody re-verified — which is also what
// the recorded_at fallback would do here, since every commit predates this
// round's 2021 recorded_at.
//
// Branch: the round file IS tracked, through a rename.
func TestAuditCarry_TheRecordCommitIsFoundAcrossARename(t *testing.T) {
	t.Parallel()
	dir := carryRepo(t, "2021-01-01T00:00:00Z", carryFailRow, carryWontfix)
	// Commit round 1's file at an earlier home, so the move below is a rename.
	old := filepath.Join(dir, ".tp-review", "old", "audit-round-1.ndjson")
	require.NoError(t, os.MkdirAll(filepath.Dir(old), 0o755))
	require.NoError(t, os.Rename(filepath.Join(dir, carryRoundRel), old))
	commitRoundFile(t, dir, ".tp-review/old/audit-round-1.ndjson")

	commitEdit(t, dir, "code.go", "edit the evidence file")
	git(t, dir, "mv", ".tp-review/old/audit-round-1.ndjson", carryRoundRel)
	git(t, dir, "commit", "-m", "move the round state")

	out, stderr, code := auditRecord(t, dir, carryFailRow)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, float64(0), out["carried"], "the record is the first add, not the rename")
	assert.Equal(t, false, out["clean"])
}

// TestAuditCarry_ARecordInTheFirstCommitMeasuresFromTheEmptyTree: when the
// commit that added the round file is the repository's FIRST, it has no parent
// and so no earlier tree to diff against. The base is the empty tree, against
// which every tracked path reads as added, so nothing carries. That is the
// honest answer: no tree records what the repository held when the round was
// recorded, and a carry would be an acceptance over code nobody re-verified.
//
// Branch: the round file IS tracked, by a commit with no parent.
func TestAuditCarry_ARecordInTheFirstCommitMeasuresFromTheEmptyTree(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\nEmpty.\n"), 0o600))
	writeTaskFileRaw(t, dir, `[]`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n"), 0o600))
	state := `{"spec":"spec.md","review_rounds":[],"audit_rounds":[` +
		`{"round":1,"findings":1,"clean":false,"recorded_at":"2021-01-01T00:00:00Z",` +
		`"file":"audit-round-1.ndjson","spec_hash":"sha256:x","id_scheme":"slug","converge_on":"all"}]}`
	writeRecordedAuditRound(t, dir, state,
		strings.TrimSuffix(carryFailRow, "}\n")+`,"resolved":`+carryWontfix+"}\n")
	// One root commit holding the round file, code.go and the spec together. Its
	// recorded_at postdates it, so the recorded_at fallback would carry.
	initGitRepo(t, dir)
	require.Len(t, strings.Fields(gitOut(t, dir, "rev-list", "HEAD")), 1,
		"the record commit is the repository's first, so it has no parent")

	out, stderr, code := auditRecord(t, dir, carryFailRow)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, float64(0), out["carried"], "against the empty tree every tracked path reads as changed")
	assert.Equal(t, false, out["clean"])
}
