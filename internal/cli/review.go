package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// findingsFileMissingHint explains a --findings path that does not exist. The
// hint used to say only "check the path", but the likelier cause is that the
// previous review round converged and therefore wrote nothing: there is no
// file to point at, and where the flag is optional the right move is to drop
// it. Shared by tp review, standalone regression and tp audit so the same
// mistake reads the same way whichever command catches it.
const findingsFileMissingHint = "a review round that converged with zero findings writes no findings file at all — where --findings is optional, omitting it is valid; otherwise check the path."

// mechanizedExclusionPrefix opens the reviewer-facing exclusion sentence both
// prompt-emission sites append. One string, for the reason engine.DivergenceHint
// is one string: a sentence written twice is a sentence that can drift, and this
// one instructs the reviewers whose findings decide convergence.
const mechanizedExclusionPrefix = "\n\nMechanically checked classes — do NOT report findings of these classes: "

// ndjsonInputFileHint is the hint for a path handed to a mode that reads loose
// NDJSON files — --merge and --report. Left hintless these sites inherit the
// code-3 default, which is TASK-file advice ("run 'tp use <file>' … 'tp init
// <spec>'") and repairs nothing here: none of these modes takes a task file or
// a spec at all.
const ndjsonInputFileHint = "check the path — this mode takes the NDJSON files the reviewers/auditors wrote, not the spec or the task file"

// ndjsonLineCap is the per-line read cap shared by every NDJSON reader in the
// review/audit family, plus tp done --batch. The caps used to disagree — 64KB
// in --merge, --report, parseFindingsFile and tp audit --findings, 1MB in
// --resolve, the audit merge and tp done --batch — so a findings file --resolve
// had just rewritten could be unreadable by --merge. One constant, one answer.
//
// tp add --bulk and tp set --bulk are in that set too: they used to read at
// bufio's default to keep their own warn-and-continue contracts, and now abort
// at this cap like every other reader. TestNDJSONReadersShareTheCap holds every
// scanner in the package to this constant and lets a site out only when it says
// on its own line what non-NDJSON input it reads instead — which is where the
// next exception would have to be recorded.
const ndjsonLineCap = 1024 * 1024

// ndjsonLineTooLongHint explains a line over ndjsonLineCap. Distinct from
// ndjsonInputFileHint because the path is not the mistake here: the file is
// exactly where the operator said it was, and one row inside it is too long.
const ndjsonLineTooLongHint = "a single line exceeded the 1MB NDJSON read cap — re-emit the file with one finding per line, shortening the oversized note"

// affectedFilesHint is the hint for a bad --affected-files entry. The flag
// takes source files for the reviewer to read, so the code-3 default's task-file
// advice names the wrong object entirely.
const affectedFilesHint = "check the --affected-files paths — the flag takes source files to read, one existing file per entry"

// reviewDirFlagHint is the hint for --docs-path/--test-path, the two review
// flags that take a directory rather than a file.
const reviewDirFlagHint = "this flag takes a directory that already exists — check the path"

// outputFileHint is the hint for a failed -o/--output write, where nothing
// about the INPUT paths is wrong.
const outputFileHint = "check -o/--output: its directory must exist and be writable"

// stateWriteHint is the fallback for a state-layer failure that is neither
// corruption nor lock contention — a write that could not land.
const stateWriteHint = "check that the .tp-review state directory exists and is writable"

// internalEncodeHint marks the one failure the caller cannot repair: tp built a
// result it could not encode. Saying so is more use than task-file advice.
const internalEncodeHint = "internal error: the result could not be encoded as JSON — please report it with the command you ran"

// ndjsonReadHint picks the hint for a failed NDJSON read — or write, since
// exitResolveError reports a failed rewrite of the same artifact through it.
// Pointing at the path repairs nothing when the path was fine and the line was
// too long: that is the one-cause-for-every-failure defect, reversed.
func ndjsonReadHint(err error) string {
	if errors.Is(err, bufio.ErrTooLong) {
		return ndjsonLineTooLongHint
	}
	return ndjsonInputFileHint
}

// specFileMissingHint is the hint for a spec-PATH mistake: tp was handed a path
// that is not a readable spec markdown file. It deliberately does not guess WHY
// — a typo, the task file passed where the spec belongs, the wrong working
// directory — it only says the path itself is the thing to check. Left hintless
// these sites inherit the code-3 default, which is TASK-file advice ("run 'tp
// use <file>' … 'tp init <spec>'"): the wrong object entirely.
//
// USE it wherever the failing call is tp's FIRST contact with that path — an
// os.Stat guard, or a read no stat preceded — across every tp review and tp
// audit mode, so the same typo reads the same way whichever one catches it.
//
// Do NOT use it once a stat or read of the SAME path has already succeeded
// earlier in the call path. There the caller's path was right and what failed
// afterwards is a real I/O or permission problem; those sites pass err.Error(),
// which names the actual cause (loadAuditSpec and runReviewVerify's per-file
// reads are the worked examples).
const specFileMissingHint = "check the spec path — this command takes the spec markdown file, not the task file"

// recordFileMissingHint explains a --record path that cannot be read. Left
// hintless, the site inherits the code-3 default, which is TASK-file advice
// ("run 'tp use <file>' … 'tp init <spec>'") — the wrong object for the NDJSON
// results file the reviewers or auditors wrote. --record is the flag every
// review and audit round ends on, so a typo here is the loop's hottest file
// error and the one that can least afford wrong-object advice.
const recordFileMissingHint = "check the --record path — this flag takes the NDJSON results file the reviewers/auditors wrote, not the spec or the task file"

// recordRowHint explains a --record file tp could read but could not parse: a
// row that is not JSON, or one the row rules reject. Left hintless, the site
// inherits the code-1 default, which is TASK-file advice ("run 'tp validate' to
// audit the task file") — an unrelated command over an unrelated file, when the
// object at fault is the NDJSON the reviewers or auditors just wrote and the
// message already names the line (§9.2). It is the fallback, not a replacement:
// a row rule with advice of its own (a pre-resolved fixed row wants the spec
// re-reviewed) still wins.
const recordRowHint = "fix the line the message names in the --record NDJSON: every non-blank line is one JSON object written by a reviewer or auditor"

type reviewFinding struct {
	Severity   string          `json:"severity"`
	Category   string          `json:"category"`
	Class      string          `json:"class,omitempty"`
	Location   string          `json:"location"`
	Finding    string          `json:"finding"`
	Suggestion string          `json:"suggestion"`
	Resolved   *resolvedStatus `json:"resolved,omitempty"`
}

type resolvedStatus struct {
	Status     string `json:"status"`
	Evidence   string `json:"evidence"`
	ResolvedAt string `json:"resolved_at"`
}

type reviewPrompt struct {
	Role       string `json:"role"`
	Category   string `json:"category"`
	Prompt     string `json:"prompt"`
	OutputPath string `json:"output_path"`
}

type reviewLoop struct {
	Round               int    `json:"round"`
	Convergence         string `json:"convergence"`
	PreviousFindings    int    `json:"previous_findings"`
	RequiredCleanRounds *int   `json:"required_clean_rounds,omitempty"`
	ConsecutiveClean    *int   `json:"consecutive_clean,omitempty"`
	Converged           *bool  `json:"converged,omitempty"`
	Stale               *bool  `json:"stale,omitempty"`
	Instruction         string `json:"instruction"`
	Mode                string `json:"mode,omitempty"`
}

type reviewResult struct {
	Spec               string                     `json:"spec"`
	SpecRef            bool                       `json:"spec_ref,omitempty"`
	SpecPath           string                     `json:"spec_path,omitempty"`
	StructuredElements *engine.StructuredElements `json:"structured_elements,omitempty"`
	Perspective        string                     `json:"perspective,omitempty"`
	DocsPath           string                     `json:"docs_path,omitempty"`
	TestPath           string                     `json:"test_path,omitempty"`
	AffectedFiles      []string                   `json:"affected_files,omitempty"`
	AffectedSummary    *engine.AffectedSummary    `json:"affected_summary,omitempty"`
	DocsStructure      *docStructure              `json:"docs_structure,omitempty"`
	TestStructure      *docStructure              `json:"test_structure,omitempty"`
	MechanicalChecks   []map[string]any           `json:"mechanical_checks,omitempty"`
	SkippedRoles       *[]engine.SkippedRole      `json:"skipped_roles,omitempty"`
	// Ungrounded is §9's advisory: the latest ground round, the floor units
	// nobody dispositioned, and the floor's size. A pointer with omitempty,
	// because §9 makes it absent ENTIRELY when every unit is dispositioned or
	// no ground round exists — on `divergence`'s precedent that a permanent
	// zero-valued key is a key every reader learns to skip.
	//
	// **The first disjunct has a second, vacuous cause, and this sentence used
	// to call it a third case.** An audit built a spec whose prose claims carry
	// no digit, no backtick span and no listed verb, so all three arms cut
	// everything: round 1 is emitted, nothing is dispositioned, and the key is
	// still absent. LatestGroundAdvisory has ONE coverage branch —
	// `Emitted - Dispositioned == 0` — and `0 - 0` satisfies it, so an all-cut
	// floor drops the key under "every unit is dispositioned" without any unit
	// having been looked at.
	//
	// The advisory stays silent there, deliberately. Its two integers are
	// counts over emitted floor units, and on an empty floor both are honestly
	// zero; emitting the key with `undispositioned: 0` would make its presence
	// mean two incompatible things — "N units are open" and "no unit exists" —
	// and a reader dividing gets 0/0. Saying it properly needs a field this
	// object does not have, which is a documented key and so a release's work
	// rather than an audit repair.
	//
	// What the audit's finding costs is therefore bounded to the reporting
	// channel: the GATE was the other half and is repaired, because §7.1's
	// `--check` is what a driver branches on. `tp ground <spec> --status
	// --check` now exits 1 on that floor and names the cut count on stderr
	// (runGroundStatus). An operator who reads only `tp review` still learns
	// nothing on such a spec.
	Ungrounded *engine.GroundAdvisory `json:"ungrounded,omitempty"`
	Prompts    []reviewPrompt         `json:"prompts"`
	ReviewLoop reviewLoop             `json:"review_loop"`
}

type docStructure struct {
	TotalFiles    int    `json:"total_files"`
	ReviewedFiles int    `json:"reviewed_files"`
	StructureMap  string `json:"structure_map"`
}

const (
	specContentCap     = engine.SpecContentCap
	findingsSummaryCap = engine.FindingsSummaryCap
	affectedPerFileCap = engine.AffectedPerFileCap
	affectedTotalCap   = engine.AffectedTotalCap
	promptBudget       = engine.PromptBudget
)

func newReviewCmd() *cobra.Command {
	var round int
	var findingsPath string
	var roleFilter string
	var perspective string
	var docsPath string
	var testPath string
	var affectedFiles []string
	var finalRound bool
	var mergeMode bool
	var resolveMode bool
	var resolveAllMode bool
	var verifyMode bool
	var reportMode bool
	var outputPath string
	var diffFrom string
	var specInline bool
	var forceFlag bool
	var recordPath string
	var statusMode bool
	var checkFlag bool
	var noState bool
	var harnessNote string

	cmd := &cobra.Command{
		Use:   "review <spec.md>",
		Short: "Generate review prompts for spec quality or planning",
		Long: `Parses a spec and generates targeted review prompts.
Default (no --perspective): 3 adversarial prompts (implementer, tester, architect).
--perspective documentation: single doc change plan prompt.
--perspective testing: single test plan prompt.
--perspective code-audit: single code audit prompt (requires --affected-files).

Modes (mutually exclusive):
--merge: merge and deduplicate findings from NDJSON files.
--resolve/--resolve-all: mark findings as fixed/wontfix/duplicate (findings NDJSON is the positional; --resolve uses a 0-based index).
--verify: lightweight verification prompt.
--report: cross-round convergence report.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := detectReviewMode(mergeMode, resolveMode, resolveAllMode, verifyMode, reportMode, recordPath != "", statusMode)
			// --merge takes only its input NDJSON files and -o <file>: it
			// records no round and reads no state, so a flag belonging to
			// another mode is named here, ahead of the generic guards. That
			// ordering is the point — --harness-note used to fall through to
			// "supply it together with --record", and --record is exactly what
			// --merge rejects next, so one mistake cost two failed calls.
			// Mirrors tp audit's exhaustive --merge rejection list.
			// --perspective/--docs-path/--test-path belong here too: --merge
			// generates no prompt, so it used to accept and silently ignore
			// them while --record/--status reject --perspective outright.
			// Together with validateModeFlags (--round/--findings/
			// --affected-files/--final-round/--diff-from/--spec-inline), the
			// --check guard below and the mode-conflict detector, every flag
			// tp review defines is now either accepted by --merge (-o and the
			// positional NDJSON inputs) or rejected by name.
			if mode == "merge" && (cmd.Flags().Changed("harness-note") || forceFlag || noState ||
				perspective != "" || docsPath != "" || testPath != "") {
				output.Error(ExitUsage, "--merge cannot be combined with --harness-note/--force/--no-state/--perspective/--docs-path/--test-path",
					"run --merge on its own: it takes only the input NDJSON files and -o <file>")
				os.Exit(ExitUsage)
				return nil
			}
			if checkFlag && mode != "status" {
				output.Error(ExitUsage, "--check requires --status")
				os.Exit(ExitUsage)
				return nil
			}
			if cmd.Flags().Changed("harness-note") && recordPath == "" {
				output.Error(ExitUsage, "--harness-note requires --record", "supply --harness-note only together with --record <file>")
				os.Exit(ExitUsage)
				return nil
			}
			if noState && (mode == "record" || mode == "status" || checkFlag) {
				output.Error(ExitUsage, "--no-state cannot be combined with --record, --status, or --check")
				os.Exit(ExitUsage)
				return nil
			}
			// -o is only read by runReviewMerge. Accepting it on any other mode
			// would silently drop the caller's redirect target while the payload
			// still went to stdout — the same silently-ignored-flag hazard the
			// --merge list guards in the opposite direction.
			if cmd.Flags().Changed("output") && mode != "merge" {
				output.Error(ExitUsage, "-o/--output requires --merge",
					"tp review writes its payload to stdout; redirect it, or use --merge to write an NDJSON file with -o")
				os.Exit(ExitUsage)
				return nil
			}
			// §4.2.2: --role selects one prompt, so it is refused by every mode
			// that emits none. The check sits before the mode dispatch so the
			// operator sees the flag conflict rather than that mode's own
			// argument complaint.
			//
			// A conflict sentinel is excluded, and that is not the same
			// exception: detectReviewMode returns "conflict:<a>+<b>" when two
			// modes are given, and interpolating it invented a flag named
			// --conflict:merge+status. §4.2.2 defers to this check over a
			// mode's own ARGUMENT validation -- a missing --findings -- not
			// over the prior question of which mode this is. Two conflicting
			// modes have a truer complaint, and validateModeFlags below makes
			// it: "--merge and --status are mutually exclusive".
			if cmd.Flags().Changed("role") && mode != "" && mode != "verify" &&
				!strings.HasPrefix(mode, "conflict:") {
				output.Error(ExitUsage, "--role cannot be combined with --"+mode,
					"--role selects one emitted prompt; --"+mode+" emits none")
				os.Exit(ExitUsage)
				return nil
			}
			if mode == "" {
				// Default review mode — requires exactly 1 spec arg
				if len(args) != 1 {
					output.Error(ExitUsage, "spec path required")
					os.Exit(ExitUsage)
					return nil
				}
				return runReview(cmd, args[0], round, findingsPath, perspective, docsPath, testPath, affectedFiles, finalRound, diffFrom, roleFilter, cmd.Flags().Changed("role"), specInline, noState)
			}
			if err := validateModeFlags(mode, round, findingsPath, affectedFiles, finalRound, diffFrom, specInline, perspective); err != nil {
				output.Error(ExitUsage, err.Error())
				os.Exit(ExitUsage)
				return nil
			}
			switch mode {
			case "merge":
				return runReviewMerge(args, outputPath)
			case "resolve":
				return runReviewResolve(args, forceFlag)
			case "resolve-all":
				return runReviewResolveAll(args, forceFlag)
			case "verify":
				if len(args) != 1 {
					output.Error(ExitUsage, "spec path required for --verify")
					os.Exit(ExitUsage)
					return nil
				}
				return runReviewVerify(args[0], findingsPath, affectedFiles, diffFrom, specInline, roleQueryFor(args[0], roleFilter, cmd.Flags().Changed("role")))
			case "report":
				return runReviewReport(args)
			case "record":
				if len(args) != 1 {
					output.Error(ExitUsage, "spec path required for --record")
					os.Exit(ExitUsage)
					return nil
				}
				return runReviewRecord(args[0], recordPath, harnessNote)
			default:
				// "status" is the only value that reaches here: detectReviewMode
				// returns one of the seven mode names, and the two remaining
				// returns — "" and a "conflict:" pair — already returned above.
				if len(args) != 1 {
					output.Error(ExitUsage, "spec path required for --status")
					os.Exit(ExitUsage)
					return nil
				}
				return runReviewStatus(args[0], checkFlag)
			}
		},
	}

	// Default review flags
	cmd.Flags().IntVar(&round, "round", 1, "Current review round number (1-indexed)")
	cmd.Flags().StringVar(&findingsPath, "findings", "", "Path to NDJSON file with previous round findings")
	cmd.Flags().StringVar(&roleFilter, "role", "", "Emit only this role's prompt (§4.2); one name, not repeatable")
	cmd.Flags().StringVar(&perspective, "perspective", "", "Review perspective: documentation, testing, or code-audit")
	cmd.Flags().StringVar(&docsPath, "docs-path", "", "Path to documentation directory (required with --perspective documentation)")
	cmd.Flags().StringVar(&testPath, "test-path", "", "Path to test directory (required with --perspective testing)")
	cmd.Flags().StringArrayVar(&affectedFiles, "affected-files", nil, "Paths to source files to inject into review context")
	cmd.Flags().BoolVar(&finalRound, "final-round", false, "Force mandatory code read-through in review prompts (requires round >= 2)")

	// New mode flags
	cmd.Flags().BoolVar(&mergeMode, "merge", false, "Merge and deduplicate findings from NDJSON files")
	cmd.Flags().BoolVar(&resolveMode, "resolve", false, "Mark a finding as fixed/wontfix/duplicate (0-based index; findings NDJSON is the positional)")
	cmd.Flags().BoolVar(&resolveAllMode, "resolve-all", false, "Mark all unresolved findings with a status (findings NDJSON is the positional)")
	cmd.Flags().BoolVar(&verifyMode, "verify", false, "Generate lightweight verification prompt")
	cmd.Flags().BoolVar(&reportMode, "report", false, "Generate cross-round convergence report")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path (for --merge)")
	cmd.Flags().StringVar(&diffFrom, "diff-from", "", "Baseline spec for diff-based review (requires --round >= 2)")
	cmd.Flags().BoolVar(&specInline, "spec-inline", false, "Embed full spec content inline (default: reference by path)")
	cmd.Flags().BoolVar(&forceFlag, "force", false, "Force re-resolve already resolved findings")
	cmd.Flags().StringVar(&recordPath, "record", "", "Record a review round from an NDJSON findings file")
	cmd.Flags().BoolVar(&statusMode, "status", false, "Show recorded review rounds and convergence state")
	cmd.Flags().BoolVar(&checkFlag, "check", false, "With --status: run registered mechanical checks")
	cmd.Flags().StringVar(&harnessNote, "harness-note", "", "With --record: store an optional free-text note on the recorded round")
	cmd.Flags().BoolVar(&noState, "no-state", false, "Disable all review-state reads and writes (pre-0.23.0 manual behavior)")

	return cmd
}

// detectReviewMode returns the active mode name, or "" for default review.
// Returns error-mode name if multiple modes are active.
func detectReviewMode(merge, resolve, resolveAll, verify, report, record, status bool) string {
	modes := make([]string, 0)
	if merge {
		modes = append(modes, "merge")
	}
	if resolve {
		modes = append(modes, "resolve")
	}
	if resolveAll {
		modes = append(modes, "resolve-all")
	}
	if verify {
		modes = append(modes, "verify")
	}
	if report {
		modes = append(modes, "report")
	}
	if record {
		modes = append(modes, "record")
	}
	if status {
		modes = append(modes, "status")
	}
	if len(modes) == 0 {
		return ""
	}
	if len(modes) > 1 {
		return "conflict:" + modes[0] + "+" + modes[1]
	}
	return modes[0]
}

// validateModeFlags checks that modifier flags are compatible with the active mode.
func validateModeFlags(mode string, round int, findingsPath string, affectedFiles []string, finalRound bool, diffFrom string, specInline bool, perspective string) error {
	if after, ok := strings.CutPrefix(mode, "conflict:"); ok {
		pair := after
		return fmt.Errorf("--%s are mutually exclusive", strings.Replace(pair, "+", " and --", 1))
	}

	// Record and status reject the prompt-generation flags and --perspective
	if (mode == "record" || mode == "status") && perspective != "" {
		return fmt.Errorf("--%s is mutually exclusive with --perspective", mode)
	}

	// Merge, resolve, resolve-all, report, record, status reject modifier flags
	if mode == "merge" || mode == "resolve" || mode == "resolve-all" || mode == "report" || mode == "record" || mode == "status" {
		if round != 1 {
			return fmt.Errorf("--%s is mutually exclusive with --round", mode)
		}
		if findingsPath != "" {
			return fmt.Errorf("--%s is mutually exclusive with --findings", mode)
		}
		if len(affectedFiles) > 0 {
			return fmt.Errorf("--%s is mutually exclusive with --affected-files", mode)
		}
		if finalRound {
			return fmt.Errorf("--%s is mutually exclusive with --final-round", mode)
		}
		if diffFrom != "" {
			return fmt.Errorf("--%s is mutually exclusive with --diff-from", mode)
		}
		if specInline {
			return fmt.Errorf("--%s is mutually exclusive with --spec-inline", mode)
		}
	}

	// Verify rejects --round, --final-round, and --perspective but allows --findings, --affected-files, --diff-from, --spec-inline
	if mode == "verify" {
		if round != 1 {
			return fmt.Errorf("--verify is mutually exclusive with --round (verification is not a numbered round)")
		}
		if finalRound {
			return fmt.Errorf("--verify is mutually exclusive with --final-round")
		}
		if perspective != "" {
			return fmt.Errorf("--verify is mutually exclusive with --perspective")
		}
	}

	return nil
}

// Stub functions for new modes — will be implemented in separate files.

// runReviewVerify — implemented in review_verify.go.
// runReviewReport — implemented in review_report.go.

// reviewPerspectives is the set --perspective accepts, in the order the refusal
// names them.
//
// It is a package-level slice rather than a map literal inside runReview so one
// declaration is both the validator's source and the refusal's, and a test can
// read the same list the code branches on instead of restating it (§6.2
// property 2: the mode list is derived from the code, not written out).
var reviewPerspectives = []string{"documentation", "testing", "code-audit", "regression"}

// invalidPerspectiveMessage renders the refusal from reviewPerspectives, so a
// perspective added to the slice cannot be missing from the message.
func invalidPerspectiveMessage(got string) string {
	quoted := make([]string, 0, len(reviewPerspectives))
	for _, p := range reviewPerspectives {
		quoted = append(quoted, "'"+p+"'")
	}
	last := len(quoted) - 1
	list := strings.Join(quoted[:last], ", ") + ", or " + quoted[last]
	return fmt.Sprintf("invalid perspective: %q (must be %s)", got, list)
}

func runReview(cmd *cobra.Command, specPath string, round int, findingsPath, perspective, docsPath, testPath string, affectedFiles []string, finalRound bool, diffFrom, roleFilter string, roleGiven, specInline, noState bool) error {
	if perspective != "" && !slices.Contains(reviewPerspectives, perspective) {
		output.Error(ExitUsage, invalidPerspectiveMessage(perspective))
		os.Exit(ExitUsage)
		return nil
	}

	if perspective == "regression" {
		return runReviewRegression(specPath, diffFrom, findingsPath, roleQueryFor(specPath, roleFilter, roleGiven))
	}

	affectedFiles = validateReviewInputs(perspective, round, findingsPath, affectedFiles, docsPath, testPath, finalRound, diffFrom, specPath)

	specContent := resolveReviewSpecContent(specPath, diffFrom, specInline)

	switch perspective {
	case "code-audit":
		return runReviewCodeAudit(specPath, specContent, affectedFiles, round, roleQueryFor(specPath, roleFilter, roleGiven))
	case "documentation":
		return runReviewDocPlan(specPath, specContent, docsPath, affectedFiles, roleQueryFor(specPath, roleFilter, roleGiven))
	case "testing":
		return runReviewTestPlan(specPath, specContent, testPath, affectedFiles, roleQueryFor(specPath, roleFilter, roleGiven))
	}

	if round < 1 {
		output.Error(ExitUsage, "round must be >= 1")
		os.Exit(ExitUsage)
		return nil
	}

	if findingsPath != "" {
		if _, err := os.Stat(findingsPath); os.IsNotExist(err) {
			output.Error(ExitFile, fmt.Sprintf("findings file not found: %s", findingsPath), findingsFileMissingHint)
			os.Exit(ExitFile)
			return nil
		}
	}

	// §2.5 item 2: resolve the reviewer panel — and with it decide the
	// empty-phase refusal — ahead of every write the emission path performs.
	// The state lifecycle below calls EnsureReviewState, which creates
	// .tp-review/<spec>/ and state.json before the round snapshot, so a
	// refusal decided any later would leave all three on disk.
	panel := resolveRolePanel(specPath, engine.PhaseReviewers)

	// State-backed round lifecycle (default three-role mode): tp numbers the
	// round, snapshots the spec, and injects previous findings automatically.
	statePrevFindings := make([]reviewFinding, 0)
	var stateRequired, stateConsecutive *int
	var stateConverged, stateStale *bool
	var reviewSt *engine.ReviewState
	if !noState {
		rs := loadReviewRoundState(cmd, specPath, round, findingsPath)
		round, reviewSt = rs.round, rs.st
		stateRequired, stateConsecutive, stateConverged, stateStale = rs.required, rs.consecutive, rs.converged, rs.stale
		statePrevFindings = rs.prevFindings
	}

	if round > 1 && findingsPath == "" && noState {
		output.Info(fmt.Sprintf("round %d without --findings: prompts will not exclude previously reported issues", round))
	}

	lines, headings, err := parseSpecFile(specPath)
	if err != nil {
		output.Error(ExitFile, err.Error(), specFileMissingHint)
		os.Exit(ExitFile)
		return nil
	}

	_, elems := engine.CheckStructuredElements(lines, headings)

	findings := statePrevFindings
	if findingsPath != "" {
		findings = mustParseFindingsFile(findingsPath)
	}

	summary := buildFindingsSummary(findings)

	var affectedContent map[string]string
	if len(affectedFiles) > 0 {
		affectedContent = engine.ReadAffectedFilesBudgetAware(affectedFiles, summary, specContent)
	}

	// Mechanical checks: workflow-derived (not state-derived), run before
	// prompt generation even under --no-state; failures never abort generation
	wfChecks, checksTaskFile := engine.ResolveWorkflow(specPath, flagFile)
	var mechChecks []map[string]any
	if len(wfChecks.Checks) > 0 {
		mechChecks, _ = runMechanicalChecks(&wfChecks, checksTaskFile)
	}

	prompts, regressionIncluded, skippedRoles := buildReviewPrompts(specPath, &panel, elems, specContent, round, summary, affectedFiles, finalRound, &wfChecks, diffFrom, noState, reviewSt)

	prompts = appendClausesReview(prompts)
	prompts = filterReviewPrompts(prompts, roleQueryFor(specPath, roleFilter, roleGiven), skippedRoles)

	uniqueCount := len(dedupFindings(findings))
	convergence, instruction := buildReviewLoopInstruction(round, findings, findingsPath, specPath, specInline, noState, stateRequired, regressionIncluded, len(wfChecks.Checks) > 0)

	// §4.2.3.1: the key is addressed to a caller holding the whole panel, and
	// --role makes it false. Narrowing runs on the finished string rather than
	// inside the builder, which assembles it across nine assignment sites —
	// there is no single quoted text to subtract from, and a subset of the
	// finished string is a subset by construction.
	if roleGiven {
		instruction = instructionForPayload(narrowInstructionForRole(instruction), len(prompts))
	}

	result := reviewResult{
		Spec:               specPath,
		StructuredElements: elems,
		MechanicalChecks:   mechChecks,
		Prompts:            prompts,
		ReviewLoop: reviewLoop{
			Round:               round,
			Convergence:         convergence,
			PreviousFindings:    uniqueCount,
			RequiredCleanRounds: stateRequired,
			ConsecutiveClean:    stateConsecutive,
			Converged:           stateConverged,
			Stale:               stateStale,
			Instruction:         instruction,
		},
	}

	// §9's advisory: ONE key in the top-level envelope, never per role — its
	// reader is the operator, not the role, and per role it would be one copy
	// per role per round of a signal no role acts on. It survives --compact
	// because it is the payload rather than commentary on the payload, and it
	// reaches no exit code: review is told, and not stopped (Non-Goal 3).
	result.Ungrounded = engine.LatestGroundAdvisory(specPath)

	if !specInline {
		absPath, _ := filepath.Abs(specPath)
		result.SpecRef = true
		result.SpecPath = absPath
	}

	if len(affectedFiles) > 0 {
		result.AffectedFiles = affectedFiles
		result.AffectedSummary = engine.BuildAffectedSummary(affectedFiles, affectedContent)
	}

	// §9.1: skipped_roles names every non-emitted role. Whether it survives
	// --compact is skippedRolesSurviveCompact's question, not this function's.
	if skippedRolesSurviveCompact(roleGiven, len(prompts)) {
		if skippedRoles == nil {
			skippedRoles = []engine.SkippedRole{}
		}
		result.SkippedRoles = &skippedRoles
	}

	return output.JSON(result)
}

// validateReviewInputs validates the non-perspective-dispatch flags for the
// default review path (mutual exclusion, round budget, docs/test paths,
// final-round, diff baseline, affected files) and returns the deduplicated
// affected-file list. It os.Exit()s with a usage or file error on any failure.
func validateReviewInputs(perspective string, round int, findingsPath string, affectedFiles []string, docsPath, testPath string, finalRound bool, diffFrom, specPath string) []string {
	if perspective != "" && perspective != "code-audit" && (round != 1 || findingsPath != "") {
		output.Error(ExitUsage, "--perspective is mutually exclusive with --round/--findings (except code-audit)")
		os.Exit(ExitUsage)
		return nil
	}

	// code-audit is exempt from that rule for --round, which it reports, but
	// NOT for --findings: runReviewCodeAudit never reads the file and answers
	// previous_findings 0 about it, so accepting the flag asserted a count over
	// a file tp never opened — the accepted-then-silently-dropped shape the
	// -o/--output and --merge guards exist to refuse.
	if perspective == "code-audit" && findingsPath != "" {
		output.Error(ExitUsage, "--perspective code-audit does not read --findings",
			"drop --findings, or run the default review panel, which does read it")
		os.Exit(ExitUsage)
		return nil
	}

	if perspective == "code-audit" && len(affectedFiles) == 0 {
		output.Error(ExitUsage, "--perspective code-audit requires at least one --affected-files")
		os.Exit(ExitUsage)
		return nil
	}

	// Round budget: default review mode refuses generation when the cap is
	// exhausted; cap-triggered state reads inherit the corrupt-state abort
	if perspective == "" {
		wfBudget, _ := engine.ResolveWorkflow(specPath, flagFile)
		if wfBudget.ReviewMaxRounds > 0 {
			stBudget, stErr := engine.LoadReviewState(specPath)
			if stErr != nil {
				// The snapshot-only window is not corruption, here as at the
				// five other state reads. Without this the budget knob decided
				// whether a review round could be emitted at all: default
				// review_max_rounds emitted fine, a non-zero one exited 3.
				if !engine.IsRebuildableStateIndex(stErr) {
					exitStateError(stErr)
					return nil
				}
				stBudget = nil
			}
			rounds := []engine.ReviewRound{}
			if stBudget != nil {
				rounds = stBudget.ReviewRounds
			}
			refuseIfBudgetExhausted("review", specPath, rounds, wfBudget.ReviewMaxRounds, wfBudget.ReviewCleanRounds, wfBudget.ReviewConvergeOn)
		}
	}

	if perspective == "documentation" && docsPath == "" {
		output.Error(ExitUsage, "--docs-path is required when --perspective=documentation")
		os.Exit(ExitUsage)
		return nil
	}

	if perspective == "testing" && testPath == "" {
		output.Error(ExitUsage, "--test-path is required when --perspective=testing")
		os.Exit(ExitUsage)
		return nil
	}

	if docsPath != "" {
		info, err := os.Stat(docsPath)
		if err != nil || !info.IsDir() {
			output.Error(ExitFile, fmt.Sprintf("docs path not found or not a directory: %s", docsPath), reviewDirFlagHint)
			os.Exit(ExitFile)
			return nil
		}
	}

	if testPath != "" {
		info, err := os.Stat(testPath)
		if err != nil || !info.IsDir() {
			output.Error(ExitFile, fmt.Sprintf("test path not found or not a directory: %s", testPath), reviewDirFlagHint)
			os.Exit(ExitFile)
			return nil
		}
	}

	if finalRound && round < 2 {
		output.Error(ExitUsage, "--final-round requires --round >= 2")
		os.Exit(ExitUsage)
		return nil
	}

	if finalRound && len(affectedFiles) == 0 {
		output.Info("final-round without affected-files: agents won't read code")
	}

	if diffFrom != "" {
		if _, err := os.Stat(diffFrom); os.IsNotExist(err) {
			// --diff-from takes a spec, so it draws the spec-path hint, not the
			// code-3 default's task-file advice.
			output.Error(ExitFile, fmt.Sprintf("diff baseline not found: %s", diffFrom), specFileMissingHint)
			os.Exit(ExitFile)
			return nil
		}
	}

	affectedFiles = engine.DedupPaths(affectedFiles)
	for _, f := range affectedFiles {
		info, err := os.Stat(f)
		if err != nil {
			output.Error(ExitFile, fmt.Sprintf("affected file not found: %s", f), affectedFilesHint)
			os.Exit(ExitFile)
			return nil
		}
		if info.IsDir() {
			output.Error(ExitFile, fmt.Sprintf("affected path is a directory, not a file: %s", f), affectedFilesHint)
			os.Exit(ExitFile)
			return nil
		}
	}
	return affectedFiles
}

const findingFormat = `
For each issue found, respond with one JSON object per line (NDJSON):
{"severity":"critical|high|medium|low","category":"completeness|ambiguity|consistency|feasibility|redundancy|regression","location":"section heading or line number","finding":"what is wrong","suggestion":"how to fix it"}

Optional "class" field: add "class":"<kebab-case-slug>" (example: "code-citation-drift") when the finding is an instance of a pattern a script could check across the whole corpus; omit it otherwise.

Only report real issues. Do not generate findings just to appear thorough.`

const specOnlyDisclaimer = `
IMPORTANT: This is a SPEC REVIEW. Review ONLY the spec document text.
Do NOT check implementation code or report "not implemented" findings.
Focus on: completeness, ambiguity, contradictions, missing edge cases, testability.
`

func appendAffectedChecklist(b *strings.Builder, n int, hasAffectedFiles bool) {
	if hasAffectedFiles {
		fmt.Fprintf(b, "%d. For each state-dependent behavior in the affected files (disabled, loading, visibility, conditional rendering, error handling), verify the spec addresses it. What controls each condition?\n", n+1)
	}
}

func appendFinalRoundInstruction(b *strings.Builder) {
	b.WriteString("\nMANDATORY: Read every file in the Affected Files section line-by-line. For each state-dependent behavior (disabled, loading, conditional rendering, class binding, error handling), verify the spec explicitly addresses it. Do NOT report \"spec is solid\" unless you have verified every state-dependent element.\n")
}

// runReviewCodeAudit emits the single-pass code-audit perspective prompt.
func runReviewCodeAudit(specPath, specContent string, affectedFiles []string, round int, q roleQuery) error {
	affectedContent := engine.ReadAffectedFiles(affectedFiles)
	summary := engine.BuildAffectedSummary(affectedFiles, affectedContent)
	prompt := generateCodeAuditPrompt(specContent, affectedContent)
	selected := filterReviewPrompts([]reviewPrompt{prompt}, q, nil)
	return output.JSON(reviewResult{
		Spec:            specPath,
		Perspective:     "code-audit",
		AffectedFiles:   affectedFiles,
		AffectedSummary: summary,
		Prompts:         selected,
		ReviewLoop: reviewLoop{
			Round:            round,
			Convergence:      "single-pass code audit",
			PreviousFindings: 0,
			Instruction:      instructionForPayload("Spawn a sub-agent with this prompt. Collect NDJSON findings. Feed findings back into spec revision or task acceptance updates.", len(selected)),
		},
	})
}

// runReviewDocPlan emits the single-pass documentation-plan perspective prompt.
func runReviewDocPlan(specPath, specContent, docsPath string, affectedFiles []string, q roleQuery) error {
	structureMap, files := walkDocTree(docsPath, ".md")
	ranked := rankFilesBySpecTerms(files, strings.Split(specContent, "\n"))
	docContent := readFilesContent(ranked, 30000)
	if len(affectedFiles) > 0 {
		maps.Copy(docContent, engine.ReadAffectedFiles(affectedFiles))
	}
	prompt := generateDocPlanPrompt(specContent, structureMap, docContent)
	selected := filterReviewPrompts([]reviewPrompt{prompt}, q, nil)
	return output.JSON(reviewResult{
		Spec:            specPath,
		Perspective:     "documentation",
		DocsPath:        docsPath,
		AffectedFiles:   affectedFiles,
		AffectedSummary: engine.BuildAffectedSummary(affectedFiles, nil),
		DocsStructure:   &docStructure{TotalFiles: len(files), ReviewedFiles: len(ranked), StructureMap: structureMap},
		Prompts:         selected,
		ReviewLoop: reviewLoop{
			Round:            1,
			Convergence:      "single-pass plan generation",
			PreviousFindings: 0,
			Instruction:      instructionForPayload("Spawn a sub-agent with this prompt. Collect the NDJSON plan. Review the plan for completeness, then append the plan to the spec.", len(selected)),
		},
	})
}

// runReviewTestPlan emits the single-pass test-plan perspective prompt.
func runReviewTestPlan(specPath, specContent, testPath string, affectedFiles []string, q roleQuery) error {
	structureMap, files := walkDocTree(testPath, "_test.go")
	ranked := rankFilesBySpecTerms(files, strings.Split(specContent, "\n"))
	testContent := readFilesContent(ranked, 20000)
	if len(affectedFiles) > 0 {
		maps.Copy(testContent, engine.ReadAffectedFiles(affectedFiles))
	}
	prompt := generateTestPlanPrompt(specContent, structureMap, testContent)
	selected := filterReviewPrompts([]reviewPrompt{prompt}, q, nil)
	return output.JSON(reviewResult{
		Spec:            specPath,
		Perspective:     "testing",
		TestPath:        testPath,
		AffectedFiles:   affectedFiles,
		AffectedSummary: engine.BuildAffectedSummary(affectedFiles, nil),
		TestStructure:   &docStructure{TotalFiles: len(files), ReviewedFiles: len(ranked), StructureMap: structureMap},
		Prompts:         selected,
		ReviewLoop: reviewLoop{
			Round:            1,
			Convergence:      "single-pass plan generation",
			PreviousFindings: 0,
			Instruction:      instructionForPayload("Spawn a sub-agent with this prompt. Collect the NDJSON plan. Review the plan for completeness, then append the plan to the spec.", len(selected)),
		},
	})
}

// buildReviewPrompts emits the round's review prompts: one per active reviewer
// role from the domain-filtered corpus with the spec-frontmatter overrides
// applied, plus the appended changed-sections block, the auto-included regression
// prompt (round >= 2 with a diff or fixed findings), and the mechanized-class
// exclusion. Returns the prompts and whether the regression prompt was included.
func buildReviewPrompts(specPath string, panel *rolePanel, elems *engine.StructuredElements, specContent string, round int, summary string, affectedFiles []string, finalRound bool, wfChecks *model.Workflow, diffFrom string, noState bool, reviewSt *engine.ReviewState) (prompts []reviewPrompt, regressionIncluded bool, skipped []engine.SkippedRole) {
	// The panel — corpus resolution (§7.1), override layering (§10.2-10.4),
	// the §2.3 drop and the §2.5 refusals — is resolved by the caller, ahead of
	// every write the emission path performs (§2.5 item 2).
	activeRoles := panel.roles

	prompts = make([]reviewPrompt, 0, len(activeRoles)+1)
	for i := range activeRoles {
		prompts = append(prompts, generateCorpusReviewPrompt(&activeRoles[i], elems, specContent, round, summary, len(affectedFiles) > 0, finalRound))
	}

	// Changed-sections block: explicit --diff-from overrides the baseline and
	// forces the block at any round; otherwise the newest earlier snapshot is
	// the baseline from round 2 on.
	diffBlock := ""
	var diffDr engine.DiffResult
	diffLabel := ""
	baselinePath := ""
	switch {
	case diffFrom != "":
		diffDr = engine.DiffSections(diffLinesOf(diffFrom), diffLinesOf(specPath))
		diffLabel = "baseline " + diffFrom
		diffBlock = buildChangedSectionsBlock(&diffDr, diffLabel)
		baselinePath = diffFrom
	case !noState && round >= 2:
		if snapRound, snapPath := newestEarlierSnapshot(specPath, round); snapPath != "" {
			diffDr = engine.DiffSections(diffLinesOf(snapPath), diffLinesOf(specPath))
			diffLabel = fmt.Sprintf("round %d", snapRound)
			diffBlock = buildChangedSectionsBlock(&diffDr, diffLabel)
			baselinePath = snapPath
		} else {
			output.Info("no earlier snapshot exists; changed-sections block omitted")
		}
	}
	// §9.1: under explicit --diff-from, a reviewer role whose focus is scoped
	// entirely (via §N.M references) to unchanged sections is skipped with reason
	// no-spec-change instead of being emitted. Generic focus always emits — the
	// reviewer self-scopes against the changed-sections block below. This never
	// fires for the snapshot-based auto-diff (round >= 2), only explicit --diff-from.
	if diffFrom != "" {
		filtered := make([]reviewPrompt, 0, len(prompts))
		for i := range activeRoles {
			if engine.RoleFocusOutsideDiff(&activeRoles[i], diffDr) {
				skipped = append(skipped, engine.SkippedRole{Role: activeRoles[i].ID, Reason: engine.SkipNoSpecChange})
				continue
			}
			filtered = append(filtered, prompts[i])
		}
		prompts = filtered
	}
	if diffBlock != "" {
		for i := range prompts {
			prompts[i].Prompt += diffBlock
		}
	}

	// Auto-append the regression prompt as a 4th entry when the round has
	// something to guard: a non-empty diff or at least one fixed finding.
	if !noState && round >= 2 {
		fixed := collectFixedFindings(specPath, reviewSt)
		if diffBlock != "" || len(fixed) > 0 {
			if len(fixed) > regressionFixedFindingsCap {
				fixed = fixed[:regressionFixedFindingsCap]
			}
			prompts = append(prompts, reviewPrompt{
				Role:     "regression",
				Category: "regression",
				Prompt:   buildRegressionPrompt(&diffDr, diffLabel, baselinePath, fixed),
			})
			regressionIncluded = true
		}
	}

	// Prompt exclusion: reviewers stop looking for mechanized classes (§3.2).
	// engine.ReviewerExclusionClasses applies the membership rule — drop the
	// entries the validator rejects, drop over-specification under §3.1's
	// exemption, then collapse duplicates keeping the first survivor, in that
	// order and registration order otherwise. Its own guard is the emptiness of
	// that result, not len(wfChecks.Checks): a workflow whose every entry is
	// invalid still runs its checks above and still emits each entry's skip
	// notice, while no sentence is appended here rather than one ending in an
	// empty list. The review_loop addendum about failing checks keeps its own
	// guard (buildReviewLoopInstruction) and is unchanged.
	if classes := engine.ReviewerExclusionClasses(wfChecks.Checks); len(classes) > 0 {
		exclusion := mechanizedExclusionPrefix + strings.Join(classes, ", ")
		for i := range prompts {
			prompts[i].Prompt += exclusion
		}
	}

	// §9.1: name every non-emitted corpus role. Domain filtering (when a user
	// corpus is present) drops roles whose domains omit the spec's domain; the
	// built-in regression role has no snapshot-round-0.md to diff at round 1.
	skipped = append(skipped, engine.DomainSkippedRoles(filepath.Dir(specPath), panel.fm.Domain, engine.PhaseReviewers)...)
	if !noState && round < 2 {
		skipped = append(skipped, engine.SkippedRole{Role: engine.RegressionRoleID, Reason: engine.SkipNoBaseline})
	}
	// §2.4: a role this spec deactivated with enabled: false is named too, so
	// the drop is visible rather than silent.
	skipped = append(skipped, engine.DisabledSkippedRoles(panel.disabled)...)

	// §10.4–§10.7: wrap every emitted prompt in tp-owned framing — the output
	// file, the reset discipline, the loop budget, and the file-reading
	// statement. Per-role inlining (§10.7): the first emitted role whose
	// affected files fit whole under the per-role reading budget inlines them
	// (complete and authoritative); every later role, and all roles when the
	// files exceed the budget, get named paths only. The regression role
	// carries no source files (it guards spec decisions, not code).
	consecutiveClean := 0
	if reviewSt != nil {
		consecutiveClean = engine.ReviewConsecutiveClean(specPath, reviewSt.ReviewRounds, wfChecks.ReviewConvergeOn)
	}
	// Budget first, content second: stat the set and read it only when it fits,
	// so an oversized caller-supplied --affected-files list is never
	// materialized in memory for nothing. This mirrors the audit path.
	// A path tp cannot read is never presented as complete (§10.7) — a role that
	// trusts a silently-missing body reports on a file it never saw — but only
	// that path is withheld: the files tp did read still travel with the prompt
	// under the "(incomplete)" header, and the role is told to read the rest
	// itself. Discarding the whole section instead would make every role reread
	// files tp already had in hand.
	contentFits := len(affectedFiles) > 0 && fileSetBytes(affectedFiles) <= perRoleReadingBudget
	affectedContent := ""
	unreadable := make([]string, 0)
	if contentFits {
		affectedContent, unreadable = fileSetRead(affectedFiles)
	}
	inlinerDone := false
	for i := range prompts {
		outputPath := roleOutputPath("review", round, prompts[i].Role)
		prompts[i].OutputPath = outputPath
		f := promptFraming{
			phase:            "review",
			round:            round,
			requiredClean:    wfChecks.ReviewCleanRounds,
			consecutiveClean: consecutiveClean,
			maxRounds:        wfChecks.ReviewMaxRounds,
			outputPath:       outputPath,
			hasFiles:         len(affectedFiles) > 0 && prompts[i].Role != engine.RegressionRoleID,
		}
		if f.hasFiles {
			if !inlinerDone && contentFits {
				prompts[i].Prompt += "\n\n" + affectedContent
				f.filesComplete = len(unreadable) == 0
				f.filesPartial = !f.filesComplete
				f.filePaths = unreadable
				inlinerDone = true
			} else {
				f.filePaths = affectedFiles
			}
		}
		prompts[i].Prompt += renderFraming(&f)
	}

	return prompts, regressionIncluded, skipped
}

// generateCorpusReviewPrompt renders one review prompt for a corpus role,
// assembling the role's instructions (persona) and focus questions with the
// shared, role-neutral scaffolding: the spec-only disclaimer, the previous-round
// findings summary, the spec content, the structured-element inventory, the
// affected-files checklist, and the finding format. It replaces the pre-v0.25.0
// hardcoded implementer/tester/architect generators (§7.1); the role's failure
// lens now comes entirely from its instructions and focus, not from Go.
func generateCorpusReviewPrompt(role *model.Role, elems *engine.StructuredElements, specContent string, round int, summary string, hasAffected, finalRound bool) reviewPrompt {
	var b strings.Builder
	if round >= 2 {
		fmt.Fprintf(&b, "%s This is review round %d — focus ONLY on issues not previously reported.\n\n", role.Instructions, round)
	} else {
		b.WriteString(role.Instructions + "\n\n")
	}
	if !hasAffected {
		b.WriteString(specOnlyDisclaimer)
	}
	if summary != "" {
		b.WriteString(summary)
		b.WriteString("\n")
	}
	b.WriteString("Spec content:\n---\n")
	b.WriteString(specContent)
	b.WriteString("\n---\n\n")
	b.WriteString("Check each of these specifically:\n")

	n := 1
	for _, t := range elems.Tables {
		fmt.Fprintf(&b, "%d. Table '%s' (line %d, %d rows): apply your review lens to each row.\n", n, t.Heading, t.Line, t.Rows)
		n++
	}
	for _, nl := range elems.NumberedLists {
		fmt.Fprintf(&b, "%d. List '%s' (line %d, %d items, #1-#%d): apply your review lens to each item.\n", n, nl.Heading, nl.Line, nl.Items, nl.LastNum)
		n++
	}
	for _, q := range role.Focus {
		fmt.Fprintf(&b, "%d. %s\n", n, q)
		n++
	}
	appendAffectedChecklist(&b, n, hasAffected)

	if finalRound {
		appendFinalRoundInstruction(&b)
	}

	b.WriteString(findingFormat)
	if summary != "" {
		b.WriteString(findingFormatRound2)
	}
	b.WriteString(outputContractInstruction(role.ID, engine.PhaseReviewers))

	return reviewPrompt{
		Role:     role.ID,
		Category: role.ID,
		Prompt:   b.String(),
	}
}

// outputContractInstruction returns the §7.3 output-contract block for a phase,
// naming the role every finding must be stamped with (Principle 2 — tp owns the
// contract). Review findings carry role, location (a §<section> anchor per §8.2,
// which is what makes dedup and the overlap report possible), class, and
// severity; audit findings additionally carry status ∈ PASS/PARTIAL/FAIL, and
// take their severity vocabulary from the audit Output Schema (error|warning|
// info) rather than the review one — see the comment on the branch below.
func outputContractInstruction(role, phase string) string {
	var b strings.Builder
	b.WriteString("\n\nOutput contract — stamp EVERY finding with the full contract:\n")
	fmt.Fprintf(&b, "- role: %q (this prompt's role, so inter-role overlap can be attributed)\n", role)
	b.WriteString("- location: a section anchor such as \"§3.2\" — the first §<n>(.<n>)* token — so findings dedup by section\n")
	b.WriteString("- class: a kebab-case slug naming the failure class (the dedup/cluster key)\n")
	if phase == engine.PhaseReviewers {
		b.WriteString("  Canonical class `over-specification`: a detail whose correctness can only be established against code, prescribed in the spec where it belongs in task acceptance instead. Raise it (typically low/medium — an altitude smell, not a blocking defect) when the spec pins mechanism a task's acceptance should own.\n")
	}
	if phase == engine.PhaseAuditors {
		// The severity vocabularies differ by phase on purpose, and this
		// stamp lands right after the Output Schema block in every auditor
		// prompt — naming the review enum here contradicted that block in
		// the same prompt. An audit row states its verdict in `status`;
		// severity only qualifies a PARTIAL or FAIL, so it uses the audit
		// schema's error|warning|info. A review finding has no status, so
		// there severity IS the blocking predicate (review_converge_on
		// blocks on critical/high) and keeps the four-level scale.
		b.WriteString("- severity: one of error, warning, info — null for PASS (the audit vocabulary from the Output Schema above, not the review one)\n")
		b.WriteString("- status: one of PASS, PARTIAL, FAIL\n")
	} else {
		b.WriteString("- severity: one of critical, high, medium, low\n")
	}
	return b.String()
}

func generateCodeAuditPrompt(specContent string, affectedContent map[string]string) reviewPrompt {
	var b strings.Builder

	b.WriteString("You are a code auditor. You have a specification and the source files it claims to change. Your goal is to systematically compare code against spec and find state-dependent behaviors the spec doesn't address.\n\n")

	b.WriteString("Spec content:\n---\n")
	b.WriteString(specContent)
	b.WriteString("\n---\n\n")

	b.WriteString("## Affected Files\n\n")
	sorted := make([]string, 0, len(affectedContent))
	for p := range affectedContent {
		sorted = append(sorted, p)
	}
	sort.Strings(sorted)
	for _, p := range sorted {
		c := affectedContent[p]
		lineCount := strings.Count(c, "\n") + 1
		fmt.Fprintf(&b, "### %s (%d lines)\n", p, lineCount)
		b.WriteString(c)
		b.WriteString("\n\n")
	}

	b.WriteString(codeAuditChecklist)
	b.WriteString(codeAuditOutputFormat)

	return reviewPrompt{
		Role:     "code-auditor",
		Category: "completeness",
		Prompt:   b.String(),
	}
}

// parseFindingsFile reads an NDJSON findings file into review findings. A read
// error is propagated, never swallowed: an existing-but-unreadable file used to
// come back as an empty set, so tp exited 0 with empty stderr while every
// previously reported finding silently vanished from the emitted prompts. A
// path that does not exist stays a nil error — callers reject that case up
// front with their own usage error.
func parseFindingsFile(path string) ([]reviewFinding, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make([]reviewFinding, 0), nil
		}
		return nil, err
	}
	defer f.Close()

	findings := make([]reviewFinding, 0)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), ndjsonLineCap)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var finding reviewFinding
		if err := json.Unmarshal([]byte(line), &finding); err != nil {
			output.Notice(fmt.Sprintf("findings line %d: skipping invalid JSON", lineNum))
			continue
		}
		if finding.Severity == "" {
			finding.Severity = "unknown"
		}
		findings = append(findings, finding)
	}
	if err := scanner.Err(); err != nil {
		// Propagated, not warned: this function's contract is that a read error
		// never comes back as an empty set, and a scan that stopped early is
		// exactly that. mustParseFindingsFile turns it into the abort.
		return nil, err
	}
	return findings, nil
}

// mustParseFindingsFile reads a findings file or aborts with tp's file exit
// code, naming the path. Losing prior findings has to be loud: a round emitted
// from a silently empty set re-reports what was already fixed and forgets what
// was not.
func mustParseFindingsFile(path string) []reviewFinding {
	findings, err := parseFindingsFile(path)
	if err != nil {
		// The MESSAGE carries the cause, the HINT carries the repair. Moving
		// the error into the hint lost "permission denied" from the output
		// entirely and advised checking a path that was correct — the same
		// wrong-object defect the hint sweep set out to remove, reintroduced by
		// it. tp audit --findings names the cause for the same file.
		output.Error(ExitFile, fmt.Sprintf("cannot read findings file: %s: %v", path, err), ndjsonReadHint(err))
		os.Exit(ExitFile)
		return nil
	}
	return findings
}

const findingPrefixLen = 80

// findingIdentityKey returns a dedup key for a finding: (category, location, finding_prefix).
// finding_prefix is the first 80 characters of the finding field.
// Used by merge dedup and report cross-round tracking.
func findingIdentityKey(category, location, finding string) string {
	prefix := finding
	if len(prefix) > findingPrefixLen {
		prefix = prefix[:findingPrefixLen]
	}
	return category + "::" + location + "::" + prefix
}

// findingFromRow converts a recorded round row into a reviewFinding.
func findingFromRow(row map[string]any) reviewFinding {
	f := reviewFinding{}
	f.Severity, _ = row["severity"].(string)
	f.Category, _ = row["category"].(string)
	f.Class, _ = row["class"].(string)
	f.Location, _ = row["location"].(string)
	f.Finding, _ = row["finding"].(string)
	f.Suggestion, _ = row["suggestion"].(string)
	if resolved, ok := row["resolved"].(map[string]any); ok {
		rs := &resolvedStatus{}
		rs.Status, _ = resolved["status"].(string)
		rs.Evidence, _ = resolved["evidence"].(string)
		rs.ResolvedAt, _ = resolved["resolved_at"].(string)
		f.Resolved = rs
	}
	return f
}

func dedupFindings(findings []reviewFinding) []reviewFinding {
	seen := make(map[string]bool)
	result := make([]reviewFinding, 0, len(findings))
	for _, f := range findings {
		key := findingIdentityKey(f.Category, f.Location, f.Finding)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, f)
	}
	return result
}

func buildFindingsSummary(findings []reviewFinding) string {
	deduped := dedupFindings(findings)
	if len(deduped) == 0 {
		return ""
	}

	// Classify findings by resolution status
	var unresolved, wontfix []reviewFinding
	resolvedCount := 0

	for _, f := range deduped {
		if f.Resolved == nil {
			// No resolved field — treat as unresolved (backward compat)
			unresolved = append(unresolved, f)
			continue
		}
		switch f.Resolved.Status {
		case "fixed", "duplicate":
			resolvedCount++
		case "wontfix":
			wontfix = append(wontfix, f)
		default:
			unresolved = append(unresolved, f)
		}
	}

	// Combine unresolved + wontfix for the detailed listing
	detailed := make([]reviewFinding, 0, len(unresolved)+len(wontfix))
	detailed = append(detailed, unresolved...)
	detailed = append(detailed, wontfix...)

	severityOrder := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3, "unknown": 4}
	sort.SliceStable(detailed, func(i, j int) bool {
		si, sj := severityOrder[detailed[i].Severity], severityOrder[detailed[j].Severity]
		if si != sj {
			return si < sj
		}
		return detailed[i].Category < detailed[j].Category
	})

	sevAbbr := map[string]string{
		"critical": "[CRIT]", "high": "[HIGH]", "medium": "[MED]", "low": "[LOW]", "unknown": "[???]",
	}

	var b strings.Builder

	// Section 1: Unresolved + wontfix findings (full detail)
	if len(detailed) > 0 {
		b.WriteString("UNRESOLVED findings from previous rounds — DO NOT re-report:\n")

		findingsCap := 50
		shown := detailed
		omitted := 0
		if len(detailed) > findingsCap {
			shown = detailed[:findingsCap]
			omitted = len(detailed) - findingsCap
		}

		for _, f := range shown {
			abbr := sevAbbr[f.Severity]
			if abbr == "" {
				abbr = "[???]"
			}
			text := f.Finding
			if len(text) > 80 {
				text = text[:80] + "..."
			}
			if f.Resolved != nil && f.Resolved.Status == "wontfix" {
				evidence := f.Resolved.Evidence
				if len(evidence) > 40 {
					evidence = evidence[:40] + "..."
				}
				fmt.Fprintf(&b, "  [WONTFIX] %s — %s: %s (wontfix: %s)\n", f.Category, f.Location, text, evidence)
			} else {
				fmt.Fprintf(&b, "  %s %s — %s: %s\n", abbr, f.Category, f.Location, text)
			}
		}

		if omitted > 0 {
			fmt.Fprintf(&b, "\n  ... and %d more (omitted for brevity)\n", omitted)
		}
		b.WriteString("\n")
	}

	// Section 2: Resolved findings — show high/critical to prevent regression
	if resolvedCount > 0 {
		fmt.Fprintf(&b, "Additionally, %d findings from previous rounds were RESOLVED (fixed or duplicate).\n", resolvedCount)

		// List high/critical resolved findings briefly to prevent regression
		var highResolved []reviewFinding
		for _, f := range deduped {
			if f.Resolved != nil && (f.Resolved.Status == "fixed" || f.Resolved.Status == "duplicate") {
				if f.Severity == "critical" || f.Severity == "high" {
					highResolved = append(highResolved, f)
				}
			}
		}
		if len(highResolved) > 0 {
			b.WriteString("Resolved high/critical (DO NOT regress):\n")
			maxShow := min(len(highResolved), 10)
			for _, f := range highResolved[:maxShow] {
				text := f.Finding
				if len(text) > 60 {
					text = text[:60] + "..."
				}
				fmt.Fprintf(&b, "  [RESOLVED] %s: %s\n", f.Location, text)
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("Do not re-report resolved issues. Focus ONLY on NEW issues in the current spec.\n")
	return b.String()
}

const findingFormatRound2 = `
Remember: only report NEW issues not covered by the previous findings listed above.`

func walkDocTree(root, ext string) (tree string, files []string) {
	var allFiles []string

	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ext) {
			allFiles = append(allFiles, path)
		}
		return nil
	})

	if len(allFiles) == 0 {
		return filepath.Base(root) + "/\n  (empty)\n", allFiles
	}

	var b bytes.Buffer
	fmt.Fprintf(&b, "%s/\n", filepath.Base(root))
	for i, f := range allFiles {
		rel, _ := filepath.Rel(root, f)
		prefix := "  ├ "
		if i == len(allFiles)-1 {
			prefix = "  └ "
		}
		fmt.Fprintf(&b, "%s%s\n", prefix, rel)
	}

	return b.String(), allFiles
}

func rankFilesBySpecTerms(files, specLines []string) []string {
	// maxCount caps how many ranked files a perspective prompt carries.
	const maxCount = 15

	if len(files) == 0 {
		return files
	}

	terms := make(map[string]bool)
	for _, line := range specLines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
			term := strings.TrimLeft(trimmed, "# ")
			term = strings.TrimSpace(term)
			words := strings.FieldsSeq(term)
			for w := range words {
				if len(w) > 3 {
					terms[strings.ToLower(w)] = true
				}
			}
		}
	}

	type scored struct {
		path  string
		score int
	}
	var scoredFiles []scored

	alwaysInclude := func(f string) bool {
		base := filepath.Base(f)
		if base == "index.md" {
			return true
		}
		lower := strings.ToLower(base)
		return slices.Contains([]string{"config.js", "config.ts", "config.mts", "config.mjs"}, lower)
	}

	always := make([]string, 0)
	var rankable []string
	for _, f := range files {
		if alwaysInclude(f) {
			always = append(always, f)
		} else {
			rankable = append(rankable, f)
		}
	}

	if len(terms) == 0 {
		result := append([]string(nil), always...)
		result = append(result, rankable...)
		if len(result) > maxCount {
			result = result[:maxCount]
		}
		return result
	}

	for _, f := range rankable {
		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		lower := strings.ToLower(string(content))
		score := 0
		for term := range terms {
			if strings.Contains(lower, term) {
				score++
			}
		}
		scoredFiles = append(scoredFiles, scored{path: f, score: score})
	}

	sort.SliceStable(scoredFiles, func(i, j int) bool {
		return scoredFiles[i].score > scoredFiles[j].score
	})

	result := make([]string, 0, len(always)+len(scoredFiles))
	result = append(result, always...)
	for _, sf := range scoredFiles {
		result = append(result, sf.path)
		if len(result) >= maxCount {
			break
		}
	}

	return result
}

// readFilesContent reads the docs/test files selected for a perspective
// prompt. A file it cannot read is named on stderr rather than dropped in
// silence: the caller's paths came from a directory walk, so an unreadable one
// is an anomaly, and a role that never sees the body would otherwise judge the
// file from its absence.
func readFilesContent(files []string, maxTotal int) map[string]string {
	// maxPerFile truncates one file; maxTotal caps the prompt and differs per
	// perspective, so it stays a parameter.
	const maxPerFile = 5000

	result := make(map[string]string)
	total := 0
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			output.Notice(fmt.Sprintf("warning: cannot read %s; its contents were dropped from the prompt (%v)", f, err))
			continue
		}
		s := string(content)
		if len(s) > maxPerFile {
			s = s[:maxPerFile] + "\n[...truncated]"
		}
		if total+len(s) > maxTotal {
			remaining := maxTotal - total
			if remaining > 100 {
				s = s[:remaining] + "\n[...truncated by total cap]"
				result[f] = s
			}
			break
		}
		result[f] = s
		total += len(s)
	}
	return result
}

// generatePlanPrompt assembles a planning prompt. The documentation and test
// planners differ only in the strings passed here; the assembly is shared.
func generatePlanPrompt(role, intro, structureLabel, filesLabel, checklist, outputFormat string,
	specContent, structureMap string, fileContents map[string]string,
) reviewPrompt {
	var b strings.Builder

	b.WriteString(intro)
	b.WriteString(specOnlyDisclaimer)
	b.WriteString("Spec content:\n---\n")
	b.WriteString(specContent)
	b.WriteString("\n---\n\n")

	b.WriteString(structureLabel)
	b.WriteString(structureMap)
	b.WriteString("\n")

	if len(fileContents) > 0 {
		b.WriteString(filesLabel)
		sortedPaths := make([]string, 0, len(fileContents))
		for p := range fileContents {
			sortedPaths = append(sortedPaths, p)
		}
		sort.Strings(sortedPaths)
		for _, path := range sortedPaths {
			fmt.Fprintf(&b, "--- FILE: %s ---\n", path)
			b.WriteString(fileContents[path])
			b.WriteString("\n--- END FILE ---\n\n")
		}
	}

	b.WriteString(checklist)
	b.WriteString(outputFormat)

	return reviewPrompt{
		Role:     role,
		Category: "completeness",
		Prompt:   b.String(),
	}
}

func generateDocPlanPrompt(specContent, structureMap string, fileContents map[string]string) reviewPrompt {
	return generatePlanPrompt(
		"documentation-planner",
		"You are a technical writer planning documentation changes for a new feature. Your goal is to compare the specification against existing documentation and produce a structured plan of changes needed.\n\n",
		"Documentation structure:\n",
		"Existing documentation files (selected by relevance):\n",
		docChecklist,
		docOutputFormat,
		specContent, structureMap, fileContents,
	)
}

func generateTestPlanPrompt(specContent, structureMap string, fileContents map[string]string) reviewPrompt {
	return generatePlanPrompt(
		"test-planner",
		"You are a QA engineer planning test coverage for a new feature. Your goal is to analyze the specification and produce a structured test plan covering all requirements.\n\n",
		"Test structure:\n",
		"Existing test files (selected by relevance):\n",
		testChecklist,
		testOutputFormat,
		specContent, structureMap, fileContents,
	)
}

const docChecklist = `
Analyze the spec against the existing documentation. For each step below,
if an issue or need is found, produce a plan item.

A1. COMPLETENESS — Spec-to-Doc Coverage
For each ## and ### heading in the spec:
- Does a corresponding doc page or section exist?
- If no -> plan "create-page" or "update-section"

A2. TERM COVERAGE — Commands, Flags, Config Keys
For each command name, CLI flag, config key, type name, or new concept in the spec:
- Is it documented anywhere in the existing docs?
- If no -> plan "update-section" in the page covering the same domain

A3. DRIFT — Accuracy of Existing Content
For each existing doc page that covers the same domain as the spec:
- Does any statement contradict the spec?
- Are code examples still valid per the spec?
- If yes -> plan "fix-drift"

A4. NAVIGATION — Structure and Discovery
- Does each new or updated page have a place in the navigation structure?
- Should any index page be updated to reference the new content?
- If yes -> plan "update-config" or "update-index"

A5. CROSS-REFERENCES — Link Integrity
- Do related existing pages reference the new feature area?
- Does the new content reference back to related pages?
- If no -> plan "add-crossref"
`

const docOutputFormat = `
Output format — respond with one JSON object per line (NDJSON):
{"id":"doc-001","action":"create-page|update-section|fix-drift|update-config|add-crossref|update-index","file":"path/to/file.md","location":"section heading or null","spec_ref":"spec section reference","description":"what needs to change","detail":{},"priority":"must|should|could","depends_on":["doc-000"]}

Action types:
- create-page: new documentation file needed
- update-section: add content to existing page
- fix-drift: correct existing content contradicting spec
- update-config: update navigation/sidebar config
- add-crossref: add links between pages
- update-index: update index/landing page

Priority: must (incorrect without it), should (significantly improves quality), could (nice to have)

Only produce plan items for real changes needed. If no changes needed, respond with an empty array (just []).
`

const testChecklist = `
Analyze the spec and plan the test coverage needed. For each step below,
if a test is needed, produce a plan item.

T1. ACCEPTANCE CRITERIA COVERAGE
For each numbered list item (acceptance criteria) in the spec:
- Can this be verified with a test?
- What test function would verify it?

T2. TABLE ROW COVERAGE
For each row in each table in the spec:
- Is there a test that exercises this row's behavior?

T3. HAPPY PATH
- What is the primary use case?
- Is it covered by existing tests or does it need a new test?

T4. ERROR PATHS
- What can go wrong? File not found, invalid input, permission denied, empty input, etc.

T5. EDGE CASES
- Boundary conditions (empty, single item, very large input)
- Zero-value / nil / default behavior

T6. INTEGRATION POINTS
- Does the feature interact with other features or external systems?

T7. FIXTURE / HELPER NEEDS
- Are new test fixtures, mock data, or helper functions needed?
`

const testOutputFormat = `
Output format — respond with one JSON object per line (NDJSON):
{"id":"test-001","action":"create-test|update-test|create-fixture","file":"path/to/test_file","location":"TestFunctionName","spec_ref":"spec section reference","description":"what to test","detail":{"type":"unit|integration","assertions":["expected behavior 1","expected behavior 2"],"inputs":{}},"priority":"must|should|could","depends_on":["test-000"]}

Action types:
- create-test: new test function needed
- update-test: existing test needs modification
- create-fixture: new test data or helper needed

Priority: must (critical path), should (important coverage), could (nice to have)

Only produce plan items for real tests needed. If no tests needed, respond with an empty array (just []).
`

const codeAuditChecklist = `
For EACH affected file, perform these steps in order:

C1. STATE-DEPENDENT BEHAVIORS
List every state-dependent behavior:
- Conditional disabling: :disabled, disabled, aria-disabled
- Loading states: :loading, loading, spinner, skeleton
- Conditional visibility: v-if, v-show, hidden, display:none, :class with conditions
- Error/success states: error messages, success indicators, color changes
- Computed/derived state: values that depend on other state
For each: what controls it? What are ALL the conditions?

C2. SPEC COVERAGE
For each state-dependent behavior from C1:
- Does the spec mention this element or condition?
- If the spec changes this behavior, does it describe the FINAL state?
- If the spec removes a condition, does it verify no OTHER code still references it?

C3. REMOVAL REACH
For each removal described in the spec:
- Search the affected files for ALL references to the removed item
- List every reference found
- Does the spec account for each reference?

C4. ACCEPTANCE COMPLETENESS
For each state-dependent behavior from C1:
- What should the acceptance criteria say?
- Format: "element X shows Y behavior when Z condition, no other condition"
- Does any task's acceptance describe this final state?

C5. SIDE EFFECTS
- Could implementing this spec cause unintended behavior changes?
- Are there shared state dependencies across affected files?
- Are there implicit contracts that could break?
`

const codeAuditOutputFormat = `
Output format — respond with one JSON object per line (NDJSON):
{"role":"code-audit","id":"ca-001","file":"path/to/file","line":42,"pattern":":disabled","current_behavior":"isFormLocked || isPhoneCheckInProgress","spec_coverage":"partial","finding":"spec removes isPhoneCheckInProgress but phone input still references it","suggestion":"Add acceptance: phone input :disabled only when isFormLocked","severity":"high","category":"gap"}

Stamp EVERY finding with role: "code-audit" (so the finding is attributed to this perspective in the per-role overlap report).
Severity: critical, high, medium, low
Category: gap, drift, side-effect, removal
spec_coverage: missing, partial, full

Only report real issues. If no issues found, respond with an empty array (just []).
`

func buildDiffSpecContent(diff *engine.DiffResult) string {
	var b strings.Builder

	if len(diff.Changed) > 0 {
		b.WriteString("## Changed Sections (review carefully)\n\n")
		for _, s := range diff.Changed {
			hashes := strings.Repeat("#", s.Level)
			fmt.Fprintf(&b, "%s %s (%s)\n", hashes, s.Heading, s.Status)
			b.WriteString(s.Content)
			b.WriteString("\n\n")
		}
	}

	if len(diff.Removed) > 0 {
		b.WriteString("## Removed Sections\n\n")
		for _, s := range diff.Removed {
			fmt.Fprintf(&b, "- \"%s\" was removed from the spec.\n", s.Heading)
		}
		b.WriteString("\n")
	}

	if len(diff.Unchanged) > 0 {
		b.WriteString("## Unchanged Sections (review only if interacting with changes)\n\n")
		for _, s := range diff.Unchanged {
			fmt.Fprintf(&b, "- %s\n", s.Heading)
		}
		b.WriteString("\n")
	}

	content := b.String()
	if len(content) > specContentCap {
		content = content[:specContentCap] + "\n[...diff truncated]"
	}
	return content
}

func buildSpecRefContent(absPath string, lineCount int, headings []*engine.Heading) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Spec file: %s (%d lines, %d sections)\n\n", absPath, lineCount, len(headings))
	b.WriteString("Read the spec file before reviewing. The spec is NOT included inline to save context.\n")
	b.WriteString("Focus your review on:\n")
	for _, h := range headings {
		indent := strings.Repeat("  ", h.Level-1)
		fmt.Fprintf(&b, "%s- %s\n", indent, h.Text)
	}
	return b.String()
}

// readSpecContent reads a spec for inline embedding (--spec-inline). A read
// error is propagated, never swallowed: an empty return reached the prompt as a
// legitimately empty spec, so `tp review nope.md --spec-inline` exited 0 with an
// empty "Spec content:" block while the same call WITHOUT --spec-inline exited
// 3. Same class as fileSetRead and parseFindingsFile before it.
//
// It reads the whole file the way the --diff-from branch above it does, rather
// than scanning lines: the scanner warned on a line over its 64KB cap and
// returned what it had, so `--spec-inline` on a spec with one long line exited
// 0 emitting a spec whose tail was silently absent — the swallowed read this
// contract rules out, arriving through the line cap instead of the open.
func readSpecContent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	lines := engine.BlankFrontmatterLines(strings.Split(string(data), "\n"))
	content := strings.Join(lines, "\n")
	if len(content) > specContentCap {
		content = content[:specContentCap] + fmt.Sprintf("\n[...truncated at %d chars]", specContentCap)
	}
	return content, nil
}

// resolveReviewSpecContent builds the spec-content block for a review prompt in
// the selected mode: diff-based (--diff-from), inline (--spec-inline), or the
// default reference mode. It os.Exit()s with a file error on a read failure.
func resolveReviewSpecContent(specPath, diffFrom string, specInline bool) string {
	switch {
	case diffFrom != "":
		baseData, err := os.ReadFile(diffFrom)
		if err != nil {
			output.Error(ExitFile, fmt.Sprintf("cannot read diff baseline: %s", diffFrom), specFileMissingHint)
			os.Exit(ExitFile)
			return ""
		}
		currData, err := os.ReadFile(specPath)
		if err != nil {
			output.Error(ExitFile, fmt.Sprintf("cannot read spec: %s", specPath), specFileMissingHint)
			os.Exit(ExitFile)
			return ""
		}
		dr := engine.DiffSections(engine.BlankFrontmatterLines(strings.Split(string(baseData), "\n")), engine.BlankFrontmatterLines(strings.Split(string(currData), "\n")))
		content := buildDiffSpecContent(&dr)
		if len(dr.Changed) == 0 && len(dr.Removed) == 0 {
			output.Info("no changes detected between baseline and current spec — review may be unnecessary")
		}
		return content
	case specInline:
		content, err := readSpecContent(specPath)
		if err != nil {
			output.Error(ExitFile, fmt.Sprintf("cannot read spec: %s", specPath), specFileMissingHint)
			os.Exit(ExitFile)
			return ""
		}
		return content
	default:
		// Default: reference mode (spec-ref) — omit inline content.
		// PRE-stat site: unlike tp audit (whose os.Stat guard runs first, so
		// loadAuditSpec's read is post-stat and reports the I/O cause), nothing
		// on the review path touches specPath before this read. It is tp's
		// FIRST contact with the path, so specFileMissingHint is the right hint
		// per its doc comment — a permission failure and a typo are not
		// distinguishable here, and the hint deliberately does not guess why.
		specData, err := os.ReadFile(specPath)
		if err != nil {
			output.Error(ExitFile, fmt.Sprintf("cannot read spec: %s", specPath), specFileMissingHint)
			os.Exit(ExitFile)
			return ""
		}
		lineCount := strings.Count(string(specData), "\n") + 1
		absPath, _ := filepath.Abs(specPath)
		headings, _ := engine.ParseHeadings(specPath)
		return buildSpecRefContent(absPath, lineCount, headings)
	}
}

// reviewRoundState bundles the outputs of the state-backed round lifecycle.
type reviewRoundState struct {
	round        int
	st           *engine.ReviewState
	required     *int
	consecutive  *int
	converged    *bool
	stale        *bool
	prevFindings []reviewFinding
}

// loadReviewRoundState runs the state-backed round lifecycle for the default
// review path: it derives the round number from recorded state, snapshots the
// spec, computes the review_loop convergence fields, and gathers previous
// findings from rounds 1..R-1. It os.Exit()s on a state or IO error.
func loadReviewRoundState(cmd *cobra.Command, specPath string, round int, findingsPath string) reviewRoundState {
	statePrevFindings := make([]reviewFinding, 0)
	st, stErr := engine.LoadReviewState(specPath)
	if stErr != nil {
		// Same snapshot-only window tp audit's emission opens; see
		// runReviewStatus. Genuine corruption still aborts.
		if !engine.IsRebuildableStateIndex(stErr) {
			exitStateError(stErr)
			return reviewRoundState{}
		}
		st = nil
	}
	recorded := 0
	if st != nil {
		recorded = len(st.ReviewRounds)
	}
	stateRound := recorded + 1
	if cmd.Flags().Changed("round") && round != stateRound {
		output.Error(ExitUsage, fmt.Sprintf("--round %d conflicts with the state-derived round %d", round, stateRound), "drop --round, or use --no-state for manual round numbering")
		os.Exit(ExitUsage)
		return reviewRoundState{}
	}
	round = stateRound

	if _, err := engine.EnsureReviewState(specPath); err != nil {
		exitStateError(err)
		return reviewRoundState{}
	}
	specBytes, readErr := os.ReadFile(specPath)
	if readErr != nil {
		// POST-read failure: resolveReviewSpecContent already read this same
		// path above, so the hint carries the real cause — not
		// specFileMissingHint (the caller did not mistype), and not the code-3
		// task-file default a hintless site would inherit.
		output.Error(ExitFile, fmt.Sprintf("cannot read spec: %s", specPath), readErr.Error())
		os.Exit(ExitFile)
		return reviewRoundState{}
	}
	// §10.2: snapshot the spec at round start (prompt emission) atomically —
	// write to snapshot-round-N.md.tmp then rename — so a partial snapshot is
	// never left on disk.
	if writeErr := engine.WriteSnapshotAtomic(specPath, engine.PhaseReview, stateRound, specBytes); writeErr != nil {
		// POST-read failure: resolveReviewSpecContent already read this same
		// path, and what failed is a state-directory write. The hint carries the
		// real cause — not specFileMissingHint (the caller did not mistype), and
		// not the code-3 task-file default a hintless site would inherit.
		output.Error(ExitFile, fmt.Sprintf("cannot write review round snapshot for %s", specPath), writeErr.Error())
		os.Exit(ExitFile)
		return reviewRoundState{}
	}

	rs := reviewRoundState{round: round, st: st, prevFindings: statePrevFindings}
	// State-derived review_loop fields
	wfState, _ := engine.ResolveWorkflow(specPath, flagFile)
	if specHash, hashErr := engine.SpecHash(specPath); hashErr == nil {
		rounds := []engine.ReviewRound{}
		if st != nil {
			rounds = st.ReviewRounds
		}
		req := wfState.ReviewCleanRounds
		cc := engine.ReviewConsecutiveClean(specPath, rounds, wfState.ReviewConvergeOn)
		conv := engine.ReviewConverged(specPath, rounds, req, specHash, wfState.ReviewConvergeOn)
		stale := engine.StateStale(rounds, specHash)
		rs.required, rs.consecutive, rs.converged, rs.stale = &req, &cc, &conv, &stale
	}

	// Previous findings from rounds 1..R-1 unless --findings overrides
	if findingsPath == "" && st != nil {
		for i := range st.ReviewRounds {
			r := st.ReviewRounds[i]
			rows, found := engine.LoadRoundRows(specPath, &r)
			if !found {
				output.Notice(fmt.Sprintf("round %d file %s is missing; skipping its rows", r.Round, r.File))
				continue
			}
			for _, row := range rows {
				statePrevFindings = append(statePrevFindings, findingFromRow(row))
			}
		}
		rs.prevFindings = dedupFindings(statePrevFindings)
	}
	return rs
}

// buildReviewLoopInstruction builds the review_loop convergence description and
// the agent instruction for the default review path, adapting to the round
// number, --no-state mode, an included regression prompt, and registered
// mechanical checks.
func buildReviewLoopInstruction(round int, findings []reviewFinding, findingsPath, specPath string, specInline, noState bool, stateRequired *int, regressionIncluded, hasChecks bool) (convergence, instruction string) {
	// blockingRule states the stop rule for BOTH review_converge_on settings
	// statically, in one string, so no prompt claims a rule the code does not
	// enforce and the prompt-generation path never reads review_converge_on
	// (§9.4; §3.3 enumerates the field's readers as tp review --status/--record).
	const blockingRule = "critical or high severity under review_converge_on=blocking, any severity under all"

	instruction = "For each prompt, spawn a sub-agent via the Agent tool. Collect JSON findings. If a surviving finding blocks convergence (" + blockingRule + "), revise spec and re-run `tp review`. Stop after 2 rounds or when no convergence-blocking finding survives."
	if round < 2 && len(findings) > 0 {
		instruction = fmt.Sprintf("For each prompt, spawn a sub-agent via the Agent tool. Collect JSON findings. If a surviving finding blocks convergence (%s), revise spec and re-run `tp review --round 2 --findings <%s>`. Stop after 2 rounds or when no convergence-blocking finding survives.", blockingRule, findingsPath)
	} else if round >= 2 {
		instruction = fmt.Sprintf("Spawn sub-agents for each prompt. Collect findings. If a surviving finding blocks convergence (%s), revise spec and re-run `tp review --round %d --findings <combined.ndjson>`. Stop after max_rounds or when no convergence-blocking finding survives.", blockingRule, round+1)
	}

	if !specInline {
		absPath, _ := filepath.Abs(specPath)
		instruction += " Read the spec at " + absPath + " before processing each prompt."
	}

	convergence = "no convergence-blocking finding survives: " + blockingRule
	if noState {
		convergence += " (convergence is not being recorded: --no-state)"
		instruction += " Convergence is not being recorded (--no-state)."
	} else {
		required := 2
		if stateRequired != nil {
			required = *stateRequired
		}
		convergence = fmt.Sprintf("no convergence-blocking finding survives verification in %d consecutive rounds (%s)", required, blockingRule)
		instruction = fmt.Sprintf("For each prompt, spawn a sub-agent via the Agent tool. Merge findings (tp review --merge), verify and resolve them, then record the round: tp review %s --record <findings.ndjson>. Repeat until tp review %s --status --check exits 0.", specPath, specPath)
		if !specInline {
			absPath, _ := filepath.Abs(specPath)
			instruction += " Read the spec at " + absPath + " before processing each prompt."
		}
	}

	if regressionIncluded {
		instruction += " Process the regression prompt first and apply its findings before or together with the three role prompts." +
			" Between counted rounds, you may run tp review " + specPath + " --perspective regression alone as an uncounted delta pass."
	}
	if hasChecks {
		instruction += " If any mechanical check failed, fix those failures before spawning sub-agents."
	}
	return convergence, instruction
}
