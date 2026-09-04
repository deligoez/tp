package cli_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// §11 row 16b's fixture: four floor units, one of which changes between the two
// rounds, and one unit all three arms cut sat among them.
//
// The cut unit is written to be one and is NOT the headings. §2.1 drops a
// heading before the arms ever see it — measured on this fixture's own first
// emission, whose index reads `# 4 in floor, 0 cut` for a document carrying two
// of them — so a fixture relying on headings to supply its cut rows has none,
// and cannot tell "the index lists every unit" from "the index lists every
// floor unit". `ground_carry_test.go`'s fixture comment states the other
// reading, and its own `require.Len` on the floor is what refutes it.
const (
	groundAskUnit1  = "The first claim measured 3 things."
	groundAskUnit2  = "The second claim measured 4 things."
	groundAskCut    = "Prose with no signal at all."
	groundAskUnit3  = "The third claim measured 5 things."
	groundAskUnit4  = "The fourth claim measured 6 things."
	groundAskEdited = "The fourth claim measured 7 things."
)

const groundAskHead = "# Fixture spec\n\n## 1. Claims\n\n" +
	groundAskUnit1 + "\n\n" + groundAskUnit2 + "\n\n" + groundAskCut + "\n\n" + groundAskUnit3 + "\n\n"

const (
	groundAskRound1 = groundAskHead + groundAskUnit4 + "\n"
	groundAskRound2 = groundAskHead + groundAskEdited + "\n"
)

// groundCarriedMark is the marker as this test states it, independently of the
// constant the engine renders it from: a test that reads its own target out of
// the code under test agrees with any spelling that code chooses.
const groundCarriedMark = " (carried)"

// groundIndexRowRe matches an index row inside the prompt. Every row opens with
// `u<N> `, which no other line of the prompt does — the commit and summary lines
// open with `#`, and the prose is prose.
var groundIndexRowRe = regexp.MustCompile(`^u\d+ `)

// groundPromptIndexRows returns the index rows the prompt carries, whole, in
// the order it lists them.
func groundPromptIndexRows(prompt string) []string {
	rows := make([]string, 0, 16)
	for _, line := range strings.Split(prompt, "\n") {
		if groundIndexRowRe.MatchString(line) {
			rows = append(rows, line)
		}
	}
	return rows
}

// groundRowIDs reads the `unit_id` off each of those lines.
func groundRowIDs(lines []string) []string {
	ids := make([]string, 0, len(lines))
	for _, line := range lines {
		ids = append(ids, strings.Fields(line)[0])
	}
	return ids
}

// groundMarkedIDs is the set of units the prompt marks as already carrying a
// disposition.
func groundMarkedIDs(prompt string) []string {
	marked := make([]string, 0, 16)
	for _, line := range groundPromptIndexRows(prompt) {
		if strings.HasSuffix(line, groundCarriedMark) {
			marked = append(marked, strings.Fields(line)[0])
		}
	}
	return marked
}

// groundFloorFileBytes reads the index an emission froze for a round, as bytes,
// because one assertion below is about what is NOT in those bytes.
func groundFloorFileBytes(t *testing.T, dir string, round int) []byte {
	t.Helper()
	data, err := os.ReadFile(groundStatePath(dir, floorFileName(round)))
	require.NoError(t, err)
	return data
}

// TestTheRoundTwoPromptAsksOnlyForTheDispositionsItOwes is §11 row 16b, end to
// end through the command: on a spec whose round 1 dispositioned every unit and
// whose round 2 changes one unit's text, the round-2 prompt asks for ONE
// disposition, its index still lists every unit, and each carrying unit is
// marked as such.
//
// Round 1 is the control and is not decoration: it is the same command on the
// same spec with nothing to carry, so the two prompts differ in exactly the
// thing under test. Row 16b names emitting the unrestricted index in every
// round as its mutant, and that mutant is invisible without the round that is
// SUPPOSED to be unrestricted.
//
// The last two assertions are the ones row 16b's second mutant needs — narrow
// the index rather than the ask. They are stated over the ids of the floor tp
// itself froze, so "every unit" is the emission's own answer rather than a
// number written here.
func TestTheRoundTwoPromptAsksOnlyForTheDispositionsItOwes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeGroundCarrySpec(t, dir, groundAskRound1)

	prompt1 := groundEmit(t, dir)["prompt"].(string)
	floor1 := groundEmittedFloor(t, dir, 1)
	emitted1, _ := groundFloorIDs(t, dir, 1)
	require.Len(t, emitted1, 4, "round 1's floor is the four sentences; the headings are cut")

	assert.Contains(t, prompt1, "This round owes a disposition for each of the 4 floor units above",
		"round 1 has no preceding round, so every floor unit is owed")
	assert.Empty(t, groundMarkedIDs(prompt1), "and nothing is marked")

	// Round 1 dispositions its whole floor, which is row 16b's premise.
	rows1 := writeGroundRows(t, dir,
		groundCarryRowFor(groundUnitFor(t, floor1, groundAskUnit1), 1),
		groundCarryRowFor(groundUnitFor(t, floor1, groundAskUnit2), 1),
		groundCarryRowFor(groundUnitFor(t, floor1, groundAskUnit3), 1),
		groundCarryRowFor(groundUnitFor(t, floor1, groundAskUnit4), 1),
	)
	_, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows1)
	require.Equal(t, 0, code, "stderr: %s", stderr)

	// One unit's text changes. Everything else is byte-identical, so exactly
	// three of the four units can be carried.
	writeGroundCarrySpec(t, dir, groundAskRound2)
	prompt2 := groundEmit(t, dir)["prompt"].(string)
	floor2 := groundEmittedFloor(t, dir, 2)

	changed := groundUnitFor(t, floor2, groundAskEdited)
	want := []string{
		groundUnitFor(t, floor2, groundAskUnit1).ID,
		groundUnitFor(t, floor2, groundAskUnit2).ID,
		groundUnitFor(t, floor2, groundAskUnit3).ID,
	}

	assert.Contains(t, prompt2, "This round owes a disposition for 1 of the 4 floor units above",
		"three of the four carry forward, so the ask is one")
	assert.ElementsMatch(t, want, groundMarkedIDs(prompt2),
		"the three units whose text did not move are marked, and only those")
	assert.NotContains(t, groundMarkedIDs(prompt2), changed.ID,
		"the unit whose text changed is not carried, however unchanged its anchor")

	// The index still lists every unit: the cut rows too, and in the order the
	// emission wrote them.
	assert.Equal(t, groundRowIDs(groundPromptIndexRows(prompt2)), rowIDsOf(floor2),
		"§8 narrows the ask and not the index: a reader who cannot see the whole floor cannot tell it what the floor missed")
	_, cut2 := groundFloorIDs(t, dir, 2)
	require.NotEmpty(t, cut2,
		"the fixture must carry cut rows, or 'every unit' and 'every floor unit' are the same claim here")
}

// Fixtures for the counts §8's ask can actually reach. The four-unit fixture
// above reaches none of them: it is 3-of-4 and 4-of-4, so every number in its
// sentences is plural and no clause is ever the whole floor.
const (
	groundAskTwoUnits = "# Fixture spec\n\n## 1. Claims\n\n" +
		groundAskUnit1 + "\n\n" + groundAskUnit2 + "\n"
	groundAskOneUnit = "# Fixture spec\n\n## 1. Claims\n\n" + groundAskUnit1 + "\n"
	groundAskNoUnits = "# Fixture spec\n\n## 1. Claims\n\n" + groundAskCut + "\n"
)

// TestTheAskAgreesWithTheCountsItStates walks the ask over every count its own
// two numbers can take, because the sentence states them and then declines them.
//
// This exists because the branch it covers shipped with nothing holding it: the
// repair that introduced the singular carried clause was verified by reading
// one emission, and deleting the whole branch afterwards left `go test ./...`
// green — the string `already carries one` occurred exactly once in the
// repository, in the production line itself. Three auditors then found three
// further counts the repair had not reached, all of them ordinary: a floor of
// one, a round where every unit carries, and a floor where every unit was cut.
// So the rule this test is written to is: **state the input set you ran, and
// run its complement at the nearest boundary.** The set is (floorSize ∈ {0, 1,
// many}) × (carried ∈ {0, 1, all}), and the reachable combinations are here.
//
// Each subtest asserts the whole clause rather than a fragment, because the
// defect being pinned is agreement BETWEEN the count and the words around it —
// a `Contains` on "1 of the 2" alone is satisfied by any grammar that follows.
func TestTheAskAgreesWithTheCountsItStates(t *testing.T) {
	t.Parallel()
	t.Run("one of two owed, one carried", func(t *testing.T) {
		dir := t.TempDir()
		writeGroundCarrySpec(t, dir, groundAskTwoUnits)
		groundEmit(t, dir)
		floor1 := groundEmittedFloor(t, dir, 1)
		rows := writeGroundRows(t, dir,
			groundCarryRowFor(groundUnitFor(t, floor1, groundAskUnit1), 1))
		_, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows)
		require.Equal(t, 0, code, "stderr: %s", stderr)

		assert.Contains(t, groundEmit(t, dir)["prompt"].(string),
			"for 1 of the 2 floor units above: the other one already carries\none from round 1, and its row ends in",
			"one carried unit is singular in both halves of the clause")
	})

	t.Run("every unit carries", func(t *testing.T) {
		dir := t.TempDir()
		writeGroundCarrySpec(t, dir, groundAskTwoUnits)
		groundEmit(t, dir)
		floor1 := groundEmittedFloor(t, dir, 1)
		rows := writeGroundRows(t, dir,
			groundCarryRowFor(groundUnitFor(t, floor1, groundAskUnit1), 1),
			groundCarryRowFor(groundUnitFor(t, floor1, groundAskUnit2), 1))
		_, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows)
		require.Equal(t, 0, code, "stderr: %s", stderr)

		prompt := groundEmit(t, dir)["prompt"].(string)
		assert.Contains(t, prompt,
			"for 0 of the 2 floor units above: all 2 already carry\none from round 1, and their rows end in",
			"when the carried set is the whole floor there is no other group for it to be the other of")
		assert.NotContains(t, prompt, "the other 2 already carry",
			"and the convergent round is the one a reader meets last, not an edge case")
	})

	t.Run("a floor of one, nothing carried", func(t *testing.T) {
		dir := t.TempDir()
		writeGroundCarrySpec(t, dir, groundAskOneUnit)
		assert.Contains(t, groundEmit(t, dir)["prompt"].(string),
			"This round owes a disposition for the 1 floor unit above.",
			"a floor of one is not 'each of the 1 floor units'")
	})

	t.Run("a floor of one, that one carried", func(t *testing.T) {
		dir := t.TempDir()
		writeGroundCarrySpec(t, dir, groundAskOneUnit)
		groundEmit(t, dir)
		floor1 := groundEmittedFloor(t, dir, 1)
		rows := writeGroundRows(t, dir,
			groundCarryRowFor(groundUnitFor(t, floor1, groundAskUnit1), 1))
		_, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows)
		require.Equal(t, 0, code, "stderr: %s", stderr)

		assert.Contains(t, groundEmit(t, dir)["prompt"].(string),
			"for 0 of the 1 floor unit above: the only one already carries\none from round 1, and its row ends in",
			"both numbers are one here, and each declines its own noun")
	})

	t.Run("an empty floor", func(t *testing.T) {
		dir := t.TempDir()
		writeGroundCarrySpec(t, dir, groundAskNoUnits)
		prompt := groundEmit(t, dir)["prompt"].(string)
		emitted, cut := groundFloorIDs(t, dir, 1)
		require.Empty(t, emitted, "the fixture must have an empty floor, or this subtest is about a different state")
		require.NotEmpty(t, cut, "and the emptiness must come from cutting, not from an empty document")
		assert.Contains(t, prompt, "This round owes no dispositions",
			"an all-cut document owes nothing, and saying 'each of the 0 floor units' asks for a set that does not exist")
		assert.NotContains(t, prompt, "each of the 0 floor units",
			"which is what §2.1's own cut rule makes reachable")
	})
}

// rowIDsOf lists every emitted row's id, the cut ones included.
func rowIDsOf(rows []engine.FloorIndexRow) []string {
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	return ids
}

// TestThePromptMarksTheUnitsTheRecordActuallyCarries is the third clause of the
// task this test file exists for: the carry set the emission marks is computed
// by the same join `--record` uses, never by a second rule.
//
// That cannot be asserted by reading the code, and it cannot be asserted by
// re-running the same function — a test calling engine.GroundCarriedRows to
// build its expectation would agree with any second rule the CLI happened to
// hold. So the verdict rests on two artifacts the command itself produced: the
// ids the ROUND-2 PROMPT marked, and the ids `--record` then wrote into round
// 2's file as carried. A second rule on either side makes the two sets differ.
//
// The payload records the one owed unit alone, which is what the prompt asked
// for — so this is also the round-trip row 16b's saving is measured in.
func TestThePromptMarksTheUnitsTheRecordActuallyCarries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeGroundCarrySpec(t, dir, groundAskRound1)
	groundEmit(t, dir)
	floor1 := groundEmittedFloor(t, dir, 1)
	rows1 := writeGroundRows(t, dir,
		groundCarryRowFor(groundUnitFor(t, floor1, groundAskUnit1), 1),
		groundCarryRowFor(groundUnitFor(t, floor1, groundAskUnit2), 1),
		groundCarryRowFor(groundUnitFor(t, floor1, groundAskUnit3), 1),
		groundCarryRowFor(groundUnitFor(t, floor1, groundAskUnit4), 1),
	)
	_, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows1)
	require.Equal(t, 0, code, "stderr: %s", stderr)

	writeGroundCarrySpec(t, dir, groundAskRound2)
	marked := groundMarkedIDs(groundEmit(t, dir)["prompt"].(string))
	require.Len(t, marked, 3, "the prompt must mark something, or the comparison below is vacuous")

	floor2 := groundEmittedFloor(t, dir, 2)
	rows2 := writeGroundRows(t, dir, groundCarryRowFor(groundUnitFor(t, floor2, groundAskEdited), 2))
	_, stderr, code = runTP(t, dir, "ground", "spec.md", "--record", rows2)
	require.Equal(t, 0, code, "stderr: %s", stderr)

	recorded := make([]string, 0, 4)
	for _, row := range groundRecordedRows(t, dir, 2) {
		if row.CarriedFrom != 0 {
			require.NotNil(t, row.UnitID, "a carried row names the unit it decides")
			recorded = append(recorded, *row.UnitID)
		}
	}
	assert.ElementsMatch(t, marked, recorded,
		"the units the prompt said it would carry are the units the record carried")
}

// TestTheEmittedFloorFileIsUnmarked pins where the marker does and does not go.
//
// The prompt's index is a VIEW for a reader; `floor-ground-round-N.txt` is the
// artifact §7.3 grades the round against, and `ParseFloorIndex` accepts §2.2's
// two row shapes and no third. A marker written into that file would be a
// second copy of a fact the record re-derives anyway — one that can go stale
// between emit and record while nothing compares them — and it would make the
// floor unreadable to the very command that reads it.
func TestTheEmittedFloorFileIsUnmarked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeGroundCarrySpec(t, dir, groundAskRound1)
	groundEmit(t, dir)
	floor1 := groundEmittedFloor(t, dir, 1)
	rows1 := writeGroundRows(t, dir, groundCarryRowFor(groundUnitFor(t, floor1, groundAskUnit1), 1))
	_, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows1)
	require.Equal(t, 0, code, "stderr: %s", stderr)

	writeGroundCarrySpec(t, dir, groundAskRound2)
	prompt := groundEmit(t, dir)["prompt"].(string)
	require.NotEmpty(t, groundMarkedIDs(prompt),
		"the prompt must mark a unit here, or the assertion below holds for the wrong reason")

	data := groundFloorFileBytes(t, dir, 2)
	assert.NotContains(t, string(data), groundCarriedMark,
		"§2.2's index is what the round is graded against, and it has two row shapes, not three")
	parsed, err := engine.ParseFloorIndex(string(data))
	require.NoError(t, err, "and it still reads back")
	assert.Equal(t, rowIDsOf(parsed), groundRowIDs(groundPromptIndexRows(prompt)),
		"the two carry the same units in the same order; only the marks differ")
}
