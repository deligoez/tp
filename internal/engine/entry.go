package engine

import (
	"fmt"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// EntryFinding describes a single-task entry-validation failure (spec §6.1,
// rules 1, 3, 4, 5, 6). The caller maps Msg to the structured "error" and Hint
// to "hint", exiting with code 1.
type EntryFinding struct {
	Rule string // "id-blank", "title-blank", "acceptance-blank", "source-anchor", "unknown-dependency"
	Msg  string
	Hint string
}

// ValidateTaskEntry applies the §6.1 per-task entry rules to a single task:
//
//  1. id present (non-blank after trim)
//  3. title present (non-blank after trim)
//  4. acceptance present (non-blank after trim)
//  5. at least one source anchor (source_sections or source_lines)
//  6. every depends_on id resolvable against resolvableIDs
//
// Rule 2 (duplicate id) is intentionally omitted: it is inherently cross-task
// and stays at the caller, which tracks the known-id set. Rule 7 (invalid JSON)
// is handled at the decode site, before a Task exists to validate. Returns nil
// when the task passes every rule.
func ValidateTaskEntry(task *model.Task, resolvableIDs map[string]bool) *EntryFinding {
	if strings.TrimSpace(task.ID) == "" {
		return &EntryFinding{
			Rule: "id-blank",
			Msg:  "task id is required",
			Hint: `provide a non-blank "id" field`,
		}
	}
	if strings.TrimSpace(task.Title) == "" {
		return &EntryFinding{
			Rule: "title-blank",
			Msg:  fmt.Sprintf("task %s: title is required", task.ID),
			Hint: `provide a non-blank "title" field`,
		}
	}
	if strings.TrimSpace(task.Acceptance) == "" {
		return &EntryFinding{
			Rule: "acceptance-blank",
			Msg:  fmt.Sprintf("task %s: acceptance is required", task.ID),
			Hint: `provide a non-blank "acceptance" field`,
		}
	}
	if len(task.SourceSections) == 0 && strings.TrimSpace(task.SourceLines) == "" {
		return &EntryFinding{
			Rule: "source-anchor",
			Msg:  fmt.Sprintf("task %s: needs source_sections or source_lines", task.ID),
			Hint: `provide at least one of "source_sections" (canonical headings) or "source_lines"`,
		}
	}
	var unknown []string
	for _, dep := range task.DependsOn {
		if !resolvableIDs[dep] {
			unknown = append(unknown, dep)
		}
	}
	if len(unknown) > 0 {
		return &EntryFinding{
			Rule: "unknown-dependency",
			Msg:  fmt.Sprintf("task %s depends on unknown id: %s", task.ID, strings.Join(unknown, ", ")),
			Hint: fmt.Sprintf("remove the unknown depends_on id(s), or add them as tasks first: %s", strings.Join(unknown, ", ")),
		}
	}
	return nil
}
