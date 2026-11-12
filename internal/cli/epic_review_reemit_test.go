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
	inbox, force := strings.Index(hint, "tp epic review --milestone A --record-inbox"), strings.Index(hint, "tp epic review --milestone A --discard")
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

// TestEpicReemit_DiscardRemovesTheEmissionAndItsFiles: --discard removes the
// unrecorded emission and the role files it named and emits nothing; the
// plain emission after it is a fresh round 1 over the changed graph, so no
// row written against the old prompts can be recorded later.
func TestEpicReemit_DiscardRemovesTheEmissionAndItsFiles(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	out := runEpicReview(t, dir, "--milestone", "A")
	require.NotEmpty(t, out.Prompts)
	written := out.Prompts[0].OutputPath
	require.NoError(t, os.WriteFile(written, []byte(`{"severity":"low","finding":"f","location":"epic:d","evidence":"e"}`+"\n"), 0o600))
	rewriteEpicGraph(t, dir, strings.Replace(epicChain, `"title": "D"`, `"title": "D renamed"`, 1))

	stdout, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--discard")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "--discard: discarded epic review round 1's unrecorded emission of --milestone A; removed 1 role file(s) it named")
	assert.NotContains(t, stdout, `"prompts"`, "a discard emits nothing")
	_, err := os.Stat(written)
	assert.True(t, os.IsNotExist(err), "the old emission's file is removed")
	_, err = os.Stat(filepath.Join(dir, out.StateDir, "snapshot-round-1.json"))
	assert.True(t, os.IsNotExist(err), "the emission's snapshot is removed")

	again := runEpicReview(t, dir, "--milestone", "A")
	assert.Equal(t, 1, again.Round)
}

// TestEpicReemit_DiscardUnderUnattendedEscalates: discarding an emission is
// the operator's decision, escalated as discard-emission, and writes nothing.
func TestEpicReemit_DiscardUnderUnattendedEscalates(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	out := runEpicReview(t, dir, "--milestone", "A")
	_, stderr, code := runTPEnv(t, dir, []string{"TP_UNATTENDED=1"}, "epic", "review", "--milestone", "A", "--discard")
	require.Equal(t, 2, code, "stderr: %s", stderr)
	_, hint := epicErrorOf(t, stderr)
	assert.Contains(t, hint, "tp escalate --decision discard-emission")
	_, err := os.Stat(filepath.Join(dir, out.StateDir, "snapshot-round-1.json"))
	assert.NoError(t, err, "the emission stays")
}

// TestEpicReemit_DiscardWithNoEmissionIsAStateError: with no unrecorded
// emission there is nothing to discard: exit 4, nothing written.
func TestEpicReemit_DiscardWithNoEmissionIsAStateError(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	_, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--discard")
	require.Equal(t, 4, code, "stderr: %s", stderr)
	_, err := os.Stat(filepath.Join(dir, ".tp", "epic-review"))
	assert.True(t, os.IsNotExist(err), "no state directory is created")
}

// TestEpicReemit_DiscardStaysOneSliceAndOneMode: --discard beside a batch
// flag or any other mode or narrowing flag exits 2, tp epic review's
// invalid-combination code, and the emission stays: each call would succeed
// alone, so the refusal is the combination's.
func TestEpicReemit_DiscardStaysOneSliceAndOneMode(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	out := runEpicReview(t, dir, "--milestone", "A")
	snapshot := filepath.Join(dir, out.StateDir, "snapshot-round-1.json")
	for name, args := range map[string][]string{
		"milestones":   {"--milestones", "A"},
		"all":          {"--all"},
		"record":       {"--milestone", "A", "--record", "rows.ndjson"},
		"record-inbox": {"--milestone", "A", "--record-inbox"},
		"resolve":      {"--milestone", "A", "--resolve", "0", "fixed"},
		"resolve-all":  {"--milestone", "A", "--resolve-all", "fixed"},
		"status":       {"--milestone", "A", "--status"},
		"check":        {"--milestone", "A", "--status", "--check"},
		"history":      {"--milestone", "A", "--history"},
		"of-epic":      {"--history", "--of-epic", "a"},
		"report":       {"--milestone", "A", "--report"},
		"grant-round":  {"--milestone", "A", "--grant-round"},
		"role":         {"--milestone", "A", "--role", "why-auditor"},
		"force":        {"--milestone", "A", "--force"},
	} {
		_, stderr, code := runTP(t, dir, append(append([]string{"epic", "review"}, args...), "--discard")...)
		assert.Equal(t, 2, code, "%s: stderr: %s", name, stderr)
		_, err := os.Stat(snapshot)
		require.NoError(t, err, "%s: the emission stays", name)
	}
}

// TestEpicReview_GrantRoundStaysAlone: --grant-round beside --record-inbox or
// --discard exits 2 like beside every other mode.
func TestEpicReview_GrantRoundStaysAlone(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	for _, flag := range []string{"--record-inbox", "--discard"} {
		_, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--grant-round", flag)
		assert.Equal(t, 2, code, "%s: stderr: %s", flag, stderr)
		assert.Contains(t, stderr, "--grant-round takes no "+flag)
	}
}

// TestEpicReemit_ForceWithoutResolveIsRefused: --force only re-resolves on
// epic review, so without --resolve or --resolve-all it exits 2, naming
// --discard for a discard, and writes nothing.
func TestEpicReemit_ForceWithoutResolveIsRefused(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	runEpicReview(t, dir, "--milestone", "A", "--force")
	rewriteEpicGraph(t, dir, strings.Replace(epicChain, `"title": "D"`, `"title": "D renamed"`, 1))
	_, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--force")
	require.Equal(t, 3, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "its emission changed since")
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
	assert.Contains(t, reason, "tp epic review --milestone A --discard")
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
