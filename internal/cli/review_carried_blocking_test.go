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

// TestCarriedBlocking_ResolvingTheRowConverges: the row clears by its own
// disposition, fixed or accepted with evidence, and nothing else changes.
func TestCarriedBlocking_ResolvingTheRowConverges(t *testing.T) {
	t.Parallel()
	for _, disposition := range [][]string{
		{"fixed", "reworded rule 1"},
		{"wontfix", "rule 1 is meant to read this way; operator accepted"},
	} {
		dir := carriedRepro(t)
		_, stderr, code := runTP(t, dir, append([]string{"review", carriedRoundFile, "--resolve", "0"}, disposition...)...)
		require.Equal(t, 0, code, "%s: %s", disposition[0], stderr)

		out, code := reviewStatusCheck(t, dir)
		assert.Equal(t, 0, code, "%s: --check exits 0 once the row is resolved", disposition[0])
		assert.Equal(t, true, out["done"], disposition[0])
		assert.NotContains(t, out, "carried_blocking", disposition[0])
	}
}

// TestCarriedBlocking_TheCapRefusalNamesTheCarriedRow: at the cap with a
// carried blocking row the loop is not done, so the next emission is refused,
// and the hint names the row in its own round file rather than only the
// latest round's.
func TestCarriedBlocking_TheCapRefusalNamesTheCarriedRow(t *testing.T) {
	t.Parallel()
	dir := carriedRepro(t)

	_, stderr, code := runTP(t, dir, "review", "spec.md")
	assert.Equal(t, 4, code, "the cap refuses a fourth round: %s", stderr)
	env := errorEnvelope(t, stderr)
	assert.Contains(t, env["error"], "round budget exhausted")
	assert.Contains(t, env["hint"], "tp review "+carriedRoundFile+" --resolve 0")
	assert.Contains(t, env["hint"], "1 carried blocking finding")
}

// TestCarriedBlocking_ResumeNamesTheRow: under the cap, tp resume stops on the
// carried row as an escalate blocker naming it, since another round cannot
// clear what its roles are told not to re-report.
func TestCarriedBlocking_ResumeNamesTheRow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSpec(t, dir, carriedSpec)
	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, dir, "set", "--workflow", "review_max_rounds=5")
	require.Equal(t, 0, code, stderr)
	for _, rows := range []string{carriedHighRow, "", ""} {
		_, stderr, code = recordRound(t, dir, rows)
		require.Equal(t, 0, code, stderr)
	}

	stdout, stderr, code := runTP(t, dir, "resume")
	require.Equal(t, 0, code, stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, "review", out["phase"])
	var found map[string]any
	bs, _ := out["blockers"].([]any)
	for _, b := range bs {
		if m, ok := b.(map[string]any); ok && m["code"] == "review-carried-blocking" {
			found = m
		}
	}
	require.NotNil(t, found, "the carried row is a blocker: %s", stdout)
	assert.Equal(t, "escalate", found["class"])
	assert.Contains(t, found["message"], "tp review "+carriedRoundFile+" --resolve 0")
	data, _ := found["data"].(map[string]any)
	assert.Equal(t, float64(1), data["count"])
	assert.Empty(t, out["next_units"], "no round is offered while the blocker stands")
}

// carriedMarker pulls the MK-<letter> marker out of a finding's text.
var carriedMarker = regexp.MustCompile(`MK-[A-Z]`)

// TestCarriedBlocking_ThePromptAndTheCheckReadOneSet: over a history holding
// every disposition kind, the rows an emission prints under "UNRESOLVED … DO
// NOT re-report" with a blocking tag are exactly the rows --check names as
// blocking, and the medium row it prints there blocks nothing.
func TestCarriedBlocking_ThePromptAndTheCheckReadOneSet(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSpec(t, dir, carriedSpec)
	row := func(sev, marker string) string {
		return `{"severity":"` + sev + `","category":"clarity","location":"§2","finding":"` + marker + ` rule text","evidence":"e","suggestion":"s"}` + "\n"
	}
	// Round 1: A open, B to be fixed, C to be accepted, D accepted with blank
	// evidence (which accepts nothing), E a medium left open, F a critical
	// marked duplicate, G open here and fixed in round 2, H an out-of-vocabulary
	// severity left open.
	_, stderr, code := recordRound(t, dir,
		row("high", "MK-A")+row("high", "MK-B")+row("high", "MK-C")+row("high", "MK-D")+
			row("medium", "MK-E")+row("critical", "MK-F")+row("high", "MK-G")+row("severe", "MK-H"))
	require.Equal(t, 0, code, stderr)
	round1 := filepath.Join(dir, carriedRoundFile)
	for _, r := range [][]string{{"1", "fixed", "reworded"}, {"2", "wontfix", "operator accepted"}, {"5", "duplicate", "same as MK-A"}} {
		_, stderr, code = runTP(t, dir, append([]string{"review", round1, "--resolve"}, r...)...)
		require.Equal(t, 0, code, stderr)
	}
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

	// Round 2: G again, then fixed; I a new open high.
	_, stderr, code = recordRound(t, dir, row("high", "MK-G")+row("high", "MK-I"))
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, dir, "review", filepath.Join(dir, ".tp-review", "spec", "review-round-2.ndjson"), "--resolve", "0", "fixed", "reworded")
	require.Equal(t, 0, code, stderr)

	out, code := reviewStatusCheck(t, dir)
	assert.Equal(t, 1, code)
	blocking := []string{}
	for _, r := range carriedRows(t, out) {
		blocking = append(blocking, carriedMarker.FindString(r["finding"].(string)))
	}
	sort.Strings(blocking)
	assert.Equal(t, float64(len(blocking)), out["carried_blocking"], "every blocking row is listed")

	stdout, stderr, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code, "emit round 3: %s", stderr)
	var emission map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &emission))
	prompts, _ := emission["prompts"].([]any)
	require.NotEmpty(t, prompts)
	unresolved := carrySection(prompts[0].(map[string]any)["prompt"].(string), "UNRESOLVED findings from previous rounds")
	require.NotEmpty(t, unresolved, "round 3 carries the unresolved rows")
	printedBlocking, printedOther := []string{}, []string{}
	for _, line := range strings.Split(unresolved, "\n")[1:] {
		marker := carriedMarker.FindString(line)
		if marker == "" {
			continue
		}
		if strings.Contains(line, "[MED]") || strings.Contains(line, "[LOW]") {
			printedOther = append(printedOther, marker)
			continue
		}
		printedBlocking = append(printedBlocking, marker)
	}
	sort.Strings(printedBlocking)

	assert.Equal(t, []string{"MK-A", "MK-D", "MK-H", "MK-I"}, printedBlocking, "the prompt's blocking rows")
	assert.Equal(t, printedBlocking, blocking, "the prompt and --check read one set")
	assert.Equal(t, []string{"MK-E"}, printedOther, "a medium is carried and blocks nothing")
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

