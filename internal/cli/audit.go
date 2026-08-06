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

type auditResult struct {
	Spec             string                  `json:"spec"`
	Files            []string                `json:"files"`
	FileSummary      *engine.AffectedSummary `json:"file_summary,omitempty"`
	Checklist        []checklistEntry        `json:"checklist"`
	ChecklistSummary checklistSummary        `json:"checklist_summary"`
	SkippedRoles     *[]engine.SkippedRole   `json:"skipped_roles,omitempty"`
	Prompts          []auditPrompt           `json:"prompts"`
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
	var findingsPath string
	var recordPath string
	var statusMode bool
	var checkFlag bool
	var mergeMode bool
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
			return runAudit(cmd, args[0], affectedFiles, base, findingsPath, affectedFromTasks)
		},
	}

	cmd.Flags().StringArrayVar(&affectedFiles, "affected-files", nil, "Source files to audit (auto-detect via git diff if omitted)")
	cmd.Flags().BoolVar(&affectedFromTasks, "affected-from-tasks", false, "Audit the union of files touched by done-task commit_shas")
	cmd.Flags().StringVar(&base, "base", "", "Git ref to diff against (omit for staged+unstaged)")
	cmd.Flags().StringVar(&findingsPath, "findings", "", "Path to NDJSON findings from tp review")
	cmd.Flags().StringVar(&recordPath, "record", "", "Record an audit round from an NDJSON results file")
	cmd.Flags().BoolVar(&statusMode, "status", false, "Show recorded audit rounds and convergence state")
	cmd.Flags().BoolVar(&checkFlag, "check", false, "With --status: exit 0 only when audit is converged")
	cmd.Flags().StringVar(&harnessNote, "harness-note", "", "With --record: store an optional free-text note on the recorded round")
	cmd.Flags().BoolVar(&mergeMode, "merge", false, "Merge and deduplicate audit-result NDJSON files")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path (for --merge)")

	return cmd
}

func runAudit(_ *cobra.Command, specPath string, affectedFiles []string, base, findingsPath string, affectedFromTasks bool) error {
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		output.Error(ExitFile, fmt.Sprintf("spec not found: %s", specPath))
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

	refuseMissingFindingsFile(findingsPath)
	refuseAuditIfBudgetExhausted(specPath)

	// Expand comma-separated values in --affected-files
	affectedFiles = expandCommaFiles(affectedFiles)
	files := determineAuditFiles(specPath, affectedFiles, base, affectedFromTasks)

	// §2.5 item 2: resolve the auditor panel — and with it decide the
	// spec-coverage and empty-phase refusals — ahead of every write the
	// emission path performs. loadAuditSpec below writes the round snapshot, so
	// a refusal decided any later would leave it on disk.
	panel := resolveRolePanel(specPath, engine.PhaseAuditors)

	specLines, specContent := loadAuditSpec(specPath)

	priorByRole := loadAuditPriorRound(specPath)

	checklist := buildChecklist(specLines, specPath, findingsPath)

	if len(checklist) == 0 {
		output.Info("no structured elements found in spec — checklist is empty")
	}

	findingsEntries := filterChecklistByType(checklist, "finding")
	mainEntries := filterChecklistByType(checklist, "")

	// Per-role file selection (§5): drop rules first, then role rules over the
	// filtered universe; --affected-files replaced the universe upstream
	inputs := &engine.AuditFileInputs{
		Universe:  files,
		DiffStats: auditDiffStats(filepath.Dir(specPath), base),
		Deleted:   auditDeletedFiles(filepath.Dir(specPath), base),
		TaskFiles: engine.GitTaskFileMapping(auditTasksOf(specPath), files),
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
		result.FileSummary = summary
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

// determineAuditFiles resolves the set of source files to audit. With
// affectedFromTasks, files are derived from done-task commit_shas (§11.2);
// otherwise the normal --affected-files / git-diff resolution applies. Errors
// abort via exitAuditNoFiles / ExitFile, matching runAudit's exit contract.
func determineAuditFiles(specPath string, affectedFiles []string, base string, affectedFromTasks bool) []string {
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
			exitAuditNoFiles(specPath, fmt.Sprintf("no files derivable from done-task commits (%s) — provide --affected-files", reason))
			return nil
		}
		return derived
	}
	resolved, err := resolveAuditFiles(specPath, affectedFiles, base)
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
			return nil
		}
		// No audit-able file in the diff (exit 4): carry suggested_files so
		// the agent can pick targets without re-deriving them from git (§11.1).
		exitAuditNoFiles(specPath, err.Error())
		return nil
	}
	return resolved
}

// loadAuditSpec reads the spec, snapshots its raw bytes at audit-round start
// (§10.2), and returns the frontmatter-blanked line slice plus the (possibly
// truncated) spec content used for prompt emission. Read, state, and snapshot
// errors abort via ExitFile / exitStateError, matching runAudit's exit contract.
func loadAuditSpec(specPath string) (specLines []string, specContent string) {
	specData, err := os.ReadFile(specPath)
	if err != nil {
		// Carry the cause: a permission or IO failure is otherwise
		// indistinguishable from a missing file at the call site.
		output.Error(ExitFile, fmt.Sprintf("cannot read spec %s: %v", specPath, err))
		os.Exit(ExitFile)
		return nil, ""
	}
	// §10.2: snapshot the raw spec at audit round start (prompt emission),
	// mirroring review — write atomically so a partial snapshot is never left
	// on disk, and an interrupted round is visible to --status and tp resume.
	auditSt, stErr := engine.LoadReviewState(specPath)
	if stErr != nil {
		if engine.IsMissingStateIndex(stErr) {
			// A prior emission wrote a snapshot that --record has not yet
			// indexed: the normal in-flight round (§10.2, InFlightRound), not
			// corruption. Treat as no recorded state and re-snapshot below;
			// only genuine corruption (unparseable state.json) or an IO error
			// aborts.
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
		output.Error(ExitFile, fmt.Sprintf("cannot write snapshot: %v", snapErr))
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
		if engine.IsMissingStateIndex(err) {
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

// refuseMissingFindingsFile aborts when --findings names a path that does not
// exist. That is a typo, not an empty finding set: readFindings answers
// os.IsNotExist with nil, so without this guard the round silently verifies
// ZERO review findings and still records as clean — a mistyped path could
// declare convergence. tp review rejects the same typo up front, and tp audit
// already refuses a missing --affected-files path; only the
// existing-but-unreadable branch of readFindings was loud. Other stat errors
// fall through to readFindings, which names them as a read failure rather than
// as "not found". An empty path means the flag was not passed, which is valid.
func refuseMissingFindingsFile(findingsPath string) {
	if findingsPath == "" {
		return
	}
	if _, err := os.Stat(findingsPath); os.IsNotExist(err) {
		output.Error(ExitFile, fmt.Sprintf("findings file not found: %s", findingsPath),
			findingsFileMissingHint+" An audit round that verifies zero findings can still record as clean.")
		os.Exit(ExitFile)
	}
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
		if !engine.IsMissingStateIndex(stErr) {
			exitStateError(stErr)
			return
		}
		// A prior emission wrote a snapshot that --record has not yet
		// indexed (§10.2, InFlightRound). tp audit never calls
		// EnsureReviewState, so state.json is legitimately absent after an
		// emission — treat it as no recorded rounds, exactly as
		// loadAuditSpec and loadAuditPriorRound do. Aborting here made the
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

func resolveAuditFiles(specPath string, affectedFiles []string, base string) ([]string, error) {
	if len(affectedFiles) > 0 {
		affectedFiles = engine.DedupPaths(affectedFiles)
		for _, f := range affectedFiles {
			info, err := os.Stat(f)
			if err != nil {
				// Carry the cause: a permission error reported as "not found"
				// sends the caller looking for the wrong problem. Wrap the
				// sentinel so the caller routes on identity, not on wording.
				if os.IsNotExist(err) {
					return nil, fmt.Errorf("%w: %s", errAffectedFileMissing, f)
				}
				return nil, fmt.Errorf("%w %s: %w", errAffectedFileUnreadable, f, err)
			}
			if info.IsDir() {
				return nil, fmt.Errorf("%w: %s", errAffectedPathIsDir, f)
			}
		}
		return affectedFiles, nil
	}

	specDir := filepath.Dir(specPath)
	files, err := detectChangedFiles(specDir, base)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		if base != "" {
			return nil, fmt.Errorf("no changed files detected (diff %s...HEAD is empty) — provide --affected-files", base)
		}
		return nil, fmt.Errorf("no changed files detected (staged+unstaged is empty) — use --base <tag> for committed changes, or --affected-files")
	}
	return files, nil
}

func detectChangedFiles(dir, base string) ([]string, error) {
	var allFiles []string

	// A base that git would read as an option must never be concatenated into
	// a revision range; runAudit rejects one up front, and this is the sink.
	if base != "" && !engine.SafeGitRev(base) {
		return nil, fmt.Errorf("invalid --base %q: must not start with %q", base, "-")
	}

	// auditDiffRanges is the single definition of "the comparison this audit
	// is about". The per-file diff stats handed to every role are derived from
	// the same list, so selection and stats can never describe different
	// ranges — the failure that let a committed change be reported as +0/-0.
	for i, rng := range auditDiffRanges(dir, base) {
		files := execGitDiffProbe(dir, "diff --name-only", append([]string{"diff", "--name-only"}, rng...)...)
		if i == 0 && len(files) == 0 && !gitExists(dir) {
			return nil, fmt.Errorf("not in a git repo — provide --affected-files or run inside a git repo")
		}
		allFiles = append(allFiles, files...)
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

	if len(filtered) > maxAutoDetectFiles {
		filtered = filtered[:maxAutoDetectFiles]
		// Notice, not Info: the caller asked for the whole changed set and got a
		// prefix of it. On the Info channel that truncation is invisible in JSON
		// mode, which is every piped run.
		output.Notice(fmt.Sprintf("more than %d files changed, auditing first %d", maxAutoDetectFiles, maxAutoDetectFiles))
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
		return nil, fmt.Errorf("no audit-able files in diff — only skipped types changed (%s). Use --base <tag> or --affected-files", strings.Join(exts, ", "))
	}

	return filtered, nil
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
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		f := strings.TrimSpace(scanner.Text())
		if f != "" {
			files = append(files, f)
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: stopped scanning git diff output early (%v); files after the over-long line were dropped (line cap is 64KB)\n", err)
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
// auto-detection applies: binary files, markdown, and task files are skipped.
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
	return true
}

func buildChecklist(specLines []string, specPath, findingsPath string) []checklistEntry {
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
	if findingsPath != "" {
		entries = append(entries, findingChecklistEntries(findingsPath)...)
	}

	return entries
}

// taskAcceptanceEntries reads the spec-adjacent task file and yields one
// task_acceptance checklist entry per task with a non-empty acceptance.
func taskAcceptanceEntries(specPath string) []checklistEntry {
	taskPath := strings.TrimSuffix(specPath, filepath.Ext(specPath)) + ".tasks.json"
	data, err := os.ReadFile(taskPath)
	if err != nil {
		// An absent task file is optional (audit may run on a spec with none);
		// a real read error (permissions, IO) is surfaced so it is not silently
		// treated as "no tasks".
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: cannot read task file %s; task acceptance entries were dropped (%v)\n", taskPath, err)
		}
		return nil
	}
	var tf struct {
		Tasks []struct {
			ID         string `json:"id"`
			Title      string `json:"title"`
			Acceptance string `json:"acceptance"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(data, &tf); err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot parse task file %s; task acceptance entries were dropped (%v)\n", taskPath, err)
		return nil
	}
	entries := make([]checklistEntry, 0, len(tf.Tasks))
	for _, task := range tf.Tasks {
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
func findingChecklistEntries(findingsPath string) []checklistEntry {
	rows := readFindings(findingsPath)
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
		output.Error(ExitFile, fmt.Sprintf("cannot read findings file: %s", path), err.Error())
		os.Exit(ExitFile)
		return nil
	}
	var results []findingRow
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping malformed line (invalid JSON) in %s\n", path)
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
		fmt.Fprintf(os.Stderr, "warning: stopped reading %s early (%v); rows after the over-long line were dropped (line cap is 64KB)\n", path, err)
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
		for _, part := range strings.Split(f, ",") {
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
		fmt.Fprintf(os.Stderr, "warning: commit sha %q was skipped; git would read it as an option\n", sha)
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
	suggested := suggestFilesFromTasks(specPath)
	output.ErrorExtras(ExitState, reason, map[string]any{
		"suggested_files": suggested,
	}, "pass --affected-files <paths>, or --affected-from-tasks to audit the files touched by done-task commits")
	os.Exit(ExitState)
}
