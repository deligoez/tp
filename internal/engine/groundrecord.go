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
	return parseGroundRows(data, nil)
}

// parseGroundRows is ParseGroundRows with §7.3's floor check folded into the
// same pass, so the refusal carries the line the table's own refusals carry and
// arrives before RecordGroundRound opens anything.
//
// A nil floor is the shape-check-only reading, which is what every reader of a
// round tp itself wrote wants: those rows were graded against a floor that is
// not this one's, and re-comparing them would refuse tp's own artifacts.
func parseGroundRows(data []byte, floor []FloorIndexRow) ([]GroundRow, error) {
	byID := groundFloorKeys(floor)
	rows := make([]GroundRow, 0, 64)
	line := 0
	for raw := range strings.SplitSeq(string(data), "\n") {
		line++
		if strings.TrimSpace(raw) == "" {
			continue
		}
		row, err := ParseGroundRow([]byte(raw))
		if err == nil {
			err = groundRowMatchesFloor(&row, byID)
		}
		if err != nil {
			return nil, &GroundLineError{Line: line, Err: err}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// groundFloorKeys indexes an emitted floor's units by `unit_id`, each carrying
// §8's whole join key — `(text_sha, ordinal)`.
//
// A unit the arms CUT is deliberately absent: §2.2 makes the absence of the hash
// the cut, so a cut row carries no key for a payload row to be compared
// against, and demanding its `("", 0)` would refuse every legal row on it.
func groundFloorKeys(floor []FloorIndexRow) map[string]groundJoinKey {
	if len(floor) == 0 {
		return nil
	}
	byID := make(map[string]groundJoinKey, len(floor))
	for _, r := range floor {
		if r.TextSHA != "" {
			byID[r.ID] = groundJoinKey{textSHA: r.TextSHA, ordinal: r.Ordinal}
		}
	}
	return byID
}

// groundRowMatchesFloor is §7.3's value check: a row whose `unit_id` names a
// floor row must carry that row's `text_sha` AND that row's `ordinal`. Every
// other cell of §7.2 is shape-checked and nothing else is compared.
//
// The exception earns itself because the alternative is a SILENT loss rather
// than a wrong number. §8's carry joins on `(text_sha, ordinal)` while coverage
// joins on `unit_id`, so a row with a valid id and a fabricated key counts as
// dispositioned in this round and fails to carry into the next, with nothing in
// either record saying why — and `--check` certifies a disposition of text
// nobody read, which is the exact failure this release exists to remove.
//
// **The whole key is compared, because half of it is not a check on the join.**
// The comparison read `text_sha` alone until an audit built both halves of what
// that leaves open and recorded each at exit 0. A row on `u1` carrying `u1`'s
// hash and `ordinal: 9` recorded, `--status` reported 3 dispositioned of 4, and
// round 2 on the unchanged spec carried 2 — verbatim the drop the paragraph
// above says the check removes. Worse, on a spec with two identical sentences a
// row naming `u2` with `ordinal: 1` recorded and round 2 marked `u1` CARRIED, so
// a unit nobody dispositioned was struck off what the next round was asked for.
// The second is reachable rather than hypothetical: 9 of 5,396 floor units
// across this repository's 54 specs carry `ordinal > 1`, and on `spec/1.0.0.md`
// every one of its 351 rows is `#1` — which is why a unit told to copy the cell
// unchanged is likelier to fill it from memory than to look.
//
// The two halves report different cells rather than one message, because a
// caller reads `Field` and the recoveries differ: a wrong hash means the row
// names the wrong text, a wrong ordinal means it names the wrong one of several
// units sharing that text.
//
// Two readings stay valid and are not this check's business. A `unit_id` the
// floor does not carry is `off_floor`, §8's fact to report at `--status` rather
// than a parse failure; and `unit_id: null` is a reader-added claim, which §7.2
// has supply its own hash over text tp never emitted.
// The row is taken by pointer because GroundRow is 192 bytes and this runs once
// per line of the record; gocritic's hugeParam flags the copy.
func groundRowMatchesFloor(row *GroundRow, byID map[string]groundJoinKey) error {
	if row.UnitID == nil {
		return nil
	}
	want, onFloor := byID[*row.UnitID]
	if !onFloor {
		return nil
	}
	if want.textSHA != row.TextSHA {
		return groundRowErr("text_sha", fmt.Sprintf(
			"the emitted floor gives %s the hash %s, and this row carries %s: "+
				"copy text_sha from the index row for the unit the row names",
			*row.UnitID, want.textSHA, row.TextSHA))
	}
	if want.ordinal != row.Ordinal {
		return groundRowErr("ordinal", fmt.Sprintf(
			"the emitted floor gives %s the ordinal %d, and this row carries %d: "+
				"copy ordinal from the index row for the unit the row names — §8 carries a "+
				"disposition forward on (text_sha, ordinal), so the hash alone does not identify "+
				"the unit when several share it",
			*row.UnitID, want.ordinal, row.Ordinal))
	}
	return nil
}

// ErrGroundRoundEmpty is what RecordGroundRound refuses a round that would write
// no row at all with — the payload's rows plus §8's carried rows both zero
// (§7.1).
//
// Zero rows would consume a round number for a round that decided nothing, and
// §8 would then join that round against no dispositions and report it as
// coverage that stalled.
//
// **It is not a rule about the payload.** §8 narrows what a round asks for to
// the units it still owes, so a re-emission on an unedited spec asks for none
// and the unit correctly writes an empty file; refusing that file is a deadlock
// reachable in three commands, and the round it refuses is the one carrying
// every disposition forward. The case the sentinel is for is the reader who ran
// nothing: an empty payload with nothing to carry.
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

// RecordGroundRound validates data and, only if every one of its rows passes and
// the round would write at least one row — its own or one §8 carries — writes it
// into specPath's state directory as `ground-round-<round>.ndjson`.
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
// floor is what §7.3's one value check is made against — a row whose `unit_id`
// names a floor row must carry that row's `text_sha` — and it is checked in the
// same pass as §7.2's table, so that refusal is atomic on the same terms as
// every other one. Everything else the floor is used for is §8's carry.
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
	rows, err = parseGroundRows(data, floor)
	if err != nil {
		return nil, nil, err
	}
	carried, err = GroundCarriedRows(specPath, round, floor, rows)
	if err != nil {
		return nil, nil, err
	}
	// After the parse and the carry, and before the write: the only point at
	// which all three facts are in hand — that every row validates, how many
	// there are, and how many §8 adds. The test is the SUM, because §7.1's rule
	// is on what the round records and not on what the operator handed in: a
	// round narrowed to owing nothing is asked for an empty payload and must be
	// able to hand one back. A guard on len(data) instead would record a file
	// of blank lines, which parses — correctly — to nothing.
	if len(rows)+len(carried) == 0 {
		return nil, nil, ErrGroundRoundEmpty
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
