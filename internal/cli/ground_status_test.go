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

// groundVerdictRow is a legal §7.2 row for one unit carrying `verdict`: a
// `document` claim reached at `read`, the one tier §4.1 grants that kind, so
// PASS and FAIL both satisfy the per-verdict tier rule and NOT-A-CLAIM carries
// the pair §7.2 makes optional there.
//
// The id is a parameter and never `u<n>` computed from a loop index: which ids
// an emission emits is a fact of the floor, and a test that assumed the
// numbering would stop naming the units the moment a fixture gained a cut one.
func groundVerdictRow(id, verdict string) string {
	return fmt.Sprintf(`{"unit_id":%q,"anchor":"§1","text_sha":"0123456789ab","ordinal":1,`+
		`"verdict":%q,"kind":"document","tier":"read","evidence":"read spec.md"}`, id, verdict)
}

// groundReaderAddedRow is §2.2's reader-added claim: `unit_id` null, with the
// unit supplying the hash over its own text.
func groundReaderAddedRow() string {
	return `{"unit_id":null,"anchor":"§1","text_sha":"abcdef012345","ordinal":1,` +
		`"verdict":"PASS","kind":"document","tier":"read","evidence":"read spec.md"}`
}

// groundFloorIDs reads back round N's emitted floor and returns the ids of the
// units carrying a hash and, apart, the ids of the ones the arms cut.
//
// Read from the file the emission wrote rather than derived here, so a test
// names the units tp actually emitted. §2.2's convention is the discriminator:
// the absence of the hash is the cut.
func groundFloorIDs(t *testing.T, dir string, round int) (emitted, cut []string) {
	t.Helper()
	data, err := os.ReadFile(groundStatePath(dir, fmt.Sprintf("floor-ground-round-%d.txt", round)))
	require.NoError(t, err)
	rows, err := engine.ParseFloorIndex(string(data))
	require.NoError(t, err)
	for i := range rows {
		if rows[i].TextSHA == "" {
			cut = append(cut, rows[i].ID)
			continue
		}
		emitted = append(emitted, rows[i].ID)
	}
	return emitted, cut
}

// groundWideFixture writes a spec whose floor holds exactly n emitted units,
// one per paragraph, each reached through §2.1's digit arm.
//
// The count is a `require` and not an assumption: §11 row 22's claim is about a
// round of 84 dispositions, and a fixture that quietly yielded 83 would still
// pass every assertion below while measuring something else.
func groundWideFixture(t *testing.T, n int) string {
	t.Helper()
	dir := t.TempDir()

	var b strings.Builder
	b.WriteString("# Wide fixture\n\n## 1. Claims\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "\nClaim %d carries 1 fact.\n", i)
	}
	text := b.String()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(text), 0o600))

	emitted := 0
	for _, row := range engine.FloorIndexRows(text, engine.FloorAnchorOf(text)) {
		if row.TextSHA != "" {
			emitted++
		}
	}
	require.Equal(t, n, emitted, "the fixture's floor must hold exactly %d emitted units", n)
	return dir
}

// groundStatus runs `tp ground spec.md --status` and returns the decoded
// payload, requiring exit 0.
func groundStatus(t *testing.T, dir string) map[string]any {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--status")
	require.Equal(t, 0, code, "stdout: %s stderr: %s", stdout, stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	return out
}

// groundStatusVerdicts pulls the per-verdict breakdown out of a --status
// payload, failing rather than panicking when the key is absent — which is
// exactly the state §11 row 22's mutant leaves it in.
func groundStatusVerdicts(t *testing.T, status map[string]any) map[string]any {
	t.Helper()
	raw, ok := status["by_verdict"]
	require.True(t, ok, "--status carries the per-verdict breakdown beside the ratio (§8): %v", status)
	byVerdict, ok := raw.(map[string]any)
	require.True(t, ok, "by_verdict is an object keyed by verdict: %v", raw)
	return byVerdict
}

// recordGroundVerdicts records one round giving each emitted floor unit the
// verdict verdicts[i] names, and returns the resulting --status payload.
func recordGroundVerdicts(t *testing.T, dir string, ids, verdicts []string) map[string]any {
	t.Helper()
	require.Len(t, verdicts, len(ids), "one verdict per emitted floor unit")
	lines := make([]string, 0, len(ids))
	for i, id := range ids {
		lines = append(lines, groundVerdictRow(id, verdicts[i]))
	}
	rows := writeGroundRows(t, dir, lines...)
	_, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	return groundStatus(t, dir)
}

// TestStatusDistinguishesARoundOfFailsFromARoundOfPassesAtIdenticalCoverage is
// §11 row 22: --status reports the latest round's per-verdict counts beside the
// ratio, so a round of 84 FAILs and a round of 84 PASSes are distinguishable at
// 100% coverage.
//
// The verdict rests on the two payloads being UNEQUAL as whole objects, not on
// a substring or a single key. Row 22's mutant — report the ratio alone —
// cannot dodge that read-back: every other field of the two runs is identical
// by construction (same spec text, same round, same coverage), which the loop
// below asserts field by field first, so the only thing left that can make them
// differ is the breakdown.
func TestStatusDistinguishesARoundOfFailsFromARoundOfPassesAtIdenticalCoverage(t *testing.T) {
	const units = 84

	roundOf := func(verdict string) map[string]any {
		dir := groundWideFixture(t, units)
		groundEmit(t, dir)
		emitted, cut := groundFloorIDs(t, dir, 1)
		require.Len(t, emitted, units)
		require.Empty(t, cut, "every unit of this fixture is in the floor, so coverage cannot differ")

		verdicts := make([]string, units)
		for i := range verdicts {
			verdicts[i] = verdict
		}
		return recordGroundVerdicts(t, dir, emitted, verdicts)
	}

	failed, passed := roundOf("FAIL"), roundOf("PASS")

	for _, key := range []string{"spec", "round", "emitted", "dispositioned", "reader_added", "off_floor"} {
		require.Equal(t, failed[key], passed[key],
			"the two rounds are identical at %s, so the breakdown is the only thing that can separate them", key)
	}
	require.Equal(t, float64(units), failed["emitted"], "the floor is the denominator")
	require.Equal(t, float64(units), failed["dispositioned"], "both rounds are at 100% coverage")

	assert.NotEqual(t, failed, passed,
		"a round of 84 FAILs and a round of 84 PASSes are distinguishable at identical coverage (§11 row 22)")
	assert.Equal(t, float64(units), groundStatusVerdicts(t, failed)["FAIL"])
	assert.Equal(t, float64(units), groundStatusVerdicts(t, passed)["PASS"])
}

// TestStatusCarriesEverySixVerdictsSoTheNotAClaimShareIsReadable is §8's "the
// NOT-A-CLAIM share is the first number to read": the breakdown names all six
// of §3's verdicts every time, so a zero is a zero rather than a key the reader
// has to know the meaning of the absence of, and the share is the two integers
// `by_verdict["NOT-A-CLAIM"]` and `emitted` sitting on one object.
//
// The six are taken from engine.GroundVerdicts() rather than written out, so a
// verdict added to §3 without a place in the breakdown fails here.
func TestStatusCarriesEverySixVerdictsSoTheNotAClaimShareIsReadable(t *testing.T) {
	const units, notAClaim = 84, 48

	dir := groundWideFixture(t, units)
	groundEmit(t, dir)
	emitted, _ := groundFloorIDs(t, dir, 1)
	require.Len(t, emitted, units)

	verdicts := make([]string, units)
	for i := range verdicts {
		verdicts[i] = "PASS"
		if i < notAClaim {
			verdicts[i] = "NOT-A-CLAIM"
		}
	}
	status := recordGroundVerdicts(t, dir, emitted, verdicts)
	byVerdict := groundStatusVerdicts(t, status)

	names := make([]string, 0, len(engine.GroundVerdicts()))
	for _, v := range engine.GroundVerdicts() {
		names = append(names, string(v))
	}
	present := make([]string, 0, len(byVerdict))
	for name := range byVerdict {
		present = append(present, name)
	}
	assert.ElementsMatch(t, names, present,
		"the breakdown names every one of §3's six verdicts, zeros included")

	assert.Equal(t, float64(notAClaim), byVerdict["NOT-A-CLAIM"])
	assert.Equal(t, float64(units-notAClaim), byVerdict["PASS"])
	assert.Equal(t, float64(0), byVerdict["FAIL"], "a verdict nobody recorded reports 0 and is not absent")
	assert.Equal(t, float64(units), status["emitted"],
		"the share's denominator is the emitted floor, on the same object as its numerator")
}

// TestStatusCountsReaderAddedAndOffFloorApart is the acceptance's third clause:
// the two counts that move neither side of the ratio are reported separately,
// because they are evidence about different halves of §2.1 — the arms never
// produced this unit, against the arms produced it and then cut it.
//
// The fixture's cut unit sits BETWEEN two floor units, and both the emitted ids
// and the cut one are read back from the floor rather than assumed.
func TestStatusCountsReaderAddedAndOffFloorApart(t *testing.T) {
	dir := writeGroundFixture(t)
	groundEmit(t, dir)
	emitted, cut := groundFloorIDs(t, dir, 1)
	require.Len(t, emitted, 2, "the fixture emits two floor units")
	require.Len(t, cut, 1, "and cuts one between them, which is what an off-floor row names")

	rows := writeGroundRows(t, dir,
		groundVerdictRow(emitted[0], "PASS"),
		groundReaderAddedRow(),
		groundVerdictRow(cut[0], "PASS"),
	)
	_, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows)
	require.Equal(t, 0, code, "stderr: %s", stderr)

	status := groundStatus(t, dir)
	assert.Equal(t, float64(2), status["emitted"])
	assert.Equal(t, float64(1), status["dispositioned"],
		"three rows, one emitted floor unit decided: neither off-ratio row raises coverage")
	assert.Equal(t, float64(1), status["reader_added"], "the row carrying unit_id null")
	assert.Equal(t, float64(1), status["off_floor"], "the row naming the cut unit's id")
}

// TestStatusReportsTheLatestEmittedRoundAndNotTheLatestRecorded pins which
// round --status is about, on §9's reading that a round exists from its
// EMISSION: between `tp ground` and `tp ground --record` nothing in the round
// has been decided, and that is the state a coverage report most needs to name.
//
// Round 1 is recorded to 100% first and asserted, so the mutant's answer is a
// real state of this fixture rather than a number no reading produces.
func TestStatusReportsTheLatestEmittedRoundAndNotTheLatestRecorded(t *testing.T) {
	dir := writeGroundFixture(t)
	groundEmit(t, dir)
	emitted, _ := groundFloorIDs(t, dir, 1)
	require.Len(t, emitted, 2)

	first := recordGroundVerdicts(t, dir, emitted, []string{"PASS", "PASS"})
	require.Equal(t, float64(1), first["round"])
	require.Equal(t, float64(2), first["emitted"])
	require.Equal(t, float64(2), first["dispositioned"], "round 1 is fully dispositioned")

	require.Equal(t, float64(2), groundEmit(t, dir)["round"])

	second := groundStatus(t, dir)
	assert.Equal(t, float64(2), second["round"], "--status is about the latest EMITTED round")
	assert.Equal(t, float64(2), second["emitted"])
	assert.Equal(t, float64(0), second["dispositioned"],
		"round 2 has been emitted and not recorded, so nothing in it has been decided")

	// Round 2 is then recorded PARTIALLY, against the ids read back from its
	// own floor. The spec has not moved, so round 2's floor is round 1's — and
	// that is what makes this discriminating: 1-of-2 is a coverage no reading
	// of round 1's rows produces, so an implementation joining round N's floor
	// to round 1's record answers 2 here.
	emittedTwo, _ := groundFloorIDs(t, dir, 2)
	require.Equal(t, emitted, emittedTwo, "the spec has not moved, so round 2's floor is round 1's")

	rows := writeGroundRows(t, dir, groundVerdictRow(emittedTwo[0], "FAIL"))
	_, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows)
	require.Equal(t, 0, code, "stderr: %s", stderr)

	third := groundStatus(t, dir)
	assert.Equal(t, float64(2), third["round"])
	assert.Equal(t, float64(1), third["dispositioned"], "round 2 decided one of its two floor units")
	assert.Equal(t, float64(1), groundStatusVerdicts(t, third)["FAIL"],
		"and the breakdown is round 2's rows, not round 1's")
	assert.Equal(t, float64(0), groundStatusVerdicts(t, third)["PASS"],
		"round 1's two PASSes belong to round 1")
}

// TestStatusWithNoEmittedRoundExitsThree is the state in which there is no
// round to report: --status refuses with §7.1's exit 3 and names the emit
// command, rather than answering 0-of-0 — a shape from which a later --check
// could exit 0 for a spec nobody has ever grounded.
//
// The second half is the other direction, and neither is decidable from the
// first: --status reads the floor the emission froze and never opens the spec,
// so a spec deleted after its emission still has a round to report.
func TestStatusWithNoEmittedRoundExitsThree(t *testing.T) {
	t.Run("nothing has been emitted", func(t *testing.T) {
		dir := writeGroundFixture(t)
		stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--status")
		require.Equal(t, 3, code, "stdout: %s stderr: %s", stdout, stderr)

		envelope := groundErrorEnvelope(t, stderr)
		assert.Equal(t, float64(3), envelope["code"])
		assert.Contains(t, fmt.Sprint(envelope["error"], " ", envelope["hint"]), "tp ground spec.md",
			"the refusal names the emit command")
	})

	t.Run("the spec is gone and the emission is not", func(t *testing.T) {
		dir := writeGroundFixture(t)
		groundEmit(t, dir)
		emitted, _ := groundFloorIDs(t, dir, 1)
		rows := writeGroundRows(t, dir, groundVerdictRow(emitted[0], "PASS"))
		_, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows)
		require.Equal(t, 0, code, "stderr: %s", stderr)

		require.NoError(t, os.Remove(filepath.Join(dir, "spec.md")),
			"the floor and the round are the round's record of the spec (§7.3)")

		status := groundStatus(t, dir)
		assert.Equal(t, float64(2), status["emitted"])
		assert.Equal(t, float64(1), status["dispositioned"])
	})
}

