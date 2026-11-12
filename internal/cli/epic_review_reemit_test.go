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

// rewriteEpicGraph replaces the project's graph file with body.
func rewriteEpicGraph(t *testing.T, dir, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "epics.json"), []byte(body), 0o600))
}

// epicErrorOf decodes the error envelope tp printed on stderr.
func epicErrorOf(t *testing.T, stderr string) (msg, hint string) {
	t.Helper()
	var e struct {
		Error string `json:"error"`
		Hint  string `json:"hint"`
	}
	require.NoError(t, json.Unmarshal([]byte(stderr), &e), "stderr: %s", stderr)
	return e.Error, e.Hint
}

// TestEpicReemit_AnIdenticalReemitIsIdempotent: nothing changed between two
// emissions of round 1, so the second emits round 1 again at exit 0.
func TestEpicReemit_AnIdenticalReemitIsIdempotent(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	first := runEpicReview(t, dir, "--milestone", "A")
	again := runEpicReview(t, dir, "--milestone", "A")
	assert.Equal(t, first.Round, again.Round)
	assert.Equal(t, first.Units, again.Units)
}

// TestEpicReemit_AGraphEditRefusesTheReemit: d's title changes after round
// 1 was emitted and before it was recorded, so emitting round 1 again would
// rebuild it under the roles still holding the old prompts. Refused at exit
// 3, as spec review refuses, naming the round and the changed units.
func TestEpicReemit_AGraphEditRefusesTheReemit(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	runEpicReview(t, dir, "--milestone", "A")
	rewriteEpicGraph(t, dir, strings.Replace(epicChain, `"title": "D"`, `"title": "D renamed"`, 1))

	_, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--role", "why-auditor")
	require.Equal(t, 3, code, "stderr: %s", stderr)
	msg, hint := epicErrorOf(t, stderr)
	assert.Contains(t, msg, "round 1")
	assert.Contains(t, msg, "units changed: epic:c, epic:d", "c's hash covers d's block too")
	inbox, force := strings.Index(hint, "tp epic review --milestone A --record-inbox"), strings.Index(hint, "tp epic review --milestone A --force")
	require.GreaterOrEqual(t, inbox, 0, hint)
	require.Greater(t, force, inbox, "recording as emitted comes first, the discard second: %s", hint)
	assert.Contains(t, hint, "operator's decision")
}

// TestEpicReemit_ARoleFileEditRefusesTheReemit: the panel is part of the
// emission, so editing a role file between emissions is a change too.
func TestEpicReemit_ARoleFileEditRefusesTheReemit(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	writeEpicRoles(t, dir, map[string]string{"plain": purposeRoles["plain"]})
	runEpicReview(t, dir, "--milestone", "A")
	writeEpicRoles(t, dir, map[string]string{
		"plain": `{"id": "plain", "title": "Plain", "instructions": "Read the epics and the gate twice."}`,
	})

	_, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A")
	require.Equal(t, 3, code, "stderr: %s", stderr)
	msg, _ := epicErrorOf(t, stderr)
	assert.Contains(t, msg, "role files changed: plain")
}

// TestEpicReemit_ForceDiscardsTheEmissionAndItsInboxFiles: --force emits
// over the changed graph and removes the inbox files the old emission named,
// so no row written against the old prompts can be recorded later.
func TestEpicReemit_ForceDiscardsTheEmissionAndItsInboxFiles(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	out := runEpicReview(t, dir, "--milestone", "A")
	require.NotEmpty(t, out.Prompts)
	written := out.Prompts[0].OutputPath
	require.NoError(t, os.WriteFile(written, []byte(`{"severity":"low","finding":"f","location":"epic:d","evidence":"e"}`+"\n"), 0o600))
	rewriteEpicGraph(t, dir, strings.Replace(epicChain, `"title": "D"`, `"title": "D renamed"`, 1))

	stdout, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--force")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "--force")
	var again epicReviewPayload
	require.NoError(t, json.Unmarshal([]byte(stdout), &again), stdout)
	assert.Equal(t, 1, again.Round)
	_, err := os.Stat(written)
	assert.True(t, os.IsNotExist(err), "the old emission's inbox file is removed")

	_, stderr, code = runTP(t, dir, "epic", "review", "--milestone", "A")
	assert.Equal(t, 0, code, "the new emission is the stored one now: %s", stderr)
}

// TestEpicReemit_ForceUnderUnattendedEscalates: discarding an emission is
// the operator's decision, escalated as discard-emission; a --force with
// nothing to discard is accepted there as everywhere.
func TestEpicReemit_ForceUnderUnattendedEscalates(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	_, stderr, code := runTPEnv(t, dir, []string{"TP_UNATTENDED=1"}, "epic", "review", "--milestone", "A", "--force")
	require.Equal(t, 0, code, "nothing to discard: %s", stderr)
	rewriteEpicGraph(t, dir, strings.Replace(epicChain, `"title": "D"`, `"title": "D renamed"`, 1))

	_, stderr, code = runTPEnv(t, dir, []string{"TP_UNATTENDED=1"}, "epic", "review", "--milestone", "A", "--force")
	require.Equal(t, 2, code, "stderr: %s", stderr)
	_, hint := epicErrorOf(t, stderr)
	assert.Contains(t, hint, "--decision discard-emission")
}

// TestEpicReemit_ForceBesideResolveKeepsItsMeaning: beside --resolve, --force
// still re-resolves a row that carries a disposition, and discards nothing.
func TestEpicReemit_ForceBesideResolveKeepsItsMeaning(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	recordEpicRows(t, dir, `{"severity":"low","finding":"f","location":"epic:d","evidence":"e"}`+"\n")
	_, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--resolve", "0", "fixed")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	_, stderr, code = runTP(t, dir, "epic", "review", "--milestone", "A", "--resolve", "0", "wontfix", "e2")
	require.Equal(t, 1, code, "a resolved row needs --force: %s", stderr)

	stdout, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--resolve", "0", "wontfix", "e2", "--force")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Contains(t, stdout, `"status": "wontfix"`)
}

// TestEpicReemit_BatchSkipsAChangedSlice: a batch emission skips a slice
// whose unrecorded emission changed, with the reason emission-changed, and
// still emits the others.
func TestEpicReemit_BatchSkipsAChangedSlice(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	runEpicReview(t, dir, "--milestone", "A")
	rewriteEpicGraph(t, dir, strings.Replace(epicChain, `"title": "D"`, `"title": "D renamed"`, 1))

	out := runEpicBatch(t, dir, "--milestones", "A,B")
	reason := epicSkippedReasons(&out)["A"]
	assert.True(t, strings.HasPrefix(reason, "emission-changed: "), reason)
	assert.Contains(t, reason, "units changed: epic:c, epic:d")
	assert.Contains(t, reason, "tp epic review --milestone A --force")
	require.Len(t, out.Slices, 1, "B still emits")
	assert.Equal(t, 1, out.Slices[0].Round)
}

// TestEpicReemit_AnOldSnapshotIsComparedOnWhatItHolds: a snapshot written
// before roles were kept holds none, so a re-emission over it compares units
// and purposes alone and is not refused for the missing field.
func TestEpicReemit_AnOldSnapshotIsComparedOnWhatItHolds(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	out := runEpicReview(t, dir, "--milestone", "A")
	path := filepath.Join(dir, out.StateDir, "snapshot-round-1.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var snap map[string]any
	require.NoError(t, json.Unmarshal(data, &snap))
	require.Contains(t, snap, "roles", "the emission stores its panel")
	delete(snap, "roles")
	old, err := json.Marshal(snap)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, old, 0o600))

	runEpicReview(t, dir, "--milestone", "A")
}

// TestEpicReemit_ProgressIsNoChange: closing d's bound task and marking c
// done between the emission and its re-emission changes no unit, purpose or
// role: the blocks and the scope fingerprint are blind to status, so the
// re-emission is not refused.
func TestEpicReemit_ProgressIsNoChange(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainThreshold)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n\n## X\n\nx\n"), 0o600))
	task := func(status string) string {
		return `{"version":1,"spec":"spec.md","epic":"d","tasks":[{"id":"t1","title":"T","status":"` + status +
			`","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["## X"]}]}`
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.tasks.json"), []byte(task("open")), 0o600))
	first := runEpicReview(t, dir, "--milestone", "A")
	require.Contains(t, first.Units, "scope:A", "the threshold gives the slice a scope unit, which reads derived states")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.tasks.json"), []byte(task("done")), 0o600))
	_, stderr, code := runTP(t, dir, "epic", "done", "c", "--evidence", "c shipped")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	again := runEpicReview(t, dir, "--milestone", "A")
	assert.Equal(t, first.Round, again.Round)
	assert.Equal(t, first.Units, again.Units)
}
