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
	// GrantedRound is the round an operator's grant lets the slice emit and
	// record past its cap while that round is not yet recorded (0: none), and
	// Grantable says the slice is at its cap and stale with no grant open, so
	// tp epic review --grant-round would give one (spec/1.7.0.md §8).
	GrantedRound int
	Grantable    bool
	// MaxRounds is the cap the slice was graded under (0: uncapped), and
	// GrantsSpent the rounds recorded past it, each of which only a grant
	// lets a slice record, so round 4 under a cap of 3 reads one grant spent
	// (spec/1.8.0.md §5).
	MaxRounds   int
	GrantsSpent int
	// Open counts the last recorded round's findings that carry no
	// disposition, by severity, every severity present, so a slice done under
	// review_converge_on blocking still shows what it left open. A finding
	// resolved fixed is not counted: it waits for the next round to read the
	// change, not for a decision.
	Open map[string]int
	// OpenByRole counts the same findings by the role that raised them, so
	// its counts sum to Open's (spec/1.8.0.md §5); a row naming no role is
	// counted under EpicRoleUnattributed.
	OpenByRole map[string]int
}

// EpicReviewLoopStatus grades the slice's recorded rounds under p. A round is
// clean when no open row blocks under review_converge_on; the slice is stale
// while a unit's hash differs from the last recorded round's snapshot or has
// none (D7), and a stale slice is neither converged nor done, at the cap too
// (D1). At the cap the loop is done when every finding of the last round is
// dispositioned and no blocking one is merely marked fixed, as in tp review.
// An open grant lifts the cap for the one round it names (spec/1.7.0.md §8).
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
	st.Open, st.OpenByRole = map[string]int{"critical": 0, "high": 0, "medium": 0, "low": 0}, map[string]int{}
	if len(rounds) > 0 {
		epicCountOpen(&st, rounds[len(rounds)-1])
	}
	st.ConsecutiveClean = epicConsecutiveClean(rounds, p.ConvergeOn)
	st.Converged = st.ConsecutiveClean >= p.CleanRounds && !st.Stale
	if _, err := os.Stat(filepath.Join(stateDir, epicSnapshotFile(st.Round+1))); err == nil {
		st.InFlightRound = st.Round + 1
	}
	granted, err := readEpicReviewGrant(stateDir)
	if err != nil {
		return EpicReviewStatus{}, err
	}
	if granted == st.Round+1 {
		st.GrantedRound = granted
	}
	st.MaxRounds, st.GrantsSpent = p.MaxRounds, epicGrantsSpent(st.Round, p.MaxRounds)
	switch {
	case st.Converged:
		st.Done, st.DoneBy = true, DoneByConverged
	case p.MaxRounds > 0 && st.Round >= p.MaxRounds && st.GrantedRound == 0:
		epicGradeCap(&st, rounds[len(rounds)-1], p.ConvergeOn)
	}
	return st, nil
}

// epicGradeCap grades a slice at its cap with no grant open: done when its
// last round is wholly dispositioned and it is not stale, grantable when it is
// stale, since only a changed graph gives one more round something to read.
func epicGradeCap(st *EpicReviewStatus, last []map[string]any, convergeOn string) {
	st.CapReached = true
	blockingFixed := 0
	st.Undisposed, blockingFixed = epicCapUndisposed(last, convergeOn)
	if st.Undisposed == 0 && blockingFixed == 0 && !st.Stale {
		st.Done, st.DoneBy = true, DoneByCap
	}
	st.Grantable = st.Stale
}

// EpicRoleUnattributed is the by-role bucket of a finding that names no role,
// as audit's "unrecognised" is the bucket of an unreadable severity.
const EpicRoleUnattributed = "unattributed"

// epicCountOpen counts the last round's rows that carry no disposition into
// st.Open by severity and st.OpenByRole by role.
func epicCountOpen(st *EpicReviewStatus, last []map[string]any) {
	for _, row := range last {
		if RowDispositioned(row) {
			continue
		}
		sev, _ := row["severity"].(string)
		role, _ := row["role"].(string)
		if role == "" {
			role = EpicRoleUnattributed
		}
		st.Open[sev]++
		st.OpenByRole[role]++
	}
}

// epicGrantsSpent counts the recorded rounds past maxRounds, 0 when the slice
// is uncapped or within its cap.
func epicGrantsSpent(round, maxRounds int) int {
	if maxRounds <= 0 || round <= maxRounds {
		return 0
	}
	return round - maxRounds
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
