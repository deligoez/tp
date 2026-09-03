package engine

// GroundCoverage is §8's answer to *did anyone look*: dispositions over the
// emitted floor, with the rows that move neither side counted apart.
//
// **Coverage says a unit was decided, never that it was decided well.** Every
// one of §3's six verdicts is a disposition — `UNVERIFIABLE` is a settled
// answer and not a gap, and a `FAIL` is a disposition too, so a spec whose
// every claim was refuted is 100% covered. Nothing in this file reads
// `verdict`, which makes that structural rather than a list of six a mutant
// could shorten by one. What was found is the per-verdict breakdown §8 puts
// beside the ratio, and it is a separate count.
//
// The ratio itself is deliberately not a field. Two integers say the same thing
// without deciding what to do when the floor is empty, and §9's advisory and
// §8's `--status` want the counts anyway — the floor's size is one of them.
type GroundCoverage struct {
	// Emitted is §8's denominator: the floor units tp emitted for the round.
	// A cut unit is not one of them — it carries no hash and owes no
	// disposition (§2.2).
	Emitted int
	// Dispositioned is §8's numerator: how many of those emitted units a
	// recorded row decided. It counts units and not rows, so it cannot exceed
	// Emitted.
	Dispositioned int
	// ReaderAdded counts the rows whose `unit_id` is null: the claims a reader
	// added because §2.2's floor is a floor. §8 reports them apart from
	// coverage and keeps them out of the denominator, because tp cannot know
	// the set it did not emit (Non-Goal 5), so a denominator that pretended to
	// would be unmeasurable.
	ReaderAdded int
	// OffFloor counts the rows carrying a `unit_id` that names no emitted floor
	// unit: a cut unit the reader graded anyway — §2.2 says three of the first
	// end-to-end run's five added claims were cut units — or an id the index
	// never emitted.
	//
	// §8 names one separate count, keyed on `unit_id: null`, and §2.2 says a
	// cut unit a reader grounds "is a reader-added row". Those two sentences
	// cannot both be read into one counter without one of them bending, so the
	// case §8's parenthetical does not reach gets its own number rather than
	// being folded into `ReaderAdded` or, worse, counted nowhere: a row that no
	// counter sees is a row an operator cannot find.
	OffFloor int
}

// GroundCoverageOf computes §8's coverage of a round's recorded rows over the
// floor its emission produced.
//
// **The numerator is derived by asking, of each emitted floor unit, whether a
// row decided it — never by counting rows.** That makes §11 row 15's mutant
// unwriteable rather than merely untested: a reader-added row carries no
// `unit_id`, so there is no unit for it to be the disposition of, and no
// quantity of added claims can raise coverage in place of dispositioning the
// floor. The same construction settles three further readings without a rule
// for each — a second row for a unit already decided disposition it once, a row
// naming a unit the index never emitted raises nothing, and the ratio is
// bounded at 1 whatever the record holds.
//
// floor is the index as emitted, cut rows included; this filters them, on §2.2's
// convention that the absence of the hash is the cut.
func GroundCoverageOf(floor []FloorIndexRow, rows []GroundRow) GroundCoverage {
	emitted := make(map[string]bool, len(floor))
	for _, r := range floor {
		if r.TextSHA == "" {
			continue
		}
		emitted[r.ID] = true
	}

	decided := make(map[string]bool, len(rows))
	cov := GroundCoverage{Emitted: len(emitted)}
	// Indexed rather than ranged by value: a GroundRow is 192 bytes and
	// `gocritic`'s rangeValCopy is in this repository's enabled set.
	for i := range rows {
		row := &rows[i]
		switch {
		case row.UnitID == nil:
			cov.ReaderAdded++
		case !emitted[*row.UnitID]:
			cov.OffFloor++
		default:
			decided[*row.UnitID] = true
		}
	}
	cov.Dispositioned = len(decided)
	return cov
}
