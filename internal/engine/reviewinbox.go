package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ReviewInboxRound is the review round tp review --record-inbox would record
// (spec/backlog/role-presence.md §2.4): the round after the recorded ones,
// whether it was emitted (its snapshot exists), and the roles it owes a file
// from, as its emission stored them; Stored is false for a round emitted
// before v1.20.0, which kept no list.
type ReviewInboxRound struct {
	Round   int
	Emitted bool
	Stored  bool
	Roles   []string
}

// reviewOwedFile is the on-disk form of a review round's owed roles.
type reviewOwedFile struct {
	Round int      `json:"round"`
	Roles []string `json:"roles"`
}

// ReviewOwedRolesPath is where a review emission keeps the roles its round
// owes a file from until the round is recorded:
// <state dir>/review-inbox/round-<N>.roles.json. Like an audit round's pools
// it ends in .json, so it is no role file, and RemoveRoundInbox takes it with
// the round's role files.
func ReviewOwedRolesPath(specPath string, round int) string {
	return filepath.Join(roleInboxDir(specPath, PhaseReview), fmt.Sprintf("round-%d.roles.json", round))
}

// ReviewInboxMergedPath is where --record-inbox writes the rows it merged from
// a round's role files before recording them: round-<N>.merged.ndjson in the
// review inbox. The name keeps it out of a round-<N>-*.ndjson glob, and
// RemoveRoundInbox takes it with the round.
func ReviewInboxMergedPath(specPath string, round int) string {
	return filepath.Join(roleInboxDir(specPath, PhaseReview), fmt.Sprintf("round-%d.merged.ndjson", round))
}

// WriteReviewOwedRoles writes
// --role filter: every prompt it built, the regression prompt included and the
// roles a --diff-from or a deactivation left out excluded. Sibling emissions
// of one round write the same list.
func WriteReviewOwedRoles(specPath string, round int, roles []string) error {
	path := ReviewOwedRolesPath(specPath, round)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(reviewOwedFile{Round: round, Roles: roles}, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, append(data, '\n'))
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
	data, err := os.ReadFile(ReviewOwedRolesPath(specPath, out.Round)) //nolint:gosec // a path built from the spec's own state dir
	switch {
	case os.IsNotExist(err):
		return out, nil
	case err != nil:
		return ReviewInboxRound{}, err
	}
	var f reviewOwedFile
	if err := json.Unmarshal(data, &f); err != nil {
		return ReviewInboxRound{}, fmt.Errorf("%s: %w", ReviewOwedRolesPath(specPath, out.Round), err)
	}
	out.Stored, out.Roles = true, f.Roles
	return out, nil
}
