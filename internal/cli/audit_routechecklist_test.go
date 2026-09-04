package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRouteChecklist_SpecCoverageOnly pins §2.1: routeChecklist returns the
// single spec-coverage checklist holding the table-row, list-item,
// task-acceptance and finding items, and never a file_check item — those are
// built in generateRoleAuditPrompts' shared arm instead.
func TestRouteChecklist_SpecCoverageOnly(t *testing.T) {
	t.Parallel()
	specEntries := []checklistEntry{
		{ID: "task-t1", Type: "task_acceptance", SpecLine: 3, Section: "## A", Text: "acceptance"},
		{ID: "row-1", Type: "table_row", SpecLine: 20, Section: "## B", Text: "row"},
		{ID: "item-1", Type: "list_item", SpecLine: 10, Section: "## A", Text: "item"},
	}
	findingsEntries := []checklistEntry{
		{ID: "finding-1", Type: "finding", Section: "## A", Text: "a finding"},
	}
	taskToFiles := map[string][]string{"t1": {"internal/cli/audit.go"}}

	items := routeChecklist(specEntries, findingsEntries, taskToFiles)

	require.Len(t, items, 4)
	types := make([]string, 0, len(items))
	ids := make([]string, 0, len(items))
	for _, it := range items {
		types = append(types, it.Type)
		ids = append(ids, it.ItemID)
		assert.NotEqual(t, "file_check", it.Type, "spec-coverage never receives file_check items")
	}
	// Structural items ascending by spec_line, then task_acceptance, then findings.
	assert.Equal(t, []string{"list_item", "table_row", "task_acceptance", "finding"}, types)
	assert.Equal(t, []string{"item-1", "row-1", "task-t1", "finding-1"}, ids)
	assert.Equal(t, "files changed by task commit: internal/cli/audit.go", items[2].ExpectedEvidence)
}
