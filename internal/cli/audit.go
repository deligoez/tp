package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// Sentinel errors for the --affected-files checks. determineAuditFiles routes
// on these with errors.Is rather than on message substrings, so rewording an
// error can no longer silently change the exit code it produces.
var (
	errAffectedFileMissing    = errors.New("affected file not found")
	errAffectedFileUnreadable = errors.New("cannot read affected file")
	errAffectedPathIsDir      = errors.New("affected path is a directory, not a file")
)

type checklistEntry struct {
	ID       string  `json:"id"`
	Type     string  `json:"type"`
	SpecLine int     `json:"spec_line"`
	Section  string  `json:"section"`
	Text     string  `json:"text"`
	Status   *string `json:"status"`
	Prompt   int     `json:"prompt"`
}

type checklistSummary struct {
	Total  int            `json:"total"`
	ByType map[string]int `json:"by_type"`
}

// ChecklistItem is one audit checklist entry, embedded inline in the prompt
// body and exposed for programmatic consumers.
type ChecklistItem struct {
	ItemID           string `json:"item_id"`
	Type             string `json:"type"` // list_item | table_row | task_acceptance | file_check | finding
	SpecLine         int    `json:"spec_line"`
	Section          string `json:"section"`
	Text             string `json:"text"`
	ExpectedEvidence string `json:"expected_evidence"`
}

// auditPrompt is one role's emitted prompt. ChecklistItems and AffectedFiles
// are pointers so --compact can OMIT them rather than emit an empty array: both
// are a verbatim duplicate of content buildRolePrompt already rendered into
// Prompt, which is the copy an agent reads. Under default output they are
// always set, so an empty routed set still serializes as [] and never null.
type auditPrompt struct {
	Role           string                   `json:"role"`
	Prompt         string                   `json:"prompt"`
	OutputPath     string                   `json:"output_path"`
	ChecklistCount int                      `json:"checklist_count"`
	ChecklistItems *[]ChecklistItem         `json:"checklist_items,omitempty"`
	AffectedFiles  *[]engine.AuditFileEntry `json:"affected_files,omitempty"`
}

// auditFileSummary is tp audit's file_summary: the shared affected-file counts
// plus the auto-detect cap's own accounting (section 8a.3). total_files keeps
// reporting the audited count, total_changed reports how many files changed
// before the cap, and truncated says whether the two differ. All three live in
// the payload rather than on stderr alone, because --quiet erases stderr and an
// agent reading stdout would otherwise take a 50-file prefix for the whole
// changed set.
type auditFileSummary struct {
	engine.AffectedSummary
	Truncated    bool `json:"truncated"`
	TotalChanged int  `json:"total_changed"`
}

type auditResult struct {
	Spec             string                `json:"spec"`
	Files            []string              `json:"files"`
	FileSummary      *auditFileSummary     `json:"file_summary,omitempty"`
	Checklist        []checklistEntry      `json:"checklist"`
	ChecklistSummary checklistSummary      `json:"checklist_summary"`
	SkippedRoles     *[]engine.SkippedRole `json:"skipped_roles,omitempty"`
	Prompts          []auditPrompt         `json:"prompts"`
}

var binaryExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true,
	".ico": true, ".woff": true, ".woff2": true, ".ttf": true, ".eot": true,
	".zip": true, ".tar": true, ".gz": true, ".pdf": true,
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".o": true, ".a": true,
}

const maxAutoDetectFiles = 50

func newAuditCmd() *cobra.Command {
	var affectedFiles []string
	var base string
	var roleFilter string
	var findingsPath string
	var recordPath string
	var statusMode bool
	var checkFlag bool
	var mergeMode bool
	var resolveMode bool
	var resolveAllMode bool
	var forceFlag bool
	var affectedFromTasks bool
	var outputPath string
	var harnessNote string

	cmd := &cobra.Command{
		Use:   "audit <spec.md>",
		Short: "Post-implementation spec review: verify code matches spec requirements",
		Long: `Post-implementation audit. Parses spec structured elements, reads changed source files,
and generates adversarial prompts that verify each requirement against actual code.

Auto-detects changed files via git diff (omit --affected-files for zero-config).
Use --findings to also verify review findings were addressed.`,
		Args:              cobra.ArbitraryArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// §3.3: --resolve/--resolve-all act on the round's results NDJSON,
			// which is the positional. They are routed ahead of every other
			// mode so the combination guard below sees the whole flag set, and
			// so a results file is never mistaken for the spec positional.
			if resolveMode && resolveAllMode {
				output.Error(ExitUsage, "--resolve and --resolve-all are mutually exclusive")
				os.Exit(ExitUsage)
				return nil
			}
			// §4.2.2: --role selects one emitted prompt, so every audit mode
			// that emits none refuses it. Placed before the mode branches so the
			// operator sees the flag conflict rather than that mode's own
			// argument complaint.
			if roleFilter != "" {
				for _, m := range []struct {
					on   bool
					name string
				}{
					{mergeMode, "--merge"},
					{recordPath != "", "--record"},
					{statusMode, "--status"},
					{resolveMode, "--resolve"},
					{resolveAllMode, "--resolve-all"},
				} {
					if m.on {
						output.Error(ExitUsage, "--role cannot be combined with "+m.name,
							"--role selects one emitted prompt; "+m.name+" emits none")
						os.Exit(ExitUsage)
						return nil
					}
				}
			}
			if resolveMode || resolveAllMode {
				flagName := "--resolve"
				if resolveAllMode {
					flagName = "--resolve-all"
				}
				if mergeMode || recordPath != "" || statusMode || len(affectedFiles) > 0 || findingsPath != "" || base != "" || affectedFromTasks || checkFlag || cmd.Flags().Changed("harness-note") || cmd.Flags().Changed("output") {
					output.Error(ExitUsage, flagName+" cannot be combined with --merge/--record/--status/--affected-files/--affected-from-tasks/--findings/--base/--check/--harness-note/-o",
						"run "+flagName+" on its own: it takes the audit results NDJSON as the positional")
					os.Exit(ExitUsage)
					return nil
				}
				if resolveMode {
					return runAuditResolve(args, forceFlag)
				}
				return runAuditResolveAll(args, forceFlag)
			}
			// --force is read only by the two resolve modes. Accepting it
			// elsewhere would let a caller believe an emission or record run had
			// overridden something it never looked at.
			if cmd.Flags().Changed("force") {
				output.Error(ExitUsage, "--force requires --resolve or --resolve-all",
					"--force re-resolves an audit row that already carries a disposition")
				os.Exit(ExitUsage)
				return nil
			}
			if mergeMode {
				// --harness-note belongs in this list too: --merge records no
				// round, so accepting it would silently drop the note while
				// the same flag on an emission run exits 2.
				if recordPath != "" || statusMode || len(affectedFiles) > 0 || findingsPath != "" || base != "" || affectedFromTasks || checkFlag || cmd.Flags().Changed("harness-note") {
					output.Error(ExitUsage, "--merge cannot be combined with --record/--status/--affected-files/--affected-from-tasks/--findings/--base/--check/--harness-note",
						"run --merge on its own: it takes only the input NDJSON files and -o <file>")
					os.Exit(ExitUsage)
					return nil
				}
				return runAuditMerge(args, outputPath)
			}
			// -o is only read by runAuditMerge above. Accepting it on any other
			// mode would silently drop the caller's redirect target while the
			// payload still went to stdout — the same silently-ignored-flag
			// hazard the --merge list guards in the opposite direction.
			if cmd.Flags().Changed("output") {
				output.Error(ExitUsage, "-o/--output requires --merge",
					"tp audit writes its payload to stdout; redirect it, or use --merge to write an NDJSON file with -o")
				os.Exit(ExitUsage)
				return nil
			}
			if recordPath != "" && statusMode {
				output.Error(ExitUsage, "--record and --status are mutually exclusive")
				os.Exit(ExitUsage)
				return nil
			}
			if checkFlag && !statusMode {
				output.Error(ExitUsage, "--check requires --status")
				os.Exit(ExitUsage)
				return nil
			}
			if cmd.Flags().Changed("harness-note") && recordPath == "" {
				output.Error(ExitUsage, "--harness-note requires --record", "supply --harness-note only together with --record <file>")
				os.Exit(ExitUsage)
				return nil
			}
			// --base belongs with emission; --record and --status neither read
			// nor store it, and accepting it silently makes a typo look like a
			// successful run against the base the caller meant.
			if (recordPath != "" || statusMode) && (len(affectedFiles) > 0 || findingsPath != "" || affectedFromTasks || base != "") {
				output.Error(ExitUsage, "--record/--status reject --affected-files/--affected-from-tasks/--findings and --base")
				os.Exit(ExitUsage)
				return nil
			}
			if affectedFromTasks && (len(affectedFiles) > 0 || base != "") {
				output.Error(ExitUsage, "--affected-from-tasks cannot be combined with --affected-files or --base")
				os.Exit(ExitUsage)
				return nil
			}
			if len(args) != 1 {
				output.Error(ExitUsage, "spec path required")
				os.Exit(ExitUsage)
				return nil
			}
			if recordPath != "" {
				return runAuditRecord(args[0], recordPath, harnessNote)
			}
			if statusMode {
				return runAuditStatus(args[0], checkFlag)
			}
			return runAudit(cmd, args[0], affectedFiles, base, findingsPath, roleFilter, affectedFromTasks)
		},
	}

	cmd.Flags().StringArrayVar(&affectedFiles, "affected-files", nil, "Source files to audit (auto-detect via git diff if omitted)")
	cmd.Flags().BoolVar(&affectedFromTasks, "affected-from-tasks", false, "Audit the union of files touched by done-task commit_shas")
	cmd.Flags().StringVar(&base, "base", "", "Git ref to diff against (omit for staged+unstaged)")
	cmd.Flags().StringVar(&roleFilter, "role", "", "Emit only this role's prompt (§4.2); one name, not repeatable")
	cmd.Flags().StringVar(&findingsPath, "findings", "", "Path to NDJSON findings from tp review")
	cmd.Flags().StringVar(&recordPath, "record", "", "Record an audit round from an NDJSON results file")
	cmd.Flags().BoolVar(&statusMode, "status", false, "Show recorded audit rounds and convergence state")
	cmd.Flags().BoolVar(&checkFlag, "check", false, "With --status: exit 0 only when audit is converged")
	cmd.Flags().StringVar(&harnessNote, "harness-note", "", "With --record: store an optional free-text note on the recorded round")
	cmd.Flags().BoolVar(&mergeMode, "merge", false, "Merge and deduplicate audit-result NDJSON files")
	cmd.Flags().BoolVar(&resolveMode, "resolve", false, "Dispose one audit row as fixed/wontfix/duplicate (selector: 0-based index or role:item_id; results NDJSON is the positional)")
	cmd.Flags().BoolVar(&resolveAllMode, "resolve-all", false, "Dispose every undisposed audit row with a status (results NDJSON is the positional)")
	cmd.Flags().BoolVar(&forceFlag, "force", false, "With --resolve/--resolve-all: re-resolve rows that already carry a disposition")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path (for --merge)")

	return cmd
}

func runAudit(_ *cobra.Command, specPath string, affectedFiles []string, base, findingsPath, roleFilter string, affectedFromTasks bool) error {
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		// First contact with the path: a spec-path mistake, so the shared
		// hint. The code-3 default names the TASK file — 'tp use' / 'tp init'
		// advice — which is the wrong object entirely here.
		output.Error(ExitFile, fmt.Sprintf("spec not found: %s", specPath), specFileMissingHint)
		os.Exit(ExitFile)
		return nil
	}

	// Reject a --base git would read as an option before any code path uses
	// it. Several helpers pass it to git, and the ones that only build diff
	// stats would otherwise ignore it silently, so a user typo looks like a
	// successful run against the wrong base.
	if base != "" && !engine.SafeGitRev(base) {
		output.Error(ExitUsage, fmt.Sprintf("invalid --base %q: must not start with %q", base, "-"),
			"pass a revision such as a tag, branch or commit sha")
		os.Exit(ExitUsage)
		return nil
	}

	// Read the findings HERE, for the same reason resolveRolePanel runs below
	// before loadAuditSpec: readFindings aborts on an unreadable file, and an
	// abort decided after loadAuditSpec left the round snapshot on disk with
	// in_flight_round already advanced — a round marked started that no prompt
	// ever came from.
	findingRows := readAuditFindings(findingsPath)
	refuseAuditIfBudgetExhausted(specPath)

	// Expand comma-separated values in --affected-files
	affectedFiles = expandCommaFiles(affectedFiles)
	files, totalChanged := determineAuditFiles(specPath, affectedFiles, base, affectedFromTasks)

	// §2.5 item 2: resolve the auditor panel — and with it decide the
	// spec-coverage and empty-phase refusals — ahead of every write the
	// emission path performs. loadAuditSpec below writes the round snapshot, so
	// a refusal decided any later would leave it on disk.
	panel := resolveRolePanel(specPath, engine.PhaseAuditors)

	specLines, specContent := loadAuditSpec(specPath)

	priorByRole := loadAuditPriorRound(specPath)

	checklist := buildChecklist(specLines, specPath, findingRows)

	if len(checklist) == 0 {
		output.Info("no structured elements found in spec — checklist is empty")
	}

	findingsEntries := filterChecklistByType(checklist, "finding")
	mainEntries := filterChecklistByType(checklist, "")

	// Per-role file selection (§5): drop rules first, then role rules over the
	// filtered universe; --affected-files replaced the universe upstream
	inputs := &engine.AuditFileInputs{
		Universe:  files,
		TaskFiles: engine.GitTaskFileMapping(filepath.Dir(specPath), auditTasksOf(specPath), files),
	}
	if len(affectedFiles) > 0 || affectedFromTasks {
		// The caller REPLACED the universe, and auditDiffRanges reproduces the
		// auto-detect comparison — a range that need not contain the paths the
		// caller named. Measuring them against it does not fail loudly: git
		// succeeds, the paths are simply absent from the result, and the
		// fallback hands every role "(diff: +0/-0)" as measured fact about a
		// file nothing measured. Annotate only what the audit's own comparison
		// covers; say nothing about the rest.
		inputs.DiffUnmeasured = true
	} else {
		inputs.DiffStats = auditDiffStats(filepath.Dir(specPath), base)
		inputs.Deleted = auditDeletedFiles(filepath.Dir(specPath), base)
	}
	sel := engine.SelectAuditFiles(inputs)

	specItems := routeChecklist(mainEntries, findingsEntries, invertTaskFiles(inputs.TaskFiles))

	// §10.6 loop budget for prompt framing: the audit round being emitted
	// (one past the last recorded), the consecutive clean rounds so far, and
	// the resolved workflow caps.
	auditWf, _ := engine.ResolveWorkflow(specPath, flagFile)
	auditRound := 1
	auditConsecutive := 0
	if st, err := engine.LoadReviewState(specPath); err == nil && st != nil {
		auditRound = len(st.AuditRounds) + 1
		auditConsecutive = engine.ConsecutiveClean(st.AuditRounds)
	}
	// §7.2: one prompt per active auditor role in the resolved panel.
	prompts, auditSkipped := generateRoleAuditPrompts(panel.roles, specItems, &sel, specContent, claudeMDExcerptFor(specPath), priorByRole, auditRound, auditWf.AuditCleanRounds, auditConsecutive, auditWf.AuditMaxRounds)

	prompts = appendClausesAudit(prompts)
	prompts = filterAuditPrompts(prompts, roleFilter)
	// §9.1: name every non-emitted auditor — empty-checklist roles above plus
	// any domain-filtered user corpus roles.
	auditSkipped = append(auditSkipped, engine.DomainSkippedRoles(filepath.Dir(specPath), panel.fm.Domain, engine.PhaseAuditors)...)
	// §2.4: plus every auditor this spec deactivated with enabled: false, so the
	// drop is visible rather than silent.
	auditSkipped = append(auditSkipped, engine.DisabledSkippedRoles(panel.disabled)...)

	summary := engine.BuildAffectedSummary(files, nil)

	byType := make(map[string]int)
	for _, e := range checklist {
		byType[e.Type]++
	}

	result := auditResult{
		Spec:      specPath,
		Files:     files,
		Checklist: checklist,
		ChecklistSummary: checklistSummary{
			Total:  len(checklist),
			ByType: byType,
		},
		Prompts: prompts,
	}

	if summary != nil {
		// Derive truncated from the two counts rather than from which
		// resolution path ran: only auto-detection caps today, but any future
		// path that hands over a prefix then reports itself honestly for free.
		result.FileSummary = &auditFileSummary{
			AffectedSummary: *summary,
			Truncated:       totalChanged > len(files),
			TotalChanged:    totalChanged,
		}
	}

	// §9.1 / §8.4: skipped_roles names every non-emitted auditor; explanatory,
	// omitted under --compact.
	if !IsCompact() {
		if auditSkipped == nil {
			auditSkipped = []engine.SkippedRole{}
		}
		result.SkippedRoles = &auditSkipped
	}

	if flagCompact {
		compactAuditChecklist(&result)
	}

	return output.JSON(result)
}

// determineAuditFiles resolves the set of source files to audit, plus the
// pre-cap count of files the resolution considered (§8a.3): the two differ only
// when auto-detection truncated the set. With affectedFromTasks, files are
// derived from done-task commit_shas (§11.2); otherwise the normal
// --affected-files / git-diff resolution applies. Errors abort via
// exitAuditNoFiles / ExitFile, matching runAudit's exit contract.
func determineAuditFiles(specPath string, affectedFiles []string, base string, affectedFromTasks bool) (files []string, totalChanged int) {
	if affectedFromTasks {
		// --affected-from-tasks bypasses diff auto-detection and audits the
		// union of files touched by done-task commit_shas directly (§11.2).
		derived := suggestFilesFromTasks(specPath)
		if len(derived) == 0 {
			// Say which of the two empty cases this is. Reporting "no done
			// task carries commit_shas" when one does — and was rejected as
			// an option-lookalike — sends the caller after the wrong problem.
			reason := "no done task carries commit_shas"
			if auditTasksCarryAnySHA(specPath) {
				reason = "every recorded commit sha was skipped as unusable"
			}
			// derived is the empty list the branch just computed; hand it over
			// rather than making exitAuditNoFiles derive the same answer again.
			exitAuditNoFilesWith(fmt.Sprintf("no files derivable from done-task commits (%s) — provide --affected-files", reason), derived)
			return nil, 0
		}
		// Nothing caps this path, so the audited set is the whole derived set.
		return derived, len(derived)
	}
	resolved, totalChanged, err := resolveAuditFiles(specPath, affectedFiles, base)
	if err != nil {
		// Route on sentinel identity. Classifying by substring meant that
		// rewording an error silently changed its exit code.
		if errors.Is(err, errAffectedFileMissing) ||
			errors.Is(err, errAffectedFileUnreadable) ||
			errors.Is(err, errAffectedPathIsDir) {
			// Point at the path the caller typed. The code-3 default hint
			// names the task file, which is not what is wrong here.
			output.Error(ExitFile, err.Error(), "check the --affected-files path, or drop the flag to auto-detect from the diff")
			os.Exit(ExitFile)
			return nil, 0
		}
		// No audit-able file in the diff (exit 4): carry suggested_files so
		// the agent can pick targets without re-deriving them from git (§11.1).
		exitAuditNoFiles(specPath, err.Error())
		return nil, 0
	}
	return resolved, totalChanged
}

// loadAuditSpec reads the spec, snapshots its raw bytes at audit-round start
// (§10.2), and returns the frontmatter-blanked line slice plus the (possibly
// truncated) spec content used for prompt emission. Read, state, and snapshot
// errors abort via ExitFile / exitStateError, matching runAudit's exit contract.
func loadAuditSpec(specPath string) (specLines []string, specContent string) {
	specData, err := os.ReadFile(specPath)
	if err != nil {
		// Carry the cause: a permission or IO failure is otherwise
		// indistinguishable from a missing file at the call site. This site is
		// POST-stat (runAudit already proved the path exists), so the hint is
		// err.Error() and not specFileMissingHint — the caller did not mistype
		// the path, and left hintless it would inherit the code-3 task-file
		// default, the wrong object entirely.
		output.Error(ExitFile, fmt.Sprintf("cannot read spec: %s", specPath), err.Error())
		os.Exit(ExitFile)
		return nil, ""
	}
	// §10.2: snapshot the raw spec at audit round start (prompt emission),
	// mirroring review — write atomically so a partial snapshot is never left
	// on disk, and an interrupted round is visible to --status and tp resume.
	auditSt, stErr := engine.LoadReviewState(specPath)
	if stErr != nil {
		if engine.IsRebuildableStateIndex(stErr) {
			// A prior emission wrote a snapshot that --record has not yet
			// indexed: the normal in-flight round (§10.2, InFlightRound), not
			// corruption. Treat as no recorded state and re-snapshot below;
			// only genuine corruption (unparseable state.json) or an IO error
			// aborts.
			//
			// Rebuildable only. Under the broader predicate a round file with
			// no index also landed here, and since the budget guard above
			// returns early on the default audit_max_rounds=0, tp audit emitted
			// a prompt over lost history and OVERWROTE its round snapshot,
			// while --record on the same directory exited 3.
			auditSt = nil
		} else {
			exitStateError(stErr)
			return nil, ""
		}
	}
	auditRecorded := 0
	if auditSt != nil {
		auditRecorded = len(auditSt.AuditRounds)
	}
	if snapErr := engine.WriteSnapshotAtomic(specPath, engine.PhaseAudit, auditRecorded+1, specData); snapErr != nil {
		// Post-stat failure: the hint carries the real cause, not spec-path
		// advice — the path was already proven readable above.
		output.Error(ExitFile, fmt.Sprintf("cannot write audit round snapshot for %s", specPath), snapErr.Error())
		os.Exit(ExitFile)
		return nil, ""
	}
	specData = engine.BlankFrontmatter(specData)
	specLines = strings.Split(string(specData), "\n")
	specContent = string(specData)
	if len(specContent) > engine.SpecContentCap {
		specContent = specContent[:engine.SpecContentCap] + "\n[...spec truncated]"
	}
	return specLines, specContent
}

// loadAuditPriorRound reads the previous recorded audit round and returns,
// per role, that role's own non-PASS rows for the round-2+ prior-round
// section (§10.2). It returns nil when no audit round is recorded (round 1)
// or the prior round's file is missing; a missing state index is the normal
// in-flight condition and yields nil, while genuine corruption aborts. The
// changed-since flag per row is true when a commit touching that row's
// evidence_file landed after the prior round's recorded_at; it is omitted
// for rows with no file path (spec-derived or FAIL rows with no evidence).
func loadAuditPriorRound(specPath string) map[string]*auditPriorRound {
	st, err := engine.LoadReviewState(specPath)
	if err != nil {
		// Rebuildable only, as at loadAuditSpec: a round file with no index is
		// history this function would otherwise report as "no prior round".
		if engine.IsRebuildableStateIndex(err) {
			return nil
		}
		exitStateError(err)
		return nil
	}
	if st == nil || len(st.AuditRounds) == 0 {
		return nil
	}
	prior := &st.AuditRounds[len(st.AuditRounds)-1]
	rows, found := engine.LoadRoundRows(specPath, prior)
	if !found {
		// Dropping the whole prior-round section silently makes a round-2
		// prompt look like a round-1 one. review.go, review_record.go and
		// review_regression.go all say so on the same condition.
		output.Notice(fmt.Sprintf("round %d file %s is missing; skipping its rows", prior.Round, prior.File))
		return nil
	}
	changedFiles := filesChangedSince(filepath.Dir(specPath), prior.RecordedAt)
	byRole := make(map[string]*auditPriorRound)
	legacy := engine.IsLegacyRound(prior)
	for _, row := range rows {
		role, _ := row["role"].(string)
		status, _ := row["status"].(string)
		if status == "PASS" {
			continue
		}
		itemID, _ := row["item_id"].(string)
		ef, _ := row["evidence_file"].(string)
		pr := priorAuditRow{Role: role, ItemID: itemID, Status: status}
		if ef != "" {
			pr.EvidenceFile = ef
			changed := changedFiles[ef]
			pr.ChangedSince = &changed
		}
		entry := byRole[role]
		if entry == nil {
			entry = &auditPriorRound{legacy: legacy}
			byRole[role] = entry
		}
		entry.rows = append(entry.rows, pr)
	}
	return byRole
}

// filesChangedSince returns the set of repo-relative paths touched by any
// commit whose commit date is at or after since (an RFC3339 timestamp) — the
// changed-since basis for the audit prior-round section (§10.2). Returns an
// empty set when since is empty or git is unavailable (e.g. not a repo), so
// the changed-since flag defaults to false rather than aborting emission.
func filesChangedSince(dir, since string) map[string]bool {
	changed := make(map[string]bool)
	if since == "" {
		return changed
	}
	for _, f := range execGitDiff(dir, "log", "--since="+since, "--name-only", "--pretty=format:") {
		changed[f] = true
	}
	return changed
}

// readAuditFindings refuses a missing --findings path and returns the rows of
// the file otherwise. It both refuses and reads because the refusal has to be
// decided where the read is: runAudit resolves every refusal ahead of the round
// snapshot write, and a read failure discovered later left a snapshot on disk
// for a round that emitted nothing.
//
// A path that does not exist is a typo, not an empty finding set: readFindings
// answers os.IsNotExist with nil, so without this guard the round silently
// verifies ZERO review findings and still records as clean — a mistyped path
// could declare convergence. tp review rejects the same typo up front, and tp
// audit already refuses a missing --affected-files path. Other stat errors fall
// through to readFindings, which names them as a read failure rather than as
// "not found". An empty path means the flag was not passed, which is valid, and
// yields no rows.
func readAuditFindings(findingsPath string) []findingRow {
	if findingsPath == "" {
		return nil
	}
	if _, err := os.Stat(findingsPath); os.IsNotExist(err) {
		output.Error(ExitFile, fmt.Sprintf("findings file not found: %s", findingsPath),
			findingsFileMissingHint+" An audit round that verifies zero findings can still record as clean.")
		os.Exit(ExitFile)
	}
	return readFindings(findingsPath)
}

// refuseAuditIfBudgetExhausted refuses audit prompt generation when the audit
// cap is exhausted; the cap-triggered state read inherits the corrupt-state
// abort, but a missing state index is the normal in-flight round, not
// corruption.
func refuseAuditIfBudgetExhausted(specPath string) {
	wfBudget, _ := engine.ResolveWorkflow(specPath, flagFile)
	if wfBudget.AuditMaxRounds <= 0 {
		return
	}
	stBudget, stErr := engine.LoadReviewState(specPath)
	if stErr != nil {
		// Rebuildable only: with a round file present and the index gone, the
		// budget would be computed from history tp cannot see.
		if !engine.IsRebuildableStateIndex(stErr) {
			exitStateError(stErr)
			return
		}
		// A prior emission wrote a snapshot that --record has not yet
		// indexed (§10.2, InFlightRound). tp audit never calls
		// EnsureReviewState, so state.json is legitimately absent after an
		// emission — treat it as no recorded rounds, exactly as
		// loadAuditSpec and loadAuditPriorRound now do. Aborting here made the
		// SECOND tp audit of a round exit 3 "state is unusable" whenever
		// audit_max_rounds was non-zero, gating a normal in-flight state on
		// an unrelated knob.
		stBudget = nil
	}
	rounds := []engine.ReviewRound{}
	if stBudget != nil {
		rounds = stBudget.AuditRounds
	}
	refuseIfBudgetExhausted("audit", specPath, rounds, wfBudget.AuditMaxRounds, wfBudget.AuditCleanRounds, "")
}

// compactAuditChecklist truncates checklist text, drops the file summary, and
// omits the two per-prompt fields that duplicate the prompt body, for --compact
// output.
//
// prompts[].checklist_items and prompts[].affected_files are a verbatim
// duplicate of what buildRolePrompt already rendered into prompts[].prompt, so
// without this every spec item shipped three times over: once in the top-level
// checklist, once as structured JSON per prompt, and once inside the prompt an
// agent actually reads. On a 3-role/5-file payload the pair is a fifth of the
// bytes, which is why --compact was measurably a no-op here.
func compactAuditChecklist(result *auditResult) {
	for i := range result.Checklist {
		if len(result.Checklist[i].Text) > 80 {
			result.Checklist[i].Text = result.Checklist[i].Text[:77] + "..."
		}
	}
	result.FileSummary = nil
	for i := range result.Prompts {
		result.Prompts[i].ChecklistItems = nil
		result.Prompts[i].AffectedFiles = nil
	}
}

func resolveAuditFiles(specPath string, affectedFiles []string, base string) (files []string, totalChanged int, err error) {
	if len(affectedFiles) > 0 {
		affectedFiles = engine.DedupPaths(affectedFiles)
		for _, f := range affectedFiles {
			info, statErr := os.Stat(f)
			if statErr != nil {
				// Carry the cause: a permission error reported as "not found"
				// sends the caller looking for the wrong problem. Wrap the
				// sentinel so the caller routes on identity, not on wording.
				if os.IsNotExist(statErr) {
					return nil, 0, fmt.Errorf("%w: %s", errAffectedFileMissing, f)
				}
				return nil, 0, fmt.Errorf("%w %s: %w", errAffectedFileUnreadable, f, statErr)
			}
			if info.IsDir() {
				return nil, 0, fmt.Errorf("%w: %s", errAffectedPathIsDir, f)
			}
		}
		// The cap belongs to auto-detection: a named set is audited whole, so
		// its pre-cap count is its own length and it never reads as truncated.
		return affectedFiles, len(affectedFiles), nil
	}

	specDir := filepath.Dir(specPath)
	files, totalChanged, err = detectChangedFiles(specDir, base)
	if err != nil {
		return nil, 0, err
	}
	if len(files) == 0 {
		if base != "" {
			return nil, 0, fmt.Errorf("no changed files detected (diff %s...HEAD is empty) — provide --affected-files", base)
		}
		return nil, 0, fmt.Errorf("no changed files detected (staged+unstaged is empty) — use --base <tag> for committed changes, or --affected-files")
	}
	return files, totalChanged, nil
}

// detectChangedFiles returns the audit-able changed files, capped at
// maxAutoDetectFiles, and — as its second result — how many there were before
// the cap (§8a.3). The caller reports both, so a truncated set is legible in
// the payload and not on stderr alone.
func detectChangedFiles(dir, base string) (files []string, totalChanged int, err error) {
	var allFiles []string

	// A base that git would read as an option must never be concatenated into
	// a revision range; runAudit rejects one up front, and this is the sink.
	if base != "" && !engine.SafeGitRev(base) {
		return nil, 0, fmt.Errorf("invalid --base %q: must not start with %q", base, "-")
	}

	// auditDiffRanges is the single definition of "the comparison this audit
	// is about". The per-file diff stats handed to every role are derived from
	// the same list, so selection and stats can never describe different
	// ranges — the failure that let a committed change be reported as +0/-0.
	for i, rng := range auditDiffRanges(dir, base) {
		changed := execGitDiffProbe(dir, "diff --name-only", append([]string{"diff", "--name-only"}, rng...)...)
		if i == 0 && len(changed) == 0 && !gitExists(dir) {
			return nil, 0, fmt.Errorf("not in a git repo — provide --affected-files or run inside a git repo")
		}
		allFiles = append(allFiles, changed...)
	}

	allFiles = engine.DedupPaths(allFiles)
	filtered := make([]string, 0, len(allFiles))
	for _, f := range allFiles {
		if !isAuditableType(f) {
			continue
		}
		filtered = append(filtered, f)
	}
	sort.Strings(filtered)

	// The pre-cap count is taken over the audit-able set, which is what the
	// cap acts on: reporting the raw diff would count files no audit would
	// ever have read and overstate the loss.
	totalChanged = len(filtered)

	if len(filtered) > maxAutoDetectFiles {
		filtered = filtered[:maxAutoDetectFiles]
		// Notice, not Info: the caller asked for the whole changed set and got a
		// prefix of it. On the Info channel that truncation is invisible in JSON
		// mode, which is every piped run.
		//
		// Name both numbers. "more than 50 files changed" read identically at 51
		// and at 113, so the caller could size neither the loss nor the
		// --affected-files list that works around it.
		output.Notice(fmt.Sprintf("%d files changed, auditing first %d — name the rest with --affected-files", totalChanged, maxAutoDetectFiles))
	}

	if len(filtered) == 0 && len(allFiles) > 0 {
		// Collect skipped file extensions for the error message
		extSet := make(map[string]bool)
		for _, f := range allFiles {
			if idx := strings.LastIndex(f, "."); idx >= 0 {
				extSet[f[idx:]] = true
			}
		}
		exts := make([]string, 0, len(extSet))
		for ext := range extSet {
			exts = append(exts, ext)
		}
		sort.Strings(exts)
		return nil, 0, fmt.Errorf("no audit-able files in diff — only skipped types changed (%s). Use --base <tag> or --affected-files", strings.Join(exts, ", "))
	}

	return filtered, totalChanged, nil
}

func gitExists(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	return cmd.Run() == nil
}

func latestGitTag(dir string) string {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func execGitDiff(dir string, args ...string) []string {
	return execGitDiffProbe(dir, strings.Join(args, " "), args...)
}

// execGitDiffProbe is execGitDiff with an explicit advisory key. Callers that
// run the SAME probe once per selection range pass a key naming the probe, not
// the invocation: a condition that breaks one range (no repository, unknown
// revision) breaks every range, and without the key one broken condition costs
// the caller one advisory per range.
func execGitDiffProbe(dir, probe string, args ...string) []string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		// git failing is not the same as an empty diff. Reporting an unknown
		// revision as "no changed files detected" sends the caller looking for
		// missing work instead of at the revision they typed.
		warnGitFailure(probe, err, args...)
		return []string{}
	}
	var files []string
	scanner := bufio.NewScanner(strings.NewReader(string(out))) // line-cap: git diff output, one path per line, not NDJSON
	for scanner.Scan() {
		f := strings.TrimSpace(scanner.Text())
		if f != "" {
			files = append(files, f)
		}
	}
	if err := scanner.Err(); err != nil {
		// Notice, not raw stderr: this is a dropped-content advisory like its
		// siblings, so --quiet silences it and JSON mode does not.
		noticeOnce("git-scan:"+probe, fmt.Sprintf("warning: stopped scanning git diff output early (%v); files after the over-long line were dropped (line cap is 64KB)", err))
	}
	return files
}

// warnGitFailure names a failed git invocation on stderr. Every caller turns
// the error into a zero value — an empty file list, an empty diff-stat map, an
// empty deleted set — and a zero value is indistinguishable from a genuinely
// unchanged tree once it reaches an auditor prompt as fact. The warning is what
// keeps "git could not answer" from reading as "nothing changed".
func warnGitFailure(probe string, err error, args ...string) {
	detail := err.Error()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
		detail = string(exitErr.Stderr)
	}
	// git answers a rejected invocation with its full usage text — hundreds of
	// lines. This advisory travels the Notice channel, which JSON mode does NOT
	// suppress, so the detail is capped to one bounded line: an uncapped dump
	// costs the agent more context than the payload it annotates.
	noticeOnce("git:"+probe, fmt.Sprintf("warning: git %s failed: %s", strings.Join(args, " "), firstLineCapped(detail)))
}

func isBinaryFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return false
	}
	return binaryExtensions[ext]
}

// isAuditableType reports whether a path survives the type filtering the
// auto-detection applies: binary files, markdown, task files, tp's own state
// directory and extensionless repository metadata are skipped.
func isAuditableType(path string) bool {
	if isBinaryFile(path) {
		return false
	}
	if strings.HasSuffix(path, ".md") {
		return false
	}
	if strings.HasSuffix(path, ".tasks.json") {
		return false
	}
	if isTPStatePath(path) {
		return false
	}
	if isRepoMetadataDotfile(path) {
		return false
	}
	return true
}

// isTPStatePath reports whether the path lies inside tp's own state directory.
// .tp/ holds the project config, the active pointer, the role corpus and the
// run state: tp's bookkeeping about the cycle, never the code a spec is audited
// against. A task that edits one commits it like any other file, so the path
// reaches an auto-detected file list honestly and costs every role its
// attention.
func isTPStatePath(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == ".tp" {
			return true
		}
	}
	return false
}

// isRepoMetadataDotfile reports whether the base name is a dotfile carrying no
// further extension — .gitignore, .gitattributes, .dockerignore.
//
// The leading dot is stripped before the check because filepath.Ext(".gitignore")
// returns ".gitignore", not "": the final dot is at index 0, so the whole name
// reads as the extension. A dotfile that does carry a real extension is left
// auditable, which is what keeps .golangci.yml and .github workflows in the set.
func isRepoMetadataDotfile(path string) bool {
	base := filepath.Base(path)
	if !strings.HasPrefix(base, ".") || base == "." || base == ".." {
		return false
	}
	return !strings.Contains(base[1:], ".")
}

func buildChecklist(specLines []string, specPath string, findingRows []findingRow) []checklistEntry {
	entries := make([]checklistEntry, 0)

	tableRows := engine.ExtractTableRows(specLines)
	currentTableIdx := -1
	rowIndex := 0
	var prevSection string
	for _, row := range tableRows {
		if row.Section != prevSection || currentTableIdx < 0 {
			currentTableIdx++
			rowIndex = 0
			prevSection = row.Section
		}
		entries = append(entries, checklistEntry{
			ID:       fmt.Sprintf("table-%d-%d", currentTableIdx, rowIndex),
			Type:     "table_row",
			SpecLine: row.Line,
			Section:  row.Section,
			Text:     row.Raw,
			Prompt:   0,
		})
		rowIndex++
	}

	listItems := engine.ExtractNumberedItems(specLines)
	listIdx := -1
	var prevListSection string
	for _, item := range listItems {
		if (item.Number == 1 && item.Section != prevListSection) || listIdx < 0 {
			listIdx++
			prevListSection = item.Section
		}
		entries = append(entries, checklistEntry{
			ID:       fmt.Sprintf("list-%d-%d", listIdx, item.Number),
			Type:     "list_item",
			SpecLine: item.Line,
			Section:  item.Section,
			Text:     item.Text,
			Prompt:   0,
		})
	}

	entries = append(entries, taskAcceptanceEntries(specPath)...)
	entries = append(entries, findingChecklistEntries(findingRows)...)

	return entries
}

// taskAcceptanceEntries reads the spec-adjacent task file and yields one
// task_acceptance checklist entry per task with a non-empty acceptance.
func taskAcceptanceEntries(specPath string) []checklistEntry {
	// auditTasksOf is the one reader of the spec-adjacent task file, and it
	// already announces a file that exists but cannot be read or parsed.
	// Parsing it a second time here produced a second, differently worded
	// advisory for the same one condition.
	tasks := auditTasksOf(specPath)
	entries := make([]checklistEntry, 0, len(tasks))
	for i := range tasks {
		task := &tasks[i]
		if task.Acceptance == "" {
			continue
		}
		entries = append(entries, checklistEntry{
			ID:       fmt.Sprintf("task-%s", task.ID),
			Type:     "task_acceptance",
			SpecLine: 0,
			Section:  task.Title,
			Text:     task.Acceptance,
			Prompt:   0,
		})
	}
	return entries
}

// findingChecklistEntries yields one finding entry per row of a --findings
// file; §3.2 puts the finding's location in the entry Section.
func findingChecklistEntries(rows []findingRow) []checklistEntry {
	entries := make([]checklistEntry, 0, len(rows))
	for i, fe := range rows {
		section := fe.location
		if section == "" {
			section = "Review Findings"
		}
		entries = append(entries, checklistEntry{
			ID:       fmt.Sprintf("finding-%d", i),
			Type:     "finding",
			SpecLine: 0,
			Section:  section,
			Text:     fe.text,
			Prompt:   0,
		})
	}
	return entries
}

// findingRow is a review finding read from a --findings file: the finding
// text and its location field (§3.2 puts the location in the item's Section).
type findingRow struct {
	text     string
	location string
}

func readFindings(path string) []findingRow {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		// Cause in the message, repair in the hint — the convention the review
		// sinks follow. This site had them inverted: a raw errno stood where the
		// repair belongs, and the message named only the path.
		output.Error(ExitFile, fmt.Sprintf("cannot read findings file: %s: %v", path, err), ndjsonInputFileHint)
		os.Exit(ExitFile)
		return nil
	}
	var results []findingRow
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 0, 64*1024), ndjsonLineCap)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			// noticeOnce: the message names the file, not the line, so N bad
			// lines produced N byte-identical copies of one advisory.
			noticeOnce("findings-malformed:"+path, fmt.Sprintf("warning: skipping malformed line (invalid JSON) in %s", path))
			continue
		}
		text := ""
		for _, field := range []string{"finding", "message", "description", "title"} {
			if v, ok := obj[field].(string); ok && v != "" {
				text = v
				break
			}
		}
		if text != "" {
			loc, _ := obj["location"].(string)
			results = append(results, findingRow{text: text, location: loc})
		}
	}
	if err := scanner.Err(); err != nil {
		// Aborting, not an advisory — a reversal of the v0.28.0 contract, made
		// deliberately. These rows become the finding-verification checklist,
		// so rows dropped after an over-long line are findings the audit never
		// asks about while still recording the round: the false-clean class the
		// loaders were swept for. The advisory was worse than it looked, since
		// output.Notice honours --quiet: exit 0, empty stderr, a quietly shorter
		// checklist. Every sibling NDJSON reader aborts, and at ndjsonLineCap
		// this fires only past 1MB on a single line.
		output.Error(ExitFile, fmt.Sprintf("cannot read %s: %v", path, err), ndjsonReadHint(err))
		os.Exit(ExitFile)
		return nil
	}
	return results
}

func filterChecklistByType(entries []checklistEntry, typ string) []checklistEntry {
	result := make([]checklistEntry, 0)
	if typ == "" {
		for _, e := range entries {
			if e.Type != "finding" {
				result = append(result, e)
			}
		}
		return result
	}
	for _, e := range entries {
		if e.Type == typ {
			result = append(result, e)
		}
	}
	return result
}

// expandCommaFiles splits comma-separated values and trims whitespace.
func expandCommaFiles(files []string) []string {
	if len(files) == 0 {
		return nil
	}
	expanded := make([]string, 0, len(files))
	for _, f := range files {
		for part := range strings.SplitSeq(f, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				expanded = append(expanded, trimmed)
			}
		}
	}
	return expanded
}

// execCommitFiles lists the paths a commit touched (its diff, not its full
// tree — §5.1c). execGitDiff is a generic "run git, return non-empty stdout
// lines" helper, so it serves git show as well as git diff.
func execCommitFiles(dir, sha string) []string {
	// A sha reaches here from the task file, which import and add also write,
	// so the entry-point check in resolveCommitSHAs is not the only writer.
	// Guard the sink (engine.SafeGitRev) and say so: a silently shrunken file
	// set makes the "no done task carries commit_shas" message a lie.
	if !engine.SafeGitRev(sha) {
		// noticeOnce: the derivation runs more than once per invocation (the
		// --affected-from-tasks probe and the exit-4 suggestion both walk the
		// same shas), and one rejected sha should cost the reader one line.
		noticeOnce("sha:"+sha, fmt.Sprintf("warning: commit sha %q was skipped; git would read it as an option", sha))
		return nil
	}
	return execGitDiff(dir, "show", "--name-only", "--pretty=format:", sha)
}

// suggestFilesFromTasks derives the union of paths touched by the commits
// recorded in commit_shas of every task with status done in the spec-adjacent
// task file, with the same type filtering detectChangedFiles applies (§11.1).
// Every SHA in each array is read, so reopened-then-redone history is covered.
func suggestFilesFromTasks(specPath string) []string {
	tasks := auditTasksOf(specPath)
	shas := make(map[string]bool)
	for i := range tasks {
		if tasks[i].Status != model.StatusDone {
			continue
		}
		for _, sha := range tasks[i].CommitSHAs {
			if sha != "" {
				shas[sha] = true
			}
		}
	}
	specDir := filepath.Dir(specPath)
	seen := make(map[string]bool)
	out := make([]string, 0)
	for sha := range shas {
		for _, p := range execCommitFiles(specDir, sha) {
			if !seen[p] && isAuditableType(p) {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	sort.Strings(out)
	return out
}

// auditTasksCarryAnySHA reports whether any done task records a commit sha at
// all. It distinguishes the two ways suggestFilesFromTasks can return nothing:
// no sha was ever recorded, or every recorded sha was rejected at the git sink.
func auditTasksCarryAnySHA(specPath string) bool {
	tasks := auditTasksOf(specPath)
	for i := range tasks {
		if tasks[i].Status != model.StatusDone {
			continue
		}
		for _, sha := range tasks[i].CommitSHAs {
			if sha != "" {
				return true
			}
		}
	}
	return false
}

// exitAuditNoFiles emits the exit-4 payload when audit finds no audit-able
// file: it carries suggested_files — the union of paths touched by the
// commit_shas of every done task, type-filtered — so the agent can pick audit
// targets without re-deriving them from git (§11.1). suggested_files is
// decision-critical and survives --compact (§8.4): it is emitted directly on
// the error path, never passed through compactAuditChecklist.
func exitAuditNoFiles(specPath, reason string) {
	exitAuditNoFilesWith(reason, suggestFilesFromTasks(specPath))
}

// exitAuditNoFilesWith is exitAuditNoFiles for a caller that already derived
// the suggestion. --affected-from-tasks computes exactly this list to decide
// whether it has any files at all, and re-deriving it here would walk every
// done task's commits a second time for a result the caller is holding.
func exitAuditNoFilesWith(reason string, suggested []string) {
	output.ErrorExtras(ExitState, reason, map[string]any{
		"suggested_files": suggested,
	}, "pass --affected-files <paths>, or --affected-from-tasks to audit the files touched by done-task commits")
	os.Exit(ExitState)
}
