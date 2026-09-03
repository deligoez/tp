package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// groundFloorSectionOf returns the prompt's floor section for a floor holding a
// cut unit, which is the state both tests below are about.
func groundFloorSectionOf(t *testing.T, specPath string) string {
	t.Helper()
	index := "# commit unknown\nu1 §1 0123456789ab #1 12B\nu2 §1 (cut)\n# 1 in floor, 1 cut\n"
	prompt := buildGroundPrompt(specPath, ".tp-review/spec/snapshot-ground-round-1.md",
		index, "ground-r1.ndjson", 1, 1, 0)
	i := strings.Index(prompt, "## The floor")
	require.GreaterOrEqual(t, i, 0, "the prompt must still carry a floor section")
	return prompt[i:]
}

// TestACutUnitIsGroundedUnderItsOwnIdAndNotUnderNull is §2.2's deleted sentence,
// asserted where it can be decided: at the recorder.
//
// **The verdict is the coverage arm, not the prose arm.** §2.2 records deleting
// the reading that a claim inside a cut unit "is a reader-added row", because a
// cut unit HAS an id — and GroundCoverageOf agrees: a row naming the cut unit
// lands in OffFloor and a `unit_id: null` row lands in ReaderAdded, two counts
// §8 keeps apart precisely because they are evidence about different halves of
// §2.1. The prompt told the reader to use `null` for both, which leaves OffFloor
// permanently 0 — "collapsing them loses which half to go and look at", the
// reason groundStatusResult's own doc gives for having two fields.
//
// Measured on a built fixture with one cut unit, before the prompt was
// corrected: the same claim recorded under `u4` gave `off_floor: 1`, and under
// `null` gave `reader_added: 1`. Both exited 0, so nothing but the instruction
// decides which count an operator ends up reading.
//
// The prose arm below is CORROBORATION and cannot be the verdict: a Contains or
// NotContains is a local assertion inside an unbounded generated text, so a
// paragraph re-adding the instruction elsewhere in the section passes both. It
// catches the sentence being deleted or reworded, which is what it is for.
func TestACutUnitIsGroundedUnderItsOwnIdAndNotUnderNull(t *testing.T) {
	floor := []engine.FloorIndexRow{
		{ID: "u1", Anchor: "§1", TextSHA: "0123456789ab", Ordinal: 1, Bytes: 12},
		{ID: "u2", Anchor: "§1", TextSHA: "", Ordinal: 0},
	}
	require.Empty(t, floor[1].TextSHA, "u2 must be a unit the arms cut, or neither arm below is about a cut")

	cutID := "u2"
	named := engine.GroundCoverageOf(floor, []engine.GroundRow{
		{UnitID: &cutID, Anchor: "§1", TextSHA: "ffffffffffff", Ordinal: 1, Verdict: engine.VerdictFail},
	})
	assert.Equal(t, 1, named.OffFloor, "a row naming the cut unit says the arms produced it and cut it wrongly")
	assert.Equal(t, 0, named.ReaderAdded)

	null := engine.GroundCoverageOf(floor, []engine.GroundRow{
		{UnitID: nil, Anchor: "§1", TextSHA: "ffffffffffff", Ordinal: 1, Verdict: engine.VerdictFail},
	})
	assert.Equal(t, 1, null.ReaderAdded, "a null row says the arms never produced the unit — the other fact")
	assert.Equal(t, 0, null.OffFloor,
		"so a prompt sending cut units to null leaves off_floor unreachable, whatever the round found")

	section := groundFloorSectionOf(t, "spec.md")
	assert.Contains(t, section, "that unit's own `unit_id`",
		"the prompt sends a claim inside a cut unit to the cut unit's id")
	assert.NotContains(t, section, `including one inside a\ncut unit — is recorded with "unit_id": null`,
		"and not to null, which §2.2 deleted")
}

// TestThePromptNamesUnitsAndMeasuresTheTextThatModePrints is §2.2's companion
// clause: the index is text-free because `--units` exists, so a prompt that
// never names it leaves the reader with a length and nowhere to spend it.
//
// **The falsifying arm is the substring one, and it is about artifacts rather
// than about wording.** The old sentence put the unit's text "in the snapshot"
// and called `<bytes>` "how you tell where it ends". A unit is canonicalised, so
// it is generally not a byte range of any file: the fixture below wraps one
// sentence across two blockquoted lines, and the canonical unit is then not a
// substring of the source at all — measured at scale on `spec/1.0.0.md`, 327 of
// 351 units. The index's own byte count is a length of the CANONICAL text, which
// is what `--units` prints, and the two agreeing is §11 row 4b's shipped test.
//
// The flag assertion is a read-back on the command tp actually registers, not a
// search for the word: a prompt naming a mode that does not exist is worse than
// one naming none.
func TestThePromptNamesUnitsAndMeasuresTheTextThatModePrints(t *testing.T) {
	source := "# Fixture\n\n## 1. Claims\n\n> The cap is `10` files, and the budget is 28800\n> seconds in every run.\n"

	units := engine.FloorUnitRows(source)
	require.Len(t, units, 1, "the fixture is one canonical unit spanning two source lines")
	require.NotContains(t, source, units[0].Text,
		"the canonical unit is NOT a substring of its own source, which is why the snapshot cannot be measured")

	index := engine.FloorIndexRows(source, func(int) string { return "§1" })
	var indexed engine.FloorIndexRow
	for _, row := range index {
		if row.ID == units[0].ID {
			indexed = row
		}
	}
	require.NotEmpty(t, indexed.TextSHA, "the listing and the index join on unit_id, and this unit is on the floor")
	require.Equal(t, engine.FloorTextSHA(units[0].Text), indexed.TextSHA,
		"the hash the index carries is the hash of what --units prints")
	assert.Equal(t, len(units[0].Text), indexed.Bytes,
		"`<bytes>` is the length of what --units prints, and of nothing that is in the snapshot")

	require.NotNil(t, newGroundCmd().Flags().Lookup("units"),
		"the prompt may only name a mode tp registers")

	section := groundFloorSectionOf(t, "spec/1.0.0.md")
	assert.Contains(t, section, fmt.Sprintf("tp ground %s --units", "spec/1.0.0.md"),
		"the prompt names the mode WITH the spec, so a reader does not have to work out the argument")
	assert.NotContains(t, section, "is how you tell where it ends",
		"and no longer says the byte count locates the unit's end in the snapshot")
}
