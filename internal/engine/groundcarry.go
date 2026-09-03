package engine

import "fmt"

// GroundCarryError reports that §8's carry-forward could not read the round it
// carries from.
//
// It is a distinct type rather than the wrapped error alone because the two
// files `--record` reads have different owners: the payload is the operator's
// and a failure in it names a cell they can fix, while `ground-round-<N-1>`
// is tp's own artifact and a failure in it names nothing the operator wrote.
// Without the distinction a truncated round file surfaces as the
// *GroundLineError inside it, and §7.1's exit-1 branch sends the operator to
// edit a line of a file they never opened.
//
// Unwrap keeps that inner error reachable, so "which file" and "what was wrong
// with it" stay two questions with two answers.
type GroundCarryError struct {
	// Round is the round that could not be read: the source of the carry, not
	// the round being recorded.
	Round int
	// Err is what reading it failed on.
	Err error
}

func (e *GroundCarryError) Error() string {
	return fmt.Sprintf("cannot read ground round %d, which round %d carries its dispositions from: %v",
		e.Round, e.Round+1, e.Err)
}

func (e *GroundCarryError) Unwrap() error { return e.Err }

// groundJoinKey is §8's join key: `(text_sha, ordinal)`.
//
// **`text_sha` alone is not unique and cannot be made so.** Measured over this
// repository's `spec/*.md`, one unit's text occurs five times in a single file
// (`**Exit codes:** 0 = success.` in `spec/0.1.0.md`), so a hash-only join
// matches the first recorded row for all five and hands every one of them the
// same disposition. `ordinal` is the 1-based index of a unit among those sharing
// its hash in emission order, which separates them and is stable under an edit
// anywhere else in the document.
//
// The two cells the key does NOT use are `unit_id` and `anchor`, and that is the
// whole of what makes a second pass cheap. `unit_id` is numbered over every unit
// §2.1 produces, so inserting one sentence renumbers every unit below it;
// `anchor` moves when a unit is moved between sections. Joining on either would
// re-decide the whole floor after an edit that changed one sentence.
type groundJoinKey struct {
	textSHA string
	ordinal int
}

// groundCarryForward is §8's second pass: the dispositions round N inherits from
// the round immediately before it.
//
// **The lookup is round N-1 alone, never the union of all earlier rounds.** A
// unit reverted in round N to a wording last dispositioned in round 1 and absent
// from round N-1 is therefore uncovered rather than resurrected: the round that
// read that text is two rounds old, the text has been away and come back since,
// and a disposition made against it is a claim about a document that no longer
// existed when round N-1 looked. The two readings diverge only at round 3, which
// is why §11 row 16's fixture has three rounds.
//
// decided is the round's own payload. A unit it decides is not also carried: the
// two rows would carry different verdicts for one unit and nothing in the record
// would say which the round meant. §8's coverage counts units and would show the
// same ratio either way, so only the file can see this.
//
// prevRound is the number of the round prev was recorded as, used for
// `carried_from` on a disposition that has not been carried before.
func groundCarryForward(floor []FloorIndexRow, prev []GroundRow, prevRound int, decided []GroundRow) []GroundRow {
	if len(floor) == 0 || len(prev) == 0 {
		return nil
	}

	source := make(map[groundJoinKey]*GroundRow, len(prev))
	for i := range prev {
		row := &prev[i]
		// A reader-added row supplies its own text_sha over text tp never
		// emitted (§7.2), so its ordinal is the reader's reckoning and not an
		// index over any floor. It is a claim, not the disposition of a unit,
		// and §8 keeps it out of the ratio for the same reason.
		if row.UnitID == nil {
			continue
		}
		key := groundJoinKey{textSHA: row.TextSHA, ordinal: row.Ordinal}
		if _, seen := source[key]; !seen {
			source[key] = row
		}
	}

	own := make(map[string]bool, len(decided))
	for i := range decided {
		if decided[i].UnitID != nil {
			own[*decided[i].UnitID] = true
		}
	}

	carried := make([]GroundRow, 0, len(floor))
	for _, unit := range floor {
		// The absence of the hash is the cut (§2.2). A cut unit owes no
		// disposition, so it inherits none.
		//
		// This half of the condition is a fence on the argument rather than a
		// branch a validated record reaches: §7.2 makes `text_sha` twelve hex
		// characters and `ordinal` at least 1, so no source key can be the
		// `("", 0)` a cut index row would look itself up under. Measured —
		// deleting it leaves the suite green until the test's own fixture
		// carries that key on both sides.
		if unit.TextSHA == "" || own[unit.ID] {
			continue
		}
		row, ok := source[groundJoinKey{textSHA: unit.TextSHA, ordinal: unit.Ordinal}]
		if !ok {
			continue
		}
		carried = append(carried, groundCarriedRow(row, unit, prevRound))
	}
	if len(carried) == 0 {
		return nil
	}
	return carried
}

// groundCarriedRow is one inherited disposition, rewritten into the carrying
// round's identity.
//
// The disposition itself — `verdict`, `kind`, `tier`, `evidence` and everything
// conditional on them — is the source row's untouched: §8 says the carried row
// keeps "its original tier and evidence", and a carry that re-derived any of
// them would be a fresh decision nobody made. What is replaced is the identity:
// `unit_id` and `anchor` are the carrying round's, because those are what a
// later reader and §8's coverage join a row to a sentence by, and both can move
// while the text does not.
//
// `carried_from` names **the round the disposition was first decided in**
// (§7.2), which is why a source row that already carries one keeps it rather
// than being restamped with prevRound. A field that were always N-1 would be
// derivable from the round it sits in and would say nothing at all; as written
// it answers how long a `FAIL` has stood unrepaired, which is the question §8
// puts on it.
func groundCarriedRow(source *GroundRow, unit FloorIndexRow, prevRound int) GroundRow {
	row := *source
	id := unit.ID
	row.UnitID = &id
	row.Anchor = unit.Anchor
	// Copied rather than shared: the carried row outlives the parse of the round
	// it came from, and two rows sharing one backing array is a defect nothing
	// in this package would report.
	if source.Causes != nil {
		row.Causes = append([]GroundCause(nil), source.Causes...)
	}
	if row.CarriedFrom == 0 {
		row.CarriedFrom = prevRound
	}
	return row
}
