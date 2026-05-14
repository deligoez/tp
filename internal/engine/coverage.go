package engine

import (
	"github.com/deligoez/tp/internal/model"
)

// AutoFillCoverage computes coverage from spec headings and task source_sections.
// Sets total_sections, mapped_sections, context_only, and unmapped automatically.
// Each task source_sections entry is resolved via ResolveSection so plain-text values
// (e.g. "4. Backend Migration") are matched against canonical headings ("## 4. Backend Migration").
// Ambiguous or unresolvable entries are skipped during the mapped-set computation.
func AutoFillCoverage(tf *model.TaskFile, specPath string) {
	headings, err := ParseHeadings(specPath)
	if err != nil || len(headings) == 0 {
		return
	}

	mapped := make(map[string]bool)
	for i := range tf.Tasks {
		for _, s := range tf.Tasks[i].SourceSections {
			resolved, ambiguous, _ := ResolveSection(s, headings)
			if resolved == "" || ambiguous {
				continue
			}
			mapped[resolved] = true
		}
	}

	contextOnly := make([]string, 0)
	for _, h := range headings {
		key := canonicalHeading(h.Level, h.Text)
		if !mapped[key] {
			contextOnly = append(contextOnly, key)
		}
	}

	tf.Coverage.TotalSections = len(headings)
	tf.Coverage.MappedSections = len(mapped)
	tf.Coverage.ContextOnly = contextOnly
	tf.Coverage.Unmapped = make([]string, 0)
}
