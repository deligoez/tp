package engine_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

func createSpecFile(t *testing.T, dir string, lines []string) string {
	t.Helper()
	specPath := filepath.Join(dir, "spec.md")
	content := strings.Join(lines, "\n")
	err := os.WriteFile(specPath, []byte(content), 0o600)
	require.NoError(t, err)
	return specPath
}

func TestExtractSpecExcerpt_ValidRange(t *testing.T) {
	dir := t.TempDir()
	lines := []string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
	}
	specPath := createSpecFile(t, dir, lines)

	excerpt := engine.ExtractSpecExcerpt(specPath, "2-4")

	assert.Equal(t, "line 2\nline 3\nline 4", excerpt)
}

func TestExtractSpecExcerpt_EmptySourceLines(t *testing.T) {
	dir := t.TempDir()
	specPath := createSpecFile(t, dir, []string{"line 1"})

	excerpt := engine.ExtractSpecExcerpt(specPath, "")
	assert.Equal(t, "", excerpt)
}

func TestExtractSpecExcerpt_EmptySpecPath(t *testing.T) {
	excerpt := engine.ExtractSpecExcerpt("", "1-5")
	assert.Equal(t, "", excerpt)
}

func TestExtractSpecExcerpt_Truncation(t *testing.T) {
	dir := t.TempDir()

	// Create a file with lines that exceed 2000 characters total
	lines := make([]string, 0, 100)
	for i := range 100 {
		lines = append(lines, strings.Repeat("x", 30)+string(rune('0'+i%10)))
	}
	specPath := createSpecFile(t, dir, lines)

	excerpt := engine.ExtractSpecExcerpt(specPath, "1-100")

	assert.LessOrEqual(t, 2000, len(excerpt), "excerpt should be at least 2000 chars (including truncation note)")
	assert.Contains(t, excerpt, "[...truncated, see spec lines 1-100]")
}

func TestExtractSpecExcerpt_MissingFile(t *testing.T) {
	excerpt := engine.ExtractSpecExcerpt("/nonexistent/path/spec.md", "1-5")
	assert.Equal(t, "", excerpt)
}

func TestExtractSpecExcerpt_SingleLine(t *testing.T) {
	dir := t.TempDir()
	lines := []string{"line 1", "line 2", "line 3"}
	specPath := createSpecFile(t, dir, lines)

	excerpt := engine.ExtractSpecExcerpt(specPath, "2-2")
	assert.Equal(t, "line 2", excerpt)
}

func TestExtractSpecExcerpt_InvalidRange(t *testing.T) {
	dir := t.TempDir()
	specPath := createSpecFile(t, dir, []string{"line 1"})

	excerpt := engine.ExtractSpecExcerpt(specPath, "not-a-range")
	assert.Equal(t, "", excerpt)
}

func TestExtractSpecExcerptForTask_SectionsOnlyHeadingAndBody(t *testing.T) {
	dir := t.TempDir()
	lines := []string{
		"# Spec Title",
		"",
		"## 1. Models",
		"",
		"Create a Task model.",
		"",
		"### 1.1 Task Model",
		"",
		"Task has title and status.",
		"",
		"## 2. API",
		"",
		"GET /tasks endpoint.",
	}
	specPath := createSpecFile(t, dir, lines)

	excerpt := engine.ExtractSpecExcerptForTask(specPath, "", []string{"### 1.1 Task Model"})

	assert.Equal(t, "### 1.1 Task Model\n\nTask has title and status.", excerpt)
}

func TestExtractSpecExcerptForTask_SectionBodyExtendsToSameOrShallower(t *testing.T) {
	dir := t.TempDir()
	lines := []string{
		"## 1. Models",
		"",
		"Create models.",
		"",
		"### 1.1 Task",
		"Task body.",
		"",
		"### 1.2 User",
		"User body.",
		"",
		"## 2. API",
		"API body.",
	}
	specPath := createSpecFile(t, dir, lines)

	// "## 1. Models" spans through the line before "## 2. API", including
	// its subsections 1.1 and 1.2.
	excerpt := engine.ExtractSpecExcerptForTask(specPath, "", []string{"## 1. Models"})

	assert.Equal(t, "## 1. Models\n\nCreate models.\n\n### 1.1 Task\nTask body.\n\n### 1.2 User\nUser body.", excerpt)
}

func TestExtractSpecExcerptForTask_MultipleSectionsJoinedByBlankLine(t *testing.T) {
	dir := t.TempDir()
	lines := []string{
		"## A",
		"Body A.",
		"## B",
		"Body B.",
	}
	specPath := createSpecFile(t, dir, lines)

	excerpt := engine.ExtractSpecExcerptForTask(specPath, "", []string{"## A", "## B"})

	assert.Equal(t, "## A\nBody A.\n\n## B\nBody B.", excerpt)
}

func TestExtractSpecExcerptForTask_SectionNotFoundContributesNothing(t *testing.T) {
	dir := t.TempDir()
	specPath := createSpecFile(t, dir, []string{"## A", "Body A."})

	excerpt := engine.ExtractSpecExcerptForTask(specPath, "", []string{"## Missing", "## A"})

	assert.Equal(t, "## A\nBody A.", excerpt)
}

func TestExtractSpecExcerptForTask_SourceLinesTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	specPath := createSpecFile(t, dir, []string{"## A", "line 2", "line 3"})

	// When both anchors are present, source_lines wins (existing behavior).
	excerpt := engine.ExtractSpecExcerptForTask(specPath, "3-3", []string{"## A"})

	assert.Equal(t, "line 3", excerpt)
}

func TestExtractSpecExcerptForTask_SectionsTruncation(t *testing.T) {
	dir := t.TempDir()
	lines := []string{"## Big"}
	for len(strings.Join(lines, "\n")) <= 2000 {
		lines = append(lines, strings.Repeat("x", 50))
	}
	specPath := createSpecFile(t, dir, lines)

	excerpt := engine.ExtractSpecExcerptForTask(specPath, "", []string{"## Big"})

	assert.LessOrEqual(t, 2000, len(excerpt), "excerpt should include the truncation note")
	assert.Contains(t, excerpt, "[...truncated, see spec sections ## Big]")
}

func TestExtractSpecExcerptForTask_EmptyAnchors(t *testing.T) {
	dir := t.TempDir()
	specPath := createSpecFile(t, dir, []string{"## A", "body"})

	assert.Equal(t, "", engine.ExtractSpecExcerptForTask(specPath, "", nil))
	assert.Equal(t, "", engine.ExtractSpecExcerptForTask("", "", []string{"## A"}))
}
