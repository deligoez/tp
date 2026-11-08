package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// EpicReviewPolicy is the convergence and cap a slice's loop resolves from the
// project layer over the built-in default; there is no task file layer (§5).
type EpicReviewPolicy struct {
	CleanRounds int
	MaxRounds   int // 0 is uncapped
	ConvergeOn  string
}

// EpicReviewStatus is a slice's loop verdict, read live from its state
// directory and the current unit hashes (spec/1.3.0.md §5). Round is the last
// recorded round, InFlightRound an emitted round not yet recorded (0: none).
type EpicReviewStatus struct {
	Round            int
	ConsecutiveClean int
	Converged        bool
	Done             bool
	DoneBy           string
	Stale            bool
	InFlightRound    int
	CapReached       bool
	Undisposed       int
}

// EpicReviewLoopStatus grades the slice's recorded rounds under p. A round is
// clean when no open row blocks under review_converge_on; the slice is stale
// while a unit's hash differs from the last recorded round's snapshot or has
// none (D7), and a stale slice is neither converged nor done, at the cap too
// (D1). At the cap the loop is done when every finding of the last round is
// dispositioned and no blocking one is merely marked fixed, as in tp review.
func EpicReviewLoopStatus(stateDir string, units []EpicReviewUnit, p EpicReviewPolicy) (EpicReviewStatus, error) {
	st := EpicReviewStatus{Round: EpicReviewNextRound(stateDir) - 1}
	rounds := make([][]map[string]any, 0, st.Round)
	for r := 1; r <= st.Round; r++ {
		path := filepath.Join(stateDir, epicRoundFile(r))
		rows, ok := readNDJSONRows(path)
		if !ok {
			return EpicReviewStatus{}, fmt.Errorf("epic review round file %s cannot be read as NDJSON", path)
		}
		rounds = append(rounds, rows)
	}
	stale, err := epicSliceStale(stateDir, st.Round, units)
	if err != nil {
		return EpicReviewStatus{}, err
	}
	st.Stale = stale
	st.ConsecutiveClean = epicConsecutiveClean(rounds, p.ConvergeOn)
	st.Converged = st.ConsecutiveClean >= p.CleanRounds && !st.Stale
	if _, err := os.Stat(filepath.Join(stateDir, epicSnapshotFile(st.Round+1))); err == nil {
		st.InFlightRound = st.Round + 1
	}
	switch {
	case st.Converged:
		st.Done, st.DoneBy = true, DoneByConverged
	case p.MaxRounds > 0 && st.Round >= p.MaxRounds:
		st.CapReached = true
		blockingFixed := 0
		st.Undisposed, blockingFixed = epicCapUndisposed(rounds[len(rounds)-1], p.ConvergeOn)
		if st.Undisposed == 0 && blockingFixed == 0 && !st.Stale {
			st.Done, st.DoneBy = true, DoneByCap
		}
	}
	return st, nil
}

// epicRowBlocking reports whether an open row holds its round open.
func epicRowBlocking(row map[string]any, convergeOn string) bool {
	return convergeOn == ReviewConvergeOnAll || ReviewRowBlocking(row)
}

// epicConsecutiveClean counts the trailing rounds with no open blocking row.
func epicConsecutiveClean(rounds [][]map[string]any, convergeOn string) int {
	n := 0
	for _, round := range slices.Backward(rounds) {
		for _, row := range round {
			if findingOpen(PhaseReview, row) && epicRowBlocking(row, convergeOn) {
				return n
			}
		}
		n++
	}
	return n
}

// epicCapUndisposed counts the last round's rows with no disposition that
// counts, and its blocking rows only marked fixed, which no round re-reads.
func epicCapUndisposed(rows []map[string]any, convergeOn string) (undisposed, blockingFixed int) {
	for _, row := range rows {
		switch {
		case rowFixed(row) && epicRowBlocking(row, convergeOn):
			blockingFixed++
		case !RowDispositioned(row):
			undisposed++
		}
	}
	return undisposed, blockingFixed
}

// epicSliceStale reports whether any unit's hash differs from the snapshot of
// the last recorded round, or has none recorded (D7). No recorded round
// records no hash, so the slice is stale.
func epicSliceStale(stateDir string, round int, units []EpicReviewUnit) (bool, error) {
	if round < 1 {
		return true, nil
	}
	snap, err := readEpicSnapshot(stateDir, round)
	if err != nil {
		return false, err
	}
	for _, u := range units {
		if hash, ok := snap[u.Key]; !ok || hash != u.Hash {
			return true, nil
		}
	}
	return false, nil
}
