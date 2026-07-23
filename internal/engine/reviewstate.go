package engine

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
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
	// MissingIndex is true when state.json is absent but round/snapshot files
	// remain — the normal in-flight-round state (an emission wrote a snapshot
	// that --record has not yet indexed), not corruption. Emission callers treat
	// this as "no recorded state" instead of aborting (§10.2, InFlightRound).
	MissingIndex bool
}

func (e *StateCorruptError) Error() string {
	return fmt.Sprintf("review state at %s is unusable: %s", e.Path, e.Reason)
}

// Hint is the actionable message accompanying the abort.
func (e *StateCorruptError) Hint() string {
	return fmt.Sprintf("repair or delete %s", e.Path)
}

// IsMissingStateIndex reports whether err is a StateCorruptError flagging the
// normal in-flight-round condition (round/snapshot files present but state.json
// absent) rather than genuine corruption. Emission callers treat this as "no
// recorded state" (§10.2, InFlightRound) instead of aborting with exit 3.
func IsMissingStateIndex(err error) bool {
	var ce *StateCorruptError
	if errors.As(err, &ce) {
		return ce.MissingIndex
	}
	return false
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
				return nil, &StateCorruptError{Path: stateDir, Reason: "round or snapshot files present but state.json is missing", MissingIndex: true}
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
// hasStateArtifacts reports whether dir contains round or snapshot files. A
// crash-leftover .tmp file from an interrupted atomic snapshot write is NOT a
// state artifact — it must not trigger a false-positive corrupt-state abort
// (§10.2 atomic write).
func hasStateArtifacts(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".tmp") {
			continue
		}
		if strings.HasPrefix(name, "review-round-") || strings.HasPrefix(name, "audit-round-") || strings.HasPrefix(name, "snapshot-round-") || strings.HasPrefix(name, "snapshot-audit-round-") {
			return true
		}
	}
	return false
}

// WriteSnapshotAtomic writes the round-N spec snapshot atomically — write to a
// sibling snapshot-round-N.md.tmp then rename — so a crash mid-write never
// leaves a partial snapshot on disk (§10.2). The state directory is created
// when absent.
//
// phase scopes the snapshot namespace so review and audit never collide:
// review keeps the legacy "snapshot-round-N.md" name (read by the regression
// baseline reader in review_autodiff.go and by existing on-disk snapshots),
// while any other phase — today, audit — is namespaced as
// "snapshot-<phase>-round-N.md" (e.g. snapshot-audit-round-N.md). Without this
// scoping, a review snapshot-round-N.md would masquerade as an in-flight audit
// round and make `tp resume` point a fresh driver at a phantom round.
func WriteSnapshotAtomic(specPath, phase string, round int, data []byte) error {
	dir := ReviewStateDir(specPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	final := filepath.Join(dir, snapshotFilename(phase, round))
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}

// snapshotFilename returns the on-disk snapshot name for a phase and round.
// Review keeps the legacy "snapshot-round-N.md" form; any other phase is
// namespaced as "snapshot-<phase>-round-N.md" so the two phases' in-flight
// snapshots never collide.
func snapshotFilename(phase string, round int) string {
	if phase == PhaseReview {
		return fmt.Sprintf("snapshot-round-%d.md", round)
	}
	return fmt.Sprintf("snapshot-%s-round-%d.md", phase, round)
}

// InFlightRound reports the in-flight round number for a phase given the count
// of recorded rounds: the next round (recordedRounds+1) whose spec snapshot
// exists but whose round file does not, or 0 when none (§10.2). A snapshot
// without a matching round file means a round was started (prompt emission)
// and never recorded. phase selects the phase-scoped snapshot path so a review
// snapshot can never read as an in-flight audit round (or vice versa).
func InFlightRound(specPath, phase string, recordedRounds int) int {
	next := recordedRounds + 1
	snap := filepath.Join(ReviewStateDir(specPath), snapshotFilename(phase, next))
	if _, err := os.Stat(snap); err != nil {
		return 0
	}
	return next
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

// RolesStale reports whether the current corpus hash differs from the latest
// recorded round's stored roles_hash (§9.3). With no recorded rounds nothing is
// stale. A pre-v0.25.0 round has no stored role hash (empty), which §9.4 treats
// as matching — upgrading tp never forces a re-review.
func RolesStale(rounds []ReviewRound, currentHash string) bool {
	if len(rounds) == 0 {
		return false
	}
	stored := rounds[len(rounds)-1].RolesHash
	if stored == "" {
		return false
	}
	return stored != currentHash
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
