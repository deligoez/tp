package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

type reviewPrompt struct {
	Role     string `json:"role"`
	Category string `json:"category"`
	Prompt   string `json:"prompt"`
}

type reviewLoop struct {
	MaxRounds   int    `json:"max_rounds"`
	Convergence string `json:"convergence"`
	Instruction string `json:"instruction"`
}

type reviewResult struct {
	Spec               string                    `json:"spec"`
	StructuredElements *engine.StructuredElements `json:"structured_elements"`
	Prompts            []reviewPrompt             `json:"prompts"`
	ReviewLoop         reviewLoop                 `json:"review_loop"`
}

func newReviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "review <spec.md>",
		Short: "Generate adversarial review prompts for spec quality",
		Long: `Parses a spec and generates 3 targeted review prompts (implementer, tester, architect).
Agent feeds these to sub-agents for adversarial review. Findings drive spec revision.
Output: {spec, structured_elements, prompts[], review_loop}`,
		Args: cobra.ExactArgs(1),
		RunE: runReview,
	}
}

func runReview(_ *cobra.Command, args []string) error {
	specPath := args[0]

	lines, headings, err := parseSpecFile(specPath)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	_, elems := engine.CheckStructuredElements(lines, headings)

	specContent := readSpecContent(specPath)

	prompts := []reviewPrompt{
		generateImplementerPrompt(elems, specContent),
		generateTesterPrompt(elems, specContent),
		generateArchitectPrompt(elems, specContent),
	}

	result := reviewResult{
		Spec:               specPath,
		StructuredElements: elems,
		Prompts:            prompts,
		ReviewLoop: reviewLoop{
			MaxRounds:   2,
			Convergence: "no new high-severity findings",
			Instruction: "For each prompt, spawn a sub-agent via the Agent tool. Collect JSON findings. If any critical/high severity, revise spec and re-run `tp review`. Stop after 2 rounds or when no new high-severity findings.",
		},
	}

	return output.JSON(result)
}

const findingFormat = `
For each issue found, respond with one JSON object per line (NDJSON):
{"severity":"critical|high|medium|low","category":"completeness|ambiguity|consistency|feasibility|redundancy","location":"section heading or line number","finding":"what is wrong","suggestion":"how to fix it"}

Only report real issues. Do not generate findings just to appear thorough.`

func generateImplementerPrompt(elems *engine.StructuredElements, specContent string) reviewPrompt {
	var b strings.Builder
	b.WriteString("You are a senior engineer who must implement this spec tomorrow. Your goal is to find requirements that are missing, underspecified, or impossible to implement as stated.\n\n")
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

	return reviewPrompt{
		Role:     "implementer",
		Category: "completeness",
		Prompt:   b.String(),
	}
}

func generateTesterPrompt(elems *engine.StructuredElements, specContent string) reviewPrompt {
	var b strings.Builder
	b.WriteString("You are a QA engineer who must write tests from this spec. Your goal is to find requirements that are ambiguous (two testers would write contradictory tests) or non-verifiable (cannot write a pass/fail test).\n\n")
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

	return reviewPrompt{
		Role:     "tester",
		Category: "ambiguity",
		Prompt:   b.String(),
	}
}

func generateArchitectPrompt(elems *engine.StructuredElements, specContent string) reviewPrompt {
	var b strings.Builder
	b.WriteString("You are a senior architect reviewing this spec for approval before implementation begins. Your goal is to find contradictions between sections, missing backward compatibility analysis, and feasibility issues.\n\n")
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

	// Cross-reference multiple lists
	if len(elems.NumberedLists) > 1 {
		names := make([]string, len(elems.NumberedLists))
		for i, nl := range elems.NumberedLists {
			names[i] = fmt.Sprintf("'%s'", nl.Heading)
		}
		fmt.Fprintf(&b, "%d. Cross-reference lists %s — are there items in one but not the other? Anything missing from the union?\n", n, strings.Join(names, " and "))
		n++
	}

	if len(elems.Tables) > 1 {
		fmt.Fprintf(&b, "%d. Do the %d tables have consistent column semantics? Could any rows be merged or are any missing?\n", n, len(elems.Tables))
		n++
	}

	fmt.Fprintf(&b, "%d. Does the implementation order match the dependency graph? Can any steps be parallelized?\n", n)

	b.WriteString(findingFormat)

	return reviewPrompt{
		Role:     "architect",
		Category: "consistency",
		Prompt:   b.String(),
	}
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
	content := strings.Join(lines, "\n")
	// Cap at 10000 chars to keep prompts reasonable
	if len(content) > 10000 {
		content = content[:10000] + "\n[...truncated at 10000 chars]"
	}
	return content
}
