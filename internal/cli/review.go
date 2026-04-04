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
	"github.com/deligoez/tp/internal/output"
)

type reviewFinding struct {
	Severity   string `json:"severity"`
	Category   string `json:"category"`
	Location   string `json:"location"`
	Finding    string `json:"finding"`
	Suggestion string `json:"suggestion"`
}

type reviewPrompt struct {
	Role     string `json:"role"`
	Category string `json:"category"`
	Prompt   string `json:"prompt"`
}

type reviewLoop struct {
	Round            int    `json:"round"`
	MaxRounds        int    `json:"max_rounds"`
	Convergence      string `json:"convergence"`
	PreviousFindings int    `json:"previous_findings"`
	Instruction      string `json:"instruction"`
}

type reviewResult struct {
	Spec               string                     `json:"spec"`
	StructuredElements *engine.StructuredElements `json:"structured_elements,omitempty"`
	Perspective        string                     `json:"perspective,omitempty"`
	DocsPath           string                     `json:"docs_path,omitempty"`
	TestPath           string                     `json:"test_path,omitempty"`
	AffectedFiles      []string                   `json:"affected_files,omitempty"`
	AffectedSummary    *engine.AffectedSummary    `json:"affected_summary,omitempty"`
	DocsStructure      *docStructure              `json:"docs_structure,omitempty"`
	TestStructure      *docStructure              `json:"test_structure,omitempty"`
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
	var specRef bool
	var forceFlag bool

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
			mode := detectReviewMode(mergeMode, resolveMode, resolveAllMode, verifyMode, reportMode)
			if mode == "" {
				// Default review mode — requires exactly 1 spec arg
				if len(args) != 1 {
					output.Error(ExitUsage, "spec path required")
					os.Exit(ExitUsage)
					return nil
				}
				return runReview(cmd, args[0], round, findingsPath, perspective, docsPath, testPath, affectedFiles, finalRound, diffFrom, specRef)
			}
			if err := validateModeFlags(mode, round, findingsPath, affectedFiles, finalRound, diffFrom, specRef); err != nil {
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
				return runReviewVerify(args[0], findingsPath, affectedFiles, diffFrom, specRef)
			case "report":
				return runReviewReport(args)
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
	cmd.Flags().StringVar(&outputPath, "o", "", "Output file path (for --merge)")
	cmd.Flags().StringVar(&diffFrom, "diff-from", "", "Baseline spec for diff-based review (requires --round >= 2)")
	cmd.Flags().BoolVar(&specRef, "spec-ref", false, "Reference spec by path instead of inline content")
	cmd.Flags().BoolVar(&forceFlag, "force", false, "Force re-resolve already resolved findings")

	return cmd
}

// detectReviewMode returns the active mode name, or "" for default review.
// Returns error-mode name if multiple modes are active.
func detectReviewMode(merge, resolve, resolveAll, verify, report bool) string {
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
	if len(modes) == 0 {
		return ""
	}
	if len(modes) > 1 {
		return "conflict:" + modes[0] + "+" + modes[1]
	}
	return modes[0]
}

// validateModeFlags checks that modifier flags are compatible with the active mode.
func validateModeFlags(mode string, round int, findingsPath string, affectedFiles []string, finalRound bool, diffFrom string, specRef bool) error {
	if strings.HasPrefix(mode, "conflict:") {
		pair := strings.TrimPrefix(mode, "conflict:")
		return fmt.Errorf("--%s are mutually exclusive", strings.Replace(pair, "+", " and --", 1))
	}

	// Merge, resolve, resolve-all, report reject all modifier flags
	if mode == "merge" || mode == "resolve" || mode == "resolve-all" || mode == "report" {
		if round != 1 {
			return fmt.Errorf("--%s is mutually exclusive with --round", mode)
		}
		if findingsPath != "" && mode != "report" {
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
		if specRef {
			return fmt.Errorf("--%s is mutually exclusive with --spec-ref", mode)
		}
	}

	// Verify rejects --round and --final-round but allows --findings, --affected-files, --diff-from, --spec-ref
	if mode == "verify" {
		if round != 1 {
			return fmt.Errorf("--verify is mutually exclusive with --round (verification is not a numbered round)")
		}
		if finalRound {
			return fmt.Errorf("--verify is mutually exclusive with --final-round")
		}
	}

	return nil
}

// Stub functions for new modes — will be implemented in separate files.
func runReviewMerge(_ []string, _ string) error {
	output.Error(ExitUsage, "merge mode not yet implemented")
	os.Exit(ExitUsage)
	return nil
}

func runReviewResolve(_ []string, _ bool) error {
	output.Error(ExitUsage, "resolve mode not yet implemented")
	os.Exit(ExitUsage)
	return nil
}

func runReviewResolveAll(_ []string, _ bool) error {
	output.Error(ExitUsage, "resolve-all mode not yet implemented")
	os.Exit(ExitUsage)
	return nil
}

func runReviewVerify(_, _ string, _ []string, _ string, _ bool) error {
	output.Error(ExitUsage, "verify mode not yet implemented")
	os.Exit(ExitUsage)
	return nil
}

func runReviewReport(_ []string) error {
	output.Error(ExitUsage, "report mode not yet implemented")
	os.Exit(ExitUsage)
	return nil
}

func runReview(_ *cobra.Command, specPath string, round int, findingsPath, perspective, docsPath, testPath string, affectedFiles []string, finalRound bool, _ string, _ bool) error {
	validPerspectives := map[string]bool{"documentation": true, "testing": true, "code-audit": true}

	if perspective != "" && !validPerspectives[perspective] {
		output.Error(ExitUsage, fmt.Sprintf("invalid perspective: %q (must be 'documentation', 'testing', or 'code-audit')", perspective))
		os.Exit(ExitUsage)
		return nil
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

	specContent := readSpecContent(specPath)

	if perspective == "code-audit" {
		affectedContent := engine.ReadAffectedFiles(affectedFiles)
		summary := engine.BuildAffectedSummary(affectedFiles, affectedContent)
		prompt := generateCodeAuditPrompt(specContent, affectedContent)
		result := reviewResult{
			Spec:            specPath,
			Perspective:     perspective,
			AffectedFiles:   affectedFiles,
			AffectedSummary: summary,
			Prompts:         []reviewPrompt{prompt},
			ReviewLoop: reviewLoop{
				Round:            round,
				MaxRounds:        1,
				Convergence:      "single-pass code audit",
				PreviousFindings: 0,
				Instruction:      "Spawn a sub-agent with this prompt. Collect NDJSON findings. Feed findings back into spec revision or task acceptance updates.",
			},
		}
		return output.JSON(result)
	}

	if perspective == "documentation" {
		structureMap, files := walkDocTree(docsPath, ".md")
		specLines := strings.Split(specContent, "\n")
		ranked := rankFilesBySpecTerms(files, specLines, 15)
		docContent := readFilesContent(ranked, 5000, 30000)
		if len(affectedFiles) > 0 {
			affectedContent := engine.ReadAffectedFiles(affectedFiles)
			for path, content := range affectedContent {
				docContent[path] = content
			}
		}
		prompt := generateDocPlanPrompt(specContent, structureMap, docContent)
		summary := engine.BuildAffectedSummary(affectedFiles, nil)
		result := reviewResult{
			Spec:            specPath,
			Perspective:     perspective,
			DocsPath:        docsPath,
			AffectedFiles:   affectedFiles,
			AffectedSummary: summary,
			DocsStructure: &docStructure{
				TotalFiles:    len(files),
				ReviewedFiles: len(ranked),
				StructureMap:  structureMap,
			},
			Prompts: []reviewPrompt{prompt},
			ReviewLoop: reviewLoop{
				Round:            1,
				MaxRounds:        1,
				Convergence:      "single-pass plan generation",
				PreviousFindings: 0,
				Instruction:      "Spawn a sub-agent with this prompt. Collect the NDJSON plan. Review the plan for completeness, then append the plan to the spec.",
			},
		}
		return output.JSON(result)
	}

	if perspective == "testing" {
		structureMap, files := walkDocTree(testPath, "_test.go")
		specLines := strings.Split(specContent, "\n")
		ranked := rankFilesBySpecTerms(files, specLines, 15)
		testContent := readFilesContent(ranked, 5000, 20000)
		if len(affectedFiles) > 0 {
			affectedContent := engine.ReadAffectedFiles(affectedFiles)
			for path, content := range affectedContent {
				testContent[path] = content
			}
		}
		prompt := generateTestPlanPrompt(specContent, structureMap, testContent)
		summary := engine.BuildAffectedSummary(affectedFiles, nil)
		result := reviewResult{
			Spec:            specPath,
			Perspective:     perspective,
			TestPath:        testPath,
			AffectedFiles:   affectedFiles,
			AffectedSummary: summary,
			TestStructure: &docStructure{
				TotalFiles:    len(files),
				ReviewedFiles: len(ranked),
				StructureMap:  structureMap,
			},
			Prompts: []reviewPrompt{prompt},
			ReviewLoop: reviewLoop{
				Round:            1,
				MaxRounds:        1,
				Convergence:      "single-pass plan generation",
				PreviousFindings: 0,
				Instruction:      "Spawn a sub-agent with this prompt. Collect the NDJSON plan. Review the plan for completeness, then append the plan to the spec.",
			},
		}
		return output.JSON(result)
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

	if round > 1 && findingsPath == "" {
		output.Info(fmt.Sprintf("round %d without --findings: prompts will not exclude previously reported issues", round))
	}

	lines, headings, err := parseSpecFile(specPath)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	_, elems := engine.CheckStructuredElements(lines, headings)

	findings := make([]reviewFinding, 0)
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

	prompts := []reviewPrompt{
		generateImplementerPrompt(elems, specContent, round, summary, affectedSection, finalRound),
		generateTesterPrompt(elems, specContent, round, summary, affectedSection, finalRound),
		generateArchitectPrompt(elems, specContent, round, summary, affectedSection, finalRound),
	}

	uniqueCount := len(dedupFindings(findings))
	instruction := "For each prompt, spawn a sub-agent via the Agent tool. Collect JSON findings. If any critical/high severity, revise spec and re-run `tp review`. Stop after 2 rounds or when no new high-severity findings."
	if round < 2 && len(findings) > 0 {
		instruction = fmt.Sprintf("For each prompt, spawn a sub-agent via the Agent tool. Collect JSON findings. If any critical/high severity, revise spec and re-run `tp review --round 2 --findings <%s>`. Stop after 2 rounds or when no new high-severity findings.", findingsPath)
	} else if round >= 2 {
		instruction = fmt.Sprintf("Spawn sub-agents for each prompt. Collect findings. If any critical/high severity, revise spec and re-run `tp review --round %d --findings <combined.ndjson>`. Stop after max_rounds or when no new high-severity findings.", round+1)
	}

	result := reviewResult{
		Spec:               specPath,
		StructuredElements: elems,
		Prompts:            prompts,
		ReviewLoop: reviewLoop{
			Round:            round,
			MaxRounds:        2,
			Convergence:      "no new high-severity findings",
			PreviousFindings: uniqueCount,
			Instruction:      instruction,
		},
	}

	if len(affectedFiles) > 0 {
		result.AffectedFiles = affectedFiles
		result.AffectedSummary = engine.BuildAffectedSummary(affectedFiles, affectedContent)
	}

	return output.JSON(result)
}

const findingFormat = `
For each issue found, respond with one JSON object per line (NDJSON):
{"severity":"critical|high|medium|low","category":"completeness|ambiguity|consistency|feasibility|redundancy","location":"section heading or line number","finding":"what is wrong","suggestion":"how to fix it"}

Only report real issues. Do not generate findings just to appear thorough.`

func appendAffectedChecklist(b *strings.Builder, n int, hasAffectedFiles bool) {
	if hasAffectedFiles {
		fmt.Fprintf(b, "%d. For each state-dependent behavior in the affected files (disabled, loading, visibility, conditional rendering, error handling), verify the spec addresses it. What controls each condition?\n", n+1)
	}
}

func appendFinalRoundInstruction(b *strings.Builder) {
	b.WriteString("\nMANDATORY: Read every file in the Affected Files section line-by-line. For each state-dependent behavior (disabled, loading, conditional rendering, class binding, error handling), verify the spec explicitly addresses it. Do NOT report \"spec is solid\" unless you have verified every state-dependent element.\n")
}

func generateImplementerPrompt(elems *engine.StructuredElements, specContent string, round int, summary, affectedSection string, finalRound bool) reviewPrompt {
	var b strings.Builder
	if round >= 2 {
		fmt.Fprintf(&b, "You are a senior engineer who must implement this spec tomorrow. This is review round %d \u2014 focus ONLY on issues not previously reported. Your goal is to find requirements that are missing, underspecified, or impossible to implement as stated.\n\n", round)
	} else {
		b.WriteString("You are a senior engineer who must implement this spec tomorrow. Your goal is to find requirements that are missing, underspecified, or impossible to implement as stated.\n\n")
	}
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
		fmt.Fprintf(&b, "%d. Table '%s' (line %d, %d rows): For each row, could you implement it without asking clarifying questions? What edge cases are missing?\n", n, t.Heading, t.Line, t.Rows)
		n++
	}
	for _, nl := range elems.NumberedLists {
		fmt.Fprintf(&b, "%d. List '%s' (line %d, %d items, #1-#%d): For each item, is there enough detail to implement? What happens when it fails?\n", n, nl.Heading, nl.Line, nl.Items, nl.LastNum)
		n++
	}
	fmt.Fprintf(&b, "%d. What happens when the happy path fails? Where are the error handling gaps?\n", n)
	n++
	fmt.Fprintf(&b, "%d. Are there implicit assumptions that should be explicit?\n", n)
	n++
	appendAffectedChecklist(&b, n, affectedSection != "")

	if finalRound {
		appendFinalRoundInstruction(&b)
	}

	b.WriteString(findingFormat)
	if summary != "" {
		b.WriteString(findingFormatRound2)
	}

	return reviewPrompt{
		Role:     "implementer",
		Category: "completeness",
		Prompt:   b.String(),
	}
}

func generateTesterPrompt(elems *engine.StructuredElements, specContent string, round int, summary, affectedSection string, finalRound bool) reviewPrompt {
	var b strings.Builder
	if round >= 2 {
		fmt.Fprintf(&b, "You are a QA engineer who must write tests from this spec. This is review round %d \u2014 focus ONLY on issues not previously reported. Your goal is to find requirements that are ambiguous (two testers would write contradictory tests) or non-verifiable (cannot write a pass/fail test).\n\n", round)
	} else {
		b.WriteString("You are a QA engineer who must write tests from this spec. Your goal is to find requirements that are ambiguous (two testers would write contradictory tests) or non-verifiable (cannot write a pass/fail test).\n\n")
	}
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
		fmt.Fprintf(&b, "%d. Table '%s' (line %d, %d rows): Can you write a deterministic pass/fail test for each row?\n", n, t.Heading, t.Line, t.Rows)
		n++
	}
	for _, nl := range elems.NumberedLists {
		fmt.Fprintf(&b, "%d. List '%s' (line %d, #1-#%d): For each item, what is the expected output? What are the boundary conditions?\n", n, nl.Heading, nl.Line, nl.LastNum)
		n++
	}
	fmt.Fprintf(&b, "%d. Which requirements use vague language ('appropriate', 'reasonable', 'properly', 'should') that prevents writing deterministic tests?\n", n)
	n++
	fmt.Fprintf(&b, "%d. Could two engineers interpret any requirement differently enough to produce incompatible implementations?\n", n)
	n++
	appendAffectedChecklist(&b, n, affectedSection != "")

	if finalRound {
		appendFinalRoundInstruction(&b)
	}

	b.WriteString(findingFormat)
	if summary != "" {
		b.WriteString(findingFormatRound2)
	}

	return reviewPrompt{
		Role:     "tester",
		Category: "ambiguity",
		Prompt:   b.String(),
	}
}

func generateArchitectPrompt(elems *engine.StructuredElements, specContent string, round int, summary, affectedSection string, finalRound bool) reviewPrompt {
	var b strings.Builder
	if round >= 2 {
		fmt.Fprintf(&b, "You are a senior architect reviewing this spec for approval before implementation begins. This is review round %d \u2014 focus ONLY on issues not previously reported. Your goal is to find contradictions between sections, missing backward compatibility analysis, and feasibility issues.\n\n", round)
	} else {
		b.WriteString("You are a senior architect reviewing this spec for approval before implementation begins. Your goal is to find contradictions between sections, missing backward compatibility analysis, and feasibility issues.\n\n")
	}
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
	fmt.Fprintf(&b, "%d. Do any sections contradict each other? Are there conflicting requirements?\n", n)
	n++
	fmt.Fprintf(&b, "%d. Is there a 'What doesn't change' or backward compatibility section? If not, what existing behavior could break?\n", n)
	n++
	fmt.Fprintf(&b, "%d. Are there performance or scalability implications not addressed?\n", n)
	n++

	if len(elems.NumberedLists) > 1 {
		names := make([]string, len(elems.NumberedLists))
		for i, nl := range elems.NumberedLists {
			names[i] = fmt.Sprintf("'%s'", nl.Heading)
		}
		fmt.Fprintf(&b, "%d. Cross-reference lists %s \u2014 are there items in one but not the other? Anything missing from the union?\n", n, strings.Join(names, " and "))
		n++
	}

	if len(elems.Tables) > 1 {
		fmt.Fprintf(&b, "%d. Do the %d tables have consistent column semantics? Could any rows be merged or are any missing?\n", n, len(elems.Tables))
		n++
	}

	fmt.Fprintf(&b, "%d. Does the implementation order match the dependency graph? Can any steps be parallelized?\n", n)
	n++
	appendAffectedChecklist(&b, n, affectedSection != "")

	if finalRound {
		appendFinalRoundInstruction(&b)
	}

	b.WriteString(findingFormat)
	if summary != "" {
		b.WriteString(findingFormatRound2)
	}

	return reviewPrompt{
		Role:     "architect",
		Category: "consistency",
		Prompt:   b.String(),
	}
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

func dedupFindings(findings []reviewFinding) []reviewFinding {
	seen := make(map[string]bool)
	result := make([]reviewFinding, 0, len(findings))
	for _, f := range findings {
		key := f.Category + "::" + f.Location
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

	severityOrder := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3, "unknown": 4}
	sort.SliceStable(deduped, func(i, j int) bool {
		si, sj := severityOrder[deduped[i].Severity], severityOrder[deduped[j].Severity]
		if si != sj {
			return si < sj
		}
		return deduped[i].Category < deduped[j].Category
	})

	sevAbbr := map[string]string{
		"critical": "[CRIT]", "high": "[HIGH]", "medium": "[MED]", "low": "[LOW]", "unknown": "[???]",
	}

	findingsCap := 50
	shown := deduped
	omitted := 0
	if len(deduped) > findingsCap {
		shown = deduped[:findingsCap]
		omitted = len(deduped) - findingsCap
	}

	var b strings.Builder
	b.WriteString("IMPORTANT \u2014 Previous Review Round(s) Found and Addressed:\n")
	b.WriteString("The following issues were identified in previous round(s) and have been addressed by the spec author.\n")
	b.WriteString("DO NOT re-report these issues unless the fix introduced a NEW, DIFFERENT problem.\n\n")

	for _, f := range shown {
		abbr := sevAbbr[f.Severity]
		if abbr == "" {
			abbr = "[???]"
		}
		text := f.Finding
		if len(text) > 80 {
			text = text[:80] + "..."
		}
		fmt.Fprintf(&b, "  %s %s \u2014 %s: %s\n", abbr, f.Category, f.Location, text)
	}

	if omitted > 0 {
		fmt.Fprintf(&b, "\n  ... and %d more (omitted for brevity)\n", omitted)
	}

	b.WriteString("\nFocus ONLY on issues NOT listed above. If you find no new issues, respond with an empty array (just []).\n")
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
	content := strings.Join(lines, "\n")
	if len(content) > specContentCap {
		content = content[:specContentCap] + fmt.Sprintf("\n[...truncated at %d chars]", specContentCap)
	}
	return content
}
