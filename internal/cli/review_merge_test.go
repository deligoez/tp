package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runTPMerge runs tp with stderr always captured (even on success).
// Unlike runTP, it does NOT prepend --json since merge outputs NDJSON.
func runTPMerge(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "NO_COLOR=1")

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()

	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("unexpected error running tp: %v", err)
	}

	return stdout, stderr, exitCode
}

func writeFindingsFile(t *testing.T, dir, name string, lines []string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	content := strings.Join(lines, "\n") + "\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestReviewMergeTwoFilesDedup(t *testing.T) {
	dir := t.TempDir()

	f1 := writeFindingsFile(t, dir, "f1.ndjson", []string{
		`{"severity":"low","category":"ambiguity","location":"## API","finding":"unclear endpoint","suggestion":"specify path"}`,
		`{"severity":"high","category":"completeness","location":"## Models","finding":"missing field validation","suggestion":"add validation"}`,
	})
	f2 := writeFindingsFile(t, dir, "f2.ndjson", []string{
		// Duplicate of f1 line 1 but with higher severity
		`{"severity":"high","category":"ambiguity","location":"## API","finding":"unclear endpoint","suggestion":"be more specific"}`,
		`{"severity":"medium","category":"consistency","location":"## Tests","finding":"test naming inconsistent","suggestion":"use convention"}`,
	})

	stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", f1, f2)
	require.Equal(t, 0, code, "merge should succeed: %s", stderr)

	// Parse NDJSON output
	lines := parseNDJSON(t, stdout)
	assert.Len(t, lines, 3, "should have 3 unique findings (1 duplicate removed)")

	// Check summary on stderr
	assert.Contains(t, stderr, "3 unique findings from 2 files (1 duplicates removed)")

	// The duplicate "unclear endpoint" should have kept the high severity version
	for _, f := range lines {
		finding, _ := f["finding"].(string)
		if finding == "unclear endpoint" {
			assert.Equal(t, "high", f["severity"], "should keep highest severity")
		}
	}

	// Verify sorted by severity: high findings first, then medium, then low
	assert.Equal(t, "high", lines[0]["severity"])
}

func TestReviewMergeThreeFilesAllUniqueSorted(t *testing.T) {
	dir := t.TempDir()

	f1 := writeFindingsFile(t, dir, "f1.ndjson", []string{
		`{"severity":"low","category":"redundancy","location":"## A","finding":"finding one","suggestion":"fix"}`,
	})
	f2 := writeFindingsFile(t, dir, "f2.ndjson", []string{
		`{"severity":"critical","category":"completeness","location":"## B","finding":"finding two","suggestion":"fix"}`,
	})
	f3 := writeFindingsFile(t, dir, "f3.ndjson", []string{
		`{"severity":"medium","category":"ambiguity","location":"## C","finding":"finding three","suggestion":"fix"}`,
	})

	stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", f1, f2, f3)
	require.Equal(t, 0, code, "merge should succeed: %s", stderr)

	lines := parseNDJSON(t, stdout)
	assert.Len(t, lines, 3)

	// Verify sort order: critical, medium, low
	assert.Equal(t, "critical", lines[0]["severity"])
	assert.Equal(t, "medium", lines[1]["severity"])
	assert.Equal(t, "low", lines[2]["severity"])

	assert.Contains(t, stderr, "3 unique findings from 3 files (0 duplicates removed)")
}

func TestReviewMergeInvalidLinesSkipped(t *testing.T) {
	dir := t.TempDir()

	f1 := writeFindingsFile(t, dir, "f1.ndjson", []string{
		`{"severity":"high","category":"completeness","location":"## A","finding":"valid finding","suggestion":"fix"}`,
		`not json at all`,
		`{"severity":"","finding":"missing severity value"}`,
		``,
		`{"severity":"medium","category":"ambiguity","location":"## B","finding":"another valid","suggestion":"fix"}`,
	})

	stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", f1)
	require.Equal(t, 0, code, "merge should succeed: %s", stderr)

	lines := parseNDJSON(t, stdout)
	assert.Len(t, lines, 2, "only valid findings should be included")

	// Warnings should appear in stderr
	assert.Contains(t, stderr, "warning: skipping")
}

func TestReviewMergeSingleFileNormalize(t *testing.T) {
	dir := t.TempDir()

	f1 := writeFindingsFile(t, dir, "f1.ndjson", []string{
		`{"severity":"low","category":"zzz","location":"## A","finding":"finding one","suggestion":"fix"}`,
		`{"severity":"high","category":"aaa","location":"## B","finding":"finding two","suggestion":"fix"}`,
		// Duplicate of first
		`{"severity":"medium","category":"zzz","location":"## A","finding":"finding one","suggestion":"different suggestion"}`,
	})

	stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", f1)
	require.Equal(t, 0, code, "merge should succeed: %s", stderr)

	lines := parseNDJSON(t, stdout)
	assert.Len(t, lines, 2, "dedup should remove duplicate")

	// Sorted: high first, then medium (the kept dup)
	assert.Equal(t, "high", lines[0]["severity"])
	assert.Equal(t, "medium", lines[1]["severity"], "should keep highest severity (medium > low)")

	assert.Contains(t, stderr, "2 unique findings from 1 files (1 duplicates removed)")
}

func TestReviewMergeZeroFiles(t *testing.T) {
	dir := t.TempDir()

	_, stderr, code := runTPMerge(t, dir, "review", "--merge")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "at least 1 file required for merge")
}

func TestReviewMergeMissingFile(t *testing.T) {
	dir := t.TempDir()

	_, stderr, code := runTPMerge(t, dir, "review", "--merge", filepath.Join(dir, "nonexistent.ndjson"))
	assert.Equal(t, 3, code)
	assert.Contains(t, stderr, "file not found")
}

func TestReviewMergePreservesExtraFields(t *testing.T) {
	dir := t.TempDir()

	f1 := writeFindingsFile(t, dir, "f1.ndjson", []string{
		`{"severity":"high","category":"completeness","location":"## A","finding":"some finding","suggestion":"fix","resolved":"fixed","custom_field":"hello","round":2}`,
	})

	stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", f1)
	require.Equal(t, 0, code, "merge should succeed: %s", stderr)

	lines := parseNDJSON(t, stdout)
	require.Len(t, lines, 1)

	// Check extra fields are preserved
	assert.Equal(t, "fixed", lines[0]["resolved"])
	assert.Equal(t, "hello", lines[0]["custom_field"])
	assert.Equal(t, float64(2), lines[0]["round"]) // JSON numbers decode as float64
}

func TestReviewMergeOutputFile(t *testing.T) {
	dir := t.TempDir()

	f1 := writeFindingsFile(t, dir, "f1.ndjson", []string{
		`{"severity":"high","category":"completeness","location":"## A","finding":"some finding","suggestion":"fix"}`,
	})
	outPath := filepath.Join(dir, "merged.ndjson")

	stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", "--output", outPath, f1)
	require.Equal(t, 0, code, "merge should succeed: %s", stderr)

	// stdout should have JSON summary when -o is used
	var summary map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &summary))
	assert.Equal(t, float64(1), summary["merged_count"])

	// Output file should exist and contain the finding
	content, err := os.ReadFile(outPath)
	require.NoError(t, err)

	lines := parseNDJSON(t, string(content))
	assert.Len(t, lines, 1)
	assert.Equal(t, "some finding", lines[0]["finding"])
}

func TestReviewMergeAllInvalid(t *testing.T) {
	dir := t.TempDir()

	f1 := writeFindingsFile(t, dir, "f1.ndjson", []string{
		`not valid json`,
		`{"no_severity": true, "finding": "has finding but no severity"}`,
	})

	_, stderr, code := runTPMerge(t, dir, "review", "--merge", f1)
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "no valid findings in input files")
}

// parseNDJSON splits NDJSON output into parsed maps.
func parseNDJSON(t *testing.T, s string) []map[string]any {
	t.Helper()
	var results []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &m), "failed to parse NDJSON line: %s", line)
		results = append(results, m)
	}
	return results
}
