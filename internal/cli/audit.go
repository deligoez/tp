package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
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

type auditPrompt struct {
	Role           string `json:"role"`
	Prompt         string `json:"prompt"`
	ChecklistCount int    `json:"checklist_count"`
}

type auditResult struct {
	Spec             string                  `json:"spec"`
	Files            []string                `json:"files"`
	FileSummary      *engine.AffectedSummary `json:"file_summary,omitempty"`
	Checklist        []checklistEntry        `json:"checklist"`
	ChecklistSummary checklistSummary        `json:"checklist_summary"`
	Prompts          []auditPrompt           `json:"prompts"`
}

var binaryExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true,
	".ico": true, ".woff": true, ".woff2": true, ".ttf": true, ".eot": true,
	".zip": true, ".tar": true, ".gz": true, ".pdf": true,
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".o": true, ".a": true,
}

const maxAutoDetectFiles = 50
const checklistSplitThreshold = 50

func newAuditCmd() *cobra.Command {
	var affectedFiles []string
	var base string
	var findingsPath string

	cmd := &cobra.Command{
		Use:   "audit <spec.md>",
		Short: "Post-implementation spec review: verify code matches spec requirements",
		Long: `Post-implementation audit. Parses spec structured elements, reads changed source files,
and generates adversarial prompts that verify each requirement against actual code.

Auto-detects changed files via git diff (omit --affected-files for zero-config).
Use --findings to also verify review findings were addressed.`,
		Args:              cobra.ExactArgs(1),
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAudit(cmd, args[0], affectedFiles, base, findingsPath)
		},
	}

	cmd.Flags().StringArrayVar(&affectedFiles, "affected-files", nil, "Source files to audit (auto-detect via git diff if omitted)")
	cmd.Flags().StringVar(&base, "base", "", "Git ref to diff against (omit for staged+unstaged)")
	cmd.Flags().StringVar(&findingsPath, "findings", "", "Path to NDJSON findings from tp review")

	return cmd
}

func runAudit(_ *cobra.Command, specPath string, affectedFiles []string, base, findingsPath string) error {
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		output.Error(ExitFile, fmt.Sprintf("spec not found: %s", specPath))
		os.Exit(ExitFile)
		return nil
	}

	files, err := resolveAuditFiles(specPath, affectedFiles, base)
	if err != nil {
		exitCode := ExitState
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "directory, not a file") {
			exitCode = ExitFile
		}
		output.Error(exitCode, err.Error())
		os.Exit(exitCode)
		return nil
	}

	specData, err := os.ReadFile(specPath)
	if err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot read spec: %s", specPath))
		os.Exit(ExitFile)
		return nil
	}
	specLines := strings.Split(string(specData), "\n")
	specContent := string(specData)
	if len(specContent) > engine.SpecContentCap {
		specContent = specContent[:engine.SpecContentCap] + "\n[...spec truncated]"
	}

	headings, _ := engine.ParseHeadings(specPath)

	checklist := buildChecklist(specLines, headings, specPath, findingsPath)

	if len(checklist) == 0 {
		output.Info("no structured elements found in spec — checklist is empty")
	}

	findingsEntries := filterChecklistByType(checklist, "finding")
	mainEntries := filterChecklistByType(checklist, "")

	budgetOther := len(specContent) + estimateChecklistText(mainEntries)
	if len(findingsEntries) > 0 {
		budgetOther += len(findingsEntries) * 200
	}

	affectedContent := engine.ReadAffectedFilesBudgetAware(files, specContent)
	if budgetOther > 0 {
		affectedContent = engine.ReadAffectedFilesBudgetAware(files, specContent, strings.Repeat("x", budgetOther-len(specContent)))
	}

	summary := engine.BuildAffectedSummary(files, affectedContent)

	prompts := generateAuditPrompts(specContent, mainEntries, affectedContent, findingsEntries)

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

	if flagCompact {
		for i := range result.Checklist {
			if len(result.Checklist[i].Text) > 80 {
				result.Checklist[i].Text = result.Checklist[i].Text[:77] + "..."
			}
		}
		result.FileSummary = nil
	}

	return output.JSON(result)
}

func resolveAuditFiles(specPath string, affectedFiles []string, base string) ([]string, error) {
	if len(affectedFiles) > 0 {
		affectedFiles = engine.DedupPaths(affectedFiles)
		for _, f := range affectedFiles {
			info, err := os.Stat(f)
			if err != nil {
				return nil, fmt.Errorf("affected file not found: %s", f)
			}
			if info.IsDir() {
				return nil, fmt.Errorf("affected path is a directory, not a file: %s", f)
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
		return nil, fmt.Errorf("no changed files detected (staged+unstaged is empty) — provide --affected-files or stage some changes")
	}
	return files, nil
}

func detectChangedFiles(dir, base string) ([]string, error) {
	var allFiles []string

	if base != "" {
		unstaged := execGitDiff(dir, "diff", "--name-only", base+"...HEAD")
		if len(unstaged) == 0 && !gitExists(dir) {
			return nil, fmt.Errorf("not in a git repo — provide --affected-files or run inside a git repo")
		}
		allFiles = append(allFiles, unstaged...)
	} else {
		unstaged := execGitDiff(dir, "diff", "--name-only")
		if len(unstaged) == 0 && !gitExists(dir) {
			return nil, fmt.Errorf("not in a git repo — provide --affected-files or run inside a git repo")
		}
		allFiles = append(allFiles, unstaged...)

		staged := execGitDiff(dir, "diff", "--name-only", "--cached")
		allFiles = append(allFiles, staged...)
	}

	allFiles = engine.DedupPaths(allFiles)
	filtered := make([]string, 0, len(allFiles))
	for _, f := range allFiles {
		if isBinaryFile(f) {
			continue
		}
		if strings.HasSuffix(f, ".md") {
			continue
		}
		if strings.HasSuffix(f, ".tasks.json") {
			continue
		}
		filtered = append(filtered, f)
	}
	sort.Strings(filtered)

	if len(filtered) > maxAutoDetectFiles {
		filtered = filtered[:maxAutoDetectFiles]
		output.Info(fmt.Sprintf("more than %d files changed, auditing first %d", maxAutoDetectFiles, maxAutoDetectFiles))
	}

	if len(filtered) == 0 && len(allFiles) > 0 {
		return nil, fmt.Errorf("no audit-able files in diff — only binary/markdown files changed")
	}

	return filtered, nil
}

func gitExists(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	return cmd.Run() == nil
}

func execGitDiff(dir string, args ...string) []string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
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
	return files
}

func isBinaryFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return false
	}
	return binaryExtensions[ext]
}

func buildChecklist(specLines []string, headings []*engine.Heading, specPath, findingsPath string) []checklistEntry {
	entries := make([]checklistEntry, 0)

	tableRows := engine.ExtractTableRows(specLines, headings)
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

	listItems := engine.ExtractNumberedItems(specLines, headings)
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

	taskPath := strings.TrimSuffix(specPath, filepath.Ext(specPath)) + ".tasks.json"
	if data, err := os.ReadFile(taskPath); err == nil {
		var tf struct {
			Tasks []struct {
				ID         string `json:"id"`
				Title      string `json:"title"`
				Acceptance string `json:"acceptance"`
			} `json:"tasks"`
		}
		if json.Unmarshal(data, &tf) == nil {
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
		}
	}

	if findingsPath != "" {
		findingsEntries := readFindings(findingsPath)
		for i, fe := range findingsEntries {
			entries = append(entries, checklistEntry{
				ID:       fmt.Sprintf("finding-%d", i),
				Type:     "finding",
				SpecLine: 0,
				Section:  "Review Findings",
				Text:     fe,
				Prompt:   0,
			})
		}
	}

	return entries
}

func readFindings(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var results []string
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(line), &obj) != nil {
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
			results = append(results, text)
		}
	}
	return results
}

func filterChecklistByType(entries []checklistEntry, typ string) []checklistEntry {
	if typ == "" {
		var result []checklistEntry
		for _, e := range entries {
			if e.Type != "finding" {
				result = append(result, e)
			}
		}
		return result
	}
	var result []checklistEntry
	for _, e := range entries {
		if e.Type == typ {
			result = append(result, e)
		}
	}
	return result
}

func estimateChecklistText(entries []checklistEntry) int {
	total := 0
	for _, e := range entries {
		total += len(e.Text) + 30
	}
	return total
}

func generateAuditPrompts(specContent string, mainEntries []checklistEntry, affectedContent map[string]string, findingsEntries []checklistEntry) []auditPrompt {
	var prompts []auditPrompt

	for i := 0; i < len(mainEntries); i += checklistSplitThreshold {
		end := i + checklistSplitThreshold
		if end > len(mainEntries) {
			end = len(mainEntries)
		}
		batch := mainEntries[i:end]

		for j := range batch {
			batch[j].Prompt = len(prompts)
		}

		var b strings.Builder
		b.WriteString("Role: implementation-auditor\n")
		b.WriteString("Task: Verify each spec requirement is implemented in the code.\n\n")
		b.WriteString("## Spec Excerpt\n")
		b.WriteString(specContent)
		b.WriteString("\n\n## Checklist\n")
		b.WriteString("For each item, mark it as:\n")
		b.WriteString("- PASS: requirement is fully implemented in the code\n")
		b.WriteString("- PARTIAL: requirement is partially implemented — explain what's missing\n")
		b.WriteString("- FAIL: requirement is not found in the code — explain what's expected\n\n")
		for _, e := range batch {
			fmt.Fprintf(&b, "- [%s] (%s, line %d) %s\n", e.ID, e.Type, e.SpecLine, e.Text)
		}
		b.WriteString("\n## Source Files\n")
		b.WriteString(engine.BuildAffectedSection(affectedContent))
		b.WriteString("\n## Rules\n")
		b.WriteString("1. Read the code carefully. State-dependent behaviors (disabled states, loading conditions, conditional rendering) count as partial unless fully covered.\n")
		b.WriteString("2. A table row describing a feature is PASS only if the feature code exists AND handles edge cases mentioned in surrounding spec context.\n")
		b.WriteString("3. A numbered list item describing a test is PASS only if a corresponding test function exists with assertions covering the described behavior.\n")
		b.WriteString("4. Task acceptance criteria are PASS only if the described behavior is observable in the code (not just a comment or placeholder).\n")
		b.WriteString("5. If a requirement mentions specific error handling, validation, or edge cases, verify those exist — don't just check the happy path.\n\n")
		b.WriteString("## Output Format\n")
		b.WriteString("For each checklist item, output one line:\n")
		b.WriteString("{ID} | {PASS|PARTIAL|FAIL} | {evidence — file:line reference or explanation}\n")

		prompts = append(prompts, auditPrompt{
			Role:           "implementation-auditor",
			Prompt:         b.String(),
			ChecklistCount: len(batch),
		})
	}

	if len(findingsEntries) > 0 {
		for j := range findingsEntries {
			findingsEntries[j].Prompt = len(prompts)
		}

		var b strings.Builder
		b.WriteString("Role: implementation-auditor\n")
		b.WriteString("Task: Verify each review finding was addressed in the code.\n\n")
		b.WriteString("## Review Findings\n")
		for _, e := range findingsEntries {
			fmt.Fprintf(&b, "- [%s] %s\n", e.ID, e.Text)
		}
		b.WriteString("\n## Checklist\n")
		for _, e := range findingsEntries {
			fmt.Fprintf(&b, "- [%s] %s\n", e.ID, e.Text)
		}
		b.WriteString("\n## Source Files\n")
		b.WriteString(engine.BuildAffectedSection(affectedContent))
		b.WriteString("\n## Rules\n")
		b.WriteString("1. For each finding, determine if the code change addresses the reported issue.\n")
		b.WriteString("2. A finding is PASS only if the specific problem described is demonstrably fixed.\n")
		b.WriteString("3. Partial fixes (e.g., adding a comment instead of actual code) count as PARTIAL.\n")
		b.WriteString("4. If multiple findings relate to the same code area, verify each independently.\n\n")
		b.WriteString("## Output Format\n")
		b.WriteString("{ID} | {PASS|PARTIAL|FAIL} | {evidence — file:line reference or explanation}\n")

		prompts = append(prompts, auditPrompt{
			Role:           "implementation-auditor",
			Prompt:         b.String(),
			ChecklistCount: len(findingsEntries),
		})
	}

	return prompts
}
