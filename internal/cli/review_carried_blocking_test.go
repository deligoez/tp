package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The spec/backlog/carried-rows-block.md fixtures: a two-section spec and the
// BUGS.md row, a high finding nobody fixes or accepts.
const (
	carriedSpec      = "# Spec\n\n## 1. Scope\n\nThe tool reads one file.\n\n## 2. Rules\n\nRule 1 applies to every row.\n"
	carriedHighRow   = `{"role":"r","severity":"high","finding":"Rule 1 is ambiguous","location":"§2","evidence":"line 9","class":"ambiguity"}` + "\n"
	carriedRoundFile = ".tp-review/spec/review-round-1.ndjson"
)

// carriedRepro is the BUGS.md reproduction: round 1 records the high row, and
// rounds 2 and 3 record nothing, the spec unchanged throughout. Three rounds
// reach the default cap of 3.
func carriedRepro(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeSpec(t, dir, carriedSpec)
	for _, rows := range []string{carriedHighRow, "", ""} {
		_, stderr, code := recordRound(t, dir, rows)
		require.Equal(t, 0, code, "record: %s", stderr)
	}
	return dir
}

// reviewStatusCheck runs tp review spec.md --status --check and returns its
// payload and exit code.
func reviewStatusCheck(t *testing.T, dir string) (out map[string]any, code int) {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--status", "--check")
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "status payload: %s", stderr)
	return out, code
}

// carriedRows returns the carried_blocking_rows of a payload.
func carriedRows(t *testing.T, out map[string]any) []map[string]any {
	t.Helper()
	raw, _ := out["carried_blocking_rows"].([]any)
	rows := make([]map[string]any, 0, len(raw))
	for _, r := range raw {
		m, ok := r.(map[string]any)
		require.True(t, ok, "a carried row is an object: %v", r)
		rows = append(rows, m)
	}
	return rows
}

// TestCarriedBlocking_EmptyRoundsDoNotConvergePastAnUnresolvedHigh is the
// BUGS.md repro: the high is carried under "DO NOT re-report", so the empty
// rounds after it say nothing about it, and the loop is not converged while
// it stands. --status names the row and how it clears.
func TestCarriedBlocking_EmptyRoundsDoNotConvergePastAnUnresolvedHigh(t *testing.T) {
	t.Parallel()
	dir := carriedRepro(t)

	out, code := reviewStatusCheck(t, dir)
	assert.Equal(t, 1, code, "--check exits 1 while a carried blocking row stands")
	assert.Equal(t, false, out["converged"])
	assert.Equal(t, false, out["done"])
	assert.Equal(t, float64(1), out["carried_blocking"])
	rows := carriedRows(t, out)
	require.Len(t, rows, 1)
	assert.Equal(t, float64(1), rows[0]["round"])
	assert.Equal(t, float64(0), rows[0]["index"])
	assert.Equal(t, "§2", rows[0]["location"])
	assert.Equal(t, "high", rows[0]["severity"])
	assert.Equal(t, "Rule 1 is ambiguous", rows[0]["finding"])
	assert.Equal(t, carriedRoundFile, rows[0]["file"])

	next, _ := out["next_action"].(string)
	assert.Contains(t, next, "tp review "+carriedRoundFile+" --resolve 0")
	assert.Contains(t, next, "--rewritten", "an epoch is named for a rewritten text")
	assert.NotContains(t, next, "decompose")
}

// TestCarriedBlocking_AnAcceptanceConverges: the row clears by an acceptance
// with evidence, at the cap too.
func TestCarriedBlocking_AnAcceptanceConverges(t *testing.T) {
	t.Parallel()
	dir := carriedRepro(t)
	_, stderr, code := runTP(t, dir, "review", carriedRoundFile, "--resolve", "0", "wontfix", "rule 1 is meant to read this way; operator accepted")
	require.Equal(t, 0, code, stderr)

	out, code := reviewStatusCheck(t, dir)
	assert.Equal(t, 0, code, "--check exits 0 once the row is accepted")
	assert.Equal(t, true, out["done"])
	assert.NotContains(t, out, "carried_blocking")
}

// carriedWithCap is a spec with a task file whose review cap is n.
func carriedWithCap(t *testing.T, n string) string {
	t.Helper()
	dir := t.TempDir()
	writeSpec(t, dir, carriedSpec)
	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, dir, "set", "--workflow", "review_max_rounds="+n)
	require.Equal(t, 0, code, stderr)
	return dir
}

// TestCarriedBlocking_AFixAtTheCapDoesNotEndTheLoop (§2.6): the BUGS.md
// sequence, then the row marked fixed. No round read the fix, so the row keeps
// blocking; at the cap no round is admitted, so the way out named is the
// operator's raise of the cap, never another fixed.
func TestCarriedBlocking_AFixAtTheCapDoesNotEndTheLoop(t *testing.T) {
	t.Parallel()
	dir := carriedRepro(t)
	_, stderr, code := runTP(t, dir, "review", carriedRoundFile, "--resolve", "0", "fixed", "reworded rule 1")
	require.Equal(t, 0, code, stderr)

	out, code := reviewStatusCheck(t, dir)
	assert.Equal(t, 1, code)
	assert.Equal(t, false, out["done"])
	assert.Nil(t, out["done_by"])
	assert.Equal(t, float64(1), out["carried_fixed_unverified"])
	rows := carriedRows(t, out)
	require.Len(t, rows, 1)
	assert.Equal(t, "fixed-unverified", rows[0]["state"])
	next, _ := out["next_action"].(string)
	assert.Contains(t, next, "raise-review-cap")
	assert.NotContains(t, next, "--resolve")
}

// TestCarriedBlocking_AFixIsVerifiedByTheNextRound: below the cap a fixed row
// blocks until a round emitted after the fix is recorded, and then the loop
// converges.
func TestCarriedBlocking_AFixIsVerifiedByTheNextRound(t *testing.T) {
	t.Parallel()
	dir := carriedWithCap(t, "5")
	for _, rows := range []string{carriedHighRow, "", ""} {
		_, stderr, code := recordRound(t, dir, rows)
		require.Equal(t, 0, code, stderr)
	}
	_, stderr, code := runTP(t, dir, "review", carriedRoundFile, "--resolve", "0", "fixed", "reworded rule 1")
	require.Equal(t, 0, code, stderr)

	out, code := reviewStatusCheck(t, dir)
	assert.Equal(t, 1, code, "the fix is not verified yet")
	assert.Contains(t, out["next_action"], "run the next review round")

	_, stderr, code = recordRound(t, dir, "")
	require.Equal(t, 0, code, stderr)
	out, code = reviewStatusCheck(t, dir)
	assert.Equal(t, 0, code, "round 4 read the fix: %v", out["next_action"])
	assert.NotContains(t, out, "carried_blocking")
}

// TestCarriedBlocking_ARoundInFlightAtTheFixDoesNotVerifyIt: a round emitted
// before the fix was written did not list it as fixed, so recording it
// verifies nothing; the round after it does.
func TestCarriedBlocking_ARoundInFlightAtTheFixDoesNotVerifyIt(t *testing.T) {
	t.Parallel()
	dir := carriedWithCap(t, "5")
	_, stderr, code := recordRound(t, dir, carriedHighRow)
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code, "emit round 2: %s", stderr)
	_, stderr, code = runTP(t, dir, "review", carriedRoundFile, "--resolve", "0", "fixed", "reworded rule 1")
	require.Equal(t, 0, code, stderr)
	_, stderr, code = recordRound(t, dir, "")
	require.Equal(t, 0, code, "record round 2: %s", stderr)

	out, code := reviewStatusCheck(t, dir)
	assert.Equal(t, 1, code, "round 2 was emitted before the fix")
	assert.Equal(t, float64(1), out["carried_fixed_unverified"])

	_, stderr, code = recordRound(t, dir, "")
	require.Equal(t, 0, code, stderr)
	out, code = reviewStatusCheck(t, dir)
	assert.Equal(t, 0, code, "round 3 was emitted after the fix: %v", out["next_action"])
}

// TestCarriedBlocking_TheCapRefusalNamesTheCarriedRow: at the cap with a
// carried blocking row the loop is not done, so the next emission is refused,
// and the hint names the row in its own round file instead of the latest
// round's, keeping only the cap's raise as the other way out.
func TestCarriedBlocking_TheCapRefusalNamesTheCarriedRow(t *testing.T) {
	t.Parallel()
	dir := carriedRepro(t)

	_, stderr, code := runTP(t, dir, "review", "spec.md")
	assert.Equal(t, 4, code, "the cap refuses a fourth round: %s", stderr)
	env := errorEnvelope(t, stderr)
	assert.Contains(t, env["error"], "round budget exhausted")
	assert.Contains(t, env["hint"], "tp review "+carriedRoundFile+" --resolve 0 wontfix|duplicate")
	assert.Contains(t, env["hint"], "1 carried blocking finding")
	assert.Contains(t, env["hint"], "raise-review-cap")
	assert.NotContains(t, env["hint"], "disposition each remaining finding in the latest recorded round file")
}

// carriedResume runs tp resume and returns its payload and blockers by code.
func carriedResume(t *testing.T, dir string) (out map[string]any, blockers []map[string]any) {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "resume")
	require.Equal(t, 0, code, stderr)
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	bs, _ := out["blockers"].([]any)
	for _, b := range bs {
		if m, ok := b.(map[string]any); ok {
			blockers = append(blockers, m)
		}
	}
	return out, blockers
}

// TestCarriedBlocking_ResumeNamesTheRow: under the cap, tp resume stops on the
// carried row as an escalate blocker naming it, since another round cannot
// clear what its roles are told not to re-report. Its data names the command
// and, as spec-stale does, the operator's way past it.
func TestCarriedBlocking_ResumeNamesTheRow(t *testing.T) {
	t.Parallel()
	dir := carriedWithCap(t, "5")
	for _, rows := range []string{carriedHighRow, "", ""} {
		_, stderr, code := recordRound(t, dir, rows)
		require.Equal(t, 0, code, stderr)
	}

	out, blockers := carriedResume(t, dir)
	assert.Equal(t, "review", out["phase"])
	require.NotEmpty(t, blockers)
	found := blockers[0]
	require.Equal(t, "review-carried-blocking", found["code"], "the carried row is a blocker: %v", blockers)
	assert.Equal(t, "escalate", found["class"])
	assert.Contains(t, found["message"], "tp review "+carriedRoundFile+" --resolve 0")
	data, _ := found["data"].(map[string]any)
	assert.Equal(t, float64(1), data["count"])
	assert.Equal(t, []any{"tp review " + carriedRoundFile + " --resolve 0 fixed|wontfix|duplicate \"<evidence>\""}, data["commands"])
	assert.Equal(t, map[string]any{"decision": "rewrite-past-blocking", "commands": []any{"tp review spec.md --rewritten \"<evidence>\""}}, data["operator"])
	assert.Empty(t, out["next_units"], "no round is offered while the blocker stands")
}

// TestCarriedBlocking_ResumeAtTheCapLeadsWithTheCarriedRow: at the cap with the
// latest round empty, the carried blocker comes first, so next_action's
// summary names the row rather than "the remaining findings of the latest
// round", and the budget blocker points at it.
func TestCarriedBlocking_ResumeAtTheCapLeadsWithTheCarriedRow(t *testing.T) {
	t.Parallel()
	dir := carriedWithCap(t, "3")
	for _, rows := range []string{carriedHighRow, "", ""} {
		_, stderr, code := recordRound(t, dir, rows)
		require.Equal(t, 0, code, stderr)
	}

	out, blockers := carriedResume(t, dir)
	require.Len(t, blockers, 2)
	assert.Equal(t, "review-carried-blocking", blockers[0]["code"])
	assert.Equal(t, "review-budget-exhausted", blockers[1]["code"])
	assert.Contains(t, blockers[1]["message"], "review-carried-blocking")
	assert.NotContains(t, blockers[1]["message"], "remaining findings of the latest round")
	na, _ := out["next_action"].(map[string]any)
	assert.Contains(t, na["summary"], "1 carried blocking finding stands unresolved")
}

// TestCarriedBlocking_TheExportCountsTheRows: tp export's review loop carries
// the count, which the dashboard drawer shows instead of "converged".
func TestCarriedBlocking_TheExportCountsTheRows(t *testing.T) {
	t.Parallel()
	dir := carriedWithCap(t, "5")
	for _, rows := range []string{carriedHighRow, "", ""} {
		_, stderr, code := recordRound(t, dir, rows)
		require.Equal(t, 0, code, stderr)
	}
	stdout, stderr, code := runTP(t, dir, "export")
	require.Equal(t, 0, code, stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	loops, _ := out["loops"].(map[string]any)
	review, _ := loops["review"].(map[string]any)
	assert.Equal(t, float64(1), review["carried_blocking"])
	assert.Equal(t, float64(2), review["streak"], "the streak alone would read converged")
}

// carriedMarker pulls the MK-<letter> marker out of a finding's text.
var carriedMarker = regexp.MustCompile(`MK-[A-Z]`)

// markersOf returns the sorted markers in lines.
func markersOf(lines []string) []string {
	out := []string{}
	for _, line := range lines {
		if m := carriedMarker.FindString(line); m != "" {
			out = append(out, m)
		}
	}
	sort.Strings(out)
	return out
}

// TestCarriedBlocking_ThePromptAndTheCheckReadOneSet: over a history holding
// every disposition kind, the rows an emission prints under "UNRESOLVED … DO
// NOT re-report" with a blocking tag, plus the rows it shows as fixed for a
// round to verify that no round has, are exactly what --check counts as
// blocking; the rows --status lists are those of them from rounds before the
// latest.
func TestCarriedBlocking_ThePromptAndTheCheckReadOneSet(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSpec(t, dir, carriedSpec)
	row := func(sev, marker string) string {
		return `{"severity":"` + sev + `","category":"clarity","location":"§2","finding":"` + marker + ` rule text","evidence":"e","suggestion":"s"}` + "\n"
	}
	// Round 1: A open, B fixed before round 2 (verified by it), C accepted, D
	// accepted with blank evidence (which accepts nothing), E a medium left
	// open, F a critical marked duplicate, G open here and fixed in round 2,
	// H an out-of-vocabulary severity left open, J fixed after round 2 (no
	// round read it), K high here and re-reported low in round 2.
	_, stderr, code := recordRound(t, dir,
		row("high", "MK-A")+row("high", "MK-B")+row("high", "MK-C")+row("high", "MK-D")+
			row("medium", "MK-E")+row("critical", "MK-F")+row("high", "MK-G")+row("severe", "MK-H")+
			row("high", "MK-J")+row("high", "MK-K"))
	require.Equal(t, 0, code, stderr)
	round1 := filepath.Join(dir, carriedRoundFile)
	resolve := func(file string, r ...string) {
		t.Helper()
		_, stderr, code := runTP(t, dir, append([]string{"review", file, "--resolve"}, r...)...)
		require.Equal(t, 0, code, stderr)
	}
	resolve(round1, "1", "fixed", "reworded")
	resolve(round1, "2", "wontfix", "operator accepted")
	resolve(round1, "5", "duplicate", "same as MK-A")
	data, err := os.ReadFile(round1)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	var d map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[3]), &d))
	d["resolved"] = map[string]any{"status": "wontfix", "evidence": "  "}
	blank, err := json.Marshal(d)
	require.NoError(t, err)
	lines[3] = string(blank)
	require.NoError(t, os.WriteFile(round1, []byte(strings.Join(lines, "\n")+"\n"), 0o600))

	// Round 2: G again, then fixed; I a new open high; K again, graded low.
	_, stderr, code = recordRound(t, dir, row("high", "MK-G")+row("high", "MK-I")+row("low", "MK-K"))
	require.Equal(t, 0, code, stderr)
	resolve(filepath.Join(dir, ".tp-review", "spec", "review-round-2.ndjson"), "0", "fixed", "reworded")
	resolve(round1, "8", "fixed", "reworded")

	out, code := reviewStatusCheck(t, dir)
	assert.Equal(t, 1, code)
	listed, states := []string{}, map[string]string{}
	for _, r := range carriedRows(t, out) {
		m := carriedMarker.FindString(r["finding"].(string))
		listed = append(listed, m)
		states[m], _ = r["state"].(string)
	}
	sort.Strings(listed)

	stdout, stderr, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code, "emit round 3: %s", stderr)
	var emission map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &emission))
	loop, _ := emission["review_loop"].(map[string]any)
	assert.Contains(t, loop["convergence"], "no carried blocking finding unresolved")
	prompts, _ := emission["prompts"].([]any)
	require.NotEmpty(t, prompts)
	prompt := prompts[0].(map[string]any)["prompt"].(string)
	unresolved := carrySection(prompt, "UNRESOLVED findings from previous rounds")
	require.NotEmpty(t, unresolved, "round 3 carries the unresolved rows")
	blockingLines, otherLines := []string{}, []string{}
	for _, line := range strings.Split(unresolved, "\n")[1:] {
		if strings.Contains(line, "[MED]") || strings.Contains(line, "[LOW]") {
			otherLines = append(otherLines, line)
			continue
		}
		blockingLines = append(blockingLines, line)
	}
	printedBlocking := markersOf(blockingLines)
	printedFixed := markersOf(strings.Split(carrySection(prompt, "Resolved high/critical (DO NOT regress)"), "\n"))

	assert.Equal(t, []string{"MK-A", "MK-D", "MK-H", "MK-I", "MK-K"}, printedBlocking,
		"the prompt's blocking rows; K keeps its high, not its later low")
	assert.Equal(t, []string{"MK-E"}, markersOf(otherLines), "a medium is carried and blocks nothing")
	assert.Equal(t, []string{"MK-B", "MK-G", "MK-J"}, printedFixed, "the fixes shown for a round to verify")
	assert.Equal(t, float64(len(printedBlocking)+2), out["carried_blocking"],
		"the count is the prompt's unresolved blocking rows and G and J, the two fixes no round has read")
	assert.Equal(t, float64(2), out["carried_fixed_unverified"])
	assert.Equal(t, []string{"MK-A", "MK-D", "MK-H", "MK-J"}, listed,
		"--status lists the blocking rows of rounds before the latest: not G, I or K")
	assert.Equal(t, "fixed-unverified", states["MK-J"])
	assert.Equal(t, "unresolved", states["MK-A"])
}

// TestCarriedBlocking_AnEpochPastABlockingRowIsTheOperators: --rewritten whose
// cut holds an unresolved blocking row is the operator's decision
// rewrite-past-blocking: under TP_UNATTENDED it exits 2 naming the escalation
// and writes nothing; by hand it runs, and says which rows it cut.
func TestCarriedBlocking_AnEpochPastABlockingRowIsTheOperators(t *testing.T) {
	t.Parallel()
	dir := rewrittenFixture(t)
	before := stateJSONBytes(t, dir)

	_, stderr, code := runTPFence(t, dir, true, "review", "spec.md", "--rewritten", "replaced the spec")
	assert.Equal(t, 2, code, stderr)
	env := errorEnvelope(t, stderr)
	assert.Contains(t, env["error"], "unresolved blocking")
	assert.Contains(t, env["hint"], "tp escalate --decision rewrite-past-blocking")
	assert.Equal(t, before, stateJSONBytes(t, dir), "a refusal writes nothing")

	stdout, stderr, code := runTPFence(t, dir, false, "review", "spec.md", "--rewritten", "replaced the spec")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stderr, "rewrite-past-blocking", "the notice names the decision taken")
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, float64(1), out["cut_blocking"])
	cut := carriedRows(t, map[string]any{"carried_blocking_rows": out["cut_blocking_rows"]})
	require.Len(t, cut, 1)
	assert.Contains(t, cut[0]["finding"], "MARKER-OLD-TEXT")
}

// TestCarriedBlocking_AnEpochPastNoBlockingRowIsOrdinary: an epoch whose cut
// holds no unresolved blocking row — here a medium left open and a high fixed —
// stays the ordinary mode 1.14.0 shipped, under TP_UNATTENDED too.
func TestCarriedBlocking_AnEpochPastNoBlockingRowIsOrdinary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSpec(t, dir, epochOldSpec)
	_, stderr, code := recordRound(t, dir,
		`{"evidence":"e","severity":"medium","category":"clarity","location":"§1","finding":"a medium","suggestion":"s"}`+"\n"+epochHighRow)
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, dir, "review", filepath.Join(dir, ".tp-review", "spec", "review-round-1.ndjson"), "--resolve", "1", "fixed", "reworded")
	require.Equal(t, 0, code, stderr)
	writeSpec(t, dir, epochNewSpec)

	stdout, stderr, code := runTPFence(t, dir, true, "review", "spec.md", "--rewritten", "replaced the spec")
	require.Equal(t, 0, code, stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.NotContains(t, out, "cut_blocking")
	assert.NotContains(t, stderr, "rewrite-past-blocking")
}
