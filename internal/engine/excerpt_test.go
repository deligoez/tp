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
