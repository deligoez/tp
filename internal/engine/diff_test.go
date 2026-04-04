package engine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func lines(s string) []string {
	return strings.Split(s, "\n")
}

func TestDiffSections_IdenticalSpecs(t *testing.T) {
	spec := lines("# Title\n\nSome content.\n\n## Section A\n\nContent A.\n")
	result := DiffSections(spec, spec)

	assert.Empty(t, result.Changed)
	assert.Empty(t, result.Removed)
	assert.Len(t, result.Unchanged, 2)
	assert.Equal(t, "Title", result.Unchanged[0].Heading)
	assert.Equal(t, "Section A", result.Unchanged[1].Heading)
}

func TestDiffSections_AddedSection(t *testing.T) {
	base := lines("# Title\n\nIntro.\n\n## Section A\n\nContent A.\n")
	current := lines("# Title\n\nIntro.\n\n## Section A\n\nContent A.\n\n## Section B\n\nNew content.\n")

	result := DiffSections(base, current)

	assert.Len(t, result.Changed, 1)
	assert.Equal(t, "Section B", result.Changed[0].Heading)
	assert.Equal(t, DiffAdded, result.Changed[0].Status)
	assert.Contains(t, result.Changed[0].Content, "New content.")

	assert.Empty(t, result.Removed)
	assert.Len(t, result.Unchanged, 2)
}

func TestDiffSections_RemovedSection(t *testing.T) {
	base := lines("# Title\n\nIntro.\n\n## Section A\n\nContent A.\n\n## Section B\n\nContent B.\n")
	current := lines("# Title\n\nIntro.\n\n## Section A\n\nContent A.\n")

	result := DiffSections(base, current)

	assert.Empty(t, result.Changed)
	assert.Len(t, result.Removed, 1)
	assert.Equal(t, "Section B", result.Removed[0].Heading)
	assert.Equal(t, DiffRemoved, result.Removed[0].Status)
	assert.Len(t, result.Unchanged, 2)
}

func TestDiffSections_ModifiedSection(t *testing.T) {
	base := lines("# Title\n\nIntro.\n\n## Section A\n\nOriginal content.\n")
	current := lines("# Title\n\nIntro.\n\n## Section A\n\nModified content.\n")

	result := DiffSections(base, current)

	assert.Len(t, result.Changed, 1)
	assert.Equal(t, "Section A", result.Changed[0].Heading)
	assert.Equal(t, DiffModified, result.Changed[0].Status)
	assert.Contains(t, result.Changed[0].Content, "Modified content.")

	assert.Empty(t, result.Removed)
	assert.Len(t, result.Unchanged, 1) // Title unchanged
}

func TestDiffSections_CodeBlockHeadingImmunity(t *testing.T) {
	spec := lines("# Title\n\nSome text.\n\n```\n## Not A Heading\ncode here\n```\n\nMore text.\n")
	result := DiffSections(spec, spec)

	// Only "Title" should be a section — "## Not A Heading" is inside a code block
	assert.Empty(t, result.Changed)
	assert.Len(t, result.Unchanged, 1)
	assert.Equal(t, "Title", result.Unchanged[0].Heading)
}

func TestDiffSections_RenamedHeading(t *testing.T) {
	base := lines("# Title\n\n## 5.2 Batch Processing\n\nContent.\n")
	current := lines("# Title\n\n## 5.2 Batch Operations\n\nContent.\n")

	result := DiffSections(base, current)

	// Renamed heading = 1 added + 1 removed
	assert.Len(t, result.Changed, 1)
	assert.Equal(t, "5.2 Batch Operations", result.Changed[0].Heading)
	assert.Equal(t, DiffAdded, result.Changed[0].Status)

	assert.Len(t, result.Removed, 1)
	assert.Equal(t, "5.2 Batch Processing", result.Removed[0].Heading)
}

func TestDiffSections_WhitespaceIgnored(t *testing.T) {
	base := lines("# Title\n\n## Section\n\n  Content with spaces.  \n")
	current := lines("# Title\n\n## Section\n\nContent with spaces.\n")

	result := DiffSections(base, current)

	assert.Empty(t, result.Changed, "whitespace-only differences should be ignored")
	assert.Len(t, result.Unchanged, 2)
}

func TestDiffSections_BlankLineDiffIgnored(t *testing.T) {
	base := lines("# Title\n\n## Section\n\nLine 1.\n\n\n\nLine 2.\n")
	current := lines("# Title\n\n## Section\n\nLine 1.\n\nLine 2.\n")

	result := DiffSections(base, current)

	assert.Empty(t, result.Changed, "blank line count differences should be ignored")
	assert.Len(t, result.Unchanged, 2)
}

func TestDiffSections_EmptyResult(t *testing.T) {
	result := DiffSections(nil, nil)

	// Should return empty slices, not nil
	assert.NotNil(t, result.Changed)
	assert.NotNil(t, result.Removed)
	assert.NotNil(t, result.Unchanged)
	assert.Empty(t, result.Changed)
	assert.Empty(t, result.Removed)
	assert.Empty(t, result.Unchanged)
}

func TestDiffSections_MultipleChanges(t *testing.T) {
	base := lines("# Spec\n\n## A\n\nOld A.\n\n## B\n\nOld B.\n\n## C\n\nOld C.\n")
	current := lines("# Spec\n\n## A\n\nNew A.\n\n## B\n\nOld B.\n\n## D\n\nNew D.\n")

	result := DiffSections(base, current)

	// A = modified, B = unchanged, C = removed, D = added
	changed := make(map[string]DiffSectionStatus)
	for _, s := range result.Changed {
		changed[s.Heading] = s.Status
	}
	assert.Equal(t, DiffModified, changed["A"])
	assert.Equal(t, DiffAdded, changed["D"])

	assert.Len(t, result.Removed, 1)
	assert.Equal(t, "C", result.Removed[0].Heading)

	unchanged := make(map[string]bool)
	for _, s := range result.Unchanged {
		unchanged[s.Heading] = true
	}
	assert.True(t, unchanged["Spec"])
	assert.True(t, unchanged["B"])
}
