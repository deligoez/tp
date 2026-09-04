package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnEmptyPayloadIsAcceptedWhenTheRoundOwesNothing is §7.1's empty-record
// rule stated on what the round RECORDS — the payload's rows plus §8's carried
// rows — rather than on the payload alone.
//
// The two clauses are one test because neither is decidable from the other, and
// because the defect they pin is the meeting of two rules written one release
// apart: §8 narrowed the ask so a round can owe nothing, and §7.1 refused an
// empty payload on its own count. Each was right alone; together they deadlock a
// re-emission on an unedited spec — the unit is asked for `0 of the 2 floor
// units`, writes the empty file it was asked for, and `--record` refuses it.
//
// The deadlock is reached HERE THROUGH THE COMMANDS rather than asserted as a
// property of RecordGroundRound, because what makes it a defect is that three
// real invocations produce it. A unit test on the engine would assert the same
// thing about a state nobody had shown was reachable.
func TestAnEmptyPayloadIsAcceptedWhenTheRoundOwesNothing(t *testing.T) {
	t.Parallel()
	t.Run("empty because every unit carries", func(t *testing.T) {
		dir := writeGroundFixture(t)

		// Round 1: emit, and decide every floor unit the emission listed.
		groundEmit(t, dir)
		floor1 := groundEmittedFloor(t, dir, 1)
		require.Len(t, floor1, 3, "the fixture indexes three units, one of them cut")
		emitted := make([]string, 0, len(floor1))
		for _, unit := range floor1 {
			// The absence of the hash is the cut (§2.2), and a cut unit owes
			// nothing. Dispositioning only the rest is what makes round 2 owe
			// zero — a fact of the fixture, required rather than assumed.
			if unit.TextSHA == "" {
				continue
			}
			emitted = append(emitted, groundCarryRowFor(unit, 1))
		}
		require.Len(t, emitted, 2, "two of the three index rows are floor units")
		_, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", writeGroundRows(t, dir, emitted...))
		require.Equal(t, 0, code, "stderr: %s", stderr)

		// Round 2, on a spec nobody edited: every unit's text stands, so §8
		// carries every disposition and the prompt asks for none of them.
		out := groundEmit(t, dir)
		require.Equal(t, float64(2), out["round"])
		require.Contains(t, out["prompt"], "owes a disposition for 0 of the 2 floor units above",
			"the round must actually ask for nothing, or the empty payload below is the test author's choice")

		// The unit writes the empty file it was asked for.
		stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", writeGroundRows(t, dir))
		require.Equal(t, 0, code, "stdout: %s stderr: %s", stdout, stderr)

		var round2 map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &round2))
		assert.Equal(t, float64(0), round2["rows"], "the payload decided nothing, because nothing was asked of it")
		assert.Equal(t, float64(2), round2["carried"], "and the round records the two dispositions §8 carried")

		recorded := groundRecordedRows(t, dir, 2)
		require.Len(t, recorded, 2, "a round whose payload is empty still writes the rows it inherited")
		for _, row := range recorded {
			assert.Equal(t, 1, row.CarriedFrom, "each names the round its disposition was decided in")
		}

		stdout, stderr, code = runTP(t, dir, "ground", "spec.md", "--status")
		require.Equal(t, 0, code, "stderr: %s", stderr)
		var status map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &status))
		assert.Equal(t, float64(2), status["emitted"])
		assert.Equal(t, float64(2), status["dispositioned"],
			"coverage does not stall on a round that decided nothing because nothing was owed")
	})

	// The control, and the case §7.1's rule was written for: the reader who ran
	// nothing. It is a SECOND round rather than a first, so the carry actually
	// runs and returns nothing — round 1 short-circuits before it, and a
	// refusal there cannot tell a rule about the sum from a rule about the
	// payload.
	t.Run("empty with nothing to carry", func(t *testing.T) {
		dir := t.TempDir()

		writeGroundCarrySpec(t, dir, groundCarryRound1)
		groundEmit(t, dir)
		floor1 := groundEmittedFloor(t, dir, 1)
		require.Len(t, floor1, 1, "round 1's floor is its one sentence")
		_, stderr, code := runTP(t, dir, "ground", "spec.md", "--record",
			writeGroundRows(t, dir, groundCarryRowFor(floor1[0], 1)))
		require.Equal(t, 0, code, "stderr: %s", stderr)

		// Round 2's text holds none of round 1's wording, so §8 carries nothing.
		writeGroundCarrySpec(t, dir, groundCarryRound2)
		out := groundEmit(t, dir)
		require.Contains(t, out["prompt"], "owes a disposition for each of the 3 floor units above",
			"nothing carries, so the round owes its whole floor")
		before := stateDirNames(t, dir)

		stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", writeGroundRows(t, dir))
		require.Equal(t, 1, code, "stdout: %s stderr: %s", stdout, stderr)
		envelope := groundErrorEnvelope(t, stderr)
		assert.Equal(t, float64(1), envelope["code"])
		assert.Equal(t, before, stateDirNames(t, dir),
			"a round that would write no row at all writes no file either")

		// The hint must be about THIS refusal. Both exit-1 refusals shared one
		// hint, so an empty record was told to "fix the row the message names"
		// when the message names no row — the reader is sent looking for a line
		// that is not there, in a file that has none.
		hint, _ := envelope["hint"].(string)
		assert.NotContains(t, hint, "the row the message names",
			"an empty record names no row, so a hint pointing at one sends the reader nowhere")
		assert.Contains(t, hint, "at least one row",
			"the hint must still say what would make the record recordable")
	})
}
