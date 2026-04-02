package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
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
	StructuredElements *engine.StructuredElements `json:"structured_elements"`
	Prompts            []reviewPrompt             `json:"prompts"`
	ReviewLoop         reviewLoop                 `json:"review_loop"`
}

func newReviewCmd() *cobra.Command {
	var round int
	var findingsPath string

	cmd := &cobra.Command{
		Use:   "review <spec.md>",
		Short: "Generate adversarial review prompts for spec quality",
		Long: `Parses a spec and generates 3 targeted review prompts (implementer, tester, architect).
Agent feeds these to sub-agents for adversarial review. Findings drive spec revision.
Output: {spec, structured_elements, prompts[], review_loop}`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReview(cmd, args[0], round, findingsPath)
		},
	}

	cmd.Flags().IntVar(&round, "round", 1, "Current review round number (1-indexed)")
	cmd.Flags().StringVar(&findingsPath, "findings", "", "Path to NDJSON file with previous round findings")

	return cmd
}

func runReview(_ *cobra.Command, specPath string, round int, findingsPath string) error {
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

	specContent := readSpecContent(specPath)

	findings := make([]reviewFinding, 0)
	if findingsPath != "" {
		findings = parseFindingsFile(findingsPath)
	}

	summary := buildFindingsSummary(findings)

	prompts := []reviewPrompt{
		generateImplementerPrompt(elems, specContent, round, summary),
		generateTesterPrompt(elems, specContent, round, summary),
		generateArchitectPrompt(elems, specContent, round, summary),
	}

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
			PreviousFindings: len(findings),
			Instruction:      instruction,
		},
	}

	return output.JSON(result)
}

const findingFormat = `
For each issue found, respond with one JSON object per line (NDJSON):
{"severity":"critical|high|medium|low","category":"completeness|ambiguity|consistency|feasibility|redundancy","location":"section heading or line number","finding":"what is wrong","suggestion":"how to fix it"}

Only report real issues. Do not generate findings just to appear thorough.`

func generateImplementerPrompt(elems *engine.StructuredElements, specContent string, round int, summary string) reviewPrompt {
	var b strings.Builder
	if round >= 2 {
		b.WriteString(fmt.Sprintf("You are a senior engineer who must implement this spec tomorrow. This is review round %d \u2014 focus ONLY on issues not previously reported. Your goal is to find requirements that are missing, underspecified, or impossible to implement as stated.\n\n", round))
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

func generateTesterPrompt(elems *engine.StructuredElements, specContent string, round int, summary string) reviewPrompt {
	var b strings.Builder
	if round >= 2 {
		b.WriteString(fmt.Sprintf("You are a QA engineer who must write tests from this spec. This is review round %d \u2014 focus ONLY on issues not previously reported. Your goal is to find requirements that are ambiguous (two testers would write contradictory tests) or non-verifiable (cannot write a pass/fail test).\n\n", round))
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

func generateArchitectPrompt(elems *engine.StructuredElements, specContent string, round int, summary string) reviewPrompt {
	var b strings.Builder
	if round >= 2 {
		b.WriteString(fmt.Sprintf("You are a senior architect reviewing this spec for approval before implementation begins. This is review round %d \u2014 focus ONLY on issues not previously reported. Your goal is to find contradictions between sections, missing backward compatibility analysis, and feasibility issues.\n\n", round))
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

	cap := 50
	shown := deduped
	omitted := 0
	if len(deduped) > cap {
		shown = deduped[:cap]
		omitted = len(deduped) - cap
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
	// Cap at 10000 chars to keep prompts reasonable
	if len(content) > 10000 {
		content = content[:10000] + "\n[...truncated at 10000 chars]"
	}
	return content
}
