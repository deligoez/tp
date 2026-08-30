package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// signalFieldKeys are §2.5's three fields, named once so the absence assertions
// cannot drift from the presence ones.
var signalFieldKeys = []string{"role_streaks", "spec_coverage_clean_rounds", "divergence"}

// auditSignalProject lays out a project whose auditor corpus is a real
// .tp/auditors directory rather than the built-in one, so a test can edit the
// corpus after a round is recorded and move the hash §2.4's condition 5
// compares. The .git marker bounds DiscoverTPDir inside the temp dir.
func auditSignalProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o750))
	audDir := filepath.Join(dir, ".tp", "auditors")
	require.NoError(t, os.MkdirAll(audDir, 0o750))
	for _, id := range []string{"spec-coverage", "go-safety"} {
		require.NoError(t, os.WriteFile(filepath.Join(audDir, id+".json"),
			[]byte(`{"id":"`+id+`","title":"`+id+`","instructions":"You audit."}`), 0o600))
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
	return dir
}

// signalRow renders one recorded audit row carrying a role, so no fixture here
// trips the roleless-row advisory unless it means to.
func signalRow(role, status string) string {
	return `{"item_id":"` + role + `-item","role":"` + role + `","status":"` + status + `"}`
}

// recordSignalRound records one audit round from the given rows and returns the
// raw stdout, so the raw-JSON assertions (§2.2's emitted [] and §2.3's emitted
// null) can be made on the bytes rather than on a decoded map.
func recordSignalRound(t *testing.T, dir string, flags []string, rows ...string) (stdout, stderr string) {
	t.Helper()
	body := ""
	if len(rows) > 0 {
		body = strings.Join(rows, "\n") + "\n"
	}
	f := filepath.Join(dir, "results.ndjson")
	require.NoError(t, os.WriteFile(f, []byte(body), 0o600))
	args := append([]string{"audit", "spec.md", "--record", f}, flags...)
	stdout, stderr, code := runTP(t, dir, args...)
	require.Equal(t, 0, code, "record failed: %s", stderr)
	return stdout, stderr
}

// decodeSignal parses an audit payload.
func decodeSignal(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "payload: %s", stdout)
	return out
}

// divergingFixture records the two rounds §2.4's conditions 1-5 all hold over:
// spec-coverage is all-PASS in both, so its streak reaches the default
// audit_clean_rounds of 2, while go-safety holds an open finding in the latest.
// It returns the dir and the raw stdout of the second --record invocation.
func divergingFixture(t *testing.T, recordFlags ...string) (dir, secondRecord string) {
	t.Helper()
	dir = auditSignalProject(t)
	recordSignalRound(t, dir, nil, signalRow("spec-coverage", "PASS"), signalRow("go-safety", "PASS"))
	secondRecord, _ = recordSignalRound(t, dir, recordFlags,
		signalRow("spec-coverage", "PASS"), signalRow("go-safety", "FAIL"))
	return dir, secondRecord
}

// Test 27 — --record computes the signal over the just-recorded round: the
// invocation that completes spec-coverage's streak emits divergence itself,
// without a following --status. An implementation computing the signal before
// recordAuditRoundEntry reports the previous round and fails here.
func TestAuditSignal_RecordComputesOverJustRecordedRound(t *testing.T) {
	_, stdout := divergingFixture(t)
	out := decodeSignal(t, stdout)

	assert.Equal(t, float64(2), out["round"], "the signal is read off the second round")
	assert.Equal(t, float64(2), out["spec_coverage_clean_rounds"],
		"the streak counts the round just recorded")
	assert.Equal(t, []any{
		map[string]any{"role": "spec-coverage", "consecutive_clean": float64(2), "open": float64(0)},
		map[string]any{"role": "go-safety", "consecutive_clean": float64(0), "open": float64(1)},
	}, out["role_streaks"])

	div, ok := out["divergence"].(map[string]any)
	require.True(t, ok, "divergence is emitted on this same --record invocation: %s", stdout)
	assert.Equal(t, "spec-coverage clean 2 rounds; 1 finding open from other roles", div["message"])
	assert.Equal(t, engine.DivergenceHint, div["hint"])
	assert.Equal(t, float64(1), div["other_roles_open"])
	assert.Equal(t, []any{"go-safety"}, div["open_roles"])
	assert.Equal(t, float64(0), div["unattributed_open"])
}

// Test 28 — tp audit --status --check carries all three fields on a
// non-converged fixture that emits divergence and exits 1. --check changes only
// the exit code, so the payload it carries is the payload --status carries
// without the flag; an implementation writing the fields after the exit-code
// branch emits nothing here.
func TestAuditSignal_StatusCheckCarriesTheSamePayload(t *testing.T) {
	dir, _ := divergingFixture(t)

	plain, stderr, code := runTP(t, dir, "audit", "spec.md", "--status")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	checked, stderr, code := runTP(t, dir, "audit", "spec.md", "--status", "--check")
	require.Equal(t, 1, code, "a non-converged sequence still fails --check: %s", stderr)

	plainOut, checkedOut := decodeSignal(t, plain), decodeSignal(t, checked)
	for _, key := range signalFieldKeys {
		require.Contains(t, checkedOut, key, "--status --check carries %s", key)
		assert.Equal(t, plainOut[key], checkedOut[key],
			"--check changes only the exit code, never %s", key)
	}
	div := checkedOut["divergence"].(map[string]any)
	assert.Equal(t, "spec-coverage clean 2 rounds; 1 finding open from other roles", div["message"])
	assert.Equal(t, engine.DivergenceHint, div["hint"])
}

// Test 10 (CLI half) — §2.1's advisory fires on both outputs over the same
// missing-file fixture. Round 1 is recorded normally and its rows file is then
// deleted; round 2 is recorded under the same corpus and leaves spec-coverage's
// streak open, so the walk reaches round 1 on both paths. On --record the
// no-rows round is necessarily below the just-recorded one, whose file always
// exists.
func TestAuditSignal_MissingRoundFileAdvisoryOnBothOutputs(t *testing.T) {
	dir := auditSignalProject(t)
	recordSignalRound(t, dir, nil, signalRow("spec-coverage", "PASS"))
	require.NoError(t, os.Remove(filepath.Join(dir, ".tp-review", "spec", "audit-round-1.ndjson")))

	const advisory = "round 1 file audit-round-1.ndjson is missing; skipping its rows"

	recordOut, recordErr := recordSignalRound(t, dir, nil, signalRow("spec-coverage", "PASS"))
	assert.Contains(t, recordErr, advisory, "--record emits the advisory")
	assert.Equal(t, float64(1), decodeSignal(t, recordOut)["spec_coverage_clean_rounds"],
		"the skipped round ends the streak at the just-recorded one")

	statusOut, statusErr, code := runTP(t, dir, "audit", "spec.md", "--status")
	require.Equal(t, 0, code)
	assert.Contains(t, statusErr, advisory, "--status emits the same advisory")
	assert.Equal(t, float64(1), decodeSignal(t, statusOut)["spec_coverage_clean_rounds"])
}

// Test 14 (CLI half) — a corpus change made after the latest recorded round
// suppresses divergence through condition 5, the half §2.1's fourth no-rows
// cause cannot reach: every stored hash still matches its neighbours, so the
// streaks are unaffected and only the object is withheld. roles_stale reads true
// on the same payload.
func TestAuditSignal_CorpusEditedAfterLatestRoundSuppressesDivergence(t *testing.T) {
	dir, secondRecord := divergingFixture(t)
	require.Contains(t, decodeSignal(t, secondRecord), "divergence",
		"the fixture must satisfy conditions 1-4 before the corpus moves")

	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "auditors", "go-safety.json"),
		[]byte(`{"id":"go-safety","title":"go-safety","instructions":"You audit deeply."}`), 0o600))

	stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--status")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	out := decodeSignal(t, stdout)
	assert.Equal(t, true, out["roles_stale"])
	assert.NotContains(t, out, "divergence", "condition 5 withholds the object")
	assert.Equal(t, float64(2), out["spec_coverage_clean_rounds"],
		"the streaks are computed against the latest round's own stored hash and are unaffected")
	assert.NotEmpty(t, out["role_streaks"])
}

// Test 17 (CLI half) — role_streaks reaches the payload as an emitted empty
// array and spec_coverage_clean_rounds as an emitted null, asserted against the
// raw JSON so the nil-slice serialization this repository records as a recurring
// defect fails the test. Both outputs are checked, over two of §2.2's states:
// no recorded round at all (--status) and a latest round recorded with zero rows
// (--record, then --status over it).
func TestAuditSignal_EmptyStatesAreEmittedArrayAndNull(t *testing.T) {
	dir := auditSignalProject(t)

	assertEmptySignal := func(what, stdout string) {
		t.Helper()
		assert.Contains(t, stdout, `"role_streaks": []`, "%s emits an empty array", what)
		assert.NotContains(t, stdout, `"role_streaks": null`, "%s must not emit a nil slice", what)
		assert.Contains(t, stdout, `"spec_coverage_clean_rounds": null`,
			"%s emits the key with a null value", what)
		assert.NotContains(t, stdout, `"divergence"`, "%s emits no divergence", what)
	}

	noRounds, stderr, code := runTP(t, dir, "audit", "spec.md", "--status")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assertEmptySignal("--status with no recorded round", noRounds)

	zeroRows, _ := recordSignalRound(t, dir, nil)
	assertEmptySignal("--record of a round with zero rows", zeroRows)

	afterZeroRows, stderr, code := runTP(t, dir, "audit", "spec.md", "--status")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assertEmptySignal("--status over a zero-row latest round", afterZeroRows)
}

// Test 22 (CLI half) — divergence is an omitted key on the emitted payload, not
// a JSON null, in both states that withhold it: a spec-coverage streak below the
// threshold, and a streak meeting it with no other role holding an open finding.
func TestAuditSignal_DivergenceKeyIsOmittedNotNull(t *testing.T) {
	below := auditSignalProject(t)
	belowOut, _ := recordSignalRound(t, below,
		nil, signalRow("spec-coverage", "PASS"), signalRow("go-safety", "FAIL"))
	assert.NotContains(t, belowOut, `"divergence"`,
		"a streak of 1 is below the required 2: %s", belowOut)
	assert.Equal(t, float64(1), decodeSignal(t, belowOut)["spec_coverage_clean_rounds"])

	quiet := auditSignalProject(t)
	recordSignalRound(t, quiet, nil, signalRow("spec-coverage", "PASS"), signalRow("go-safety", "PASS"))
	quietOut, _ := recordSignalRound(t, quiet,
		nil, signalRow("spec-coverage", "PASS"), signalRow("go-safety", "PASS"))
	assert.NotContains(t, quietOut, `"divergence"`,
		"no other role holds an open finding: %s", quietOut)
	assert.Equal(t, float64(2), decodeSignal(t, quietOut)["spec_coverage_clean_rounds"])

	statusOut, _, code := runTP(t, quiet, "audit", "spec.md", "--status")
	require.Equal(t, 0, code)
	assert.NotContains(t, statusOut, `"divergence"`, "the same holds on --status")
}

// Test 24 (CLI half) — condition 4 withholds the object where conditions 1, 2, 3
// and 5 all hold: at a task-file audit_clean_rounds of 0 reaching the resolver,
// engine.Converged reduces to "not stale", so the gate is already open while the
// latest round holds a non-spec-coverage open finding. divergence is absent and
// tp audit --status --check exits 0.
func TestAuditSignal_ConvergedGateWithholdsDivergence(t *testing.T) {
	dir := auditSignalProject(t)
	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	taskFile := filepath.Join(dir, "spec.tasks.json")
	data, err := os.ReadFile(taskFile)
	require.NoError(t, err)
	var tf map[string]any
	require.NoError(t, json.Unmarshal(data, &tf))
	tf["workflow"] = map[string]any{"audit_clean_rounds": 0}
	patched, err := json.Marshal(tf)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(taskFile, patched, 0o600))

	stdout, _ := recordSignalRound(t, dir, nil,
		signalRow("spec-coverage", "PASS"), signalRow("go-safety", "FAIL"))
	out := decodeSignal(t, stdout)
	require.Equal(t, float64(0), out["required_clean_rounds"], "the 0 must reach the resolver")
	assert.Equal(t, true, out["converged"])
	assert.Equal(t, float64(1), out["spec_coverage_clean_rounds"], "condition 1 holds at a threshold of 0")
	assert.NotContains(t, out, "divergence", "condition 4 withholds the object beside an open gate")
	assert.NotContains(t, stdout, `"divergence"`)

	statusOut, stderr, code := runTP(t, dir, "audit", "spec.md", "--status", "--check")
	assert.Equal(t, 0, code, "a converged sequence passes --check: %s", stderr)
	assert.NotContains(t, statusOut, `"divergence"`)
	assert.Contains(t, statusOut, `"role_streaks": [`, "the other two fields are still emitted")
	assert.Contains(t, statusOut, `"spec_coverage_clean_rounds": 1`)
}

// Test 29 — --compact keeps all three whole on both outputs, with
// divergence.message and divergence.hint byte-identical to their non-compact
// values, and the audit fields --compact already drops stay dropped. The
// null-valued spec_coverage_clean_rounds key needs its own fixture, since
// divergence is emitted only when that value is non-null.
func TestAuditSignal_CompactKeepsAllThreeWhole(t *testing.T) {
	// --record: two fixtures built the same way, one recorded with --compact.
	_, fullRecord := divergingFixture(t)
	_, compactRecord := divergingFixture(t, "--compact")
	fullRecordOut, compactRecordOut := decodeSignal(t, fullRecord), decodeSignal(t, compactRecord)
	for _, key := range signalFieldKeys {
		require.Contains(t, compactRecordOut, key, "--record --compact keeps %s", key)
		assert.Equal(t, fullRecordOut[key], compactRecordOut[key],
			"--record --compact keeps %s whole", key)
	}
	assert.NotContains(t, compactRecordOut, "harness_stale",
		"the audit fields --compact already drops are unchanged")

	// --status over the same state, with and without --compact.
	dir, _ := divergingFixture(t)
	fullStatus, stderr, code := runTP(t, dir, "audit", "spec.md", "--status")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	compactStatus, stderr, code := runTP(t, dir, "audit", "spec.md", "--status", "--compact")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	fullStatusOut, compactStatusOut := decodeSignal(t, fullStatus), decodeSignal(t, compactStatus)
	for _, key := range signalFieldKeys {
		require.Contains(t, compactStatusOut, key, "--status --compact keeps %s", key)
		assert.Equal(t, fullStatusOut[key], compactStatusOut[key],
			"--status --compact keeps %s whole", key)
	}
	assert.NotContains(t, compactStatusOut, "harness_stale")
	assert.NotContains(t, compactStatusOut, "overlap_report")

	compactDiv := compactStatusOut["divergence"].(map[string]any)
	assert.Equal(t, "spec-coverage clean 2 rounds; 1 finding open from other roles", compactDiv["message"])
	assert.Equal(t, engine.DivergenceHint, compactDiv["hint"])

	// The null-valued key survives --compact on a fixture that emits no
	// divergence at all.
	quiet := auditSignalProject(t)
	quietStatus, stderr, code := runTP(t, quiet, "audit", "spec.md", "--status", "--compact")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Contains(t, quietStatus, `"spec_coverage_clean_rounds": null`,
		"--compact keeps the key when its value is null")
	assert.Contains(t, quietStatus, `"role_streaks": []`)
	assert.NotContains(t, quietStatus, `"divergence"`)

	quietRecord, _ := recordSignalRound(t, quiet, []string{"--compact"})
	assert.Contains(t, quietRecord, `"spec_coverage_clean_rounds": null`)
	assert.Contains(t, quietRecord, `"role_streaks": []`)
}

// Test 31 — the three fields appear nowhere else: not on tp audit --merge, not
// on tp audit <spec> prompt emission, and not on tp review <spec> --record or
// tp review <spec> --status, which pins the review-side half of §2.5's absence
// rule (Non-Goal 3).
func TestAuditSignal_AbsentFromEveryOtherOutput(t *testing.T) {
	dir, _ := divergingFixture(t)
	rows := filepath.Join(dir, "results.ndjson")

	assertNoSignal := func(what, stdout string) {
		t.Helper()
		for _, key := range signalFieldKeys {
			assert.NotContains(t, stdout, `"`+key+`"`, "%s emits no %s", what, key)
		}
	}

	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0o600))
	prompt, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "a.go")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assertNoSignal("tp audit <spec> prompt emission", prompt)

	merged, stderr, code := runTP(t, dir, "audit", "--merge", rows, "-o", filepath.Join(dir, "merged.ndjson"))
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assertNoSignal("tp audit --merge", merged)

	reviewRecord, stderr, code := runTP(t, dir, "review", "spec.md", "--record", rows)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assertNoSignal("tp review <spec> --record", reviewRecord)

	reviewStatus, stderr, code := runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assertNoSignal("tp review <spec> --status", reviewStatus)
}

// Test 30 — the guard test for §2.6. The fixture is the diverging one, whose
// latest recorded round holds only a non-spec-coverage finding, and every gate
// assertion is made on an invocation that itself emits divergence: that pairing
// is §2.6's claim, and asserting the gate on a quiet fixture would prove
// nothing. It must fail any implementation that lets the divergence signal reach
// the convergence arithmetic or the next_action precedence.
//
// §5's Non-Goals 1, 2, 4, 5, 6 and 8 are all restatements of "the gate is
// untouched". Non-Goal 4 is the next_action half below and Non-Goal 10 is
// TestAuditGate_ResumeIsUnchangedByTheDivergence. Non-Goals 1 and 6 — no scope
// field on a row, no per-role threshold — are guarded by construction, since
// nothing in this release parses either, and the shape assertion in
// TestAuditGate_NoEscapeHatchFromTheGate is what would notice one arriving;
// that test also pins Non-Goals 2 and 5, whose absence is assertable as a
// rejected knob and a rejected flag. Non-Goal 8 is guarded by construction too:
// tp stores no acceptance of a divergence, so the only assertion available is
// the --status --check exit code beside the emitted object, made here.
func TestAuditGate_DivergenceReachesNeitherConvergenceNorNextAction(t *testing.T) {
	const fixDirective = "address the findings, then re-audit: tp audit spec.md --record <file>"
	dir, record := divergingFixture(t)
	specPath := filepath.Join(dir, "spec.md")

	// --record: the invocation that emits divergence reports an unclean,
	// non-converged round and branch 2 of the audit precedence.
	recordOut := decodeSignal(t, record)
	require.Contains(t, recordOut, "divergence", "the fixture must diverge: %s", record)
	assert.Equal(t, false, recordOut["clean"],
		"a round holding only non-spec-coverage findings is still not clean")
	assert.Equal(t, false, recordOut["converged"])
	assert.Equal(t, float64(0), recordOut["consecutive_clean"],
		"and still counts toward no streak")
	assert.Equal(t, fixDirective, recordOut["next_action"],
		"next_action gains no divergence branch on the invocation that emits divergence")

	// --status --check over the same state: still exit 1, still the same
	// directive, with the divergence object on the very same payload.
	checked, stderr, code := runTP(t, dir, "audit", "spec.md", "--status", "--check")
	require.Equal(t, 1, code, "the gate stays shut over an unclean round: %s", stderr)
	status := decodeSignal(t, checked)
	require.Contains(t, status, "divergence",
		"the exit code and the object are asserted on one invocation: %s", checked)
	assert.Equal(t, false, status["converged"])
	assert.Equal(t, float64(0), status["consecutive_clean"])
	assert.Equal(t, fixDirective, status["next_action"])

	// The stored per-round clean flag, read from state.json rather than from a
	// payload, so an implementation that only rewrote the reported value fails.
	st, err := engine.LoadReviewState(specPath)
	require.NoError(t, err)
	require.Len(t, st.AuditRounds, 2)
	assert.True(t, st.AuditRounds[0].Clean, "round 1 is stored clean")
	assert.False(t, st.AuditRounds[1].Clean,
		"round 2 holds only non-spec-coverage findings and is stored unclean")

	// The three engine entry points §2.6 names, called directly over the very
	// rounds the diverging fixture recorded.
	specHash, err := engine.SpecHash(specPath)
	require.NoError(t, err)
	assert.Equal(t, 0, engine.ConsecutiveClean(st.AuditRounds))
	assert.False(t, engine.Converged(st.AuditRounds, 2, specHash),
		"two rounds, the latest of them unclean, is not two consecutive clean rounds")
	assert.Equal(t, fixDirective, engine.AuditNextAction("spec.md", false, true),
		"branch 2 of the three-state audit precedence is unchanged")
}

// Test 30 (Non-Goal 10) — tp resume is unchanged. The fixture is taken all the
// way to the audit phase, because a divergence branch added to the oracle would
// live in exactly that payload: resume names the next audit round and carries
// none of §2.5's three fields while the recorded audit state diverges.
func TestAuditGate_ResumeIsUnchangedByTheDivergence(t *testing.T) {
	dir, record := divergingFixture(t)
	require.Contains(t, decodeSignal(t, record), "divergence", "the fixture must diverge")

	// Reach the audit phase: a task file, a converged review, and no open task.
	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	empty := filepath.Join(dir, "review.ndjson")
	require.NoError(t, os.WriteFile(empty, nil, 0o600))
	for range 2 {
		_, stderr, code = runTP(t, dir, "review", "spec.md", "--record", empty)
		require.Equal(t, 0, code, "stderr: %s", stderr)
	}
	_, stderr, code = runTP(t, dir, "add",
		`{"id":"t1","title":"T","estimate_minutes":5,"acceptance":"Done.","source_sections":["# Spec"]}`)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	_, stderr, code = runTP(t, dir, "done", "t1", "--", "- implemented and verified")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	stdout, stderr, code := runTP(t, dir, "resume")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	out := decodeSignal(t, stdout)
	require.Equal(t, "audit", out["phase"], "the oracle must reach the audit phase: %s", stdout)
	for _, key := range signalFieldKeys {
		assert.NotContains(t, stdout, `"`+key+`"`, "tp resume carries no %s (Non-Goal 10)", key)
	}
	next, ok := out["next_action"].(map[string]any)
	require.True(t, ok, "resume payload: %s", stdout)
	assert.Equal(t, "tp audit spec.md", next["command"],
		"resume still names the next audit round beside a diverging state")
}

// Test 30 (Non-Goals 1, 2, 5 and 6) — the gate has no escape hatch and the
// signal carries no gate input. There is no audit_converge_on knob to flip
// (Non-Goal 2 of v0.33.0, deferred again by v0.34.0 §11 and still carried in
// spec/0.34.0-candidates.md); a per-role streak entry carries the three keys of
// §2.2 and nothing else, so neither a per-role threshold (Non-Goal 6) nor a
// scope label (Non-Goal 1) can arrive unnoticed.
//
// v0.35.0 §3.3 adds the audit-side --resolve this test used to pin as absent,
// because audit-fix has no other way to record the no-code-change outcome. What
// Non-Goal 5 actually protects survives that and is what is asserted here
// instead: a disposition is not a way to park a finding past the gate.
// v0.35.0 §6.3 states the mechanism — tp audit --record counts every row whose
// status is not exactly PASS and reads no disposition at all — so disposing the
// open row leaves the round's finding, the role streak and --check exactly
// where they were.
func TestAuditGate_NoEscapeHatchFromTheGate(t *testing.T) {
	dir, record := divergingFixture(t)

	stdout, stderr, code := runTP(t, dir, "set", "--workflow", "--project", "audit_converge_on=blocking")
	assert.NotEqual(t, 0, code, "audit_converge_on is not a workflow field")
	assert.Contains(t, stdout+stderr, "unknown workflow field: audit_converge_on")

	// divergingFixture left the second round's rows in results.ndjson: one PASS
	// and go-safety's open FAIL. Dispose the FAIL and re-record from the same
	// file the round was recorded from.
	results := filepath.Join(dir, "results.ndjson")
	_, stderr, code = runTP(t, dir, "audit", results, "--resolve", "go-safety:go-safety-item", "wontfix", "parked")
	require.Equal(t, 0, code, "v0.35.0 §3.3 exposes an audit-side --resolve: %s", stderr)

	reRecord, stderr, code := runTP(t, dir, "audit", "spec.md", "--record", results)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Contains(t, decodeSignal(t, reRecord)["role_streaks"],
		map[string]any{"role": "go-safety", "consecutive_clean": float64(0), "open": float64(1)},
		"a wontfix disposition does not clear go-safety's open finding")

	_, stderr, code = runTP(t, dir, "audit", "spec.md", "--status", "--check")
	assert.Equal(t, 1, code, "the gate is still shut over the disposed finding: %s", stderr)

	streaks, ok := decodeSignal(t, record)["role_streaks"].([]any)
	require.True(t, ok, "record payload: %s", record)
	require.NotEmpty(t, streaks)
	for _, entry := range streaks {
		e, ok := entry.(map[string]any)
		require.True(t, ok)
		assert.Len(t, e, 3, "a streak entry carries role, consecutive_clean and open only: %v", e)
		for _, key := range []string{"role", "consecutive_clean", "open"} {
			assert.Contains(t, e, key)
		}
	}
}
