package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

func runReviewVerify(specPath, findingsPath string, affectedFiles []string, diffFrom string, specInline bool) error {
	if findingsPath == "" {
		output.Error(ExitUsage, "--verify requires --findings (nothing to verify without previous findings)")
		os.Exit(ExitUsage)
		return nil
	}

	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		output.Error(ExitFile, fmt.Sprintf("spec not found: %s", specPath))
		os.Exit(ExitFile)
		return nil
	}

	if _, err := os.Stat(findingsPath); os.IsNotExist(err) {
		output.Error(ExitFile, fmt.Sprintf("findings file not found: %s", findingsPath))
		os.Exit(ExitFile)
		return nil
	}

	// Validate affected files
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

	// Read and classify findings
	findings := readVerifyFindings(findingsPath)
	var fixed, wontfix, unresolved []verifyFinding
	for _, f := range findings {
		if f.Resolved == nil {
			unresolved = append(unresolved, f)
			continue
		}
		switch f.Resolved.Status {
		case "fixed", "duplicate":
			fixed = append(fixed, f)
		case "wontfix":
			wontfix = append(wontfix, f)
		default:
			unresolved = append(unresolved, f)
		}
	}

	// Build spec content
	var specContent string
	switch {
	case diffFrom != "":
		if _, err := os.Stat(diffFrom); os.IsNotExist(err) {
			output.Error(ExitFile, fmt.Sprintf("diff baseline not found: %s", diffFrom))
			os.Exit(ExitFile)
			return nil
		}
		baseData, _ := os.ReadFile(diffFrom)
		currData, _ := os.ReadFile(specPath)
		dr := engine.DiffSections(strings.Split(string(baseData), "\n"), strings.Split(string(currData), "\n"))
		specContent = buildDiffSpecContent(&dr)
	case specInline:
		specContent = readSpecContent(specPath)
	default:
		// Default: reference mode — omit inline content
		specData, _ := os.ReadFile(specPath)
		lineCount := strings.Count(string(specData), "\n") + 1
		absPath, _ := filepath.Abs(specPath)
		headings, _ := engine.ParseHeadings(specPath)
		specContent = buildSpecRefContent(absPath, lineCount, headings)
	}

	// Build prompt
	prompt := buildVerifyPrompt(specContent, fixed, wontfix, unresolved, affectedFiles)

	// Build result
	absPath, _ := filepath.Abs(specPath)
	result := reviewResult{
		Spec: specPath,
		Prompts: []reviewPrompt{{
			Role:     "verifier",
			Category: "verification",
			Prompt:   prompt,
		}},
		ReviewLoop: reviewLoop{
			Round:            0,
			MaxRounds:        1,
			Convergence:      "verification pass",
			PreviousFindings: len(findings),
			Instruction:      "If verifier finds 0 issues, review is complete. If issues found, run a full review round.",
			Mode:             "verification",
		},
	}

	if !specInline {
		result.SpecRef = true
		result.SpecPath = absPath
	}

	if len(affectedFiles) > 0 {
		affectedContent := engine.ReadAffectedFilesBudgetAware(affectedFiles, specContent)
		result.AffectedFiles = affectedFiles
		result.AffectedSummary = engine.BuildAffectedSummary(affectedFiles, affectedContent)
	}

	return output.JSON(result)
}

type verifyFinding struct {
	Severity string          `json:"severity"`
	Category string          `json:"category"`
	Location string          `json:"location"`
	Finding  string          `json:"finding"`
	Resolved *resolvedStatus `json:"resolved,omitempty"`
}

func readVerifyFindings(path string) []verifyFinding {
	data, err := os.ReadFile(path)
	if err != nil {
		return make([]verifyFinding, 0)
	}
	findings := make([]verifyFinding, 0)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var f verifyFinding
		if json.Unmarshal([]byte(line), &f) != nil {
			continue
		}
		if f.Severity == "" {
			f.Severity = "unknown"
		}
		findings = append(findings, f)
	}
	return findings
}

func buildVerifyPrompt(specContent string, fixed, wontfix, unresolved []verifyFinding, affectedFiles []string) string {
	var b strings.Builder

	totalFindings := len(fixed) + len(wontfix) + len(unresolved)
	fmt.Fprintf(&b, "You are a verification reviewer. Previous review rounds produced %d findings:\n", totalFindings)
	fmt.Fprintf(&b, "- %d were addressed (fixed or duplicate)\n", len(fixed))
	fmt.Fprintf(&b, "- %d were marked won't-fix\n", len(wontfix))
	fmt.Fprintf(&b, "- %d remain unresolved\n\n", len(unresolved))

	b.WriteString("Your job is NOT to do a full review — only:\n\n")
	b.WriteString("1. Verify each fixed/duplicate finding is genuinely resolved in the spec.\n")
	b.WriteString("2. For unresolved findings, confirm they are still present.\n")
	b.WriteString("3. Check if any fix introduced a NEW critical or high-severity issue.\n")
	b.WriteString("4. If everything checks out, respond with zero lines (empty output).\n\n")

	// Fixed findings section (if any)
	if len(fixed) > 0 {
		b.WriteString("Fixed findings to verify:\n")
		for i, f := range fixed {
			evidence := ""
			if f.Resolved != nil {
				evidence = f.Resolved.Evidence
			}
			if evidence != "" {
				fmt.Fprintf(&b, "  %d. [%s] %s — %s: %s (evidence: %s)\n", i+1, f.Severity, f.Category, f.Location, f.Finding, evidence)
			} else {
				fmt.Fprintf(&b, "  %d. [%s] %s — %s: %s\n", i+1, f.Severity, f.Category, f.Location, f.Finding)
			}
		}
		b.WriteString("\n")
	}

	// Wontfix findings section (if any)
	if len(wontfix) > 0 {
		b.WriteString("Won't-fix findings (acknowledged, not bugs):\n")
		for i, f := range wontfix {
			evidence := ""
			if f.Resolved != nil {
				evidence = f.Resolved.Evidence
			}
			fmt.Fprintf(&b, "  %d. [%s] %s — %s: %s (reason: %s)\n", i+1, f.Severity, f.Category, f.Location, f.Finding, evidence)
		}
		b.WriteString("\n")
	}

	// Unresolved findings section (if any)
	if len(unresolved) > 0 {
		b.WriteString("Unresolved findings (still open):\n")
		for i, f := range unresolved {
			fmt.Fprintf(&b, "  %d. [%s] %s — %s: %s\n", i+1, f.Severity, f.Category, f.Location, f.Finding)
		}
		b.WriteString("\n")
	}

	b.WriteString("Spec content:\n---\n")
	b.WriteString(specContent)
	b.WriteString("\n---\n\n")

	// Affected files
	if len(affectedFiles) > 0 {
		affectedContent := engine.ReadAffectedFilesBudgetAware(affectedFiles, specContent)
		b.WriteString(engine.BuildAffectedSection(affectedContent))
	}

	b.WriteString("Respond with one JSON finding per line (NDJSON). If no issues found, respond with zero lines (empty output).\n")
	b.WriteString("For findings that were reported as fixed but are NOT actually resolved, set category to \"regression\".\n")

	return b.String()
}
