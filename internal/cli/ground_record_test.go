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

	"github.com/deligoez/tp/internal/engine"
)

// groundRecordRow is a legal §7.2 row for unit u<n> of groundFixtureSpec's
// floor: a `document` claim reached at `read`, the one tier §4.1 grants that
// kind, so the row satisfies the per-verdict tier rule as well as the field
// table. Each row's evidence differs, so a test can tell which rows reached the
// file.
//
// `text_sha` is DERIVED from the fixture's own index rather than written as a
// literal, because §7.3's one value check compares it: a row naming a floor unit
// must carry that unit's hash. A literal was what these fixtures used, and it
// matched nothing — every one of them was the shape of row the check exists to
// refuse.
func groundRecordRow(n int) string {
	return fmt.Sprintf(`{"unit_id":"u%d","anchor":"§1","text_sha":%q,"ordinal":1,`+
		`"verdict":"PASS","kind":"document","tier":"read","evidence":"read spec.md line %d"}`,
		n, groundFixtureTextSHA(n), n)
}

// groundFixtureTextSHA is the hash groundFixtureSpec's floor gives u<n>, taken
// through the engine's own derivation so a fixture row cannot drift from the
// index tp emits for the same text.
//
// A unit the arms CUT has no hash in the index and none to match, so it keeps a
// well-formed placeholder: §7.3's check skips a cut row, and a row on one must
// still satisfy §7.2's shape.
func groundFixtureTextSHA(n int) string {
	rows := engine.FloorIndexRows(groundFixtureSpec, engine.FloorAnchorOf(groundFixtureSpec))
	id := fmt.Sprintf("u%d", n)
	for _, r := range rows {
		if r.ID == id && r.TextSHA != "" {
			return r.TextSHA
		}
	}
	return "0123456789ab"
}

// writeGroundRows puts a --record payload in dir and returns the relative name
// tp is invoked with. The payload is newline-terminated, which is the shape a
// unit's writer produces and the one §7.1 says must not be read as a partial
// trailing line.
func writeGroundRows(t *testing.T, dir string, lines ...string) string {
	t.Helper()
	body := ""
	for _, line := range lines {
		body += line + "\n"
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "rows.ndjson"), []byte(body), 0o600))
	return "rows.ndjson"
}

// groundErrorEnvelope decodes the {error, code, hint} object tp writes to stderr
// under --json, so a test's verdict rests on the envelope's own fields rather
// than on a substring of a sentence.
func groundErrorEnvelope(t *testing.T, stderr string) map[string]any {
	t.Helper()
	// The envelope is the LAST line, not the whole stream. stderr is also
	// output.Notice's channel, so a command that has something to say before it
	// fails writes a plain-text line first -- `--units --json` does exactly
	// that. Parsing the whole of stderr made every such command untestable
	// through this helper, and the failure looked like a broken error envelope
	// rather than a helper that had assumed one writer.
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	last := lines[len(lines)-1]
	var envelope map[string]any
	require.NoError(t, json.Unmarshal([]byte(last), &envelope),
		"stderr's last line is the JSON error envelope under --json: %q", stderr)
	return envelope
}

// groundStatePath names a file inside the fixture's state directory.
func groundStatePath(dir, name string) string {
	return filepath.Join(dir, ".tp-review", "spec", name)
}

// TestGroundRecordWritesTheRoundFileBesideTheEmission is the acceptance's third
// clause: --record writes ground-round-N.ndjson.
//
// The listing is asserted as a SET, for the reason the emit test gives: the
// negative half of §11 row 12 — no state.json — is invisible to a pair of Stat
// calls, and the round file must land BESIDE the two artifacts emit wrote
// rather than in place of them.
func TestGroundRecordWritesTheRoundFileBesideTheEmission(t *testing.T) {
	t.Parallel()
	dir := writeGroundFixture(t)
	groundEmit(t, dir)
	rows := writeGroundRows(t, dir, groundRecordRow(1), groundRecordRow(2))

	stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows)
	require.Equal(t, 0, code, "stderr: %s", stderr)

	assert.Equal(t, []string{
		"floor-ground-round-1.txt",
		"ground-round-1.ndjson",
		"snapshot-ground-round-1.md",
	}, stateDirNames(t, dir), "--record adds the round file beside the emission and writes no state.json")

	payload, err := os.ReadFile(filepath.Join(dir, rows))
	require.NoError(t, err)
	written, err := os.ReadFile(groundStatePath(dir, "ground-round-1.ndjson"))
	require.NoError(t, err)
	assert.Equal(t, string(payload), string(written),
		"the recorded round holds the payload's bytes, not a re-serialisation of the parsed rows (§7.1)")

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, float64(1), out["round"])
	assert.Equal(t, float64(2), out["rows"])
	assert.Equal(t, filepath.Join(".tp-review", "spec", "ground-round-1.ndjson"), out["file"])
	assert.Equal(t, filepath.Join(".tp-review", "spec", "floor-ground-round-1.txt"), out["floor"],
		"the result names the floor the round was recorded against")
}

// TestGroundRecordReadsTheEmittedFloorAndNotTheCurrentSpec is the acceptance's
// first clause, asserted in the two directions that separate reading the
// recorded floor from re-deriving one.
//
// Neither half is decidable from the other. A re-deriving implementation
// SUCCEEDS on the first — the spec is still there, so it can build a floor for a
// round whose emission is gone — and FAILS on the second, where the artifact it
// would derive from no longer exists. Together they pin that the floor on disk
// is the input and the spec at record time is not one.
func TestGroundRecordReadsTheEmittedFloorAndNotTheCurrentSpec(t *testing.T) {
	t.Parallel()
	t.Run("the emitted floor is gone", func(t *testing.T) {
		dir := writeGroundFixture(t)
		groundEmit(t, dir)
		require.NoError(t, os.Remove(groundStatePath(dir, "floor-ground-round-1.txt")))
		require.FileExists(t, groundStatePath(dir, "snapshot-ground-round-1.md"),
			"only the floor is removed, so a refusal here is about the floor and not about the directory")
		require.FileExists(t, filepath.Join(dir, "spec.md"),
			"the spec remains, which is what a re-deriving implementation would floor from")

		rows := writeGroundRows(t, dir, groundRecordRow(1))
		stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows)
		require.Equal(t, 3, code, "stdout: %s stderr: %s", stdout, stderr)

		envelope := groundErrorEnvelope(t, stderr)
		assert.Equal(t, float64(3), envelope["code"])
		assert.Contains(t, fmt.Sprint(envelope["error"], " ", envelope["hint"]), "tp ground spec.md",
			"the refusal names the emit command (§7.3)")
	})

	t.Run("the spec is gone", func(t *testing.T) {
		dir := writeGroundFixture(t)
		groundEmit(t, dir)
		require.NoError(t, os.Remove(filepath.Join(dir, "spec.md")),
			"nothing --record needs is in the spec: the floor and the snapshot are the round's record of it")

		rows := writeGroundRows(t, dir, groundRecordRow(1))
		stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows)
		require.Equal(t, 0, code, "stdout: %s stderr: %s", stdout, stderr)
		require.FileExists(t, groundStatePath(dir, "ground-round-1.ndjson"),
			"the round records against the floor the emission froze")
	})
}

// TestRecordOnAMissingSpecDiagnosesTheSpecLikeItsSiblings is --status's repair
// applied to the mode beside it. A path naming no file was reported as a round
// with no emitted floor, and the hint sent the operator to
// `tp ground <the same wrong path>` — which fails too.
//
// The stat runs inside the no-floor branch and not at the top of --record, for
// the reason the subtest above measures: a spec deleted AFTER its emission
// records at exit 0 (§7.3 grades the round against the floor, not the text),
// and a top-level existence check would refuse it. That state never reaches
// this branch, because its floor is on disk. os.Stat is not a read, so §7.3's
// rule that --record never opens the spec is untouched, and ExitFile is the
// code on both paths, so nothing branching on the code sees a change.
//
// The verdict is the three envelopes being EQUAL, not any phrase inside them:
// a reworded ground message cannot satisfy it, and the same typo reads the same
// way whichever mode catches it.
func TestRecordOnAMissingSpecDiagnosesTheSpecLikeItsSiblings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rows := writeGroundRows(t, dir, groundRecordRow(1))

	envelopeOf := func(t *testing.T, mode string) map[string]any {
		t.Helper()
		stdout, stderr, code := runTP(t, dir, mode, "nope.md", "--record", rows)
		require.Equal(t, 3, code, "%s --record on a missing spec: stdout %s stderr %s", mode, stdout, stderr)
		return groundErrorEnvelope(t, stderr)
	}

	ground := envelopeOf(t, "ground")
	assert.Equal(t, envelopeOf(t, "review"), ground,
		"tp review --record and tp ground --record must name the same fault on the same typo")
	assert.Equal(t, envelopeOf(t, "audit"), ground,
		"and so must tp audit --record")
}

// TestARecordHoldingNoRowsIsRejectedAndConsumesNoRoundNumber is the acceptance's
// second clause and §7.1's empty-record rule.
//
// The re-emission at the end is what makes "cannot consume a round number" an
// assertion rather than a hope: a round file written for a payload that decided
// nothing would advance N, and the second emission would come back as round 2.
func TestARecordHoldingNoRowsIsRejectedAndConsumesNoRoundNumber(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
	}{
		{"an empty file", ""},
		{"blank lines only", "\n\n   \n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeGroundFixture(t)
			groundEmit(t, dir)
			before := stateDirNames(t, dir)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "rows.ndjson"), []byte(tc.body), 0o600))

			stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", "rows.ndjson")
			require.Equal(t, 1, code, "stdout: %s stderr: %s", stdout, stderr)
			assert.Equal(t, float64(1), groundErrorEnvelope(t, stderr)["code"])
			assert.Equal(t, before, stateDirNames(t, dir), "a rejected record writes no round file")

			assert.Equal(t, float64(1), groundEmit(t, dir)["round"],
				"the refused round number is still free: the next emission is round 1 again")
		})
	}
}

// TestGroundRecordWithNoPriorEmitExitsThree is the acceptance's fourth clause,
// over the two shapes "no prior emit" takes in a real flow.
//
// The second is the one an operator reaches by accident: round 1 was emitted and
// recorded, so N is now 2, and a --record run without a second emission is
// grading rows against a floor that was never written.
func TestGroundRecordWithNoPriorEmitExitsThree(t *testing.T) {
	t.Parallel()
	t.Run("no state directory at all", func(t *testing.T) {
		dir := writeGroundFixture(t)
		rows := writeGroundRows(t, dir, groundRecordRow(1))

		stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows)
		require.Equal(t, 3, code, "stdout: %s stderr: %s", stdout, stderr)
		assert.Contains(t, fmt.Sprint(groundErrorEnvelope(t, stderr)["hint"]), "tp ground spec.md")
	})

	t.Run("the previous round was recorded and not re-emitted", func(t *testing.T) {
		dir := writeGroundFixture(t)
		groundEmit(t, dir)
		rows := writeGroundRows(t, dir, groundRecordRow(1))
		_, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows)
		require.Equal(t, 0, code, "stderr: %s", stderr)

		before := stateDirNames(t, dir)
		stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows)
		require.Equal(t, 3, code, "stdout: %s stderr: %s", stdout, stderr)
		assert.Equal(t, before, stateDirNames(t, dir),
			"round 2 has no floor, so nothing is written under its number")
	})

	// The floor is checked before the payload is read, so an operator who has
	// emitted nothing is told that rather than being sent to look at a file
	// whose absence is not the reason the round cannot be recorded.
	t.Run("the record path is missing too", func(t *testing.T) {
		dir := writeGroundFixture(t)
		stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", "absent.ndjson")
		require.Equal(t, 3, code, "stdout: %s stderr: %s", stdout, stderr)
		assert.Contains(t, fmt.Sprint(groundErrorEnvelope(t, stderr)["hint"]), "tp ground spec.md")
	})
}

// TestARowFailingSection72IsRejectedAndWritesNoRoundFile is the acceptance's
// second clause in the other direction: every row is validated against §7.2, and
// one failure refuses the whole round.
//
// The bad row is §7.1's own named exit-1 input — `{kind: behaviour, tier: read,
// verdict: PASS}` — and it sits SECOND, so an implementation validating only the
// first row records this payload instead of refusing it.
func TestARowFailingSection72IsRejectedAndWritesNoRoundFile(t *testing.T) {
	t.Parallel()
	dir := writeGroundFixture(t)
	groundEmit(t, dir)
	before := stateDirNames(t, dir)

	bad := `{"unit_id":"u2","anchor":"§1","text_sha":"0123456789ab","ordinal":1,` +
		`"verdict":"PASS","kind":"behaviour","tier":"read","evidence":"read internal/cli/ground.go"}`
	rows := writeGroundRows(t, dir, groundRecordRow(1), bad)

	stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows)
	require.Equal(t, 1, code, "stdout: %s stderr: %s", stdout, stderr)

	envelope := groundErrorEnvelope(t, stderr)
	assert.Equal(t, float64(1), envelope["code"])
	message := fmt.Sprint(envelope["error"])
	assert.Contains(t, message, "line 2", "the rejection places the row in the file the operator opens")
	assert.Contains(t, message, "tier", "and names the cell of §7.2's table that failed")
	assert.Equal(t, before, stateDirNames(t, dir), "one invalid row writes no round file (§11 row 10)")
}

// TestTheMissingRecordFileHintIsGroundsOwnAndNotTheSharedOne is the live half of
// the carve-out the in-package constant test states: what tp actually prints on
// exit 3 for a --record path it cannot open is not what `tp review --record`
// prints for the same failure.
//
// **The verdict rests on the comparison, not on matched text.** The two
// invocations are the same failure of the same flag against the same spec, so a
// hint that is byte-identical between them is the shared constant still in place
// whatever either sentence says; a Contains over one of them would be a presence
// check inside an unbounded string, and the wording it pins can be restated
// beside it. Which words grounding's hint must not carry is asserted where the
// subject is bounded — over the constant itself, in package cli.
//
// The round is emitted first, deliberately. Without it the floor check refuses
// earlier and this measures the no-prior-emit path instead, which is the
// subtest directly above.
func TestTheMissingRecordFileHintIsGroundsOwnAndNotTheSharedOne(t *testing.T) {
	t.Parallel()
	dir := writeGroundFixture(t)
	groundEmit(t, dir)

	_, groundStderr, groundCode := runTP(t, dir, "ground", "spec.md", "--record", "absent.ndjson")
	require.Equal(t, 3, groundCode, "a --record path tp cannot open is exit 3: %s", groundStderr)
	groundHint := fmt.Sprint(groundErrorEnvelope(t, groundStderr)["hint"])
	require.NotEmpty(t, groundHint, "§13.2: every non-zero envelope carries a hint")

	_, reviewStderr, reviewCode := runTP(t, dir, "review", "spec.md", "--record", "absent.ndjson")
	require.Equal(t, 3, reviewCode, "the same failure of the same flag on the panelled command: %s", reviewStderr)
	reviewHint := fmt.Sprint(groundErrorEnvelope(t, reviewStderr)["hint"])
	require.NotEmpty(t, reviewHint)

	assert.NotEqual(t, reviewHint, groundHint,
		"grounding has no reviewers, no auditors and no task file (§7.1, Non-Goal 4), "+
			"so it does not answer this failure with the panelled commands' sentence")
}

// groundQuestionRow is §7.2's QUESTION row for one floor unit: `causes` carries
// three ranked {cause, prediction} objects, which is §6's lower bound, and the
// pair is `{behaviour, read}` — a tier §4.1 does NOT grant that kind, which is
// legal here because §7.2 exempts QUESTION from the acceptability rule in both
// directions and derives §3's `tier-unreached` shape from exactly that mismatch.
func groundQuestionRow(t *testing.T, dir, id string) string {
	t.Helper()
	return fmt.Sprintf(`{"unit_id":%q,"anchor":"§1","text_sha":%q,"ordinal":1,`+
		`"verdict":"QUESTION","kind":"behaviour","tier":"read",`+
		`"evidence":"read internal/cli/ground.go",`+
		`"causes":[`+
		`{"cause":"the shipped command was never run","prediction":"running it settles the claim"},`+
		`{"cause":"the tier reached says nothing about behaviour","prediction":"a run at tier run decides it"},`+
		`{"cause":"the sentence names two subjects","prediction":"splitting it settles each half"}]}`,
		id, groundEmittedSHA(t, dir, id))
}

// TestARoundHoldingAQuestionRecordsAndSoDoesEveryRowBesideIt is §11 row 9: a
// QUESTION is a checkpoint and not an escalation, so the round records and
// every other row in it records with it.
//
// The payload puts a row on EACH SIDE of the question, and that placement is
// what makes this row 9's test rather than a test that a QUESTION row is a legal
// shape. Row 9's mutant exits on the first question: a payload whose question
// came last would record every row before it and pass, and one holding nothing
// but a question would not distinguish a round refused from a round truncated.
//
// The verdict rests on the recorded round read back through `--status` rather
// than on the exit code, because a mutant that recorded a truncated round would
// also exit 0. The two floor units and the reader-added row are counted
// separately there, so the breakdown says which rows survived and the coverage
// says the question dispositioned its unit like any other verdict.
func TestARoundHoldingAQuestionRecordsAndSoDoesEveryRowBesideIt(t *testing.T) {
	t.Parallel()
	dir := writeGroundFixture(t)
	groundEmit(t, dir)

	emitted, _ := groundFloorIDs(t, dir, 1)
	require.Len(t, emitted, 2,
		"the fixture's floor must carry a unit on each side of the question for the placement to mean anything")

	rows := writeGroundRows(t, dir,
		groundVerdictRow(t, dir, emitted[0], "PASS"),
		groundQuestionRow(t, dir, emitted[1]),
		groundReaderAddedRow(),
	)
	stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows)
	require.Equal(t, 0, code, "a QUESTION does not stop the round (§6): stdout %s stderr %s", stdout, stderr)

	status := groundStatus(t, dir)
	assert.EqualValues(t, 2, status["emitted"])
	assert.EqualValues(t, 2, status["dispositioned"],
		"the question dispositions its floor unit like any other verdict (§8)")
	assert.EqualValues(t, 1, status["reader_added"],
		"the row after the question reached the round file")

	byVerdict := groundStatusVerdicts(t, status)
	assert.EqualValues(t, 1, byVerdict["QUESTION"], "the question itself recorded")
	assert.EqualValues(t, 2, byVerdict["PASS"], "so did the rows on both sides of it")
}
