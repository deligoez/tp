package engine

import (
	"errors"
	"fmt"
	"strings"
)

// GroundLineError places a row failure in the file it came from, wrapping the
// failure itself.
//
// The line is a separate field rather than part of a formatted sentence for the
// reason GroundRowError gives: what a caller needs to act on is data, so a
// rewording cannot quietly stop it holding. Unwrap keeps the wrapped
// *GroundRowError reachable, so "which line" and "which cell" are two questions
// with two answers rather than one string to parse.
type GroundLineError struct {
	// Line is the 1-based line of the --record file, counting every line the
	// file has including the blank ones a reader can see.
	Line int
	// Err is what that line failed on.
	Err error
}

func (e *GroundLineError) Error() string {
	return fmt.Sprintf("line %d: %v", e.Line, e.Err)
}

func (e *GroundLineError) Unwrap() error { return e.Err }

// ParseGroundRows validates every line of a `--record` file against §7.2's
// table and returns the rows in file order.
//
// Blank lines are skipped and every other line must be a row, which is the rule
// the shipped record path already applies (`parseRecordRows`,
// `internal/cli/review_record.go:246`). A trailing partial line is therefore
// not blank and is rejected, as §7.1's exit 1 requires.
//
// It stops at the first failure. Nothing downstream can act on the second
// failure of a round that is not going to be written, and reporting one
// rejection with its line and its field is what an operator fixes.
func ParseGroundRows(data []byte) ([]GroundRow, error) {
	rows := make([]GroundRow, 0, 64)
	line := 0
	for raw := range strings.SplitSeq(string(data), "\n") {
		line++
		if strings.TrimSpace(raw) == "" {
			continue
		}
		row, err := ParseGroundRow([]byte(raw))
		if err != nil {
			return nil, &GroundLineError{Line: line, Err: err}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// ErrGroundRoundEmpty is what RecordGroundRound refuses a payload holding no
// rows with (§7.1).
//
// Zero rows would consume a round number for a round that decided nothing, and
// §8 would then join that round against no dispositions and report it as
// coverage that stalled — which is indistinguishable from a round that ran and
// found nothing, except that the second cannot happen: the prompt asks a
// question of every unit in the index.
//
// It is a sentinel rather than a *GroundLineError because there is no line to
// name: the failure is a property of the file, not of one of its rows. The
// caller that maps failures onto §7.1's exit codes reads it back with errors.Is
// and reports it as the same validation refusal (exit 1) a bad row gets.
var ErrGroundRoundEmpty = errors.New("the record holds no rows, so there is nothing to record")

// groundRoundFileName is the name a recorded ground round takes in the state
// directory (§7.3).
//
// It is deliberately not the prompt's `output_path`: `ground-rN.ndjson` is the
// scratch file a unit writes and an operator collects, and this is what
// `--record` writes beside the snapshot and the floor. The shipped convention
// is the same one `roleOutputPath` follows for review and audit.
func groundRoundFileName(round int) string {
	return fmt.Sprintf("ground-round-%d.ndjson", round)
}

// RecordGroundRound validates data and, only if it holds at least one row and
// every one of them passes, writes it into specPath's state directory as
// `ground-round-<round>.ndjson`.
//
// **Validation completes before anything is opened, created or truncated**, so
// a payload holding one invalid row leaves the state directory byte-identical
// to what the preceding emission left there (§11 row 10). §7.2 states the
// requirement and the reason: "a partially valid round would make coverage a
// lie" — a round recorded with its bad rows dropped is counted by §8 as a round
// in which those units were decided, and nothing in the record says otherwise.
// Validating row by row as each is appended would satisfy the wording for a
// payload whose first row is bad and break it for every other one.
//
// The bytes written are the ones handed in, not a re-marshalling of the rows:
// §7.2's table is what a row must satisfy, not the exhaustive set of what it
// may carry through a reader, and the recorded round is the artifact a later
// round joins against.
//
// The state directory is not created here. §7.3 has emit write the snapshot and
// the floor `--record` validates against, so a missing directory is the
// no-prior-emit case and surfaces as the file error it is, rather than as a
// round file sitting alone in a directory with no floor to have been graded
// against.
func RecordGroundRound(specPath string, round int, data []byte, floor []FloorIndexRow) (rows, carried []GroundRow, err error) {
	rows, err = ParseGroundRows(data)
	if err != nil {
		return nil, nil, err
	}
	// After the parse and before the write, which is the only point at which
	// both facts are in hand: that every row validates, and that there are
	// none. A guard on len(data) instead would record a file of blank lines,
	// which parses — correctly — to nothing.
	if len(rows) == 0 {
		return nil, nil, ErrGroundRoundEmpty
	}

	carried, err = GroundCarriedRows(specPath, round, floor, rows)
	if err != nil {
		return nil, nil, err
	}
	out := data
	if len(carried) > 0 {
		if out, err = appendGroundRows(data, carried); err != nil {
			return nil, nil, err
		}
	}

	path := GroundRoundPath(specPath, round)
	if err := writeFileAtomic(path, out); err != nil {
		return nil, nil, err
	}
	return rows, carried, nil
}

// GroundCarriedRows reads the immediately preceding round and returns the
// dispositions round carries forward from it (§8).
//
// It is exported because §8 has two readers of the same answer and they must
// not be two rules. `--record` asks it what to write into the round file, with
// the payload's own rows as decided; the EMISSION asks it which units to mark
// `(carried)` and leave out of what the prompt asks for, with decided nil —
// nothing has been decided in a round that has only just been emitted. A second
// implementation on the emit side would let the prompt promise a carry the
// record then does not make, and nothing downstream compares the two.
//
// round is the round being emitted or recorded; the source is always round-1,
// and round 1 carries nothing because there is no round 0.
func GroundCarriedRows(specPath string, round int, floor []FloorIndexRow, decided []GroundRow) ([]GroundRow, error) {
	if round <= 1 {
		return nil, nil
	}
	prev, err := readGroundRoundRows(specPath, round-1)
	if err != nil {
		return nil, &GroundCarryError{Round: round - 1, Err: err}
	}
	return groundCarryForward(floor, prev, round-1, decided), nil
}
