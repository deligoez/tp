package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// groundCarriedSource is a recorded row for a unit whose identity in the round
// it was recorded in is deliberately NOT the identity it will have in the round
// that carries it.
//
// `unit_id` and `anchor` are the two cells §8's join does not use, and the two
// that must be replaced by the carrying round's own: `unit_id` is numbered over
// every unit §2.1 produces, so an edit anywhere above a unit renumbers it, and a
// unit can move between sections without its text changing. A carried row
// keeping the source's id would name a different sentence, and §8's coverage
// joins on `unit_id` — so the disposition would land on whatever unit now holds
// that number, or on none.
func groundCarriedSource(sha string, ordinal int, v GroundVerdict) GroundRow {
	id := "u99"
	return GroundRow{
		UnitID:   &id,
		Anchor:   "§99",
		TextSHA:  sha,
		Ordinal:  ordinal,
		Verdict:  v,
		Kind:     KindDocument,
		Tier:     TierRead,
		Evidence: "read spec/1.0.0.md line 737",
	}
}

// TestAUnitUnchangedFromThePrecedingRoundCarriesItsDisposition is §8's carry
// forward in its simplest shape, and it pins the four things the carried row
// takes from two different places.
//
// The disposition — verdict, kind, tier, evidence — is the SOURCE round's, byte
// for byte: §8 says the carried row keeps "its original tier and evidence". The
// identity — unit_id and anchor — is the CARRYING round's, because those are
// what a later reader and §8's own coverage join the row to a sentence by. The
// source row therefore carries an id and an anchor that exist in no floor here,
// so an implementation copying the row whole is visible rather than merely
// unasserted.
func TestAUnitUnchangedFromThePrecedingRoundCarriesItsDisposition(t *testing.T) {
	floor := []FloorIndexRow{
		{ID: "u1", Anchor: "§1", TextSHA: "aaaaaaaaaaaa", Ordinal: 1, Bytes: 30},
		{ID: "u2", Anchor: "§2", TextSHA: "bbbbbbbbbbbb", Ordinal: 1, Bytes: 30},
	}
	prev := []GroundRow{groundCarriedSource("aaaaaaaaaaaa", 1, VerdictFail)}

	carried := groundCarryForward(floor, prev, 2, nil)

	require.Len(t, carried, 1, "one floor unit matches the preceding round, and only one")
	require.NotNil(t, carried[0].UnitID)
	assert.Equal(t, "u1", *carried[0].UnitID, "the carrying round's id, not the source round's u99")
	assert.Equal(t, "§1", carried[0].Anchor, "the carrying round's anchor, not the source round's §99")
	assert.Equal(t, "aaaaaaaaaaaa", carried[0].TextSHA)
	assert.Equal(t, 1, carried[0].Ordinal)
	assert.Equal(t, VerdictFail, carried[0].Verdict,
		"§8 makes an unrepaired FAIL permanent while its text stands")
	assert.Equal(t, KindDocument, carried[0].Kind)
	assert.Equal(t, TierRead, carried[0].Tier, "its ORIGINAL tier")
	assert.Equal(t, "read spec/1.0.0.md line 737", carried[0].Evidence, "its ORIGINAL evidence")
	assert.Equal(t, 2, carried[0].CarriedFrom, "carried_from names the round the disposition was made in")
}

// TestAUnitTheRoundDecidedItselfIsNotAlsoCarried keeps a round from holding two
// dispositions for one unit.
//
// A second pass covers the units whose text changed (§8), so a unit the round
// decided for itself is not one the preceding round's answer is wanted for. The
// two rows would carry different verdicts and nothing in the record would say
// which one the round meant; §8's coverage would count the unit once either way,
// so the ratio cannot see this and only the file can.
func TestAUnitTheRoundDecidedItselfIsNotAlsoCarried(t *testing.T) {
	floor := []FloorIndexRow{{ID: "u1", Anchor: "§1", TextSHA: "aaaaaaaaaaaa", Ordinal: 1, Bytes: 30}}
	prev := []GroundRow{groundCarriedSource("aaaaaaaaaaaa", 1, VerdictFail)}
	fresh := "u1"
	decided := []GroundRow{{
		UnitID: &fresh, Anchor: "§1", TextSHA: "aaaaaaaaaaaa", Ordinal: 1,
		Verdict: VerdictPass, Kind: KindDocument, Tier: TierRead, Evidence: "re-read it",
	}}

	assert.Empty(t, groundCarryForward(floor, prev, 2, decided),
		"the round's own disposition stands; the preceding round's is not appended beside it")
}

// TestAReaderAddedRowIsNotACarriableDisposition fences the lookup to rows that
// decided a floor unit.
//
// A reader-added row carries `unit_id: null` and supplies its own `text_sha`
// over text tp never emitted (§7.2), so its `ordinal` is the reader's own
// reckoning rather than an index over the round's floor. Joining on it would let
// a claim the floor did not carry become the disposition of a unit the floor
// later does carry, on a key the two never shared a namespace for.
func TestAReaderAddedRowIsNotACarriableDisposition(t *testing.T) {
	floor := []FloorIndexRow{{ID: "u1", Anchor: "§1", TextSHA: "aaaaaaaaaaaa", Ordinal: 1, Bytes: 30}}
	added := groundCarriedSource("aaaaaaaaaaaa", 1, VerdictFail)
	added.UnitID = nil

	assert.Empty(t, groundCarryForward(floor, []GroundRow{added}, 2, nil),
		"a claim the floor did not emit is not a disposition of a floor unit")
}

// TestACutUnitCarriesNothing keeps §2.2's cut units out of the carry for the
// same reason §8 keeps them out of the denominator: a cut unit owes no
// disposition, so there is nothing for a later round to inherit.
//
// The absence of the hash is the cut, so a carried row for one is only
// producible by an implementation that ignores the hash — which is exactly the
// implementation that would match every cut unit of round N against every cut
// unit of round N-1, all of them on the empty key.
func TestACutUnitCarriesNothing(t *testing.T) {
	floor := []FloorIndexRow{{ID: "u1", Anchor: "§1"}}
	prev := []GroundRow{groundCarriedSource("", 1, VerdictFail)}

	assert.Empty(t, groundCarryForward(floor, prev, 2, nil),
		"a unit with no hash owes no disposition and inherits none")
}

// TestCarriedFromNamesTheRoundTheDispositionWasFirstDecidedIn is §7.2's cell:
// "the round it was first decided in".
//
// The two readings of §8's "the round it came from" diverge only on a unit that
// has already been carried once, which no two-round fixture reaches. Here the
// source row is round 2's own carried copy of a round-1 disposition, and the
// answer is 1: the carrying round is 3, the source round is 2, and neither is
// the round the disposition was made in.
//
// A field that were always N-1 would be derivable from the round it sits in and
// would carry no information at all, which is this repository's standing reason
// against a key whose value is fixed.
func TestCarriedFromNamesTheRoundTheDispositionWasFirstDecidedIn(t *testing.T) {
	floor := []FloorIndexRow{{ID: "u1", Anchor: "§1", TextSHA: "aaaaaaaaaaaa", Ordinal: 1, Bytes: 30}}
	source := groundCarriedSource("aaaaaaaaaaaa", 1, VerdictFail)
	source.CarriedFrom = 1

	carried := groundCarryForward(floor, []GroundRow{source}, 2, nil)

	require.Len(t, carried, 1)
	assert.Equal(t, 1, carried[0].CarriedFrom,
		"a disposition carried a second time still names the round it was made in, not the round it was copied from")
}

// groundIdenticalUnits is a spec text whose n paragraphs are byte-identical, so
// §2.1 produces n units sharing one text_sha and §2.2 numbers them 1..n.
func groundIdenticalUnits(n int) string {
	return strings.Repeat("**Exit codes:** 0 = success.\n\n", n)
}

// TestFiveIdenticalUnitsCarryIndependently is §11 row 18c.
//
// Five units with identical canonical text share one `text_sha`, so the join
// key has to be the pair: on `text_sha` alone every one of the five matches the
// first recorded row, and all five inherit that row's verdict. Each source row
// therefore carries a DIFFERENT verdict, so a hash-only join is visible as five
// copies of one answer rather than as a count that still comes to five.
//
// The fixture's own premise — that the five units really do share a hash and
// really are numbered 1 to 5 — is required rather than assumed, because a
// canonicalisation that made them differ would leave this test passing for a
// reason it does not name.
func TestFiveIdenticalUnitsCarryIndependently(t *testing.T) {
	text := groundIdenticalUnits(5)
	floor := FloorIndexRows(text, FloorAnchorOf(text))
	require.Len(t, floor, 5, "the fixture must produce five units")

	verdicts := []GroundVerdict{
		VerdictPass, VerdictFail, VerdictPartial, VerdictUnverifiable, VerdictNotAClaim,
	}
	prev := make([]GroundRow, 0, len(floor))
	for i, unit := range floor {
		require.Equal(t, floor[0].TextSHA, unit.TextSHA, "the five units must share one hash")
		require.Equal(t, i+1, unit.Ordinal, "the five units must be numbered 1..5 in emission order")
		prev = append(prev, groundCarriedSource(unit.TextSHA, unit.Ordinal, verdicts[i]))
	}

	carried := groundCarryForward(floor, prev, 1, nil)

	require.Len(t, carried, 5)
	got := make([]GroundVerdict, 0, len(carried))
	ids := make([]string, 0, len(carried))
	for i := range carried {
		require.NotNil(t, carried[i].UnitID)
		ids = append(ids, *carried[i].UnitID)
		got = append(got, carried[i].Verdict)
	}
	assert.Equal(t, []string{"u1", "u2", "u3", "u4", "u5"}, ids)
	assert.Equal(t, verdicts, got,
		"each unit is matched by its own (text_sha, ordinal), so each keeps its own verdict")
}

// groundChangedAt60 returns two versions of one floor unit that share their
// first 60 bytes and differ at byte 61, with that property asserted rather than
// counted by eye.
func groundChangedAt60(t *testing.T) (before, after string) {
	t.Helper()
	const prefix = "The changing claim measured 5 things across the whole corpus"
	require.Len(t, prefix, 60, "the shared prefix must be exactly 60 bytes for the change to be at 61")
	before, after = prefix+" of specs.", prefix+", of specs."
	require.Equal(t, before[:60], after[:60], "the two versions agree up to byte 60")
	require.NotEqual(t, before[60], after[60], "and differ at byte 61")
	return before, after
}

// TestAUnitChangedAtCharacter61DoesNotCarry is §11 row 16's second clause, and
// the one that kills a join on `anchor` alone.
//
// The edit is late in a long sentence and leaves the anchor, the ordinal and the
// unit's position untouched: everything about the unit is what it was except the
// text. `text_sha` covers the whole unit and not a prefix, so the hash moves and
// the disposition does not follow it — which is the whole reason a second pass
// covers the units whose text changed.
func TestAUnitChangedAtCharacter61DoesNotCarry(t *testing.T) {
	before, after := groundChangedAt60(t)

	beforeFloor := FloorIndexRows(before, FloorAnchorOf(before))
	afterFloor := FloorIndexRows(after, FloorAnchorOf(after))
	require.Len(t, beforeFloor, 1)
	require.Len(t, afterFloor, 1)
	require.Equal(t, beforeFloor[0].ID, afterFloor[0].ID, "the unit keeps its id")
	require.Equal(t, beforeFloor[0].Anchor, afterFloor[0].Anchor, "and its anchor")
	require.Equal(t, beforeFloor[0].Ordinal, afterFloor[0].Ordinal, "and its ordinal")
	require.NotEqual(t, beforeFloor[0].TextSHA, afterFloor[0].TextSHA, "only the hash moves")

	prev := []GroundRow{groundCarriedSource(beforeFloor[0].TextSHA, 1, VerdictPass)}

	assert.Empty(t, groundCarryForward(afterFloor, prev, 1, nil),
		"a unit whose text changed is uncovered until this round decides it")
	assert.Len(t, groundCarryForward(beforeFloor, prev, 1, nil), 1,
		"the control: the same source row does carry onto the unedited text")
}

// groundRoundOnDisk writes a recorded round file directly, so a test can build
// the history a carry reads without recording each round through the writer
// under test.
func groundRoundOnDisk(t *testing.T, specPath string, round int, rows ...GroundRow) {
	t.Helper()
	require.NoError(t, os.MkdirAll(ReviewStateDir(specPath), 0o755))
	data, err := appendGroundRows(nil, rows)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(GroundRoundPath(specPath, round), data, 0o600))

	// The fixture must be a record tp would accept, or a test built on it says
	// nothing about a real round.
	parsed, err := ParseGroundRows(data)
	require.NoError(t, err, "the fixture round must pass §7.2's table")
	require.Len(t, parsed, len(rows))
}

// TestARecordedRoundHoldsItsCarriedDispositions is §8's "written into round N's
// own file, rather than being computed at --status".
//
// The payload decides one unit and the file holds two rows: the payload's bytes
// first, unchanged, and the carried row after them. Asserting the payload is a
// PREFIX rather than merely present is what keeps §7.1's other rule intact — the
// recorded file holds the payload's bytes, not a re-serialisation of them — so
// an implementation that round-trips the operator's rows through the parser
// fails here even though every row survives.
func TestARecordedRoundHoldsItsCarriedDispositions(t *testing.T) {
	specPath := groundEmittedDir(t, 2)
	floor := []FloorIndexRow{
		{ID: "u1", Anchor: "§1", TextSHA: "aaaaaaaaaaaa", Ordinal: 1, Bytes: 30},
		{ID: "u2", Anchor: "§2", TextSHA: "bbbbbbbbbbbb", Ordinal: 1, Bytes: 30},
	}
	groundRoundOnDisk(t, specPath, 1, groundCarriedSource("bbbbbbbbbbbb", 1, VerdictFail))

	payload := groundRecordPayload(groundNumberedRow(t, 1))
	rows, carried, err := RecordGroundRound(specPath, 2, payload, floor)
	require.NoError(t, err)
	require.Len(t, rows, 1, "the rows the unit wrote")
	require.Len(t, carried, 1, "and the one the round inherited")

	written, err := os.ReadFile(filepath.Join(ReviewStateDir(specPath), "ground-round-2.ndjson"))
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(written), string(payload)),
		"the payload's own bytes open the file unchanged")

	back, err := ParseGroundRows(written)
	require.NoError(t, err, "round N's file is a record tp can read back")
	require.Len(t, back, 2)
	require.NotNil(t, back[1].UnitID)
	assert.Equal(t, "u2", *back[1].UnitID)
	assert.Equal(t, VerdictFail, back[1].Verdict)
	assert.Equal(t, TierRead, back[1].Tier)
	assert.Equal(t, "read spec/1.0.0.md line 737", back[1].Evidence)
	assert.Equal(t, 1, back[1].CarriedFrom)

	assert.Equal(t, GroundCoverage{Emitted: 2, Dispositioned: 2},
		GroundCoverageOf(floor, back), "the carried disposition covers its unit")
}

// TestAWordingLastSeenTwoRoundsAgoIsNotResurrected is §11 row 16's third clause
// and the reason the fixture has three rounds.
//
// Round 1 decided the unit, round 2 does not hold it at all, and round 3 carries
// the same text again. The lookup is round N-1 alone, so the answer is that the
// unit is uncovered — an implementation searching every earlier round finds
// round 1's row and resurrects a disposition made against a wording that has
// since been away and come back, and a two-round fixture cannot tell the two
// apart.
func TestAWordingLastSeenTwoRoundsAgoIsNotResurrected(t *testing.T) {
	specPath := groundEmittedDir(t, 3)
	floor := []FloorIndexRow{
		{ID: "u1", Anchor: "§1", TextSHA: "aaaaaaaaaaaa", Ordinal: 1, Bytes: 30},
		{ID: "u2", Anchor: "§2", TextSHA: "cccccccccccc", Ordinal: 1, Bytes: 30},
	}
	groundRoundOnDisk(t, specPath, 1, groundCarriedSource("aaaaaaaaaaaa", 1, VerdictFail))
	groundRoundOnDisk(t, specPath, 2, groundCarriedSource("cccccccccccc", 1, VerdictPass))

	payload := groundRecordPayload(groundNumberedRow(t, 3))
	_, carried, err := RecordGroundRound(specPath, 3, payload, floor)
	require.NoError(t, err)

	require.Len(t, carried, 1, "only round 2's row is a source, and it matches one unit")
	require.NotNil(t, carried[0].UnitID)
	assert.Equal(t, "u2", *carried[0].UnitID,
		"u1's wording was last decided in round 1 and is absent from round 2, so it is uncovered")
	assert.Equal(t, 2, carried[0].CarriedFrom)
}

// TestACorruptPrecedingRoundIsNotReportedAsABadPayload separates the two files
// --record reads.
//
// The operator's payload is impeccable and the failure is in an artifact tp
// itself wrote, so reporting it through the same error a bad row takes would
// send the operator to edit a file that has nothing wrong with it. The verdict
// rests on the error's TYPE rather than on its sentence, and on the round file
// not appearing: a round whose carry could not be computed would claim coverage
// it did not inherit.
func TestACorruptPrecedingRoundIsNotReportedAsABadPayload(t *testing.T) {
	specPath := groundEmittedDir(t, 2)
	require.NoError(t, os.MkdirAll(ReviewStateDir(specPath), 0o755))
	require.NoError(t, os.WriteFile(GroundRoundPath(specPath, 1), []byte("{\n"), 0o600))

	_, _, err := RecordGroundRound(specPath, 2, groundRecordPayload(groundNumberedRow(t, 1)), nil)

	require.Error(t, err)
	var carryErr *GroundCarryError
	require.ErrorAs(t, err, &carryErr, "the failure names the round it could not read")
	assert.Equal(t, 1, carryErr.Round)

	_, statErr := os.Stat(GroundRoundPath(specPath, 2))
	assert.True(t, os.IsNotExist(statErr), "no round file is written over an unreadable history")
}

// TestACarriedRowRendersAsARowTheTableAccepts closes the loop between the
// writer and the reader.
//
// Round N's file is read back by §8's next carry, by §8's coverage and by §9's
// advisory, all of them through ParseGroundRow — so a carried row that the
// serialiser renders in a shape that table rejects would make the whole of round
// N unreadable, and every reader of it silent or refusing. The row exercised
// here carries every conditional cell §7.2 has, because the cells that can be
// rendered wrongly are exactly the ones whose presence is conditional.
func TestACarriedRowRendersAsARowTheTableAccepts(t *testing.T) {
	id := "u4"
	row := GroundRow{
		UnitID: &id, Anchor: "§7.2", TextSHA: "0123456789ab", Ordinal: 2,
		Verdict: VerdictQuestion, Kind: KindBehaviour, Tier: TierRun,
		Evidence: "ran tp ground spec/1.0.0.md --status",
		Causes: []GroundCause{
			{Cause: "the floor moved", Prediction: "the hash differs from round 1's"},
			{Cause: "the arms changed", Prediction: "the unit is cut"},
			{Cause: "the anchor moved", Prediction: "§7.3 holds it now"},
		},
		CarriedFrom: 2,
		Note:        "carried",
	}

	data, err := appendGroundRows(nil, []GroundRow{row})
	require.NoError(t, err)

	back, err := ParseGroundRows(data)
	require.NoError(t, err, "a serialised carried row passes §7.2's table")
	require.Len(t, back, 1)
	assert.Equal(t, row, back[0], "every cell survives the round trip unchanged")
}

// TestAnAbsentCellIsNotRenderedAsAnEmptyOne is the half of the serialiser the
// round trip above cannot see.
//
// A NOT-A-CLAIM row omits kind, tier and evidence, and §7.2 rejects an empty
// string on any of them — so an encoder writing every field unconditionally
// produces a row that no longer parses, and the round trip would fail with a
// message about a cell rather than about the encoder. Asserting on the bytes
// says which of the two it is.
func TestAnAbsentCellIsNotRenderedAsAnEmptyOne(t *testing.T) {
	id := "u4"
	data, err := appendGroundRows(nil, []GroundRow{{
		UnitID: &id, Anchor: "§7.2", TextSHA: "0123456789ab", Ordinal: 2, Verdict: VerdictNotAClaim,
	}})
	require.NoError(t, err)

	line := strings.TrimSpace(string(data))
	for _, absent := range []string{"kind", "tier", "evidence", "partial_kind", "held_at", "causes", "carried_from", "note"} {
		assert.NotContains(t, line, `"`+absent+`"`,
			"a cell the row does not carry is absent from the wire, not present and empty")
	}
	assert.Contains(t, line, `"unit_id":"u4"`, "and the cells it does carry are there")
	assert.Contains(t, line, `"verdict":"NOT-A-CLAIM"`)
}

// TestAReaderAddedRowRendersItsNullUnitID is the one cell where absent and null
// differ (§7.2), so an encoder using omitempty for the pointer would turn a
// reader-added claim into a row with no `unit_id` key — which ParseGroundRow
// rejects.
func TestAReaderAddedRowRendersItsNullUnitID(t *testing.T) {
	data, err := appendGroundRows(nil, []GroundRow{{
		Anchor: "§7.2", TextSHA: "0123456789ab", Ordinal: 1, Verdict: VerdictNotAClaim,
	}})
	require.NoError(t, err)

	assert.Contains(t, string(data), `"unit_id":null`)
	back, err := ParseGroundRows(data)
	require.NoError(t, err)
	require.Len(t, back, 1)
	assert.Nil(t, back[0].UnitID)
}

// TestAppendGroundRowsTerminatesAPayloadThatDoesNot is the join between the
// operator's bytes and tp's own.
//
// A --record file whose last line has no newline is legal — ParseGroundRows
// reads it as a row — so appending to it without terminating it first would glue
// the operator's last row and tp's first carried row into one line, which parses
// as neither.
func TestAppendGroundRowsTerminatesAPayloadThatDoesNot(t *testing.T) {
	id := "u1"
	unterminated := []byte(`{"unit_id":"u9","anchor":"§1","text_sha":"0123456789ab","ordinal":1,"verdict":"NOT-A-CLAIM"}`)

	data, err := appendGroundRows(unterminated, []GroundRow{{
		UnitID: &id, Anchor: "§1", TextSHA: "0123456789ab", Ordinal: 2, Verdict: VerdictNotAClaim,
	}})
	require.NoError(t, err)

	rows, err := ParseGroundRows(data)
	require.NoError(t, err)
	require.Len(t, rows, 2, "the payload's last row and the carried row are two lines, not one")
	require.NotNil(t, rows[0].UnitID)
	assert.Equal(t, "u9", *rows[0].UnitID)
}
