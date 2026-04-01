package engine

import (
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// AutoFillCoverage computes coverage from spec headings and task source_sections.
// Sets total_sections, mapped_sections, context_only, and unmapped automatically.
func AutoFillCoverage(tf *model.TaskFile, specPath string) {
	headings, err := ParseHeadings(specPath)
	if err != nil || len(headings) == 0 {
		return
	}

	// Build set of all headings as "## Heading Text" format
	allHeadings := make(map[string]bool)
	for _, h := range headings {
		prefix := strings.Repeat("#", h.Level) + " "
		allHeadings[prefix+h.Text] = true
	}

	// Build set of mapped headings from task source_sections
	mapped := make(map[string]bool)
	for i := range tf.Tasks {
		for _, s := range tf.Tasks[i].SourceSections {
			if allHeadings[s] {
				mapped[s] = true
			}
		}
	}

	// Everything not mapped is context_only
	contextOnly := make([]string, 0)
	for _, h := range headings {
		prefix := strings.Repeat("#", h.Level) + " "
		key := prefix + h.Text
		if !mapped[key] {
			contextOnly = append(contextOnly, key)
		}
	}

	tf.Coverage.TotalSections = len(headings)
	tf.Coverage.MappedSections = len(mapped)
	tf.Coverage.ContextOnly = contextOnly
	tf.Coverage.Unmapped = make([]string, 0)
}
