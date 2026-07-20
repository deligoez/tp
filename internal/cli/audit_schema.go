package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/deligoez/tp/internal/engine"
)

// renderAuditOutputSchema renders the NDJSON output schema block embedded in
// every audit prompt: one row per checklist item, the field requirements
// table including the optional class row, and the category enum with its
// resolution precedence.
func renderAuditOutputSchema() string {
	var b strings.Builder
	b.WriteString("## Output Schema\n")
	b.WriteString("Return one NDJSON line per checklist item:\n")
	b.WriteString(`{"item_id": "list-2-3", "status": "PASS", "evidence_file": "internal/cli/importcmd.go", "evidence_lines": "127-131", "category": null, "severity": null, "notes": ""}` + "\n\n")
	b.WriteString("Field requirements:\n")
	b.WriteString("- item_id: always; must match a checklist item id from this prompt\n")
	b.WriteString("- status: always; one of PASS, PARTIAL, FAIL\n")
	b.WriteString(`- evidence_file: repo-relative path when status is PASS or PARTIAL; null for FAIL` + "\n")
	b.WriteString(`- evidence_lines: "42-58" (range) or "42" (single line) when status is PASS or PARTIAL; null for FAIL — both forms are valid` + "\n")
	b.WriteString("- category: field MUST exist in every row\n")
	b.WriteString("- severity: field MUST exist; null for PASS, one of error|warning|info for PARTIAL or FAIL\n")
	b.WriteString(`- notes: always; short string, max 500 chars; "" if no notes` + "\n")
	b.WriteString("- class: optional; kebab-case slug naming a mechanically checkable pattern; omit when not classifiable\n\n")
	b.WriteString(engine.RenderAuditCategoryText())
	b.WriteString("\n")
	return b.String()
}

// checklistItemsOf converts checklist entries into prompt checklist items
// with the deterministic expected_evidence fallback.
func checklistItemsOf(entries []checklistEntry) []ChecklistItem {
	items := make([]ChecklistItem, 0, len(entries))
	for _, e := range entries {
		evidence := fmt.Sprintf("search code under section %q for keywords from item text", e.Section)
		if e.Type == "finding" {
			text := e.Text
			if len(text) > 120 {
				text = text[:120]
			}
			evidence = "verify the fix for: " + text
		}
		items = append(items, ChecklistItem{
			ItemID:           e.ID,
			Type:             e.Type,
			SpecLine:         e.SpecLine,
			Section:          e.Section,
			Text:             e.Text,
			ExpectedEvidence: evidence,
		})
	}
	return items
}

// affectedFilesOf renders the affected-files entries for prompt output,
// alphabetical by path.
func affectedFilesOf(affectedContent map[string]string) []auditAffectedFile {
	paths := make([]string, 0, len(affectedContent))
	for p := range affectedContent {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	out := make([]auditAffectedFile, 0, len(paths))
	for _, p := range paths {
		out = append(out, auditAffectedFile{Path: p, Tasks: []string{}})
	}
	return out
}
