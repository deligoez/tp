package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

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
	Role     string `json:"role"`
	Category string `json:"category"`
	Prompt   string `json:"prompt"`
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
	Prompts            []reviewPrompt             `json:"prompts"`
	ReviewLoop         reviewLoop                 `json:"review_loop"`
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
--resolve/--resolve-all: mark findings as fixed/wontfix/duplicate.
--verify: lightweight verification prompt.
--report: cross-round convergence report.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := detectReviewMode(mergeMode, resolveMode, resolveAllMode, verifyMode, reportMode, recordPath != "", statusMode)
			if checkFlag && mode != "status" {
				output.Error(ExitUsage, "--check requires --status")
				os.Exit(ExitUsage)
				return nil
			}
			if noState && (mode == "record" || mode == "status" || checkFlag) {
				output.Error(ExitUsage, "--no-state cannot be combined with --record, --status, or --check")
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
				return runReview(cmd, args[0], round, findingsPath, perspective, docsPath, testPath, affectedFiles, finalRound, diffFrom, specInline, noState)
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
				return runReviewVerify(args[0], findingsPath, affectedFiles, diffFrom, specInline)
			case "report":
				return runReviewReport(args)
			case "record":
				if len(args) != 1 {
					output.Error(ExitUsage, "spec path required for --record")
					os.Exit(ExitUsage)
					return nil
				}
				return runReviewRecord(args[0], recordPath)
			case "status":
				if len(args) != 1 {
					output.Error(ExitUsage, "spec path required for --status")
					os.Exit(ExitUsage)
					return nil
				}
				return runReviewStatus(args[0], checkFlag)
			default:
				output.Error(ExitUsage, fmt.Sprintf("unknown mode: %s", mode))
				os.Exit(ExitUsage)
				return nil
			}
		},
	}

	// Default review flags
	cmd.Flags().IntVar(&round, "round", 1, "Current review round number (1-indexed)")
	cmd.Flags().StringVar(&findingsPath, "findings", "", "Path to NDJSON file with previous round findings")
	cmd.Flags().StringVar(&perspective, "perspective", "", "Review perspective: documentation, testing, or code-audit")
	cmd.Flags().StringVar(&docsPath, "docs-path", "", "Path to documentation directory (required with --perspective documentation)")
	cmd.Flags().StringVar(&testPath, "test-path", "", "Path to test directory (required with --perspective testing)")
	cmd.Flags().StringArrayVar(&affectedFiles, "affected-files", nil, "Paths to source files to inject into review context")
	cmd.Flags().BoolVar(&finalRound, "final-round", false, "Force mandatory code read-through in review prompts (requires round >= 2)")

	// New mode flags
	cmd.Flags().BoolVar(&mergeMode, "merge", false, "Merge and deduplicate findings from NDJSON files")
	cmd.Flags().BoolVar(&resolveMode, "resolve", false, "Mark a finding as fixed/wontfix/duplicate")
	cmd.Flags().BoolVar(&resolveAllMode, "resolve-all", false, "Mark all unresolved findings with a status")
	cmd.Flags().BoolVar(&verifyMode, "verify", false, "Generate lightweight verification prompt")
	cmd.Flags().BoolVar(&reportMode, "report", false, "Generate cross-round convergence report")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path (for --merge)")
	cmd.Flags().StringVar(&diffFrom, "diff-from", "", "Baseline spec for diff-based review (requires --round >= 2)")
	cmd.Flags().BoolVar(&specInline, "spec-inline", false, "Embed full spec content inline (default: reference by path)")
	cmd.Flags().BoolVar(&forceFlag, "force", false, "Force re-resolve already resolved findings")
	cmd.Flags().StringVar(&recordPath, "record", "", "Record a review round from an NDJSON findings file")
	cmd.Flags().BoolVar(&statusMode, "status", false, "Show recorded review rounds and convergence state")
	cmd.Flags().BoolVar(&checkFlag, "check", false, "With --status: run registered mechanical checks")
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
	if strings.HasPrefix(mode, "conflict:") {
		pair := strings.TrimPrefix(mode, "conflict:")
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

func runReview(cmd *cobra.Command, specPath string, round int, findingsPath, perspective, docsPath, testPath string, affectedFiles []string, finalRound bool, diffFrom string, specInline, noState bool) error {
	validPerspectives := map[string]bool{"documentation": true, "testing": true, "code-audit": true, "regression": true}

	if perspective != "" && !validPerspectives[perspective] {
		output.Error(ExitUsage, fmt.Sprintf("invalid perspective: %q (must be 'documentation', 'testing', 'code-audit', or 'regression')", perspective))
		os.Exit(ExitUsage)
		return nil
	}

	if perspective == "regression" {
		return runReviewRegression(specPath, diffFrom, findingsPath)
	}

	if perspective != "" && perspective != "code-audit" && (round != 1 || findingsPath != "") {
		output.Error(ExitUsage, "--perspective is mutually exclusive with --round/--findings (except code-audit)")
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
				exitStateError(stErr)
				return nil
			}
			rounds := []engine.ReviewRound{}
			if stBudget != nil {
				rounds = stBudget.ReviewRounds
			}
			refuseIfBudgetExhausted("review", specPath, rounds, wfBudget.ReviewMaxRounds, wfBudget.ReviewCleanRounds)
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
			output.Error(ExitFile, fmt.Sprintf("docs path not found or not a directory: %s", docsPath))
			os.Exit(ExitFile)
			return nil
		}
	}

	if testPath != "" {
		info, err := os.Stat(testPath)
		if err != nil || !info.IsDir() {
			output.Error(ExitFile, fmt.Sprintf("test path not found or not a directory: %s", testPath))
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
			output.Error(ExitFile, fmt.Sprintf("diff baseline not found: %s", diffFrom))
			os.Exit(ExitFile)
			return nil
		}
	}

	affectedFiles = engine.DedupPaths(affectedFiles)
	for _, f := range affectedFiles {
		info, err := os.Stat(f)
		if err != nil {
			output.Error(ExitFile, fmt.Sprintf("affected file not found: %s", f))
			os.Exit(ExitFile)
			return nil
		}
		if info.IsDir() {
			output.Error(ExitFile, fmt.Sprintf("affected path is a directory, not a file: %s", f))
			os.Exit(ExitFile)
			return nil
		}
	}

	// Build spec content based on mode: spec-ref (default), diff-based, or inline
	var specContent string
	var diffResult *engine.DiffResult
	switch {
	case diffFrom != "":
		baseData, err := os.ReadFile(diffFrom)
		if err != nil {
			output.Error(ExitFile, fmt.Sprintf("cannot read diff baseline: %s", diffFrom))
			os.Exit(ExitFile)
			return nil
		}
		currData, err := os.ReadFile(specPath)
		if err != nil {
			output.Error(ExitFile, fmt.Sprintf("cannot read spec: %s", specPath))
			os.Exit(ExitFile)
			return nil
		}
		dr := engine.DiffSections(engine.BlankFrontmatterLines(strings.Split(string(baseData), "\n")), engine.BlankFrontmatterLines(strings.Split(string(currData), "\n")))
		diffResult = &dr
		specContent = buildDiffSpecContent(diffResult)
		if len(diffResult.Changed) == 0 && len(diffResult.Removed) == 0 {
			output.Info("no changes detected between baseline and current spec — review may be unnecessary")
		}
	case specInline:
		specContent = readSpecContent(specPath)
	default:
		// Default: reference mode (spec-ref) — omit inline content
		specData, err := os.ReadFile(specPath)
		if err != nil {
			output.Error(ExitFile, fmt.Sprintf("cannot read spec: %s", specPath))
			os.Exit(ExitFile)
			return nil
		}
		lineCount := strings.Count(string(specData), "\n") + 1
		absPath, _ := filepath.Abs(specPath)
		headings, _ := engine.ParseHeadings(specPath)
		specContent = buildSpecRefContent(absPath, lineCount, headings)
	}

	switch perspective {
	case "code-audit":
		return runReviewCodeAudit(specPath, specContent, affectedFiles, round)
	case "documentation":
		return runReviewDocPlan(specPath, specContent, docsPath, affectedFiles)
	case "testing":
		return runReviewTestPlan(specPath, specContent, testPath, affectedFiles)
	}

	if round < 1 {
		output.Error(ExitUsage, "round must be >= 1")
		os.Exit(ExitUsage)
		return nil
	}

	if findingsPath != "" {
		if _, err := os.Stat(findingsPath); os.IsNotExist(err) {
			output.Error(ExitUsage, fmt.Sprintf("findings file not found: %s", findingsPath))
			os.Exit(ExitUsage)
			return nil
		}
	}

	// State-backed round lifecycle (default three-role mode): tp numbers the
	// round, snapshots the spec, and injects previous findings automatically.
	statePrevFindings := make([]reviewFinding, 0)
	var stateRequired, stateConsecutive *int
	var stateConverged, stateStale *bool
	var reviewSt *engine.ReviewState
	if !noState {
		st, stErr := engine.LoadReviewState(specPath)
		if stErr != nil {
			exitStateError(stErr)
			return nil
		}
		reviewSt = st
		recorded := 0
		if st != nil {
			recorded = len(st.ReviewRounds)
		}
		stateRound := recorded + 1
		if cmd.Flags().Changed("round") && round != stateRound {
			output.Error(ExitUsage, fmt.Sprintf("--round %d conflicts with the state-derived round %d", round, stateRound), "drop --round, or use --no-state for manual round numbering")
			os.Exit(ExitUsage)
			return nil
		}
		round = stateRound

		if _, err := engine.EnsureReviewState(specPath); err != nil {
			exitStateError(err)
			return nil
		}
		specBytes, readErr := os.ReadFile(specPath)
		if readErr != nil {
			output.Error(ExitFile, fmt.Sprintf("cannot read spec: %v", readErr))
			os.Exit(ExitFile)
			return nil
		}
		snapshotPath := filepath.Join(engine.ReviewStateDir(specPath), fmt.Sprintf("snapshot-round-%d.md", stateRound))
		if writeErr := os.WriteFile(snapshotPath, specBytes, 0o600); writeErr != nil {
			output.Error(ExitFile, fmt.Sprintf("cannot write snapshot: %v", writeErr))
			os.Exit(ExitFile)
			return nil
		}

		// State-derived review_loop fields
		wfState, _ := engine.ResolveWorkflow(specPath, flagFile)
		if specHash, hashErr := engine.SpecHash(specPath); hashErr == nil {
			rounds := []engine.ReviewRound{}
			if st != nil {
				rounds = st.ReviewRounds
			}
			req := wfState.ReviewCleanRounds
			cc := engine.ConsecutiveClean(rounds)
			conv := engine.Converged(rounds, req, specHash)
			stale := engine.StateStale(rounds, specHash)
			stateRequired, stateConsecutive, stateConverged, stateStale = &req, &cc, &conv, &stale
		}

		// Previous findings from rounds 1..R-1 unless --findings overrides
		if findingsPath == "" && st != nil {
			for i := range st.ReviewRounds {
				r := st.ReviewRounds[i]
				rows, found := engine.LoadRoundRows(specPath, &r)
				if !found {
					output.Info(fmt.Sprintf("round %d file %s is missing; skipping its rows", r.Round, r.File))
					continue
				}
				for _, row := range rows {
					statePrevFindings = append(statePrevFindings, findingFromRow(row))
				}
			}
			statePrevFindings = dedupFindings(statePrevFindings)
		}
	}

	if round > 1 && findingsPath == "" && noState {
		output.Info(fmt.Sprintf("round %d without --findings: prompts will not exclude previously reported issues", round))
	}

	lines, headings, err := parseSpecFile(specPath)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	_, elems := engine.CheckStructuredElements(lines, headings)

	findings := statePrevFindings
	if findingsPath != "" {
		findings = parseFindingsFile(findingsPath)
	}

	summary := buildFindingsSummary(findings)

	var affectedSection string
	var affectedContent map[string]string
	if len(affectedFiles) > 0 {
		affectedContent = engine.ReadAffectedFilesBudgetAware(affectedFiles, summary, specContent)
		affectedSection = engine.BuildAffectedSection(affectedContent)
	}

	// Mechanical checks: workflow-derived (not state-derived), run before
	// prompt generation even under --no-state; failures never abort generation
	wfChecks, checksTaskFile := engine.ResolveWorkflow(specPath, flagFile)
	var mechChecks []map[string]any
	if len(wfChecks.Checks) > 0 {
		mechChecks, _ = runMechanicalChecks(&wfChecks, checksTaskFile)
	}

	prompts, regressionIncluded := buildReviewPrompts(specPath, elems, specContent, round, summary, affectedSection, finalRound, &wfChecks, diffFrom, noState, reviewSt)

	uniqueCount := len(dedupFindings(findings))
	instruction := "For each prompt, spawn a sub-agent via the Agent tool. Collect JSON findings. If any critical/high severity, revise spec and re-run `tp review`. Stop after 2 rounds or when no new high-severity findings."
	if round < 2 && len(findings) > 0 {
		instruction = fmt.Sprintf("For each prompt, spawn a sub-agent via the Agent tool. Collect JSON findings. If any critical/high severity, revise spec and re-run `tp review --round 2 --findings <%s>`. Stop after 2 rounds or when no new high-severity findings.", findingsPath)
	} else if round >= 2 {
		instruction = fmt.Sprintf("Spawn sub-agents for each prompt. Collect findings. If any critical/high severity, revise spec and re-run `tp review --round %d --findings <combined.ndjson>`. Stop after max_rounds or when no new high-severity findings.", round+1)
	}

	if !specInline {
		absPath, _ := filepath.Abs(specPath)
		instruction += " Read the spec at " + absPath + " before processing each prompt."
	}

	convergence := "no new high-severity findings"
	if noState {
		convergence += " (convergence is not being recorded: --no-state)"
		instruction += " Convergence is not being recorded (--no-state)."
	} else {
		required := 2
		if stateRequired != nil {
			required = *stateRequired
		}
		convergence = fmt.Sprintf("no findings surviving verification (any severity) in %d consecutive rounds", required)
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
	if len(wfChecks.Checks) > 0 {
		instruction += " If any mechanical check failed, fix those failures before spawning sub-agents."
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

	if !specInline {
		absPath, _ := filepath.Abs(specPath)
		result.SpecRef = true
		result.SpecPath = absPath
	}

	if len(affectedFiles) > 0 {
		result.AffectedFiles = affectedFiles
		result.AffectedSummary = engine.BuildAffectedSummary(affectedFiles, affectedContent)
	}

	return output.JSON(result)
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

func appendSpecOnlyDisclaimer(b *strings.Builder, affectedSection string) {
	if affectedSection == "" {
		b.WriteString(specOnlyDisclaimer)
	}
}

// runReviewCodeAudit emits the single-pass code-audit perspective prompt.
func runReviewCodeAudit(specPath, specContent string, affectedFiles []string, round int) error {
	affectedContent := engine.ReadAffectedFiles(affectedFiles)
	summary := engine.BuildAffectedSummary(affectedFiles, affectedContent)
	prompt := generateCodeAuditPrompt(specContent, affectedContent)
	return output.JSON(reviewResult{
		Spec:            specPath,
		Perspective:     "code-audit",
		AffectedFiles:   affectedFiles,
		AffectedSummary: summary,
		Prompts:         []reviewPrompt{prompt},
		ReviewLoop: reviewLoop{
			Round:            round,
			Convergence:      "single-pass code audit",
			PreviousFindings: 0,
			Instruction:      "Spawn a sub-agent with this prompt. Collect NDJSON findings. Feed findings back into spec revision or task acceptance updates.",
		},
	})
}

// runReviewDocPlan emits the single-pass documentation-plan perspective prompt.
func runReviewDocPlan(specPath, specContent, docsPath string, affectedFiles []string) error {
	structureMap, files := walkDocTree(docsPath, ".md")
	ranked := rankFilesBySpecTerms(files, strings.Split(specContent, "\n"), 15)
	docContent := readFilesContent(ranked, 5000, 30000)
	if len(affectedFiles) > 0 {
		for path, content := range engine.ReadAffectedFiles(affectedFiles) {
			docContent[path] = content
		}
	}
	prompt := generateDocPlanPrompt(specContent, structureMap, docContent)
	return output.JSON(reviewResult{
		Spec:            specPath,
		Perspective:     "documentation",
		DocsPath:        docsPath,
		AffectedFiles:   affectedFiles,
		AffectedSummary: engine.BuildAffectedSummary(affectedFiles, nil),
		DocsStructure:   &docStructure{TotalFiles: len(files), ReviewedFiles: len(ranked), StructureMap: structureMap},
		Prompts:         []reviewPrompt{prompt},
		ReviewLoop: reviewLoop{
			Round:            1,
			Convergence:      "single-pass plan generation",
			PreviousFindings: 0,
			Instruction:      "Spawn a sub-agent with this prompt. Collect the NDJSON plan. Review the plan for completeness, then append the plan to the spec.",
		},
	})
}

// runReviewTestPlan emits the single-pass test-plan perspective prompt.
func runReviewTestPlan(specPath, specContent, testPath string, affectedFiles []string) error {
	structureMap, files := walkDocTree(testPath, "_test.go")
	ranked := rankFilesBySpecTerms(files, strings.Split(specContent, "\n"), 15)
	testContent := readFilesContent(ranked, 5000, 20000)
	if len(affectedFiles) > 0 {
		for path, content := range engine.ReadAffectedFiles(affectedFiles) {
			testContent[path] = content
		}
	}
	prompt := generateTestPlanPrompt(specContent, structureMap, testContent)
	return output.JSON(reviewResult{
		Spec:            specPath,
		Perspective:     "testing",
		TestPath:        testPath,
		AffectedFiles:   affectedFiles,
		AffectedSummary: engine.BuildAffectedSummary(affectedFiles, nil),
		TestStructure:   &docStructure{TotalFiles: len(files), ReviewedFiles: len(ranked), StructureMap: structureMap},
		Prompts:         []reviewPrompt{prompt},
		ReviewLoop: reviewLoop{
			Round:            1,
			Convergence:      "single-pass plan generation",
			PreviousFindings: 0,
			Instruction:      "Spawn a sub-agent with this prompt. Collect the NDJSON plan. Review the plan for completeness, then append the plan to the spec.",
		},
	})
}

// buildReviewPrompts emits the round's review prompts: one per active reviewer
// role from the domain-filtered corpus with the spec-frontmatter overrides
// applied, plus the appended changed-sections block, the auto-included regression
// prompt (round >= 2 with a diff or fixed findings), and the mechanized-class
// exclusion. Returns the prompts and whether the regression prompt was included.
func buildReviewPrompts(specPath string, elems *engine.StructuredElements, specContent string, round int, summary, affectedSection string, finalRound bool, wfChecks *model.Workflow, diffFrom string, noState bool, reviewSt *engine.ReviewState) (prompts []reviewPrompt, regressionIncluded bool) {
	fmState := engine.ParseFrontmatter(specPath)

	// Emit one prompt per active reviewer role from the domain-filtered corpus
	// (§7.1). A malformed reviewer role aborts review (§3.6, exit 3).
	activeRoles, corpusWarnings, corpusErr := engine.ResolveActiveCorpus(filepath.Dir(specPath), fmState.Domain, engine.PhaseReviewers)
	if corpusErr != nil {
		output.Error(ExitFile, corpusErr.Error(), "repair or delete the offending role file under .tp/reviewers/")
		os.Exit(ExitFile)
	}
	for _, w := range corpusWarnings {
		output.Info(w)
	}
	// Layer the spec-frontmatter role overrides (tp.review_roles / legacy lens
	// shim) onto each active role's corpus focus (§10.2-10.4).
	activeRoles, overrideWarnings := engine.ResolveOverrideFocus(activeRoles, fmState, engine.PhaseReviewers)
	for _, w := range overrideWarnings {
		output.Info(w)
	}
	prompts = make([]reviewPrompt, 0, len(activeRoles)+1)
	for i := range activeRoles {
		prompts = append(prompts, generateCorpusReviewPrompt(&activeRoles[i], elems, specContent, round, summary, affectedSection, finalRound))
	}

	// Changed-sections block: explicit --diff-from overrides the baseline and
	// forces the block at any round; otherwise the newest earlier snapshot is
	// the baseline from round 2 on.
	diffBlock := ""
	var diffDr engine.DiffResult
	diffLabel := ""
	switch {
	case diffFrom != "":
		diffDr = engine.DiffSections(diffLinesOf(diffFrom), diffLinesOf(specPath))
		diffLabel = "baseline " + diffFrom
		diffBlock = buildChangedSectionsBlock(&diffDr, diffLabel)
	case !noState && round >= 2:
		if snapRound, snapPath := newestEarlierSnapshot(specPath, round); snapPath != "" {
			diffDr = engine.DiffSections(diffLinesOf(snapPath), diffLinesOf(specPath))
			diffLabel = fmt.Sprintf("round %d", snapRound)
			diffBlock = buildChangedSectionsBlock(&diffDr, diffLabel)
		} else {
			output.Info("no earlier snapshot exists; changed-sections block omitted")
		}
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
				Prompt:   buildRegressionPrompt(&diffDr, diffLabel, fixed),
			})
			regressionIncluded = true
		}
	}

	// Prompt exclusion: reviewers stop looking for mechanized classes.
	if len(wfChecks.Checks) > 0 {
		classes := make([]string, 0, len(wfChecks.Checks))
		for i := range wfChecks.Checks {
			classes = append(classes, wfChecks.Checks[i].Class)
		}
		exclusion := "\n\nMechanically checked classes — do NOT report findings of these classes: " + strings.Join(classes, ", ")
		for i := range prompts {
			prompts[i].Prompt += exclusion
		}
	}

	return prompts, regressionIncluded
}

// generateCorpusReviewPrompt renders one review prompt for a corpus role,
// assembling the role's instructions (persona) and focus questions with the
// shared, role-neutral scaffolding: the spec-only disclaimer, the previous-round
// findings summary, the spec content, the structured-element inventory, the
// affected-files checklist, and the finding format. It replaces the pre-v0.25.0
// hardcoded implementer/tester/architect generators (§7.1); the role's failure
// lens now comes entirely from its instructions and focus, not from Go.
func generateCorpusReviewPrompt(role *model.Role, elems *engine.StructuredElements, specContent string, round int, summary, affectedSection string, finalRound bool) reviewPrompt {
	var b strings.Builder
	if round >= 2 {
		fmt.Fprintf(&b, "%s This is review round %d — focus ONLY on issues not previously reported.\n\n", role.Instructions, round)
	} else {
		b.WriteString(role.Instructions + "\n\n")
	}
	appendSpecOnlyDisclaimer(&b, affectedSection)
	if summary != "" {
		b.WriteString(summary)
		b.WriteString("\n")
	}
	b.WriteString("Spec content:\n---\n")
	b.WriteString(specContent)
	b.WriteString("\n---\n\n")
	if affectedSection != "" {
		b.WriteString(affectedSection)
	}
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
	appendAffectedChecklist(&b, n, affectedSection != "")

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
// severity; audit findings additionally carry status ∈ PASS/PARTIAL/FAIL.
func outputContractInstruction(role, phase string) string {
	var b strings.Builder
	b.WriteString("\n\nOutput contract — stamp EVERY finding with the full contract:\n")
	fmt.Fprintf(&b, "- role: %q (this prompt's role, so inter-role overlap can be attributed)\n", role)
	b.WriteString("- location: a section anchor such as \"§3.2\" — the first §<n>(.<n>)* token — so findings dedup by section\n")
	b.WriteString("- class: a kebab-case slug naming the failure class (the dedup/cluster key)\n")
	b.WriteString("- severity: one of critical, high, medium, low\n")
	if phase == engine.PhaseAuditors {
		b.WriteString("- status: one of PASS, PARTIAL, FAIL\n")
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

func parseFindingsFile(path string) []reviewFinding {
	f, err := os.Open(path)
	if err != nil {
		return make([]reviewFinding, 0)
	}
	defer f.Close()

	findings := make([]reviewFinding, 0)
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var finding reviewFinding
		if err := json.Unmarshal([]byte(line), &finding); err != nil {
			output.Info(fmt.Sprintf("findings line %d: skipping invalid JSON", lineNum))
			continue
		}
		if finding.Severity == "" {
			finding.Severity = "unknown"
		}
		findings = append(findings, finding)
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
			maxShow := 10
			if len(highResolved) < maxShow {
				maxShow = len(highResolved)
			}
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

func rankFilesBySpecTerms(files, specLines []string, maxCount int) []string {
	if len(files) == 0 {
		return files
	}

	terms := make(map[string]bool)
	for _, line := range specLines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
			term := strings.TrimLeft(trimmed, "# ")
			term = strings.TrimSpace(term)
			words := strings.Fields(term)
			for _, w := range words {
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
		for _, cfg := range []string{"config.js", "config.ts", "config.mts", "config.mjs"} {
			if lower == cfg {
				return true
			}
		}
		return false
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

func readFilesContent(files []string, maxPerFile, maxTotal int) map[string]string {
	result := make(map[string]string)
	total := 0
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
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

func generateDocPlanPrompt(specContent, structureMap string, fileContents map[string]string) reviewPrompt {
	var b strings.Builder

	b.WriteString("You are a technical writer planning documentation changes for a new feature. Your goal is to compare the specification against existing documentation and produce a structured plan of changes needed.\n\n")
	b.WriteString(specOnlyDisclaimer)
	b.WriteString("Spec content:\n---\n")
	b.WriteString(specContent)
	b.WriteString("\n---\n\n")

	b.WriteString("Documentation structure:\n")
	b.WriteString(structureMap)
	b.WriteString("\n")

	if len(fileContents) > 0 {
		b.WriteString("Existing documentation files (selected by relevance):\n")
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

	b.WriteString(docChecklist)
	b.WriteString(docOutputFormat)

	return reviewPrompt{
		Role:     "documentation-planner",
		Category: "completeness",
		Prompt:   b.String(),
	}
}

func generateTestPlanPrompt(specContent, structureMap string, fileContents map[string]string) reviewPrompt {
	var b strings.Builder

	b.WriteString("You are a QA engineer planning test coverage for a new feature. Your goal is to analyze the specification and produce a structured test plan covering all requirements.\n\n")
	b.WriteString(specOnlyDisclaimer)
	b.WriteString("Spec content:\n---\n")
	b.WriteString(specContent)
	b.WriteString("\n---\n\n")

	b.WriteString("Test structure:\n")
	b.WriteString(structureMap)
	b.WriteString("\n")

	if len(fileContents) > 0 {
		b.WriteString("Existing test files (selected by relevance):\n")
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

	b.WriteString(testChecklist)
	b.WriteString(testOutputFormat)

	return reviewPrompt{
		Role:     "test-planner",
		Category: "completeness",
		Prompt:   b.String(),
	}
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
{"id":"ca-001","file":"path/to/file","line":42,"pattern":":disabled","current_behavior":"isFormLocked || isPhoneCheckInProgress","spec_coverage":"partial","finding":"spec removes isPhoneCheckInProgress but phone input still references it","suggestion":"Add acceptance: phone input :disabled only when isFormLocked","severity":"high","category":"gap"}

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

func readSpecContent(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	lines = engine.BlankFrontmatterLines(lines)
	content := strings.Join(lines, "\n")
	if len(content) > specContentCap {
		content = content[:specContentCap] + fmt.Sprintf("\n[...truncated at %d chars]", specContentCap)
	}
	return content
}
