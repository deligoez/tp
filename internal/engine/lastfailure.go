package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// LastFailure is §4.2's record of the last unit of a cycle that failed: the
// unit's identity, the code it exited with, and a summary tp itself authored.
//
// It is deliberately not part of the run state. The run state bounds one run
// (§3.5) and is replaced when the next one starts, while this record's whole
// purpose is to survive the run that wrote it and reach the fresh process that
// re-enters the same wall. It is also advisory: nothing reads it back to decide
// which unit runs next, so a record that could not be written or parsed costs a
// hint and never a wrong decision.
//
// Summary is written by tp — the gate's own output for the `tp done` writer,
// the failing command and the log path for the driver's — and never copies the
// child's prose, so a unit cannot narrate its way into the next unit's context.
type LastFailure struct {
	UnitKind UnitKind `json:"unit_kind"`
	UnitID   string   `json:"unit_id"`
	Phase    string   `json:"phase"`
	ExitCode int      `json:"exit_code"`
	Summary  string   `json:"summary"`
	At       string   `json:"at"`
}

// LastFailurePath returns a cycle's last-failure file,
// .tp/last_failure-<base>.json under the repository root — named per task file
// and absolute for the same reasons the run state is (§3.5): two cycles in one
// repository never collide, and the writers and readers of this path do not
// share a working directory.
func LastFailurePath(root, taskFile string) string {
	return absoluteUnder(root, filepath.Join(".tp", "last_failure-"+RunBase(taskFile)+".json"))
}

// ReadLastFailure returns the cycle's record, or nil when there is none.
//
// Every failure — absent, unreadable, unparseable — reads as nil rather than as
// an error, because the record is advisory (§4.2): a surface that refused to
// report a phase because a hint file was corrupt would have turned an advisory
// into a blocker.
func ReadLastFailure(root, taskFile string) *LastFailure {
	data, err := os.ReadFile(LastFailurePath(root, taskFile))
	if err != nil {
		return nil
	}
	var f LastFailure
	if err := json.Unmarshal(data, &f); err != nil {
		return nil
	}
	return &f
}

// WriteLastFailure records f as the cycle's one last failure, replacing any
// record already there: §4.2 holds at most one object, so a second failure
// overwrites the first rather than accumulating a log.
//
// At is stamped here when the caller left it empty, so the two writers cannot
// disagree about the clock. The write is atomic for the reason the run state's
// is — a reader takes no lock and must never see a half-written file.
func WriteLastFailure(root, taskFile string, f *LastFailure) error {
	record := *f
	if record.At == "" {
		record.At = time.Now().UTC().Format(time.RFC3339Nano)
	}
	path := LastFailurePath(root, taskFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(&record, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, append(data, '\n'))
}

// ClearLastFailure removes the record when it belongs to (kind, id), and leaves
// it alone otherwise.
//
// Both halves of the key are compared because id alone collides (§4.2): a
// review-record unit and an audit-record unit are both identified by a round
// number, so a succeeding audit-record 2 would otherwise clear the review's
// failure at round 2 and hide it from the unit that still has to face it.
func ClearLastFailure(root, taskFile string, kind UnitKind, id string) error {
	f := ReadLastFailure(root, taskFile)
	if f == nil || f.UnitKind != kind || f.UnitID != id {
		return nil
	}
	if err := os.Remove(LastFailurePath(root, taskFile)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
