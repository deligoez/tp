package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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
	findings = append(findings, engine.CheckDuplicateConsecutiveLines(lines)...)
	findings = append(findings, engine.CheckNumberingGaps(headings)...)

	structFindings, structElems := engine.CheckStructuredElements(lines, headings)
	findings = append(findings, structFindings...)

	taskFindings := checkTaskFileQuality(specPath)
	findings = append(findings, taskFindings...)

	specScopeFindings := checkAffectedFilesScope(lines, headings)
	findings = append(findings, specScopeFindings...)

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

func resolveTaskFilePath(specPath string) string {
	dir := filepath.Dir(specPath)
	base := strings.TrimSuffix(filepath.Base(specPath), filepath.Ext(specPath))
	return filepath.Join(dir, base+".tasks.json")
}

func checkTaskFileQuality(specPath string) []engine.Finding {
	taskPath := resolveTaskFilePath(specPath)
	if _, err := os.Stat(taskPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(taskPath)
	if err != nil {
		return nil
	}

	var tf struct {
		Tasks []struct {
			Acceptance string `json:"acceptance"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(data, &tf); err != nil {
		return nil
	}

	var findings []engine.Finding

	removalVerbs := regexp.MustCompile(`(?i)\b(removed|deleted|dropped|silindi|kaldırıldı)\b`)
	completionVerbs := regexp.MustCompile(`(?i)\b(corrected|fixed|updated|adjusted|düzenlendi|güncellendi)\b`)

	for _, task := range tf.Tasks {
		acc := task.Acceptance
		if acc == "" {
			continue
		}

		words := strings.Fields(acc)
		if len(words) < 10 {
			findings = append(findings, engine.Finding{
				Severity: "info",
				Rule:     "acceptance-quality",
				Message:  fmt.Sprintf("acceptance is very short (%d words), may lack specificity", len(words)),
			})
		}

		hasRemoval := removalVerbs.MatchString(acc)
		if hasRemoval {
			findings = append(findings, engine.Finding{
				Severity: "warning",
				Rule:     "acceptance-quality",
				Message:  "acceptance describes removal but not final state",
				Context:  acc,
			})
		}

		hasCompletion := completionVerbs.MatchString(acc)
		if hasCompletion && !hasRemoval {
			findings = append(findings, engine.Finding{
				Severity: "warning",
				Rule:     "acceptance-quality",
				Message:  "acceptance uses vague completion verb without describing result",
				Context:  acc,
			})
		}
	}

	return findings
}

func checkAffectedFilesScope(lines []string, headings []*engine.Heading) []engine.Finding {
	var findings []engine.Finding

	affectedHeadingIdx := -1
	for i, h := range headings {
		lower := strings.ToLower(h.Text)
		if strings.Contains(lower, "affected") && strings.Contains(lower, "files") {
			affectedHeadingIdx = i
			break
		}
	}
	if affectedHeadingIdx == -1 {
		return findings
	}

	tableStart, tableEnd := findTableRange(lines, headings, affectedHeadingIdx)
	if tableStart < 0 {
		return findings
	}

	tableLines := lines[tableStart:tableEnd]
	if len(tableLines) == 0 {
		return findings
	}

	headerCells := parseTableRow(tableLines[0])
	if len(headerCells) == 0 {
		return findings
	}

	scopeCol := -1
	actionCol := -1
	modifyPattern := regexp.MustCompile(`(?i)^(modify|change|değiştir|güncelle)\b`)
	noChangePattern := regexp.MustCompile(`(?i)^no\b`)
	actionHeader := regexp.MustCompile(`(?i)^(action|type|change type|op)$`)
	scopeHeader := regexp.MustCompile(`(?i)^(scope|description|details|note)$`)
	for i, cell := range headerCells {
		trimmed := strings.TrimSpace(cell)
		if scopeCol < 0 && scopeHeader.MatchString(trimmed) {
			scopeCol = i
		}
		if actionCol < 0 && actionHeader.MatchString(trimmed) {
			actionCol = i
		}
	}
	if scopeCol < 0 || actionCol < 0 {
		return findings
	}

	for _, line := range tableLines[1:] {
		cells := parseTableRow(line)
		if len(cells) <= scopeCol || len(cells) <= actionCol {
			continue
		}
		actionValue := strings.TrimSpace(cells[actionCol])
		if !modifyPattern.MatchString(actionValue) || noChangePattern.MatchString(actionValue) {
			continue
		}

		scopeValue := strings.TrimSpace(cells[scopeCol])
		if len(scopeValue) < 10 {
			findings = append(findings, engine.Finding{
				Severity: "warning",
				Rule:     "affected-files-scope",
				Message:  fmt.Sprintf("affected files row with '%s' action has no scope description (need >= 10 chars)", actionValue),
				Context:  strings.Join(cells, " | "),
			})
		}
	}

	return findings
}

func findTableRange(lines []string, headings []*engine.Heading, headingIdx int) (start, end int) {
	if headingIdx >= len(headings) {
		return -1, -1
	}
	startLine := headings[headingIdx].Line

	nextHeadingLine := len(lines) + 1
	if headingIdx+1 < len(headings) {
		nextHeadingLine = headings[headingIdx+1].Line
	}

	tableStart := -1
	for i := startLine; i < nextHeadingLine && i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "|") && strings.HasSuffix(strings.TrimSpace(lines[i]), "|") {
			tableStart = i
			break
		}
	}
	if tableStart < 0 {
		return -1, -1
	}

	tableEnd := tableStart + 1
	for i := tableStart + 1; i < nextHeadingLine && i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "|") {
			break
		}
		tableEnd = i + 1
	}

	return tableStart, tableEnd
}

func parseTableRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")
	parts := strings.Split(trimmed, "|")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		result = append(result, strings.TrimSpace(p))
	}
	return result
}
