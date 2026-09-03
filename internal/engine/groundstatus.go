package engine

import "errors"

// ErrNoGroundEmission reports that a spec has no emitted ground round, so there
// is no round for `--status` to be about.
//
// It is a distinct sentinel rather than a zero-valued status because the two
// states differ in what an operator should do: a round emitted and not recorded
// is 0-of-N and wants dispositions, while nothing emitted at all wants
// `tp ground <spec>`. Collapsing them would also hand a later `--check` a
// 0-of-0 it could read as converged for a spec nobody has ever grounded.
var ErrNoGroundEmission = errors.New("no ground round has been emitted")

// GroundStatus is what §7.1's `--status` reports: the latest emitted round,
// §8's coverage of it, and the per-verdict breakdown §8 puts beside the ratio.
//
// **Coverage and the breakdown count different things, and that is the point.**
// Coverage counts emitted floor UNITS, answering *did anyone look*; the
// breakdown counts recorded ROWS, answering *what did they find*. So the
// breakdown's total is the round's row count and need not equal Dispositioned —
// a reader-added row and an off-floor row each carry a verdict while moving
// neither side of the ratio. Without the breakdown a round of 84 `FAIL`s and a
// round of 84 `PASS`es are one number, and `--check` exiting 0 reads as *the
// premises hold* (§11 row 22).
type GroundStatus struct {
	// Round is the latest EMITTED ground round, on §9's reading that a round
	// exists from its emission: between `tp ground` and `tp ground --record`
	// nothing in it has been decided, which is the state a coverage report
	// most needs to name.
	Round int
	// Coverage is §8's ratio and its two off-ratio counts.
	Coverage GroundCoverage
	// Cut is how many units of the emitted index the arms dropped (§2.2: the
	// absence of the hash is the cut). It is NOT part of §8's ratio — a cut
	// unit owes no disposition and is neither numerator nor denominator.
	//
	// It is here for one question the ratio cannot answer: whether a
	// denominator of zero means the arms dropped everything or §2.1 produced
	// nothing. `0 in floor, 4 cut` and `0 in floor, 0 cut` are both 0-of-0 to
	// coverage, and only the first is a document nobody checked.
	Cut int
	// ByVerdict counts the round's recorded rows by §3's verdict, and carries
	// all six every time — zeros included.
	//
	// A complete map rather than only what the round holds, for two reasons.
	// §8 makes the `NOT-A-CLAIM` share the first number to read, and a number
	// a reader must first decide whether an absence stands for is not a
	// number they can read. And §9's precedent against a permanently
	// zero-valued key does not reach here: none of the six is permanently
	// zero — any of them can be the whole round.
	ByVerdict map[GroundVerdict]int
}

// LatestGroundStatus answers §7.1's `--status` for specPath.
//
// It reads the floor the emission froze and the round file beside it, and never
// opens the spec — the same rule `--record` follows (§7.3), and for the same
// reason: the round is graded against the text it was emitted over, so a spec
// edited or deleted since cannot re-floor it.
//
// Unlike LatestGroundAdvisory this reports its failures. The advisory is a
// remark inside another command's envelope and §9 forbids it stopping anything,
// so silence is its honest answer to an unreadable artifact; `--status` is the
// whole of what its own invocation produces, and a coverage report derived from
// a file it could not read would be a number an operator has no way to doubt.
func LatestGroundStatus(specPath string) (*GroundStatus, error) {
	round, err := latestEmittedGroundRoundErr(specPath)
	if err != nil {
		return nil, err
	}
	if round == 0 {
		return nil, ErrNoGroundEmission
	}

	floor, err := readGroundFloor(specPath, round)
	if err != nil {
		return nil, err
	}
	rows, err := readGroundRoundRows(specPath, round)
	if err != nil {
		return nil, err
	}

	cut := 0
	for i := range floor {
		if floor[i].TextSHA == "" {
			cut++
		}
	}
	return &GroundStatus{
		Round:     round,
		Coverage:  GroundCoverageOf(floor, rows),
		Cut:       cut,
		ByVerdict: groundVerdictCounts(rows),
	}, nil
}

// groundVerdictCounts is §8's breakdown: how many of the round's rows carry
// each of §3's six verdicts.
//
// The six are seeded from GroundVerdicts() before anything is counted, so the
// set of keys is §3's set and not the set this round happened to use. A verdict
// added to §3 appears here with no edit; a row's verdict cannot be outside the
// six, because ParseGroundRow rejects one that is.
func groundVerdictCounts(rows []GroundRow) map[GroundVerdict]int {
	verdicts := GroundVerdicts()
	counts := make(map[GroundVerdict]int, len(verdicts))
	for _, v := range verdicts {
		counts[v] = 0
	}
	// Indexed rather than ranged by value, as GroundCoverageOf is and for the
	// same reason: a GroundRow is large enough for `gocritic`'s rangeValCopy.
	for i := range rows {
		counts[rows[i].Verdict]++
	}
	return counts
}
