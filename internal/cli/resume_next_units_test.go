package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nextUnitsOf returns the raw next_units array of a resume result, asserting it
// is present and an array (never null).
func nextUnitsOf(t *testing.T, res map[string]any) []map[string]any {
	t.Helper()
	raw, ok := res["next_units"]
	require.True(t, ok, "resume carries next_units")
	require.NotNil(t, raw, "an empty next_units serializes as [], never null")
	arr, ok := raw.([]any)
	require.True(t, ok, "next_units is an array")
	out := make([]map[string]any, 0, len(arr))
	for _, e := range arr {
		out = append(out, e.(map[string]any))
	}
	return out
}

// unitOf returns next_action.payload.unit — the object that makes next_action a
// rendering of next_units[0] rather than a second opinion (§4.1).
func unitOf(t *testing.T, res map[string]any) map[string]any {
	t.Helper()
	na := res["next_action"].(map[string]any)
	payload, ok := na["payload"].(map[string]any)
	require.True(t, ok, "next_action carries a payload")
	unit, _ := payload["unit"].(map[string]any)
	return unit
}

// assertRendersFirstUnit is test 38's linkage half: next_action names the very
// unit next_units[0] does.
func assertRendersFirstUnit(t *testing.T, res map[string]any) {
	t.Helper()
	units := nextUnitsOf(t, res)
	require.NotEmpty(t, units, "this phase returns at least one unit")
	assert.Equal(t, map[string]any{"kind": units[0]["kind"], "id": units[0]["id"]}, unitOf(t, res))
}

// TestResume_NextUnitsImplement: test 38 — the implement phase returns exactly
// one implement unit carrying {kind, id, brief_command}, round is null, and
// next_action renders it.
func TestResume_NextUnitsImplement(t *testing.T) {
	t.Parallel()
	dir := newPayloadRepo(t, `[{"id":"t1","title":"T","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
	res := resumeResult(t, dir)

	units := nextUnitsOf(t, res)
	require.Len(t, units, 1, "implement runs alone")
	assert.Equal(t, map[string]any{
		"kind":          "implement",
		"id":            "t1",
		"brief_command": "tp next --brief",
	}, units[0], "an entry carries exactly {kind, id, brief_command}")
	assert.Nil(t, res["round"], "implement is not a round-based phase")
	assertRendersFirstUnit(t, res)
}

// TestResume_NextUnitsReviewRoles: test 38 for a round-based phase — one
// concurrent role unit per active role, the collecting round beside them, and
// next_action rendering the first.
func TestResume_NextUnitsReviewRoles(t *testing.T) {
	t.Parallel()
	dir := newPayloadRepo(t, `[]`)
	res := resumeResult(t, dir)

	units := nextUnitsOf(t, res)
	kinds := make([]string, 0, len(units))
	ids := make([]string, 0, len(units))
	for _, u := range units {
		kinds = append(kinds, u["kind"].(string))
		ids = append(ids, u["id"].(string))
		assert.Equal(t, "tp review spec.md --role "+u["id"].(string), u["brief_command"],
			"each role unit asks for its own prompt (v0.36.0 §4.2.3)")
	}
	assert.Equal(t, []string{"review-role", "review-role", "review-role"}, kinds,
		"only the role kinds may share an array")
	assert.Equal(t, []string{"implementer", "tester", "architect"}, ids)
	assert.Equal(t, float64(1), res["round"], "the first round is the one being collected")
	assertRendersFirstUnit(t, res)
}

// TestResume_NextUnitsAuditRoles mirrors the review half for the audit phase.
func TestResume_NextUnitsAuditRoles(t *testing.T) {
	t.Parallel()
	dir := newPayloadRepo(t, `[{"id":"t1","title":"T","status":"done","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
	res := resumeResult(t, dir)

	units := nextUnitsOf(t, res)
	ids := make([]string, 0, len(units))
	for _, u := range units {
		assert.Equal(t, "audit-role", u["kind"])
		assert.Equal(t, "tp audit spec.md --role "+u["id"].(string), u["brief_command"],
			"each role unit asks for its own prompt (v0.36.0 §4.2.3)")
		ids = append(ids, u["id"].(string))
	}
	assert.Equal(t, []string{"spec-coverage", "security", "maintainability-conventions"}, ids)
	assert.Equal(t, float64(1), res["round"])
	assertRendersFirstUnit(t, res)
}

// TestResume_NextUnitsDecompose: decompose is one unit whose subject is the spec
// base name (§3.1.1) and whose brief is tp resume, with a null round.
func TestResume_NextUnitsDecompose(t *testing.T) {
	t.Parallel()
	dir := newPayloadRepo(t, `[]`)
	writeConvergedRounds(t, dir, 2, 0)
	res := resumeResult(t, dir)

	units := nextUnitsOf(t, res)
	require.Len(t, units, 1)
	assert.Equal(t, map[string]any{
		"kind":          "decompose",
		"id":            "spec",
		"brief_command": "tp resume",
	}, units[0])
	assert.Nil(t, res["round"])
	assertRendersFirstUnit(t, res)
}

// TestResume_NextUnitsEmptyOnReleaseAndWhenBlocked: §4.1's empty cases — the
// releasable phase, and a phase whose work is blocked. Both emit [] and leave
// next_action's payload without a unit.
func TestResume_NextUnitsEmptyOnReleaseAndWhenBlocked(t *testing.T) {
	t.Parallel()
	release := newPayloadRepo(t, `[{"id":"t1","title":"T","status":"done","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
	writeConvergedRounds(t, release, 0, 2)
	res := resumeResult(t, release)
	assert.Equal(t, "release", res["phase"])
	assert.Empty(t, nextUnitsOf(t, res), "the cycle is releasable: there is no unit to run")
	assert.Nil(t, unitOf(t, res))

	blocked := newPayloadRepo(t, `[{"id":"t1","title":"T","status":"open","depends_on":["nope"],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
	res = resumeResult(t, blocked)
	assert.Equal(t, "implement", res["phase"])
	assert.Empty(t, nextUnitsOf(t, res), "no task is ready: the phase's work is blocked")
	assert.Nil(t, unitOf(t, res))
}

// TestResume_NextUnitsSurviveCompact: next_units and round are the machine
// surface the driver parses, so --compact strips nothing from them.
func TestResume_NextUnitsSurviveCompact(t *testing.T) {
	t.Parallel()
	dir := newPayloadRepo(t, `[{"id":"t1","title":"T","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
	out, stderr, code := runTP(t, dir, "resume", "--compact")
	require.Equal(t, 0, code, "resume --compact: %s", stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res))

	units := nextUnitsOf(t, res)
	require.Len(t, units, 1)
	assert.Equal(t, map[string]any{
		"kind":          "implement",
		"id":            "t1",
		"brief_command": "tp next --brief",
	}, units[0])
	require.Contains(t, res, "round")
	assertRendersFirstUnit(t, res)
}

// TestResume_ObjectSet pins tp resume's exact top-level object set (§10.8): the
// shape gains next_units, round and last_failure, and nothing else moves.
// guidance is present only in the implement phase, which is why the phase is
// fixed here; last_failure is always present, null when the cycle carries none,
// so a driver parses one shape whatever happened.
func TestResume_ObjectSet(t *testing.T) {
	t.Parallel()
	dir := newPayloadRepo(t, `[{"id":"t1","title":"T","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
	res := resumeResult(t, dir)

	keys := make([]string, 0, len(res))
	for k := range res {
		keys = append(keys, k)
	}
	assert.ElementsMatch(t, []string{
		"phase", "spec", "changes", "kept", "bookkeeping", "guidance",
		"next_units", "round", "last_failure", "next_action", "blockers",
	}, keys)
}

// TestResume_NextUnitsRoundAdvancesWithRecordedRounds: the reported round is the
// one being collected, so a second run of the same phase names the next round —
// which is what the driver substitutes into TP_ROUND and TP_ROUND_DIR.
func TestResume_NextUnitsRoundAdvancesWithRecordedRounds(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\na\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.tasks.json"),
		[]byte(`{"spec":"spec.md","tasks":[]}`), 0o600))

	assert.Equal(t, float64(1), resumeResult(t, dir)["round"])

	findings := filepath.Join(dir, "r1.ndjson")
	require.NoError(t, os.WriteFile(findings, []byte("\n"), 0o600))
	_, stderr, code := runTP(t, dir, "review", "spec.md", "--record", findings)
	require.Equal(t, 0, code, "record round 1: %s", stderr)

	res := resumeResult(t, dir)
	require.Equal(t, "review", res["phase"], "one clean round does not converge the default policy")
	assert.Equal(t, float64(2), res["round"], "round 2 is now the one being collected")
	assert.NotEmpty(t, nextUnitsOf(t, res))
}

// firstRoundDirOf returns (and creates) round 1's own directory inside a test
// repo — the .tp/rounds/<base>/<phase>-r1 the driver hands a unit as
// TP_ROUND_DIR (§3.1.1). Every case below works the first round, so the round
// is fixed here rather than passed.
func firstRoundDirOf(t *testing.T, dir, phase string) string {
	t.Helper()
	p := filepath.Join(dir, ".tp", "rounds", "spec", phase+"-r1")
	require.NoError(t, os.MkdirAll(p, 0o755))
	return p
}

// TestResume_NextUnitsOmitsRolesThatAlreadyAnsweredTheRound is test 45 through
// the command: a resumed round omits the role whose findings file is present and
// wholly parseable, and still returns the role that left a malformed one.
func TestResume_NextUnitsOmitsRolesThatAlreadyAnsweredTheRound(t *testing.T) {
	t.Parallel()
	dir := newPayloadRepo(t, `[]`)
	round1 := firstRoundDirOf(t, dir, "review")
	require.NoError(t, os.WriteFile(filepath.Join(round1, "role-implementer.ndjson"),
		[]byte("{\"id\":\"f1\"}\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(round1, "role-tester.ndjson"),
		[]byte("{\"id\":\"f2\"}\nnot json\n"), 0o600))

	res := resumeResult(t, dir)
	units := nextUnitsOf(t, res)
	ids := make([]string, 0, len(units))
	for _, u := range units {
		ids = append(ids, u["id"].(string))
	}
	assert.Equal(t, []string{"tester", "architect"}, ids,
		"implementer finished the round; tester's file is malformed; architect wrote none")
	assert.Equal(t, float64(1), res["round"])
	assertRendersFirstUnit(t, res)
}

// TestResume_NextUnitsReviewResolveAfterARecordedRound: §4.1 — a recorded round
// whose merged findings are not all disposed returns the single review-resolve
// unit, carrying the round just recorded rather than the next one.
func TestResume_NextUnitsReviewResolveAfterARecordedRound(t *testing.T) {
	t.Parallel()
	dir := newPayloadRepo(t, `[]`)
	writeConvergedRounds(t, dir, 1, 0)
	round1 := firstRoundDirOf(t, dir, "review")
	require.NoError(t, os.WriteFile(filepath.Join(round1, "merged.ndjson"),
		[]byte("{\"id\":\"f1\",\"resolved\":{\"disposition\":\"fixed\"}}\n{\"id\":\"f2\"}\n"), 0o600))

	res := resumeResult(t, dir)
	require.Equal(t, "review", res["phase"], "one clean round does not converge the default policy")
	units := nextUnitsOf(t, res)
	require.Len(t, units, 1, "review-resolve runs alone")
	assert.Equal(t, map[string]any{
		"kind":          "review-resolve",
		"id":            "spec",
		"brief_command": "tp review spec.md --status",
	}, units[0])
	assert.Equal(t, float64(1), res["round"], "the round just recorded, not the one being collected")
	assertRendersFirstUnit(t, res)
}

// TestResume_NextUnitsAuditFixAfterARecordedRound mirrors it on the audit side:
// one audit-fix unit for the first row that is neither PASS nor disposed, keyed
// role:item_id.
func TestResume_NextUnitsAuditFixAfterARecordedRound(t *testing.T) {
	t.Parallel()
	dir := newPayloadRepo(t, `[{"id":"t1","title":"T","status":"done","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
	writeConvergedRounds(t, dir, 2, 1)
	round1 := firstRoundDirOf(t, dir, "audit")
	require.NoError(t, os.WriteFile(filepath.Join(round1, "merged.ndjson"), []byte(
		"{\"role\":\"spec-coverage\",\"item_id\":\"i1\",\"status\":\"PASS\"}\n"+
			"{\"role\":\"security\",\"item_id\":\"i2\",\"status\":\"FAIL\"}\n"), 0o600))

	res := resumeResult(t, dir)
	require.Equal(t, "audit", res["phase"])
	units := nextUnitsOf(t, res)
	require.Len(t, units, 1, "audit-fix runs alone: one finding at a time")
	assert.Equal(t, map[string]any{
		"kind":          "audit-fix",
		"id":            "security:i2",
		"brief_command": "tp audit spec.md --status",
	}, units[0], "the PASS row is not a finding to fix")
	assert.Equal(t, float64(1), res["round"])
	assertRendersFirstUnit(t, res)
}

// TestResume_NextUnitsRecordUnitOnceEveryRoleAnswered is test 45a through the
// command: once every role of the panel has written a parseable findings file
// for the collecting round, the oracle hands the driver that round's single
// record unit instead of emptying next_units and stopping the run with no-units.
func TestResume_NextUnitsRecordUnitOnceEveryRoleAnswered(t *testing.T) {
	t.Parallel()
	dir := newPayloadRepo(t, `[]`)
	round1 := firstRoundDirOf(t, dir, "review")
	for _, role := range []string{"implementer", "tester", "architect"} {
		require.NoError(t, os.WriteFile(filepath.Join(round1, "role-"+role+".ndjson"),
			[]byte("{\"id\":\"f1\"}\n"), 0o600))
	}

	res := resumeResult(t, dir)
	units := nextUnitsOf(t, res)
	require.Len(t, units, 1, "a record unit runs alone")
	assert.Equal(t, map[string]any{
		"kind": "review-record",
		"id":   "1",
		"brief_command": "[ -f $TP_ROUND_DIR/merged.ndjson ] || " +
			"tp review --merge $TP_ROUND_DIR/role-*.ndjson -o $TP_ROUND_DIR/merged.ndjson; " +
			"tp review spec.md --record $TP_ROUND_DIR/merged.ndjson",
	}, units[0], "the id is the round number the driver also passes as TP_ROUND")
	assert.Equal(t, float64(1), res["round"])
	assertRendersFirstUnit(t, res)
}
