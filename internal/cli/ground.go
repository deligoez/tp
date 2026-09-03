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
type groundResult struct {
	Spec       string `json:"spec"`
	Round      int    `json:"round"`
	Snapshot   string `json:"snapshot"`
	Floor      string `json:"floor"`
	OutputPath string `json:"output_path"`
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
const groundRecordRowHint = "fix the row the message names in the --record NDJSON, or record a file holding at least one row: every non-blank line is one JSON object carrying one floor unit's disposition"

// groundCarrySourceHint answers the one failure whose subject is neither the
// operator's file nor this round: §8 reads the immediately preceding round to
// carry its dispositions forward, and that file is tp's own.
//
// It names the recovery rather than the cell, because there is no cell to fix —
// the round file was written by an earlier --record and the operator's only
// levers are the artifact itself and re-recording the round that wrote it.
const groundCarrySourceHint = "the preceding ground round's file is tp's own artifact and could not be read back: restore or re-record spec/.tp-review/<base>/ground-round-<N-1>.ndjson — §8 carries its dispositions into this round"

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
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				output.Error(ExitUsage, "spec path required")
				os.Exit(ExitUsage)
				return nil
			}
			// §7.1 names each mode on its own line and never pairs two. The
			// combination is refused rather than resolved by the dispatch's
			// order: --status reports a round, --record writes one and --units
			// prints the floor's text, so running whichever came first would
			// hand the operator an exit 0 for the mode they did not ask for.
			// The refusal is taken before any of the three opens a file, so a
			// pairing that also names a missing --record path is still usage
			// (2) rather than the file error (3) that path alone would give.
			if groundModesPassed(unitsMode, statusMode, recordPath) > 1 {
				output.Error(ExitUsage, "--units, --status and --record are separate modes: pass one",
					"tp ground <spec> --record <file> writes the round; --status reports it; --units prints the floor's units")
				os.Exit(ExitUsage)
				return nil
			}
			// §7.1's second exit-2 input. --check is one bit added to
			// --status's answer and has no reading of its own: on --record it
			// would gate a mode that reports no coverage, and on the emission
			// it would gate a round nobody has dispositioned yet. Refusing it
			// here is also what keeps the code a RULE — an unregistered flag
			// exits 2 through cobra, which is the same number for a different
			// reason and tells the operator nothing about --status.
			if checkMode && !statusMode {
				output.Error(ExitUsage, "--check requires --status",
					"run tp ground <spec> --status --check: --check is the exit code of the coverage --status reports")
				os.Exit(ExitUsage)
				return nil
			}
			if unitsMode {
				return runGroundUnits(args[0])
			}
			if statusMode {
				return runGroundStatus(args[0], checkMode)
			}
			if recordPath != "" {
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

// groundModesPassed counts how many of §7.1's three mode-selecting flags the
// operator passed. --check is not one of them: it modifies --status's answer
// rather than choosing a mode, and its own refusal is stated separately.
func groundModesPassed(units, status bool, recordPath string) int {
	n := 0
	for _, passed := range []bool{units, status, recordPath != ""} {
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
		output.Error(ExitFile, fmt.Sprintf("cannot read the state directory for %s: %v", specPath, err),
			"check the permissions on spec/.tp-review/<base>/")
		os.Exit(ExitFile)
		return nil
	}

	// The index is derived from the bytes written as the snapshot — one read,
	// so the floor and the text it claims to be over cannot disagree.
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
	return output.JSON(groundResult{
		Spec:       specPath,
		Round:      round,
		Snapshot:   snapshotPath,
		Floor:      engine.GroundFloorPath(specPath, round),
		OutputPath: outputPath,
		Prompt: buildGroundPrompt(specPath, snapshotPath,
			engine.FormatFloorIndexCarried(commit, rows, carried), outputPath, round,
			groundFloorSize(rows), len(carried)),
	})
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
func runGroundUnits(specPath string) error {
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

	fmt.Print(engine.FormatFloorUnits(engine.FloorUnitRows(string(data))))
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
// check is §7.1's fifth invocation: exit 1 when a unit of the emitted floor
// carries no disposition, 0 otherwise. It gates on §8's coverage and on nothing
// else — a round of nothing but FAILs is fully covered and exits 0, because
// coverage answers *did anyone look* and the verdicts beside it answer *what
// did they find*. The branch is taken AFTER the payload is written, so the
// invocation a gated driver actually runs prints exactly what `--status` prints
// (Non-Goal 3: the code is a read-back, never a refusal).
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
		ByVerdict:     byVerdict,
	}); err != nil {
		// Exiting rather than returning: a truncated payload followed by a 0 —
		// or by a 1 under --check — is a status a caller cannot tell from a
		// complete one.
		output.Error(ExitFile, err.Error(), internalEncodeHint)
		os.Exit(ExitFile)
		return nil
	}

	// §8's ratio, read back as one bit. Dispositioned is derived by asking of
	// each EMITTED floor unit whether a row decided it, so it cannot exceed
	// Emitted and the comparison needs no upper guard.
	if check && status.Coverage.Dispositioned < status.Coverage.Emitted {
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
	var result error
	if lockErr := engine.WithFileLock(specPath, func() error {
		result = recordGroundRoundLocked(specPath, recordPath)
		return nil
	}); lockErr != nil {
		exitStateError(lockErr)
		return nil
	}
	return result
}

// recordGroundRoundLocked is --record's body, run under the state-directory
// write lock: it computes the round from the directory, reads the floor that
// round's emission froze, validates the payload against §7.2's table and writes
// the round file.
//
// Its failures abort through output.Error and os.Exit as the rest of this
// package's command bodies do; the flock is released by the process exiting.
func recordGroundRoundLocked(specPath, recordPath string) error {
	round, err := engine.NextGroundRound(specPath)
	if err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot read the state directory for %s: %v", specPath, err),
			"check the permissions on spec/.tp-review/<base>/")
		os.Exit(ExitFile)
		return nil
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
		return nil
	}
	floor, err := engine.ParseFloorIndex(string(floorData))
	if err != nil {
		output.Error(ExitFile,
			fmt.Sprintf("the emitted floor for ground round %d does not parse: %s: %v", round, floorPath, err),
			"re-emit the round with tp ground <spec>: the floor is tp's own artifact, and a round cannot be graded against an index it cannot read back")
		os.Exit(ExitFile)
		return nil
	}

	data, err := os.ReadFile(recordPath)
	if err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot read dispositions file: %s: %v", recordPath, err), recordFileMissingHint)
		os.Exit(ExitFile)
		return nil
	}

	rows, carried, err := engine.RecordGroundRound(specPath, round, data, floor)
	if err != nil {
		exitGroundRecordError(err)
		return nil
	}

	return output.JSON(groundRecordResult{
		Spec:    specPath,
		Round:   round,
		Floor:   floorPath,
		File:    engine.GroundRoundPath(specPath, round),
		Rows:    len(rows),
		Carried: len(carried),
	})
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
	var lineErr *engine.GroundLineError
	if errors.As(err, &lineErr) || errors.Is(err, engine.ErrGroundRoundEmpty) {
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
// the floor at 898,131 bytes against the index's 157,654, and the prefix shapes
// that tried to split the difference lost the one thing a prefix cannot carry —
// where a unit ends — which is why the byte length is on every row instead.
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
			groundPromptFloor(index, outputPath, round, floorSize, carried))
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
partial_kind — required on PARTIAL: two-readings, reason-not-conclusion, or
  true-when-written.
held_at — required when partial_kind is true-when-written: the commit, tag or
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
func groundPromptFloor(index, outputPath string, round, floorSize, carried int) string {
	var b strings.Builder

	b.WriteString("\n## The floor\n\n")
	b.WriteString(index)
	b.WriteString(`
Each row is ` + "`<unit_id> <anchor> <text_sha> #<ordinal> <bytes>B`" + `, and a row ending
in ` + "`(cut)`" + ` is a unit the arms dropped: it owes no disposition. The index carries
no unit text; the text is in the snapshot named at the top of this prompt. A unit
is one sentence of it, canonicalised — its wrapped lines joined, whitespace
collapsed to single spaces, a list or blockquote marker dropped, and a table row's
cells joined with an em dash; ` + "`<bytes>`" + ` is that text's length in UTF-8 bytes, which
is how you tell where it ends.

This is a FLOOR, not the set of claims. A claim it missed — including one inside a
cut unit — is recorded with "unit_id": null and is reported apart from coverage.

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
