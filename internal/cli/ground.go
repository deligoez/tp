package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// groundResult is what `tp ground <spec>` prints: the round it emitted, the two
// files it wrote, and the one prompt (§7.1) with the scratch file that prompt
// tells its unit to write.
//
// Prompt is a string and not a slice. Grounding asks one question of every unit
// of one floor, so there is no panel to fan out over (§7.1, Non-Goal 4), and a
// one-element array would invite a caller to loop over a set that cannot grow.
// FloorSize and Carried are the two numbers the prompt's ask is built from, in
// the envelope because they are the two an operator branches on. They were
// prose-only until an audit measured the asymmetry: --record's envelope has
// carried `rows` and `carried` as integers since it shipped, so the same two
// figures were machine-readable from one mode and English from the other,
// costing a caller a second process for `floor_size` and offering no reading of
// `carried` at all before the round was recorded.
//
// FloorSize is the floor, NOT the ask -- the ask is the difference, and the
// prompt states it. On an unedited spec the floor does not move between rounds
// while `carried` climbs to meet it, which is what makes the pair readable and
// either number alone misleading.
//
// Carried is also the only machine-readable sign of a carry that FAILED. An
// unreadable round N-1 leaves this mode at exit 0 with an unchanged-shape
// envelope asking for every unit; the notice saying so goes to stderr and
// --quiet removes it. `carried: 0` beside a floor that did not change is the
// fact that survives.
type groundResult struct {
	Spec       string `json:"spec"`
	Round      int    `json:"round"`
	Snapshot   string `json:"snapshot"`
	Floor      string `json:"floor"`
	OutputPath string `json:"output_path"`
	FloorSize  int    `json:"floor_size"`
	Carried    int    `json:"carried"`
	Prompt     string `json:"prompt"`
}

// groundRecordResult is what `tp ground <spec> --record <file>` prints: the
// round it wrote, where it wrote it, and the floor those rows were graded
// against (§7.3).
//
// Floor is carried rather than left implicit because it is the whole claim this
// mode makes: the round was recorded against the index the emission froze, not
// against the spec as it stands now.
type groundRecordResult struct {
	Spec  string `json:"spec"`
	Round int    `json:"round"`
	Floor string `json:"floor"`
	File  string `json:"file"`
	// Rows counts the rows the unit wrote, and Carried the dispositions §8
	// carried into this round's file from the one before it.
	//
	// Two counts and not one sum. `rows` is what the operator handed in and can
	// check against their own file; `carried` is what tp added, and folding the
	// two would report a number that matches neither artifact. `carried` is
	// present at zero rather than omitted, because a second pass reporting
	// nothing carried is the answer an operator most needs to see — it means
	// every unit's text moved, or the preceding round is gone.
	Rows    int `json:"rows"`
	Carried int `json:"carried"`
}

// groundStatusResult is what `tp ground <spec> --status` prints: which round it
// is about, §8's coverage of it, and the per-verdict breakdown that says what
// the round found (§8, §11 row 22).
//
// **The ratio is two integers and the share is two integers, deliberately.**
// A fraction would have to decide what to report for an empty floor, and a
// reader wanting one can divide — the same reading GroundCoverage and
// GroundAdvisory already ship. So §8's "NOT-A-CLAIM share is the first number
// to read" is served by putting its numerator and its denominator on one
// object: `by_verdict["NOT-A-CLAIM"]` over `emitted`.
//
// ReaderAdded and OffFloor are two fields and not one because §8 keeps them
// apart: they are evidence about different halves of §2.1 — the arms never
// produced this unit, against the arms produced it and then cut it — and
// collapsing them loses which half to go and look at.
//
// **Cut is here because `--check` gates on it.** §7.1 makes `--check` the exit
// code of the coverage this payload reports, so a quantity the gate reads and
// the payload hides makes the two disagree: `emitted: 0, dispositioned: 0`
// satisfies the payload's own coverage predicate while the exit says the
// opposite. The reason it is a key and not the stderr line alone was measured
// rather than preferred — `--quiet` suppresses `output.Notice`
// (internal/output/output.go), so under that flag a driver gets the exit code
// and nothing else. This release already made the same call twice for the same
// reason: audit's `file_summary.truncated`, and grounding's own `carried`.
type groundStatusResult struct {
	Spec  string `json:"spec"`
	Round int    `json:"round"`
	// Emitted and Dispositioned are §8's denominator and numerator, over
	// floor UNITS.
	Emitted       int `json:"emitted"`
	Dispositioned int `json:"dispositioned"`
	// ReaderAdded and OffFloor are the rows that move neither side.
	ReaderAdded int `json:"reader_added"`
	OffFloor    int `json:"off_floor"`
	// Cut is how many units §2.1 produced and the arms dropped. It is no part
	// of the ratio — a cut unit owes no disposition — and it is present at
	// zero rather than omitted, because it is the one key that separates the
	// two states a denominator of zero can mean, and a key a reader must
	// first decide whether an absence stands for is not a key they can read.
	Cut int `json:"cut"`
	// ByVerdict counts the round's ROWS, keyed by §3's verdict, all six
	// present. It is not a partition of Dispositioned: the two counts above
	// carry a verdict each while dispositioning nothing.
	ByVerdict map[string]int `json:"by_verdict"`
}

// groundStatusEmitHint tells an operator with no emitted round what to run.
const groundStatusEmitHint = "emit the round first: tp ground %s — --status reports the coverage of the latest EMITTED round, and this spec has none"

// groundRecordRowHint explains a --record file tp could read but would not
// record.
//
// Its own constant rather than recordRowHint, which names the reviewers and
// auditors who write review and audit rounds: a ground round's rows come from
// the one prompt `tp ground` emits, and the two refusals this hint answers —
// a row that fails the field table, and a file holding no rows at all — are
// both about that prompt's schema.
const groundRecordRowHint = "fix the row the message names in the --record NDJSON: every non-blank line is one JSON object carrying one floor unit's disposition"

// groundRecordEmptyHint is the other exit-1 refusal, and it needs its own words.
// The two shared one hint until an empty record was measured being told to "fix
// the row the message names" when the message names no row -- a reader sent
// looking for a line that is not in a file that has none. This refusal is
// reachable only when nothing carries either (§7.1), so the way out is to
// disposition something, never to edit a row.
const groundRecordEmptyHint = "record a file holding at least one row, or ground the units the emitted prompt asks for: the round carried nothing from a preceding round, so an empty payload would record nothing at all"

// groundRecordFileHint is the third of --record's hints to be carved out of the
// shared constant, and it is carved out for the reason the two above it were.
//
// recordFileMissingHint names "the NDJSON results file the reviewers/auditors
// wrote, not the spec or the task file". Grounding has neither role — §7.1
// emits ONE prompt and Non-Goal 4 says why there is no panel — and no task
// file is involved in this mode at all, so the shared sentence sent an operator
// looking for artifacts nothing in this command produces. It says instead where
// the file this mode wants comes from: the scratch path the emission's own
// `output_path` named.
const groundRecordFileHint = "check the --record path — this flag takes the NDJSON dispositions file the ground prompt's unit wrote to the emission's output_path, not the spec and not the floor"

// groundCarrySourceHint answers the one failure whose subject is neither the
// operator's file nor this round: §8 reads the immediately preceding round to
// carry its dispositions forward, and that file is tp's own.
//
// It names the recovery rather than the cell, because there is no cell to fix —
// the round file was written by an earlier --record and the operator's only
// levers are the artifact itself and re-recording the round that wrote it.
const groundCarrySourceHint = "the preceding ground round's file is tp's own artifact and could not be read back: restore or re-record spec/.tp-review/<base>/ground-round-<N-1>.ndjson — §8 carries its dispositions into this round"

// groundStateDirError is the one refusal both writing modes make for the same
// reason: NextGroundRound would not answer, so neither the emission nor the
// record knows which round it is.
//
// It is a function and not two literals because it was two literals -- the same
// message and the same hint, written out at both sites, in a file that hoists
// four other hints into named constants. An audit counted them; the one hint
// that was actually repeated was the one left inline.
func groundStateDirError(specPath string, err error) {
	output.Error(ExitFile, fmt.Sprintf("cannot read the state directory for %s: %v", specPath, err),
		"check the permissions on spec/.tp-review/<base>/")
	os.Exit(ExitFile)
}

// groundStdoutHint is what every mode of this command says when the result it
// built could not be written to stdout: a closed pipe, a read-only redirect, a
// full disk. Nothing the operator named is at fault and no other file repairs
// it.
//
// Its own words rather than internalEncodeHint, which two of the four modes
// cannot honestly use: --units prints TEXT and encodes nothing, and the failure
// is at the SINK rather than in the marshalling either way. internalEncodeHint
// tells the reader to report a bug; an fd 1 they cannot write to is theirs to
// fix, and telling them so is the difference between a hint and a dead end.
const groundStdoutHint = "tp built its result and could not write it to stdout: check that stdout is open for writing — a closed pipe, a read-only redirect or a full disk"

// groundRecordWrittenHint is --record's, and it carries the one fact the exit
// code cannot: the round file was written BEFORE the report failed, so the
// round exists. Re-running --record would take the NEXT round number, for which
// no emission has frozen a floor, and answer exit 3 about a missing floor
// instead — which is how a failed report turns into a story about the wrong
// file.
const groundRecordWrittenHint = "the round was recorded at %s before its report could be written: read that file rather than re-running --record, which would take the next round number and find no emitted floor for it"

func newGroundCmd() *cobra.Command {
	var (
		recordPath string
		statusMode bool
		checkMode  bool
		unitsMode  bool
	)

	cmd := &cobra.Command{
		Use:   "ground <spec.md>",
		Short: "Check a spec's claims against the world, before review is told they hold",
		Long: `Emit one prompt asking for a disposition on every unit of the spec's floor.

tp lint checks the document's form, tp validate checks the plan against the spec,
and tp audit checks the code against the spec. None of them checks the spec against
the world — review is explicitly told the spec is complete and authoritative. This
is what makes that true before it is said.

The emission writes the spec's text as this round reads it, plus the index derived
from that text, into spec/.tp-review/<base>/. Editing the spec afterwards does not
change the floor the round is graded against.`,
		Args:              cobra.ArbitraryArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				output.Error(ExitUsage, "spec path required")
				os.Exit(ExitUsage)
				return nil
			}
			// Whether --record was PASSED and what it was passed are two
			// different questions; groundModesPassed says why the first is
			// the one a mode is chosen by.
			recordPassed := cmd.Flags().Changed("record")
			groundRefuseUsage(unitsMode, statusMode, checkMode, recordPassed, recordPath)

			if unitsMode {
				// Same question as recordPassed above, asked of --json:
				// output.IsJSON() is true for every piped invocation, so it
				// cannot tell a caller who ASKED for JSON from one who was
				// simply not on a terminal. Only the first is being ignored,
				// and only the first is told.
				return runGroundUnits(args[0], cmd.Flags().Changed("json"))
			}
			if statusMode {
				return runGroundStatus(args[0], checkMode)
			}
			if recordPassed {
				return runGroundRecord(args[0], recordPath)
			}
			return runGround(args[0])
		},
	}
	cmd.Flags().StringVar(&recordPath, "record", "", "Record a ground round from an NDJSON dispositions file")
	cmd.Flags().BoolVar(&statusMode, "status", false, "Report the latest emitted round's coverage and per-verdict breakdown")
	cmd.Flags().BoolVar(&checkMode, "check", false, "With --status: exit 0 only when every emitted floor unit carries a disposition")
	cmd.Flags().BoolVar(&unitsMode, "units", false, "Print the floor's units with their full text, one per line")
	return cmd
}

// groundRefuseUsage takes §7.1's three exit-2 inputs. Every one exits the
// process, so returning at all is the answer "none of these applies".
//
// It is a function rather than three blocks inside RunE because the command
// body is long enough to trip the funlen ratchet as one, and the usage rules
// are its own seam: nothing above them refuses and nothing in them dispatches.
//
// All three are taken before any mode opens a file, which is what makes a
// pairing that ALSO names a missing --record path still usage (2) rather than
// the file error (3) that path alone would give.
func groundRefuseUsage(units, status, check, recordPassed bool, recordPath string) {
	// §7.1 names each mode on its own line and never pairs two. The
	// combination is refused rather than resolved by the dispatch's order:
	// --status reports a round, --record writes one and --units prints the
	// floor's text, so running whichever came first would hand the operator an
	// exit 0 for the mode they did not ask for.
	if groundModesPassed(units, status, recordPassed) > 1 {
		output.Error(ExitUsage, "--units, --status and --record are separate modes: pass one",
			"tp ground <spec> --record <file> writes the round; --status reports it; --units prints the floor's units")
		os.Exit(ExitUsage)
		return
	}
	// §7.1's second exit-2 input. --check is one bit added to --status's
	// answer and has no reading of its own: on --record it would gate a mode
	// that reports no coverage, and on the emission it would gate a round
	// nobody has dispositioned yet. Refusing it here is also what keeps the
	// code a RULE — an unregistered flag exits 2 through cobra, which is the
	// same number for a different reason and tells the operator nothing about
	// --status.
	if check && !status {
		output.Error(ExitUsage, "--check requires --status",
			"run tp ground <spec> --status --check: --check is the exit code of the coverage --status reports")
		os.Exit(ExitUsage)
		return
	}
	// §7.1's exit-2 row names `--record` with no path argument, and an empty
	// path is that input: cobra refuses a bare `--record` before this command's
	// body runs, so `--record ""` is the only spelling of it that reaches here.
	// Unrefused, it selected no mode at all and fell through to the EMISSION,
	// which rewrites the floor §7.3 freezes for a round already in flight.
	if recordPassed && recordPath == "" {
		output.Error(ExitUsage, "--record needs a path argument",
			"run tp ground <spec> --record <file>: --record names the NDJSON the round is recorded from, and an empty path names none")
		os.Exit(ExitUsage)
	}
}

// groundModesPassed counts how many of §7.1's three mode-selecting flags the
// operator passed. --check is not one of them: it modifies --status's answer
// rather than choosing a mode, and its own refusal is stated separately.
//
// record is whether the flag was PASSED, not whether its value is non-empty.
// Counting `--record ""` as unpassed let `--record "" --status` see one mode
// and run --status, reporting exit 0 for a mode nobody asked for on its own.
func groundModesPassed(units, status, record bool) int {
	n := 0
	for _, passed := range []bool{units, status, record} {
		if passed {
			n++
		}
	}
	return n
}

// runGround emits one ground round: it writes the snapshot and the floor index
// derived from it (§7.3), and prints the prompt naming ground-r<N>.ndjson.
func runGround(specPath string) error {
	data, err := os.ReadFile(specPath)
	if err != nil {
		if os.IsNotExist(err) {
			// First contact with the path: a spec-path mistake, so the shared
			// hint rather than the code-3 default's task-file advice.
			output.Error(ExitFile, fmt.Sprintf("spec not found: %s", specPath), specFileMissingHint)
			os.Exit(ExitFile)
			return nil
		}
		output.Error(ExitFile, fmt.Sprintf("cannot read spec: %s", specPath), err.Error())
		os.Exit(ExitFile)
		return nil
	}
	text := string(data)

	round, err := engine.NextGroundRound(specPath)
	if err != nil {
		// An unreadable state directory, which NextGroundRound refuses to
		// answer 1 for: emitting under a guessed number would hand this round
		// an identifier an existing one already holds.
		groundStateDirError(specPath, err)
		return nil
	}

	// The index is derived from the bytes handed to WriteGroundEmission as the
	// snapshot, so THIS process's floor is over the same read of the spec as
	// THIS process's snapshot. That is a property of this function, not of the
	// directory it writes into: the two files land as two unpaired writes under
	// no lock, so what a later reader finds on disk may still be a snapshot and
	// a floor derived from different texts. WriteGroundEmission's doc carries
	// the limit, why the lock is not added, and how to re-measure it.
	commit := groundCommit(specPath)
	rows := engine.FloorIndexRows(text, engine.FloorAnchorOf(text))
	index := engine.FormatFloorIndex(commit, rows)

	if err := engine.WriteGroundEmission(specPath, round, data, []byte(index)); err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot write the ground round's artifacts: %v", err),
			"check that spec/.tp-review/<base>/ is writable")
		os.Exit(ExitFile)
		return nil
	}

	// §8's second pass, asked at emit. What goes to disk above is §2.2's index
	// — the artifact §7.3 grades the round against, in the two row shapes
	// ParseFloorIndex reads back — and what goes to the prompt below is the
	// same rows with the carried ones marked. The marks are deliberately NOT
	// written to the floor: `--record` re-derives the carry from round N-1
	// itself, so a copy in the floor file would be a second statement of the
	// same fact with nothing comparing the two.
	carried := groundCarriedUnits(specPath, round, rows)

	snapshotPath := engine.GroundSnapshotPath(specPath, round)
	outputPath := fmt.Sprintf("ground-r%d.ndjson", round)
	// The snapshot and the floor are already on disk, and a re-emission of an
	// unrecorded round rewrites the same two files, so the recovery here is
	// simply to run it again with a stdout that works — which is why this
	// carries the shared hint and --record's carries its own.
	if err := output.JSON(groundResult{
		Spec:       specPath,
		Round:      round,
		Snapshot:   snapshotPath,
		Floor:      engine.GroundFloorPath(specPath, round),
		OutputPath: outputPath,
		FloorSize:  groundFloorSize(rows),
		Carried:    len(carried),
		Prompt: buildGroundPrompt(specPath, snapshotPath,
			engine.FormatFloorIndexCarried(commit, rows, carried), outputPath, round,
			groundFloorSize(rows), len(carried)),
	}); err != nil {
		output.Error(ExitFile, err.Error(), groundStdoutHint)
		os.Exit(ExitFile)
	}
	return nil
}

// groundCarriedUnits is the set of this round's floor units that already carry
// a disposition from the round before it (§8), as `unit_id`s.
//
// It asks engine.GroundCarriedRows with no decided rows, because a round that
// has only just been emitted has decided nothing — and it is the same call
// `--record` makes, so the units the prompt marks are the units the record
// carries. A rule of its own here would let the prompt promise a carry the
// record does not make.
//
// A preceding round tp cannot read back answers "nothing carried" rather than
// refusing the emission. §7.1's table gives the emission no failure of its own
// for this, and asking for every unit is the honest ask when nothing can be
// carried: the rows a unit then writes are a superset of what is owed, and the
// round still records once the operator repairs the artifact. The refusal
// belongs at `--record`, the sink that acts on the carry, and is already there
// (exit 3). The notice is what keeps that from being silent.
func groundCarriedUnits(specPath string, round int, rows []engine.FloorIndexRow) map[string]bool {
	carried, err := engine.GroundCarriedRows(specPath, round, rows, nil)
	if err != nil {
		output.Notice(fmt.Sprintf("nothing carried into ground round %d: %v", round, err))
		return nil
	}
	units := make(map[string]bool, len(carried))
	for i := range carried {
		if carried[i].UnitID != nil {
			units[*carried[i].UnitID] = true
		}
	}
	return units
}

// groundFloorSize counts the units the round owes a disposition for before §8's
// carry is taken off, on §2.2's convention that the absence of the hash is the
// cut. A cut unit is in the index and owes nothing.
func groundFloorSize(rows []engine.FloorIndexRow) int {
	n := 0
	for _, r := range rows {
		if r.TextSHA != "" {
			n++
		}
	}
	return n
}

// runGroundUnits implements `tp ground <spec> --units` (§7.1): print the
// floor's units, one line per unit, each carrying the whole canonical text and
// the same `text_sha` the index row for that unit carries (§11 row 4b).
//
// It reads the spec named on the command line, takes no lock and writes
// nothing: no round is emitted, and a spec with no state directory answers
// exactly as one with ten rounds behind it. §2.2 gives the reason the mode
// exists — the index carries no unit text, and a prefix locates a unit while
// hiding where it ends, so a reader who needs the text asks for all of it once.
//
// The listing goes to stdout as TEXT and not as JSON, deliberately. §11 row 4b
// asserts a property of LINES — one per floor unit, with the hash and the text
// the hash is over on the same line — and jsonMode is on for every piped
// invocation, so an envelope would leave the shipped shape reachable from a
// terminal alone. Nothing is lost by it: the row is three tab-separated fields
// and the unit itself cannot hold a tab, since §2.1 step 3 collapses every
// whitespace run in a prose block and joins a table row's cells with an em dash.
func runGroundUnits(specPath string, jsonAsked bool) error {
	// --units prints text whatever --json says, and that is deliberate: row 4b's
	// assertion is about LINES, and jsonMode is on for every piped invocation,
	// so an envelope would leave the shipped shape reachable only from a
	// terminal. What was wrong was the SILENCE -- exit 0, TSV on stdout, zero
	// bytes on stderr, so a caller that asked for JSON and got TSV had nothing
	// to read. output.Notice's own doc names "a flag ignored" as what the
	// channel is for, and this is that case exactly.
	if jsonAsked {
		output.Notice("--json is ignored by --units: the floor's units are printed as text, one per line, because each line is a unit's whole text and an envelope would quote it")
	}
	data, err := os.ReadFile(specPath)
	if err != nil {
		if os.IsNotExist(err) {
			output.Error(ExitFile, fmt.Sprintf("spec not found: %s", specPath), specFileMissingHint)
			os.Exit(ExitFile)
			return nil
		}
		output.Error(ExitFile, fmt.Sprintf("cannot read spec: %s", specPath), err.Error())
		os.Exit(ExitFile)
		return nil
	}

	// fmt.Print's error is read, and that is the whole of this mode's sink.
	// Dropped, a stdout tp cannot write to made --units the one mode that
	// printed nothing, said nothing and exited 0 — a listing a caller cannot
	// tell from an empty floor.
	if _, err := fmt.Print(engine.FormatFloorUnits(engine.FloorUnitRows(string(data)))); err != nil {
		output.Error(ExitFile, err.Error(), groundStdoutHint)
		os.Exit(ExitFile)
	}
	return nil
}

// runGroundStatus implements `tp ground <spec> --status` (§7.1): report the
// latest emitted round's coverage and the per-verdict breakdown beside it.
//
// It takes no lock and writes nothing — reads are lock-free in this package —
// and it never opens the spec, on §7.3's rule that the round is graded against
// the floor its emission froze.
//
// A spec with no emitted round is refused with exit 3 rather than reported as
// 0-of-0. There is no round for a status to be about, and the shape a vacuous
// answer would take is the one a later `--check` reads as converged.
//
// check is §7.1's fifth invocation, and it has TWO conditions. Exit 1 when a
// unit of the emitted floor carries no disposition; exit 1 also when the
// emitted floor is empty and the arms cut units to empty it, which is the one
// state coverage certifies falsely. It gates on nothing else — a round of
// nothing but FAILs is fully covered and exits 0, because coverage answers *did
// anyone look* and the verdicts beside it answer *what did they find*.
//
// Both conditions read a key of the payload — `dispositioned` against
// `emitted`, then `emitted` against `cut` — so the code is reconstructible from
// what the invocation printed. The branch is taken AFTER the payload is
// written, so the invocation a gated driver actually runs prints exactly what
// `--status` prints (Non-Goal 3: the code is a read-back, never a refusal).
func runGroundStatus(specPath string, check bool) error {
	status, err := engine.LatestGroundStatus(specPath)
	if err != nil {
		if errors.Is(err, engine.ErrNoGroundEmission) {
			output.Error(ExitFile, fmt.Sprintf("no emitted ground round for %s", specPath),
				fmt.Sprintf(groundStatusEmitHint, specPath))
			os.Exit(ExitFile)
			return nil
		}
		output.Error(ExitFile, fmt.Sprintf("cannot read the ground round for %s: %v", specPath, err),
			"check the permissions and the contents of spec/.tp-review/<base>/")
		os.Exit(ExitFile)
		return nil
	}

	byVerdict := make(map[string]int, len(status.ByVerdict))
	for verdict, n := range status.ByVerdict {
		byVerdict[string(verdict)] = n
	}

	if err := output.JSON(groundStatusResult{
		Spec:          specPath,
		Round:         status.Round,
		Emitted:       status.Coverage.Emitted,
		Dispositioned: status.Coverage.Dispositioned,
		ReaderAdded:   status.Coverage.ReaderAdded,
		OffFloor:      status.Coverage.OffFloor,
		Cut:           status.Cut,
		ByVerdict:     byVerdict,
	}); err != nil {
		// Exiting rather than returning: a truncated payload followed by a 0 —
		// or by a 1 under --check — is a status a caller cannot tell from a
		// complete one.
		output.Error(ExitFile, err.Error(), groundStdoutHint)
		os.Exit(ExitFile)
		return nil
	}

	if !check {
		return nil
	}

	// §8's ratio, read back as one bit. Dispositioned is derived by asking of
	// each EMITTED floor unit whether a row decided it, so it cannot exceed
	// Emitted and the comparison needs no upper guard.
	if status.Coverage.Dispositioned < status.Coverage.Emitted {
		os.Exit(ExitValidation)
	}

	// The one state the ratio alone certifies falsely: an emitted floor of NO
	// units. `0 < 0` is false, so a spec on which nothing was checked read as
	// covered, and both signals SKILL.md tells an agent to look at said so.
	//
	// **Which zero it is decides the answer, and only the cut count can tell.**
	// A document §2.1 produced no unit from is honestly 0-of-0 — there is no
	// claim the round skipped, and refusing there would refuse every spec that
	// is all headings. A document whose sentences the arms all DROPPED is the
	// opposite: units existed, none was checked, and §2.2's whole reason for
	// announcing the cut set is that both end-to-end runs found defects inside
	// it. Measured on a fixture carrying four prose claims that no arm keeps:
	// round 1 emitted, nothing dispositioned, and this exited 0.
	//
	// The payload carries the reason, and the notice says it in words. That
	// order was a repair: this comment used to argue against the key on the
	// ground that one line of stderr answers the state, and `--quiet`
	// suppresses `output.Notice` — measured, the flag leaves an unattended
	// driver exit 1, an empty stderr, and a payload whose own coverage
	// predicate holds. `--check` is a read-back and prints no error (Non-Goal
	// 3), so `cut` is what makes the code reconstructible from what it read
	// back; the notice is for the operator reading a terminal.
	// The notice names the FLOOR file and not `--units`, which was the first
	// wording and does not work: FloorUnitRows emits a row only for a unit the
	// arms kept, so on this exact input `tp ground <spec> --units` prints zero
	// lines and exits 0. The index is where a cut unit is visible at all — it
	// carries the id and the anchor of every one (§2.2's announcement of the
	// cut set), which is what an operator needs to go and look.
	if status.Coverage.Emitted == 0 && status.Cut > 0 {
		output.Notice(fmt.Sprintf(
			"ground round %d has no floor to cover: §2.1 produced %d units and the arms cut every one, "+
				"so nothing in %s has been checked against the world — the cut units are listed by id "+
				"and anchor in %s, and --units prints nothing here because it prints floor units",
			status.Round, status.Cut, specPath, engine.GroundFloorPath(specPath, status.Round)))
		os.Exit(ExitValidation)
	}
	return nil
}

// runGroundRecord implements `tp ground <spec> --record <file>`: validate every
// row of the file against §7.2's table and, only then, write it into the state
// directory as ground-round-<N>.ndjson (§7.3).
//
// N is the round the last emission wrote, which is what NextGroundRound answers:
// only a recorded round advances the number, so the floor and the snapshot on
// disk belong to the round these rows are for.
//
// The spec's own text is never read here. §7.3 makes the recorded floor the
// artifact a round is graded against precisely so that a spec edited — or gone —
// between emit and record cannot re-floor it; re-deriving one from the current
// text is the emit-time-hash defect this repository already has open elsewhere.
func runGroundRecord(specPath, recordPath string) error {
	// §7.1's exit 3, and the one input of its two that this mode does not
	// reach by opening a file: a state directory tp cannot read is one no
	// round may be added to. Recording into it writes a ground round beside
	// review and audit artifacts tp has already lost track of, and the
	// operator learns nothing until the next command that loads the index
	// aborts on it.
	//
	// A ground-only directory is NOT that case and must not be refused here
	// (§11 rows 11 and 14): ground's artifacts are their own prefix list, so
	// LoadReviewState answers (nil, nil) for a directory holding a ground
	// round and its emission and nothing else. The rebuildable window — a
	// review or audit snapshot whose round is still in flight — is tolerated
	// on the same terms as every other reader in this package.
	if _, err := engine.LoadReviewState(specPath); err != nil && !engine.IsRebuildableStateIndex(err) {
		exitStateError(err)
		return nil
	}

	// §7.1's exit 4. The round number is read from the directory and the round
	// file is written into it, so both belong inside one write lock: unlocked,
	// two concurrent records compute the same N and the second overwrites the
	// first, each exiting 0.
	//
	// The lock's target is the SPEC PATH, not the state.json that
	// WithReviewStateLock keys on. Two reasons, the first measured.
	// LockFilePath resolves .tp/ from the target's own directory, and for
	// <dir>/.tp-review/<base>/state.json outside a git repository that resolves
	// to a .tp/ created INSIDE the state directory — a lock file among the
	// round's own artifacts, which the ground tests caught as a fourth entry in
	// a directory §11 row 12 states exhaustively. And the spec path is what
	// this command is keyed on: ground writes no state.json (§7.3), so what it
	// must exclude is another ground round on the same spec, never a review
	// record whose files and round numbers are disjoint from it.
	//
	// Contention that outlasts lock_timeout_seconds comes back as a
	// *LockTimeoutError, which exitStateError maps to exit 4 with the hint
	// naming the lock and the wait.
	// recordGroundRoundLocked returns nothing: every failure inside it is an
	// os.Exit through output.Error, so a returned error could only ever be nil
	// and a caller reading one would be reading a value that carries no case.
	// unparam says the same thing.
	if lockErr := engine.WithFileLock(specPath, func() error {
		recordGroundRoundLocked(specPath, recordPath)
		return nil
	}); lockErr != nil {
		exitStateError(lockErr)
		return nil
	}
	return nil
}

// recordGroundRoundLocked is --record's body, run under the state-directory
// write lock: it computes the round from the directory, reads the floor that
// round's emission froze, validates the payload against §7.2's table and writes
// the round file.
//
// Its failures abort through output.Error and os.Exit as the rest of this
// package's command bodies do; the flock is released by the process exiting.
func recordGroundRoundLocked(specPath, recordPath string) {
	round, err := engine.NextGroundRound(specPath)
	if err != nil {
		groundStateDirError(specPath, err)
		return
	}

	// The floor is read, not derived, and its absence is the no-prior-emit case
	// (§7.3). It is also PARSED here rather than only opened: §8's carry-forward
	// asks, of each emitted floor unit, whether the preceding round decided the
	// same text, and the index is where the units of this round are. A floor
	// that cannot be read back is refused for ParseFloorIndex's own reason —
	// reading it short silently shrinks §8's denominator, and a denominator that
	// is too small makes coverage look higher than it is.
	floorPath := engine.GroundFloorPath(specPath, round)
	floorData, err := os.ReadFile(floorPath)
	if err != nil {
		output.Error(ExitFile,
			fmt.Sprintf("no emitted floor for ground round %d: cannot read %s: %v", round, floorPath, err),
			fmt.Sprintf("emit the round first: tp ground %s — --record validates against the floor that emission froze, never against the spec as it now stands", specPath))
		os.Exit(ExitFile)
		return
	}
	floor, err := engine.ParseFloorIndex(string(floorData))
	if err != nil {
		output.Error(ExitFile,
			fmt.Sprintf("the emitted floor for ground round %d does not parse: %s: %v", round, floorPath, err),
			"re-emit the round with tp ground <spec>: the floor is tp's own artifact, and a round cannot be graded against an index it cannot read back")
		os.Exit(ExitFile)
		return
	}

	data, err := os.ReadFile(recordPath)
	if err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot read dispositions file: %s: %v", recordPath, err), groundRecordFileHint)
		os.Exit(ExitFile)
		return
	}

	rows, carried, err := engine.RecordGroundRound(specPath, round, data, floor)
	if err != nil {
		exitGroundRecordError(err)
		return
	}

	roundPath := engine.GroundRoundPath(specPath, round)
	// The round is on disk by now, so a report that cannot be written is a
	// failure ABOUT the report and not about the payload. Returning the error
	// exited 1 with task-file advice, which reads as "fix your file and run it
	// again" — and the re-run takes the next round number, for which no
	// emission has frozen a floor.
	if err := output.JSON(groundRecordResult{
		Spec:    specPath,
		Round:   round,
		Floor:   floorPath,
		File:    roundPath,
		Rows:    len(rows),
		Carried: len(carried),
	}); err != nil {
		output.Error(ExitFile, err.Error(), fmt.Sprintf(groundRecordWrittenHint, roundPath))
		os.Exit(ExitFile)
	}
}

// exitGroundRecordError maps a RecordGroundRound failure onto §7.1's exit codes.
//
// A row that fails §7.2's table and a file holding no rows are both refusals of
// what the operator handed in, so both are validation (exit 1); the two are told
// apart by their types rather than by their wording. A preceding round tp could
// not read back is not one of those — the operator's file is fine and the broken
// artifact is tp's own — so it takes exit 3, the code §7.1 gives a file that
// cannot be used. Anything else reached the state layer, where exitStateError
// already separates corrupt state (3) from a write lock it could not take (4).
func exitGroundRecordError(err error) {
	// The carry's own failure is checked FIRST, and the order is the assertion.
	// A *GroundCarryError wraps whatever reading the preceding round failed on,
	// which for a truncated round file is a *GroundLineError — so the branch
	// below matches it too, and would send the operator to fix a line of a file
	// they did not write and cannot see from the message.
	var carryErr *engine.GroundCarryError
	if errors.As(err, &carryErr) {
		output.Error(ExitFile, err.Error(), groundCarrySourceHint)
		os.Exit(ExitFile)
		return
	}
	if errors.Is(err, engine.ErrGroundRoundEmpty) {
		output.Error(ExitValidation, err.Error(), groundRecordEmptyHint)
		os.Exit(ExitValidation)
		return
	}
	var lineErr *engine.GroundLineError
	if errors.As(err, &lineErr) {
		output.Error(ExitValidation, err.Error(), groundRecordRowHint)
		os.Exit(ExitValidation)
		return
	}
	exitStateError(err)
}

// groundCommit is the commit §2.2 puts on the index's first line, discovered
// here and passed to the renderer: a formatter that shelled out to git would
// produce output no test could state. An answer git cannot give is "", which
// FormatFloorIndex renders as `unknown`.
func groundCommit(specPath string) string {
	sha, err := gitHeadShortSHA(filepath.Dir(specPath))
	if err != nil {
		return ""
	}
	return sha
}

// groundKindSubject is §4.1's "the claim is about" column, keyed by kind. It is
// prose for a reader and nothing reads it back; the normative column — which
// tiers are acceptable — is derived from the engine's own sets below rather
// than restated here, so the prompt cannot tell a unit a rule the recorder does
// not enforce.
var groundKindSubject = map[engine.GroundKind]string{
	engine.KindDocument:      "what a document says",
	engine.KindCodeStructure: "how code is structured",
	engine.KindCorpus:        "what the corpus contains",
	engine.KindBehaviour:     "existing behaviour",
	engine.KindMechanism:     "whether a proposed mechanism works",
	engine.KindDefect:        "whether a defect is real",
	engine.KindGuard:         "whether a guard measures anything",
}

// groundTierDid is §4.1's second table: what each tier means the unit did.
var groundTierDid = map[engine.GroundTier]string{
	engine.TierRead:            "read the artifact",
	engine.TierQuery:           "ran a query over the corpus",
	engine.TierRun:             "ran the shipped command",
	engine.TierProbe:           "built a probe and ran it",
	engine.TierRedGreen:        "wrote the test, watched it red, fixed, watched it green",
	engine.TierBreakAndControl: "broke the subject, ran the suite, THEN ran the control",
}

// groundVerdictMeaning is §3's table: what each of the six verdicts says.
var groundVerdictMeaning = map[engine.GroundVerdict]string{
	engine.VerdictPass:         "evidence at a tier acceptable for the claim's kind supports it",
	engine.VerdictPartial:      "true under one reading and not another, or the conclusion holds while the stated reason does not, or it was true when written",
	engine.VerdictFail:         "evidence at an acceptable tier contradicts it",
	engine.VerdictUnverifiable: "no acceptable tier is reachable at all; tier records the deepest one attempted",
	engine.VerdictQuestion:     "the attempt did not settle it, either because the result was unclear or because the tier it reached is not one the kind accepts",
	engine.VerdictNotAClaim:    "the unit is a decision, a prediction, or prose carrying no assertion about the world",
}

// buildGroundPrompt renders the single prompt an emission carries: what a row
// is, which evidence answers which kind of claim, and the floor index every one
// of those rows is owed for.
//
// The index travels whole and the spec's text does not. §2.2 measured inlining
// every floor unit's text against sending the index alone, and the prefix shapes
// that tried to split the difference lost the one thing a prefix cannot carry —
// where a unit ends — which is why the byte length is on every row instead.
//
// **The derivation, not the count.** The index half is the size of the floor
// files `tp ground <spec>` writes; the inlining half is that plus every unit's
// text, which `tp ground <spec> --units` prints and nothing shipped re-derives
// on its own. This comment carried a byte pair for both halves until an audit
// re-derived the index half two ways and got neither of them, which is why the
// recipe is here and the numbers are not.
//
// The three sections are three functions because the prompt is long enough to
// trip the funlen ratchet as one, and its seams are its own headings.
// floorSize is the units in the index owing a disposition — every unit the arms
// kept — and carried how many of them §8 inherits, so the ask the floor section
// states is derived from the emission's own two numbers rather than counted out
// of the rendered index.
func buildGroundPrompt(specPath, snapshotPath, index, outputPath string, round, floorSize, carried int) string {
	return appendClausesGround(
		groundPromptRow(specPath, snapshotPath, round) +
			groundPromptEvidence() +
			groundPromptFloor(specPath, index, outputPath, round, floorSize, carried))
}

// groundPromptRow opens the prompt and states the row: what the unit is being
// asked, and the six verdicts it may answer with.
func groundPromptRow(specPath, snapshotPath string, round int) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Ground round %d — check this spec's claims against the world\n\n", round)
	fmt.Fprintf(&b, "Spec: %s\nText this round reads: %s (the spec's bytes as of this emission)\n\n", specPath, snapshotPath)
	b.WriteString(`Every claim in this document is a premise the next review round is told to treat
as settled. Your job is to decide, for EVERY unit of the floor below this round
asks you for, what it is worth against the world outside the document. The floor
section says which those are. You are not reviewing the spec:
wording, coherence and design are the next phase's subject, not yours.

## The row

One JSON object per line. Copy unit_id, anchor, text_sha and ordinal from the
index row unchanged — they are how a later round joins your disposition to the
sentence you graded. Required on every row: unit_id, anchor, text_sha, ordinal,
verdict. An unknown top-level key is rejected, and one rejected row refuses the
whole round.

verdict — exactly one of:
`)
	for _, v := range engine.GroundVerdicts() {
		fmt.Fprintf(&b, "  %-13s %s\n", v, groundVerdictMeaning[v])
	}
	b.WriteString(`
kind, tier — required unless the verdict is NOT-A-CLAIM, where the pair is
  optional: a unit that states a decision AND a checked fact records both.
evidence — required whenever tier is present: the command you ran, or the
  artifact and line you read. Free text, because a later reader re-runs it.
`)
	// Derived from the enum for the reason the kind-tier table below is: this
	// list is what a unit reads before writing a row, and ParseGroundPartialKind
	// is what refuses the whole round when it wrote something else.
	partialKinds := make([]string, 0, len(engine.GroundPartialKinds()))
	for _, kind := range engine.GroundPartialKinds() {
		partialKinds = append(partialKinds, string(kind))
	}
	fmt.Fprintf(&b, "partial_kind — required on PARTIAL: exactly one of %s.\n", strings.Join(partialKinds, ", "))
	b.WriteString(`held_at — required when partial_kind is true-when-written: the commit, tag or
  date at which the claim held.
causes — required on QUESTION: three to five {"cause": ..., "prediction": ...}
  objects, ranked, each prediction an observation that would confirm or kill its
  cause.
note — free text, optional.
unit_id is null on a claim you add that the floor did not carry; supply the
  text_sha yourself, over the claim's own text.
`)
	return b.String()
}

// groundPromptEvidence renders §4.1: what each tier means, which tiers say
// anything about which kind of claim, and §7.2's per-verdict reading of that
// rule.
//
// The kind–tier column is derived from TierAcceptableFor rather than written
// out, because that predicate is what rejects a row at --record: a prompt
// stating the rule in its own words can drift from the recorder, and the unit
// pays for the drift with a refused round.
func groundPromptEvidence() string {
	var b strings.Builder

	b.WriteString(`
## Which evidence answers which claim

tier records what you ACTUALLY did, never what the kind wanted:

`)
	for _, tier := range engine.GroundTiers() {
		fmt.Fprintf(&b, "  %-18s %s\n", tier, groundTierDid[tier])
	}
	b.WriteString(`
The tiers are NOT ordered. Past ` + "`run`" + ` a "deeper" tier is not more of the same
evidence but evidence about a different subject: probe, red-green and
break-and-control rank rigour on an artifact you built, while read, query and run
examine the real one. So acceptability is a set per kind, not a threshold:

`)
	for _, kind := range engine.GroundKinds() {
		acceptable := make([]string, 0, len(engine.GroundTiers()))
		for _, tier := range engine.GroundTiers() {
			if engine.TierAcceptableFor(kind, tier) {
				acceptable = append(acceptable, string(tier))
			}
		}
		fmt.Fprintf(&b, "  %-15s %-36s %s\n", kind, groundKindSubject[kind], strings.Join(acceptable, ", "))
	}
	b.WriteString(`
On PASS, PARTIAL and FAIL the tier MUST be one of the kind's, or the row is
rejected. On QUESTION either relation is legal — reaching an acceptable tier and
not being settled by the result is one shape of question, not reaching one is the
other. On UNVERIFIABLE the tier names the deepest attempt.

## Where to start, and what to do when a claim resists

Start with the cheap sentences: a number borrowed from another context, a rule
left unstated, a reason that sounds right. That is where the defects were. A claim
whose verdict depends on an unsettled claim belongs to a later pass.

Before concluding on a claim your first attempt did not settle, name three to five
falsifiable causes with their predictions and test them in rank order. Find facts
yourself — two things reach the operator and nothing else: a fact only the author
holds, and a decision. A QUESTION does not stop the round; carry on with every
claim that does not depend on the answer.
`)
	return b.String()
}

// groundPromptFloor carries the emitted index whole, says how to read a row,
// states what this round is actually asked for, and names the file its rows go
// to.
//
// **It names `--units`, and the byte length it states is a length of what that
// mode prints.** The sentence said the unit's text was "in the snapshot named at
// the top of this prompt" and that `<bytes>` "is how you tell where it ends",
// and an audit measured both halves false on this repository's own spec. A unit
// is CANONICALISED — §2.1 step 3 joins its wrapped lines, collapses whitespace
// and drops a list or blockquote marker — so it is generally not a byte range of
// any file: on `spec/1.0.0.md` most floor units are not substrings of the
// snapshot at all, and `u1` said 168B against a raw span of 170B because the
// snapshot wraps that sentence across two `> `-prefixed lines — a reader
// following the old sentence read 168 bytes and truncated `rounds.` to `round`.
//
// **The derivation, not the count.** Emit a round on a copy of the spec, then
// compare `tp ground <copy> --units` against the snapshot beside it: the share
// that are not substrings is the figure, and the same run gives 0 sha and 0
// length mismatches against the index. Two runs a few hours apart returned
// 327-of-351 and 338-of-362 on this file, because the spec was being edited
// between them — which is why the recipe is here and the number is not.
// The agreement half is §11 row 4b's shipped test rather than a claim here.
//
// specPath is threaded down for the one reason a prompt needs it: `--units` is
// useless to a reader who has to work out what to put after it, and the emission
// already knows.
func groundPromptFloor(specPath, index, outputPath string, round, floorSize, carried int) string {
	var b strings.Builder

	b.WriteString("\n## The floor\n\n")
	b.WriteString(index)
	b.WriteString(`
Each row is ` + "`<unit_id> <anchor> <text_sha> #<ordinal> <bytes>B`" + `, and a row ending
in ` + "`(cut)`" + ` is a unit the arms dropped: it owes no disposition. The index carries
no unit text. A unit is one sentence of the spec, CANONICALISED — its wrapped lines
joined, whitespace collapsed to single spaces, a list or blockquote marker dropped,
and a table row's cells joined with an em dash — so a unit is generally NOT a byte
range of any file, and cannot be read off the snapshot by counting.

`)
	fmt.Fprintf(&b, "Run `tp ground %s --units` for the text. It prints one line per floor unit,\n", specPath)
	b.WriteString(`` + "`<unit_id>\\t<text_sha>\\t<text>`" + `, each carrying its unit WHOLE; ` + "`<bytes>`" + ` is the UTF-8
length of THAT text and ` + "`<text_sha>`" + ` its hash, so the listing and the index above
join on ` + "`unit_id`" + ` and agree on both cells. The snapshot named at the top of this
prompt is the spec as this round found it: read it for a unit's surroundings — its
section, what precedes it, what the sentence is about — never to measure a unit.

This is a FLOOR, not the set of claims. A claim the floor did not carry still gets a
row, and the row says WHICH kind of miss it was. The two are reported apart from
coverage and apart from each other:

  the index never carried the claim      ` + "`\"unit_id\": null`" + `
  the claim is inside a ` + "`(cut)`" + ` unit     that unit's own ` + "`unit_id`" + `

A cut unit HAS an id, so naming it is what says the arms produced the unit and cut
it wrongly; ` + "`null`" + ` there reports the opposite — that the arms never produced it —
and leaves the cut nobody can go and look at. Either way supply ` + "`text_sha`" + ` yourself
over the claim's own text: the index carries no hash for a cut unit.

`)
	b.WriteString(groundPromptAsk(round, floorSize, carried))
	fmt.Fprintf(&b, "\nWrite this round's rows to: %s\n", outputPath)

	return b.String()
}

// groundPromptAsk is §8's narrowed ask: what this round owes, and why the rest
// of the index is not being asked about.
//
// The index above it lists every unit and this sentence says which of them the
// round is for — §8 narrows the ask and not the index, because "a reader who
// cannot see the whole floor cannot tell it what the floor missed" (§2.2).
//
// The count is stated rather than left to be counted off the marks. It is what
// §8's saving is measured in — a one-sentence edit left 1 disposition owed and
// the shipped prompt asked for 308 — and a reader who has to derive it from a
// 300-row index is paying the cost the number exists to report.
func groundPromptAsk(round, floorSize, carried int) string {
	if carried == 0 {
		return fmt.Sprintf(
			"This round owes a disposition for each of the %d floor units above.\n", floorSize)
	}
	return fmt.Sprintf(`This round owes a disposition for %d of the %d floor units above: the other %d
already carry one from round %d, and their rows end in `+"`(carried)`"+`. A carried
disposition stands while its unit's text stands (§8) — do not decide those units
again, and write no row for them.
`, floorSize-carried, floorSize, carried, round-1)
}
