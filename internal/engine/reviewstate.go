package engine

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const stateFileName = "state.json"

// ReviewRound is one recorded round entry in state.json — used for both
// review_rounds and audit_rounds.
type ReviewRound struct {
	Round      int    `json:"round"`
	Findings   int    `json:"findings"`
	Clean      bool   `json:"clean"`
	RecordedAt string `json:"recorded_at"`
	File       string `json:"file"`
	SpecHash   string `json:"spec_hash"`
	// RolesHash is the phase's corpus hash at record time (§9.2): the reviewer
	// corpus hash for a review round, the auditor corpus hash for an audit round.
	// Empty on a pre-v0.25.0 round, which §9.4 treats as matching.
	RolesHash string `json:"roles_hash,omitempty"`
}

// ReviewState is the round index stored in state.json.
type ReviewState struct {
	Spec         string        `json:"spec"`
	ReviewRounds []ReviewRound `json:"review_rounds"`
	AuditRounds  []ReviewRound `json:"audit_rounds"`
}

// StateCorruptError marks an unusable state directory. Callers abort with
// exit 3 and the repair hint; tp never silently rebuilds the index.
type StateCorruptError struct {
	Path   string
	Reason string
}

func (e *StateCorruptError) Error() string {
	return fmt.Sprintf("review state at %s is unusable: %s", e.Path, e.Reason)
}

// Hint is the actionable message accompanying the abort.
func (e *StateCorruptError) Hint() string {
	return fmt.Sprintf("repair or delete %s", e.Path)
}

// ReviewStateDir returns <spec-dir>/.tp-review/<spec-base> for a spec path.
func ReviewStateDir(specPath string) string {
	dir := filepath.Dir(specPath)
	base := strings.TrimSuffix(filepath.Base(specPath), filepath.Ext(specPath))
	return filepath.Join(dir, ".tp-review", base)
}

// reviewStatePath returns the state.json path for a spec.
func reviewStatePath(specPath string) string {
	return filepath.Join(ReviewStateDir(specPath), stateFileName)
}

// WithReviewStateLock runs fn under the state-directory flock. All
// state-directory writes go through this lock; writers put round files first
// and the index entry second, so the loser of a concurrent record sees the
// winner's entry and records the next round number.
func WithReviewStateLock(specPath string, fn func() error) error {
	return WithFileLock(reviewStatePath(specPath), fn)
}

// LoadReviewState reads the round index for a spec.
// Returns (nil, nil) when no state directory exists. A directory containing
// round or snapshot files without state.json, or an unparseable state.json,
// returns a StateCorruptError.
func LoadReviewState(specPath string) (*ReviewState, error) {
	stateDir := ReviewStateDir(specPath)
	data, err := os.ReadFile(reviewStatePath(specPath))
	if err != nil {
		if os.IsNotExist(err) {
			if hasStateArtifacts(stateDir) {
				return nil, &StateCorruptError{Path: stateDir, Reason: "round or snapshot files present but state.json is missing"}
			}
			return nil, nil
		}
		return nil, err
	}

	var st ReviewState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, &StateCorruptError{Path: stateDir, Reason: fmt.Sprintf("state.json is unparseable: %v", err)}
	}
	if st.ReviewRounds == nil {
		st.ReviewRounds = []ReviewRound{}
	}
	if st.AuditRounds == nil {
		st.AuditRounds = []ReviewRound{}
	}
	return &st, nil
}

// EnsureReviewState loads the state, creating the directory and the initial
// {spec, review_rounds: [], audit_rounds: []} index when absent — so a
// directory without an index never arises in normal operation.
func EnsureReviewState(specPath string) (*ReviewState, error) {
	st, err := LoadReviewState(specPath)
	if err != nil {
		return nil, err
	}
	if st != nil {
		return st, nil
	}
	if err := os.MkdirAll(ReviewStateDir(specPath), 0o755); err != nil {
		return nil, err
	}
	st = &ReviewState{Spec: specPath, ReviewRounds: []ReviewRound{}, AuditRounds: []ReviewRound{}}
	if err := SaveReviewState(specPath, st); err != nil {
		return nil, err
	}
	return st, nil
}

// SaveReviewState writes state.json; call under WithReviewStateLock when
// other processes may write concurrently.
func SaveReviewState(specPath string, st *ReviewState) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(reviewStatePath(specPath), append(data, '\n'), 0o600)
}

// hasStateArtifacts reports whether dir contains round or snapshot files.
func hasStateArtifacts(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "review-round-") || strings.HasPrefix(name, "audit-round-") || strings.HasPrefix(name, "snapshot-round-") {
			return true
		}
	}
	return false
}

// SpecHash returns "sha256:<hex>" of the spec file's bytes.
func SpecHash(specPath string) (string, error) {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256(data)), nil
}

// ConsecutiveClean returns the length of the trailing run of clean rounds.
func ConsecutiveClean(rounds []ReviewRound) int {
	n := 0
	for i := len(rounds) - 1; i >= 0; i-- {
		if !rounds[i].Clean {
			break
		}
		n++
	}
	return n
}

// StateStale reports whether the current spec hash differs from the last
// recorded round's spec_hash (the spec changed after that round). With no
// recorded rounds nothing can be stale.
func StateStale(rounds []ReviewRound, currentHash string) bool {
	if len(rounds) == 0 {
		return false
	}
	return rounds[len(rounds)-1].SpecHash != currentHash
}

// Converged reports convergence: enough trailing clean rounds and a spec
// unchanged since the last recorded round.
func Converged(rounds []ReviewRound, requiredCleanRounds int, currentHash string) bool {
	return ConsecutiveClean(rounds) >= requiredCleanRounds && !StateStale(rounds, currentHash)
}

// LoadRoundRows reads a recorded round's NDJSON rows from the state
// directory. A round entry whose file is missing returns (nil, false) so the
// caller can skip it with a warning — the round still counts in round
// arithmetic. Blank lines are skipped; unparseable lines are ignored.
func LoadRoundRows(specPath string, entry *ReviewRound) (rows []map[string]any, found bool) {
	data, err := os.ReadFile(filepath.Join(ReviewStateDir(specPath), entry.File))
	if err != nil {
		return nil, false
	}
	rows = make([]map[string]any, 0)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue
		}
		rows = append(rows, m)
	}
	return rows, true
}
