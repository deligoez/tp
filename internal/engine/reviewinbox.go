package engine

import (
	"errors"
	"os"
	"path/filepath"
)

// ReviewInboxRound is the review round tp review --record-inbox would record
// (spec/backlog/role-presence.md §2.4): the round after the recorded ones,
// whether it was emitted (its snapshot exists), and the roles it owes a file
// from. The roles are the spec's whole review panel, as the emission resolves
// it before any --role filter, plus the regression prompt when the round
// carries one; a role the spec deactivates is not owed.
type ReviewInboxRound struct {
	Round   int
	Emitted bool
	Roles   []string
}

// ReviewInboxPanel resolves the round --record-inbox records. A state index
// that does not exist yet, or holds only the in-flight snapshot, is a round
// with nothing recorded before it.
func ReviewInboxPanel(specPath string) (ReviewInboxRound, error) {
	st, err := LoadReviewState(specPath)
	var corrupt *StateCorruptError
	switch {
	case errors.As(err, &corrupt) && corrupt.OnlySnapshots:
		st = nil
	case err != nil:
		return ReviewInboxRound{}, err
	}
	recorded := 0
	if st != nil {
		recorded = len(st.ReviewRounds)
	}
	out := ReviewInboxRound{Round: recorded + 1}
	if _, statErr := os.Stat(filepath.Join(ReviewStateDir(specPath), snapshotFilename(PhaseReview, out.Round))); statErr == nil {
		out.Emitted = true
	}
	panel, err := ResolveRolePanel(specPath, PhaseReviewers)
	if err != nil {
		return ReviewInboxRound{}, err
	}
	for i := range panel.Roles {
		out.Roles = append(out.Roles, panel.Roles[i].ID)
	}
	if st == nil {
		st = &ReviewState{}
	}
	if len(out.Roles) > 0 && RegressionDue(specPath, out.Round, st) {
		out.Roles = append(out.Roles, RegressionRoleID)
	}
	return out, nil
}
