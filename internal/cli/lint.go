package cli

import (
	"bufio"
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

type lintResult struct {
	File               string                     `json:"file"`
	Errors             int                        `json:"errors"`
	Warnings           int                        `json:"warnings"`
	Info               int                        `json:"info"`
	Findings           []engine.Finding           `json:"findings"`
	StructuredElements *engine.StructuredElements `json:"structured_elements,omitempty"`
}

func newLintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint <spec.md>",
		Short: "Deterministic spec quality checks",
		Args:  cobra.ExactArgs(1),
		RunE:  runLint,
	}
}

func runLint(_ *cobra.Command, args []string) error {
	specPath := args[0]

	lines, headings, err := parseSpecFile(specPath)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	totalLines := len(lines)

	var findings []engine.Finding
	findings = append(findings, engine.CheckHeadingHierarchy(headings)...)
	findings = append(findings, engine.CheckEmptySections(headings, totalLines)...)
	findings = append(findings, engine.CheckDuplicateHeadings(headings)...)
	findings = append(findings, engine.CheckOrphanReferences(lines, headings)...)
	findings = append(findings, engine.CheckVagueLanguage(lines)...)
	findings = append(findings, engine.CheckSectionSize(headings, totalLines, 50)...)
	findings = append(findings, engine.CheckSpecSize(totalLines, 500)...)

	// Structured element detection
	structFindings, structElems := engine.CheckStructuredElements(lines, headings)
	findings = append(findings, structFindings...)

	result := lintResult{File: specPath, Findings: findings, StructuredElements: structElems}
	for _, f := range findings {
		switch f.Severity {
		case "error":
			result.Errors++
		case "warning":
			result.Warnings++
		case "info":
			result.Info++
		}
	}

	if err := output.JSON(result); err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
	}

	if result.Errors > 0 {
		os.Exit(ExitValidation)
	}
	return nil
}

func parseSpecFile(path string) ([]string, []*engine.Heading, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	f2, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f2.Close()

	headings, err := engine.ParseHeadingsFromScanner(bufio.NewScanner(f2))
	if err != nil {
		return nil, nil, err
	}

	return lines, headings, nil
}
