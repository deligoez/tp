package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// inboxRow is one finding on epic:d, with role naming its writer when set.
func inboxRow(role string) string {
	row := `{"severity":"low","finding":"d's title is terse","location":"epic:d","evidence":"title: D"`
	if role != "" {
		row += `,"role":"` + role + `"`
	}
	return row + "}\n"
}

// writeInbox writes each prompt's output file of out: the body bodies names
// for its role, an empty file for every other role.
func writeInbox(t *testing.T, out *epicReviewPayload, bodies map[string]string) {
	t.Helper()
	for _, p := range out.Prompts {
		require.NoError(t, os.WriteFile(p.OutputPath, []byte(bodies[p.Role]), 0o600))
	}
}

// recordedRows reads round's recorded rows in dir's state directory.
func recordedRows(t *testing.T, dir string, out *epicReviewPayload, round int) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, out.StateDir, "round-"+string(rune('0'+round))+".ndjson"))
	require.NoError(t, err)
	var rows []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var row map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &row), line)
		rows = append(rows, row)
	}
	return rows
}

// TestEpicReviewRecordInbox_StampsEachRowWithItsFilesRole: a row its role
// wrote without a role is stamped from the file it came from, so no inbox
// row counts as unattributed.
func TestEpicReviewRecordInbox_StampsEachRowWithItsFilesRole(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	out := runEpicReview(t, dir, "--milestone", "A")
	require.GreaterOrEqual(t, len(out.Prompts), 2)
	first, second := out.Prompts[0].Role, out.Prompts[1].Role
	writeInbox(t, &out, map[string]string{first: inboxRow(""), second: inboxRow(second)})

	stdout, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--record-inbox")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Contains(t, stdout, `"findings": 2`)
	rows := recordedRows(t, dir, &out, 1)
	require.Len(t, rows, 2)
	assert.Equal(t, first, rows[0]["role"], "panel order, the first role's row first")
	assert.Equal(t, second, rows[1]["role"])

	status, _, _ := runTP(t, dir, "epic", "review", "--milestone", "A", "--status")
	assert.NotContains(t, status, "unattributed")
}

// TestEpicReviewRecordInbox_RefusesARowNamingAnotherRole: a row naming a
// role other than its file's is refused at exit 1, and no round is written.
func TestEpicReviewRecordInbox_RefusesARowNamingAnotherRole(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	out := runEpicReview(t, dir, "--milestone", "A")
	first := out.Prompts[0].Role
	writeInbox(t, &out, map[string]string{first: inboxRow("someone-else")})

	_, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--record-inbox")
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, `names role \"someone-else\"`)
	assert.Contains(t, stderr, first)
	_, err := os.Stat(filepath.Join(dir, out.StateDir, "round-1.ndjson"))
	assert.True(t, os.IsNotExist(err), "no round is recorded")
}

// TestEpicReviewRecordInbox_RefusesAMissingFileNamingTheRole: a role that
// wrote no file may never have run; the contract asks a role that found
// nothing for an empty file, so a missing one is refused, naming the role.
func TestEpicReviewRecordInbox_RefusesAMissingFileNamingTheRole(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	out := runEpicReview(t, dir, "--milestone", "A")
	writeInbox(t, &out, nil)
	missing := out.Prompts[len(out.Prompts)-1]
	require.NoError(t, os.Remove(missing.OutputPath))

	_, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--record-inbox")
	require.Equal(t, 3, code, "stderr: %s", stderr)
	msg, hint := epicErrorOf(t, stderr)
	assert.Contains(t, msg, "no findings file from roles "+missing.Role+" in "+filepath.Join(out.StateDir, "inbox"))
	assert.Contains(t, hint, "run the prompts of roles "+missing.Role+", each writing its file (empty when it finds nothing), then tp epic review --milestone A --record-inbox")
}

// TestEpicReviewRecordInbox_AcceptsEmptyFiles: every role found nothing and
// wrote an empty file, which records a round with no findings.
func TestEpicReviewRecordInbox_AcceptsEmptyFiles(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	out := runEpicReview(t, dir, "--milestone", "A")
	writeInbox(t, &out, nil)

	stdout, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--record-inbox")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Contains(t, stdout, `"round": 1`)
	assert.Contains(t, stdout, `"findings": 0`)
}

// TestEpicReviewRecordInbox_ADiscardedEmissionsFilesAreNotRecorded: after a
// --force discard the old emission's inbox files are gone, so the new
// round's --record-inbox cannot pick up rows written against old prompts:
// it refuses until each role of the new emission writes its file.
func TestEpicReviewRecordInbox_ADiscardedEmissionsFilesAreNotRecorded(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	old := runEpicReview(t, dir, "--milestone", "A")
	writeInbox(t, &old, map[string]string{old.Prompts[0].Role: inboxRow("")})
	rewriteEpicGraph(t, dir, strings.Replace(epicChain, `"title": "D"`, `"title": "D renamed"`, 1))
	fresh := runEpicReview(t, dir, "--milestone", "A", "--force")
	for _, p := range old.Prompts {
		_, err := os.Stat(p.OutputPath)
		assert.True(t, os.IsNotExist(err), "%s's old file is removed", p.Role)
	}

	_, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--record-inbox")
	require.Equal(t, 3, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "no findings file from roles")

	writeInbox(t, &fresh, nil)
	stdout, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--record-inbox")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Contains(t, stdout, `"findings": 0`)
}

// TestEpicReviewRecordInbox_TheEmissionNamesTheFlag: the slice's next_action
// names --record-inbox, which reads every output path itself.
func TestEpicReviewRecordInbox_TheEmissionNamesTheFlag(t *testing.T) {
	t.Parallel()
	out := runEpicReview(t, epicGraphDir(t, epicChain), "--milestone", "A")
	assert.Contains(t, out.NextAction, "tp epic review --milestone A --record-inbox")
}

// TestEpicReviewRecordInbox_BesideRecordIsAUsageError: one flag records the
// inbox, the other a file the caller made; both at once exits 2.
func TestEpicReviewRecordInbox_BesideRecordIsAUsageError(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	runEpicReview(t, dir, "--milestone", "A")
	_, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--record-inbox", "--record", "rows.ndjson")
	assert.Equal(t, 2, code, "stderr: %s", stderr)
}

// storedPanel reads round's panel from the slice's snapshot.
func storedPanel(t *testing.T, dir string, out *epicReviewPayload, round int) []struct {
	ID         string `json:"id"`
	OutputPath string `json:"output_path"`
} {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, out.StateDir, "snapshot-round-"+string(rune('0'+round))+".json"))
	require.NoError(t, err)
	var snap struct {
		Roles []struct {
			ID         string `json:"id"`
			OutputPath string `json:"output_path"`
		} `json:"roles"`
	}
	require.NoError(t, json.Unmarshal(data, &snap))
	return snap.Roles
}

// TestEpicReviewRecordInbox_AMovedRepositoryStillRecords: the snapshot
// stores each output path relative to the repository root, so after the
// repository moves, the role files written at the new place record.
func TestEpicReviewRecordInbox_AMovedRepositoryStillRecords(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	out := runEpicReview(t, dir, "--milestone", "A")
	panel := storedPanel(t, dir, &out, 1)
	require.NotEmpty(t, panel)
	moved := dir + "-moved"
	require.NoError(t, os.Rename(dir, moved))
	for _, r := range panel {
		require.False(t, filepath.IsAbs(r.OutputPath), "stored relative: %s", r.OutputPath)
		require.NoError(t, os.WriteFile(filepath.Join(moved, filepath.FromSlash(r.OutputPath)), nil, 0o600))
	}

	stdout, stderr, code := runTP(t, moved, "epic", "review", "--milestone", "A", "--record-inbox")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Contains(t, stdout, `"round": 1`)
}

// TestEpicReviewRecordInbox_ASkippedLineIsNamedByFileAndLine: a line the
// reader cannot parse refuses the round at exit 1, naming the file as stored
// and the line.
func TestEpicReviewRecordInbox_ASkippedLineIsNamedByFileAndLine(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	out := runEpicReview(t, dir, "--milestone", "A")
	first := storedPanel(t, dir, &out, 1)[0]
	writeInbox(t, &out, map[string]string{first.ID: inboxRow("") + "not json\n"})

	_, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--record-inbox")
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, first.OutputPath+" (role "+first.ID+") line 2: invalid JSON")
}
