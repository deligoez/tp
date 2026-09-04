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
// cut unit.
//
// The cut unit is what one caller is about; the others use this helper for a
// floor section on any index and do not turn on it. The doc said "both tests
// below" when there were two callers and only one of them was about the cut
// unit, and a third arrived with a later repair — so the sentence was counting
// callers rather than saying what the helper hands back.
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
	t.Parallel()
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
	// One line of the deleted sentence, not a span across its line break: the
	// prompt is a raw string with real newlines in it, and a `\n` written inside
	// a Go raw literal is two characters that cannot occur there — so the
	// spanning form of this assertion passes on every input, including the text
	// it was written to reject. Caught by running it against that text.
	assert.NotContains(t, section, `cut unit — is recorded with "unit_id": null`,
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
// substring of the source at all — at scale on `spec/1.0.0.md` that is most of
// the floor, and groundPromptFloor's doc carries the derivation rather than a
// number, because two runs a few hours apart disagreed while the spec moved
// under them. The index's own byte count is a length of the CANONICAL text,
// which is what `--units` prints, and the two agreeing is §11 row 4b's test.
//
// The flag assertion is a read-back on the command tp actually registers, not a
// search for the word: a prompt naming a mode that does not exist is worse than
// one naming none.
func TestThePromptNamesUnitsAndMeasuresTheTextThatModePrints(t *testing.T) {
	t.Parallel()
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

// groundSHAForID returns the hash the emitted index carries for one unit_id.
func groundSHAForID(t *testing.T, index []engine.FloorIndexRow, id string) string {
	t.Helper()
	for _, row := range index {
		if row.ID == id {
			return row.TextSHA
		}
	}
	t.Fatalf("the index carries no row for %s", id)
	return ""
}

// TestTheListingAndTheIndexAgreeOnlyWhileTheSpecHasNotMoved is §11 row 4c: the
// two artifacts join on `unit_id` CONDITIONALLY, and the emission owes the
// reader the check rather than the assurance.
//
// **The defect the row exists for.** `--units` reads the spec as it now stands
// while the index was frozen at emission, and `unit_id` is numbered over every
// unit §2.1 produces — so one sentence inserted above a unit renumbers every
// unit below it (`groundcarry.go`'s `groundJoinKey`, which is why §8's key is
// `(text_sha, ordinal)` and not the id). Measured end to end on a copy of this
// release's own spec before the prompt was corrected: after the insert, the
// listing's `u4` was the new sentence and the index's `u4` was the old one, a
// row copying `unit_id`/`text_sha`/`ordinal` from the index while grading the
// listing's text recorded at **exit 0**, and `--status --check` reported the
// unit covered. `groundRowMatchesFloor` cannot see it — the cells ARE the
// index's — so the only place it can be caught is before the grading.
//
// **The verdict is the two hash arms, not the prose.** They are bounded
// read-backs over two derived artifacts, and together they say the prescribed
// check is worth prescribing: it fires on the edit and is silent on the spec
// the round emitted from. A prompt sentence is an unbounded text, so the
// Contains/NotContains below is corroboration that this repair is still in it —
// it catches the sentence being deleted or reverted, and nothing else.
func TestTheListingAndTheIndexAgreeOnlyWhileTheSpecHasNotMoved(t *testing.T) {
	t.Parallel()
	// The opening sentence carries no digit, no backtick and no listed verb, so
	// the arms cut it. That is not decoration: it is what makes the unedited arm
	// discriminating. Measured — on a fixture whose every unit is in the floor,
	// numbering over the floor alone and numbering over every unit agree, and
	// this test stayed GREEN under that mutant until the cut unit was added.
	const emitted = "# Fixture\n\n## 1. Claims\n\n" +
		"It writes nothing at all. " +
		"The cap is `10` files here. The budget is 28800 seconds in every run.\n"
	const edited = "# Fixture\n\n## 1. Claims\n\n" +
		"It writes nothing at all. A probe sentence measured 1 thing. " +
		"The cap is `10` files here. The budget is 28800 seconds in every run.\n"

	anchor := func(int) string { return "§1" }
	index := engine.FloorIndexRows(emitted, anchor)
	require.Len(t, index, 3, "the emitted index is the frozen artifact both arms are read against")
	require.Empty(t, index[0].TextSHA, "u1 must be a unit the arms cut, or the unedited arm below decides nothing")

	// The control. On the text the round emitted from, every id agrees on its
	// hash — so a mismatch means the spec moved and never means the check is
	// noisy. Without this arm the edited arm passes under a `--units` that
	// numbered over the floor alone, where the ids disagree unconditionally and
	// the prescribed comparison would fire on every round and mean nothing.
	unmoved := engine.FloorUnitRows(emitted)
	require.Len(t, unmoved, 2, "the cut unit gets an id and no row")
	for _, row := range unmoved {
		require.Equal(t, groundSHAForID(t, index, row.ID), engine.FloorTextSHA(row.Text),
			"unedited, the listing and the index agree on both cells at every id")
	}

	// The break. One sentence inserted above the floor's first unit.
	listing := engine.FloorUnitRows(edited)
	require.Len(t, listing, 3, "the inserted sentence carries a digit, so the arms keep it")
	require.Equal(t, "A probe sentence measured 1 thing.", listing[0].Text)
	require.Equal(t, "u2", listing[0].ID, "the id the index gives the first floor unit")

	assert.NotEqual(t, groundSHAForID(t, index, "u2"), engine.FloorTextSHA(listing[0].Text),
		"the listing's u2 is now the inserted sentence, which the index's u2 does not name")
	assert.Equal(t, groundSHAForID(t, index, "u2"), engine.FloorTextSHA(listing[1].Text),
		"the sentence the index calls u2 has moved to u3 — the join holds on neither cell of that id")

	section := groundFloorSectionOf(t, "spec.md")
	assert.Contains(t, section, "COMPARE THE HASHES BEFORE YOU GRADE",
		"the emission owes the reader the check, not the assurance")
	assert.NotContains(t, section, "join on `unit_id` and agree on both cells. The snapshot",
		"and no longer states the agreement unconditionally")

	// The same repair in the section a unit reads FIRST. `## The row` gives the
	// copy instruction and `## The floor`, further down, gives the check; read
	// in order they agree, but a unit acting on the copy instruction alone does
	// exactly what the check exists to prevent, and the two hash arms above are
	// why that is not hypothetical. So the condition has to travel with the
	// instruction, not only with the explanation.
	//
	// This is only a reversion detector: it catches the clause being deleted or
	// the old wording coming back, and it cannot see a third sentence added
	// below that contradicts it. That limit is measured rather than reasoned —
	// appending a paragraph to groundPromptRow's output telling the reader the
	// comparison is a formality and to copy the cells unchanged leaves the whole
	// package green. The return is a built string and not a constant, so there
	// is an elsewhere for a negation to sit, which is the shape §11 row 4c's own
	// comment warns about.
	row := groundPromptRow("spec.md", ".tp-review/spec/snapshot-ground-round-1.md", 1)
	assert.Contains(t, row, "and copy them only once the floor section's hash",
		"the copy instruction carries the condition where the copying is asked for")
	assert.NotContains(t, row, "sentence you graded. Required on every row",
		"and no longer runs straight from the copy instruction to the field list")
}
