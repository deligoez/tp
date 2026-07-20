package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
)

// Audit role names in the fixed prompt order.
const (
	roleSpecCoverage    = "spec-coverage"
	roleSecurity        = "security"
	roleMaintainability = "maintainability-conventions"
)

const claudeMDExcerptLineCap = 50

// roleRules embeds 3-5 role-specific bullet rules per prompt.
var roleRules = map[string][]string{
	roleSpecCoverage: {
		"State-dependent behaviors (disabled states, loading conditions, conditional rendering) count as PARTIAL unless fully covered.",
		"A table row describing a feature is PASS only if the feature code exists AND handles edge cases mentioned in surrounding spec context.",
		"A numbered list item describing a test is PASS only if a corresponding test function exists with assertions covering the described behavior.",
		"Task acceptance criteria are PASS only if the described behavior is observable in the code (not just a comment or placeholder).",
		"If a requirement mentions specific error handling, validation, or edge cases, verify those exist — don't just check the happy path.",
	},
	roleSecurity: {
		"Every lock acquire must have a paired release on all paths.",
		"No swallowed errors: `_ = err` and ignored returns are findings.",
		"No string concatenation when building queries or file paths from input.",
		"Files written by the tool use permissions 0o600, not 0o644.",
		"Input is validated before use.",
	},
	roleMaintainability: {
		"Errors are wrapped with %w when propagated.",
		"Exported symbols have doc comments.",
		"Functions stay under ~80 lines.",
		"Naming follows project patterns: lowercase packages, camelCase symbols, descriptive names.",
		"No leftover TODO/FIXME without a ticket reference.",
	},
}

// routeChecklist routes items disjointly: spec-derived and finding items go
// to spec-coverage; security and maintainability carry synthetic file_check
// items, one per affected file. Spec items follow the pinned order: table_row
// and list_item ascending by spec_line, then task_acceptance in task-file
// order, then finding by index.
func routeChecklist(specEntries, findingsEntries []checklistEntry, sel *engine.AuditFileSelection, taskToFiles map[string][]string) (spec, sec, maint []ChecklistItem) {
	structural := make([]checklistEntry, 0, len(specEntries))
	taskItems := make([]checklistEntry, 0)
	for _, e := range specEntries {
		if e.Type == "task_acceptance" {
			taskItems = append(taskItems, e)
		} else {
			structural = append(structural, e)
		}
	}
	sort.SliceStable(structural, func(i, j int) bool { return structural[i].SpecLine < structural[j].SpecLine })

	spec = make([]ChecklistItem, 0, len(specEntries)+len(findingsEntries))
	for i := range structural {
		spec = append(spec, specItemOf(&structural[i], taskToFiles))
	}
	for i := range taskItems {
		spec = append(spec, specItemOf(&taskItems[i], taskToFiles))
	}
	for i := range findingsEntries {
		spec = append(spec, specItemOf(&findingsEntries[i], taskToFiles))
	}

	sec = fileCheckItems(sel.Security, "file-sec-", roleSecurity)
	maint = fileCheckItems(sel.Maintainability, "file-maint-", roleMaintainability)
	return spec, sec, maint
}

// specItemOf converts a spec-derived or finding entry into a checklist item
// with its deterministic expected_evidence.
func specItemOf(e *checklistEntry, taskToFiles map[string][]string) ChecklistItem {
	evidence := fmt.Sprintf("search code under section %q for keywords from item text", e.Section)
	switch e.Type {
	case "task_acceptance":
		taskID := strings.TrimPrefix(e.ID, "task-")
		if paths := taskToFiles[taskID]; len(paths) > 0 {
			evidence = "files changed by task commit: " + strings.Join(paths, ", ")
		}
	case "finding":
		text := e.Text
		if len(text) > 120 {
			text = text[:120]
		}
		evidence = "verify the fix for: " + text
	}
	return ChecklistItem{
		ItemID:           e.ID,
		Type:             e.Type,
		SpecLine:         e.SpecLine,
		Section:          e.Section,
		Text:             e.Text,
		ExpectedEvidence: evidence,
	}
}

// fileCheckItems builds one synthetic checklist item per affected file, ids
// indexed by the role's affected-files list order.
func fileCheckItems(files []engine.AuditFileEntry, idPrefix, role string) []ChecklistItem {
	items := make([]ChecklistItem, 0, len(files))
	for n, f := range files {
		items = append(items, ChecklistItem{
			ItemID:           idPrefix + strconv.Itoa(n),
			Type:             "file_check",
			SpecLine:         0,
			Section:          f.Path,
			Text:             fmt.Sprintf("Apply the %s role rules to %s", role, f.Path),
			ExpectedEvidence: "inspect file: " + f.Path,
		})
	}
	return items
}

// invertTaskFiles converts path->tasks into task->sorted paths.
func invertTaskFiles(taskFiles map[string][]string) map[string][]string {
	out := make(map[string][]string)
	for path, ids := range taskFiles {
		for _, id := range ids {
			out[id] = append(out[id], path)
		}
	}
	for id := range out {
		sort.Strings(out[id])
	}
	return out
}

// buildRolePrompt renders the §3.1 body order for one role.
func buildRolePrompt(role string, items []ChecklistItem, files []engine.AuditFileEntry, specContent, claudeExcerpt string) auditPrompt {
	var b strings.Builder
	b.WriteString("## Role\n" + role + "\n\n")

	b.WriteString("## Role Rules\n")
	for _, r := range roleRules[role] {
		b.WriteString("- " + r + "\n")
	}
	b.WriteString("\n")

	if role == roleSpecCoverage {
		b.WriteString("## Spec Excerpt\n" + specContent + "\n\n")
	}
	if role == roleMaintainability && claudeExcerpt != "" {
		b.WriteString("## Project Context\n" + claudeExcerpt + "\n\n")
	}

	b.WriteString("## Checklist\n[\n")
	for i, item := range items {
		data, _ := json.Marshal(item)
		b.Write(data)
		if i < len(items)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("]\n\n")

	b.WriteString("## Affected Files (max 20)\n")
	for _, f := range files {
		if role == roleSpecCoverage && len(f.Tasks) > 0 {
			fmt.Fprintf(&b, "- %s (tasks: %s; diff: %s)\n", f.Path, strings.Join(f.Tasks, ", "), f.DiffSummary)
		} else {
			fmt.Fprintf(&b, "- %s (diff: %s)\n", f.Path, f.DiffSummary)
		}
	}
	b.WriteString("\n")

	b.WriteString(renderAuditOutputSchema())

	return auditPrompt{
		Role:           role,
		Prompt:         b.String(),
		ChecklistCount: len(items),
		ChecklistItems: items,
		AffectedFiles:  files,
	}
}

// generateRoleAuditPrompts emits one prompt per non-empty role in the fixed
// order spec-coverage, security, maintainability-conventions.
func generateRoleAuditPrompts(specItems, secItems, maintItems []ChecklistItem, sel *engine.AuditFileSelection, specContent, claudeExcerpt string) []auditPrompt {
	prompts := make([]auditPrompt, 0, 3)
	if len(specItems) > 0 {
		prompts = append(prompts, buildRolePrompt(roleSpecCoverage, specItems, sel.SpecCoverage, specContent, ""))
	}
	if len(secItems) > 0 {
		prompts = append(prompts, buildRolePrompt(roleSecurity, secItems, sel.Security, "", ""))
	}
	if len(maintItems) > 0 {
		prompts = append(prompts, buildRolePrompt(roleMaintainability, maintItems, sel.Maintainability, "", claudeExcerpt))
	}
	return prompts
}

// claudeMDExcerptFor resolves CLAUDE.md next to the resolved task file, then
// in the git repository root, and returns the ## Conventions section span
// (capped at 50 lines) or the first 50 lines. Empty when CLAUDE.md exists in
// neither place.
func claudeMDExcerptFor(specPath string) string {
	candidates := make([]string, 0, 2)
	if _, tfPath := engine.ResolveWorkflow(specPath, flagFile); tfPath != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(tfPath), "CLAUDE.md"))
	}
	if out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output(); err == nil {
		candidates = append(candidates, filepath.Join(strings.TrimSpace(string(out)), "CLAUDE.md"))
	}
	for _, c := range candidates {
		data, err := os.ReadFile(c)
		if err != nil {
			continue
		}
		return claudeConventionsExcerpt(strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n"))
	}
	return ""
}

// claudeConventionsExcerpt returns the ## Conventions section span (through
// the line before the next same-or-higher-level heading), capped at 50 lines;
// without that heading, the first 50 lines.
func claudeConventionsExcerpt(lines []string) string {
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "## Conventions" {
			start = i
			break
		}
	}
	var span []string
	if start >= 0 {
		span = append(span, lines[start])
		for i := start + 1; i < len(lines); i++ {
			trimmed := strings.TrimSpace(lines[i])
			if strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "## ") {
				break
			}
			span = append(span, lines[i])
		}
	} else {
		span = lines
	}
	if len(span) > claudeMDExcerptLineCap {
		span = span[:claudeMDExcerptLineCap]
	}
	return strings.Join(span, "\n")
}

// auditDiffStats parses `git diff --numstat` into path -> {added, deleted}.
func auditDiffStats(base string) map[string][2]int {
	args := []string{"diff", "--numstat"}
	if base != "" {
		args = append(args, base)
	}
	out, err := exec.Command("git", args...).Output()
	stats := make(map[string][2]int)
	if err != nil {
		return stats
	}
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		added, aErr := strconv.Atoi(parts[0])
		deleted, dErr := strconv.Atoi(parts[1])
		if aErr != nil || dErr != nil {
			continue // binary entries use "-"
		}
		stats[strings.Join(parts[2:], " ")] = [2]int{added, deleted}
	}
	return stats
}

// auditDeletedFiles lists files deleted in the diff.
func auditDeletedFiles(base string) map[string]bool {
	args := []string{"diff", "--name-only", "--diff-filter=D"}
	if base != "" {
		args = append(args, base)
	}
	out, err := exec.Command("git", args...).Output()
	deleted := make(map[string]bool)
	if err != nil {
		return deleted
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			deleted[line] = true
		}
	}
	return deleted
}

// auditTasksOf reads the spec-adjacent task file's tasks; empty when absent.
func auditTasksOf(specPath string) []model.Task {
	taskPath := strings.TrimSuffix(specPath, filepath.Ext(specPath)) + ".tasks.json"
	tf, err := model.ReadTaskFile(taskPath)
	if err != nil {
		return nil
	}
	return tf.Tasks
}
