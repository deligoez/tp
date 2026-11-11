package engine

import (
	"fmt"
	"path/filepath"
)

// EpicHistory is a slice's graph history (spec/1.8.0.md §11): its recorded
// rounds in order, and the difference from the last of them to the graph as
// it now stands. Current is nil when no round is recorded or the history was
// narrowed to two rounds.
type EpicHistory struct {
	Rounds  []EpicHistoryRound  `json:"rounds"`
	Current *EpicHistoryCurrent `json:"current,omitempty"`
}

// EpicHistoryRound is one recorded round: its findings by role, a finding
// naming none under EpicRoleUnattributed, and its difference from the round
// DiffFrom names; both are nil on the first round shown.
type EpicHistoryRound struct {
	Round          int                         `json:"round"`
	FindingsByRole map[string][]map[string]any `json:"findings_by_role"`
	DiffFrom       *int                        `json:"diff_from"`
	Diff           *EpicGraphDiff              `json:"diff"`
}

// EpicHistoryCurrent is the difference from round FromRound to the graph as
// it now stands.
type EpicHistoryCurrent struct {
	FromRound int           `json:"from_round"`
	Diff      EpicGraphDiff `json:"diff"`
}

// EpicRoundOutOfRangeError is a --from or --to naming a round the slice did
// not record.
type EpicRoundOutOfRangeError struct {
	Round, Recorded int
}

func (e *EpicRoundOutOfRangeError) Error() string {
	return fmt.Sprintf("round %d is not recorded: the slice recorded rounds 1 to %d", e.Round, e.Recorded)
}

// EpicReviewHistory reads the history of the slice at stateDir whose units
// the graph now renders as units. With from and to both zero it holds every
// recorded round, each diffed from the round before, and the difference to
// the current graph; otherwise it holds rounds from and to alone, to diffed
// from from. A round the slice did not record is an EpicRoundOutOfRangeError.
func EpicReviewHistory(stateDir string, units []EpicReviewUnit, from, to int) (EpicHistory, error) {
	last := EpicReviewNextRound(stateDir) - 1
	if from != 0 || to != 0 {
		return epicNarrowHistory(stateDir, last, from, to)
	}
	h := EpicHistory{Rounds: make([]EpicHistoryRound, 0, last)}
	for r := 1; r <= last; r++ {
		round, err := epicHistoryRound(stateDir, r, r-1)
		if err != nil {
			return EpicHistory{}, err
		}
		h.Rounds = append(h.Rounds, round)
	}
	if last > 0 {
		diff, err := EpicReviewCurrentDiff(stateDir, last, units)
		if err != nil {
			return EpicHistory{}, err
		}
		h.Current = &EpicHistoryCurrent{FromRound: last, Diff: diff}
	}
	return h, nil
}

// epicNarrowHistory holds rounds from and to, to diffed from from.
func epicNarrowHistory(stateDir string, last, from, to int) (EpicHistory, error) {
	for _, r := range []int{from, to} {
		if r < 1 || r > last {
			return EpicHistory{}, &EpicRoundOutOfRangeError{Round: r, Recorded: last}
		}
	}
	first, err := epicHistoryRound(stateDir, from, 0)
	if err != nil {
		return EpicHistory{}, err
	}
	second, err := epicHistoryRound(stateDir, to, from)
	if err != nil {
		return EpicHistory{}, err
	}
	return EpicHistory{Rounds: []EpicHistoryRound{first, second}}, nil
}

// epicHistoryRound reads round's findings by role and, when base is a round,
// its difference from base.
func epicHistoryRound(stateDir string, round, base int) (EpicHistoryRound, error) {
	path := filepath.Join(stateDir, epicRoundFile(round))
	rows, ok := readNDJSONRows(path)
	if !ok {
		return EpicHistoryRound{}, fmt.Errorf("epic review round file %s cannot be read as NDJSON", path)
	}
	out := EpicHistoryRound{Round: round, FindingsByRole: map[string][]map[string]any{}}
	for _, row := range rows {
		role, _ := row["role"].(string)
		if role == "" {
			role = EpicRoleUnattributed
		}
		out.FindingsByRole[role] = append(out.FindingsByRole[role], row)
	}
	if base < 1 {
		return out, nil
	}
	diff, err := EpicReviewRoundsDiff(stateDir, base, round)
	if err != nil {
		return EpicHistoryRound{}, err
	}
	out.DiffFrom, out.Diff = &base, &diff
	return out, nil
}

// WithoutContext drops each difference's unchanged context, keeping every
// change: the compact form of the history.
func (h *EpicHistory) WithoutContext() {
	for i := range h.Rounds {
		if h.Rounds[i].Diff != nil {
			h.Rounds[i].Diff.Unchanged = nil
		}
	}
	if h.Current != nil {
		h.Current.Diff.Unchanged = nil
	}
}
