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
	// IDScheme marks which id scheme this round's rows use (§10.9). Value "slug"
	// for an audit round recorded under v0.30.0+, whose rows carry stable slug ids
	// (file-<role>-<slug>, §10.3); empty on a legacy round recorded before this
	// release, whose rows use the old positional ids (file-<role>-<n>). tp never
	// rewrites recorded files, so a project mid-loop when it upgrades holds
	// legacy (marker-less) rounds. Recorded on audit rounds only — review rounds
	// dedup on (location, class) and neither carry nor consume the marker.
	IDScheme string `json:"id_scheme,omitempty"`
	// HarnessNote is an optional free-text note the orchestrator records per
	// round via --harness-note (§6.2/§6.3). It is stored verbatim and is empty
	// when the operator omitted the flag or the round predates the field. The
	// staleness comparison (HarnessStale) judges it trimmed, but the stored
	// value is never trimmed.
	HarnessNote string `json:"harness_note,omitempty"`
}

// IDSchemeSlug is the marker value recorded on a v0.30.0+ audit round, whose
// rows carry stable slug ids (§10.9).
const IDSchemeSlug = "slug"

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
	// OnlySnapshots narrows MissingIndex to the case where the artifacts found
	// are exclusively snapshots — no round file has ever been written, so the
	// index records nothing and rebuilding it loses nothing. tp audit's emission
	// never calls EnsureReviewState, so this is exactly the state a first audit
	// round leaves behind, and reading it as corruption made that round
	// unrecordable. A round file beside a missing index is the opposite case:
	// recorded history is gone, and only the operator can decide what to do.
	OnlySnapshots bool
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

// IsRebuildableStateIndex reports whether err is a missing index with nothing
// but snapshots beside it — the state a first audit round leaves behind, where
// rebuilding the index loses no recorded round. Distinct from
// IsMissingStateIndex, which is also true when round files are present and the
// index that referenced them is gone.
func IsRebuildableStateIndex(err error) bool {
	var ce *StateCorruptError
	if errors.As(err, &ce) {
		return ce.MissingIndex && ce.OnlySnapshots
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
				return nil, &StateCorruptError{
					Path:          stateDir,
					Reason:        "round or snapshot files present but state.json is missing",
					MissingIndex:  true,
					OnlySnapshots: !hasRecordedRoundFiles(stateDir),
				}
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
		// Snapshots with no index is the state tp audit's emission leaves
		// behind, since it never calls this function: rebuilding the index
		// there loses nothing, and refusing made a fresh spec's first audit
		// round impossible to record. A round file beside a missing index
		// still aborts — that index referenced history tp must not discard.
		if !IsRebuildableStateIndex(err) {
			return nil, err
		}
		st = nil
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
	// Atomic, like WriteSnapshotAtomic beside it: the index is read lock-free by
	// every status and resume path, and a plain write let a concurrent reader
	// see a half-written file. Two --record runs tore it 6 times in 40, and the
	// loser aborted with "state.json is unparseable" and the repair-or-delete
	// hint — destructive advice about a directory that was fine.
	path := reviewStatePath(specPath)
	// A UNIQUE temp file, not path+".tmp": two concurrent writers sharing one
	// temp path race to rename it, and the loser's rename fails with ENOENT
	// after the winner has already moved it away — which is what a first
	// attempt at this fix produced, 1 in 20 concurrent records. The name still
	// ends in .tmp, so a crash leftover is still skipped as a state artifact.
	tmp, err := os.CreateTemp(filepath.Dir(path), "state.json.*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

// roundFilePrefixes name the artifacts --record writes: the recorded history
// state.json indexes.
var roundFilePrefixes = []string{"review-round-", "audit-round-"}

// snapshotPrefixes name the artifacts an emission writes before any round is
// recorded.
var snapshotPrefixes = []string{"snapshot-round-", "snapshot-audit-round-"}

// hasRecordedRoundFiles reports whether dir holds a round file — the artifact
// only --record writes. Snapshots do not count: an emission writes those before
// any round exists, so their presence alone says nothing was ever recorded.
func hasRecordedRoundFiles(dir string) bool {
	return hasAnyPrefixed(dir, roundFilePrefixes)
}

// hasStateArtifacts reports whether dir contains round or snapshot files. A
// crash-leftover .tmp file from an interrupted atomic snapshot write is NOT a
// state artifact — it must not trigger a false-positive corrupt-state abort
// (§10.2 atomic write).
func hasStateArtifacts(dir string) bool {
	return hasRecordedRoundFiles(dir) || hasAnyPrefixed(dir, snapshotPrefixes)
}

// hasAnyPrefixed reports whether dir holds a non-.tmp entry with one of the
// prefixes. An UNREADABLE directory answers true: it may hold anything, and
// answering false made a state tp could not list read as a state that does not
// exist — which let --record build a fresh index over an existing round file
// and turn a recorded FAIL round into a clean round 1. Refusing to guess sends
// it to the corrupt-state abort, whose hint is what an operator needs here.
func hasAnyPrefixed(dir string, prefixes []string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return !os.IsNotExist(err)
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".tmp") {
			continue
		}
		for _, prefix := range prefixes {
			if strings.HasPrefix(name, prefix) {
				return true
			}
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

// HarnessStale reports whether the two most recently recorded rounds both carry
// non-empty harness notes that differ (§6.2/§6.3). Non-emptiness and difference
// are both judged on the note trimmed of surrounding whitespace; the note is
// stored verbatim but compared trimmed. Any empty (or whitespace-only) note —
// a round that predates the field, or an operator who omitted the flag — is a
// no-signal: it never sets the flag and is never treated as "different". With
// fewer than two recorded rounds the result is false. The comparison is
// per-phase: callers pass only review rounds or only audit rounds.
func HarnessStale(rounds []ReviewRound) bool {
	if len(rounds) < 2 {
		return false
	}
	last := strings.TrimSpace(rounds[len(rounds)-1].HarnessNote)
	prev := strings.TrimSpace(rounds[len(rounds)-2].HarnessNote)
	if last == "" || prev == "" {
		return false
	}
	return last != prev
}

// LatestHarnessNote returns the most recently recorded round's harness note
// verbatim (untrimmed), or "" when there are no rounds. Callers emit it only
// when HarnessStale is true.
func LatestHarnessNote(rounds []ReviewRound) string {
	if len(rounds) == 0 {
		return ""
	}
	return rounds[len(rounds)-1].HarnessNote
}

// IsLegacyRound reports whether r is a legacy round whose rows carry the old
// positional ids (file-<role>-<n>) rather than stable slug ids (§10.9).
// Detection is by the stored id_scheme marker — not by parsing the id — so a
// round recorded before this release (which carries no marker) is legacy. The
// marker is recorded on audit rounds only: a review round has no marker but is
// never "legacy" here, since review rows dedup on (location, class), not
// (role, item_id). Callers inspecting prior audit rounds use this to decide
// whether a prior round's ids are comparable to the current round's.
func IsLegacyRound(r *ReviewRound) bool {
	return r.IDScheme == ""
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
