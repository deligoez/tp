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
		`{"severity":"low","category":"ambiguity","class":"ambiguity","location":"## API","finding":"unclear endpoint","suggestion":"specify path"}`,
		`{"severity":"high","category":"completeness","location":"## Models","finding":"missing field validation","suggestion":"add validation"}`,
	})
	f2 := writeFindingsFile(t, dir, "f2.ndjson", []string{
		// Duplicate of f1 line 1 but with higher severity
		`{"severity":"high","category":"ambiguity","class":"ambiguity","location":"## API","finding":"unclear endpoint","suggestion":"be more specific"}`,
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
		`{"severity":"low","category":"zzz","class":"redundancy","location":"## A","finding":"finding one","suggestion":"fix"}`,
		`{"severity":"high","category":"aaa","location":"## B","finding":"finding two","suggestion":"fix"}`,
		// Duplicate of first
		`{"severity":"medium","category":"zzz","class":"redundancy","location":"## A","finding":"finding one","suggestion":"different suggestion"}`,
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

// TestReviewMerge_RejectsSpecPositionalExit2 covers §4.1: --merge takes only
// its explicit NDJSON inputs, so a spec-looking positional (a .md) among them
// is rejected at entry with exit 2 rather than silently parsed as data.
func TestReviewMerge_RejectsSpecPositionalExit2(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(spec, []byte("# Spec\n## 1. A\nbody\n"), 0o600))
	f1 := writeFindingsFile(t, dir, "a.ndjson", []string{
		`{"severity":"low","location":"## 1. A","finding":"x","class":"x"}`,
	})

	// Spec mixed in among real inputs is rejected.
	_, stderr, code := runTPMerge(t, dir, "review", "--merge", spec, f1)
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "looks like a spec")
	assert.Contains(t, stderr, "--merge takes NDJSON input files only")

	// A spec alone is likewise rejected (not parsed, no per-line warnings).
	_, stderr, code = runTPMerge(t, dir, "review", "--merge", spec)
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "looks like a spec")
	assert.NotContains(t, stderr, "warning:")
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

	stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", "--json", f1)
	require.Equal(t, 0, code, "all-invalid input is a clean result (§3.3), not an error: %s", stderr)

	// No findings survive: the JSON summary reports merged_count 0.
	var summary map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &summary))
	assert.Equal(t, float64(0), summary["merged_count"])

	// Both lines were skipped with a stderr warning naming malformed vs incomplete.
	assert.Contains(t, stderr, "warning: skipping malformed")
	assert.Contains(t, stderr, "warning: skipping incomplete")
}

// TestReviewMergeAllEmptyFilesCreateEmptyOutput covers §3.3 row 2: files present
// but holding zero findings succeed (exit 0), create a zero-byte -o file, and
// report merged_count 0 with no warnings.
func TestReviewMergeAllEmptyFilesCreateEmptyOutput(t *testing.T) {
	dir := t.TempDir()

	f1 := writeFindingsFile(t, dir, "f1.ndjson", []string{})
	f2 := writeFindingsFile(t, dir, "f2.ndjson", []string{})
	out := filepath.Join(dir, "merged.ndjson")

	stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", "-o", out, f1, f2)
	require.Equal(t, 0, code, "all-empty input is a clean result (§3.3): %s", stderr)

	info, err := os.Stat(out)
	require.NoError(t, err, "-o file must be created even when there are no findings")
	assert.Equal(t, int64(0), info.Size(), "-o file must be zero bytes")

	var summary map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &summary))
	assert.Equal(t, float64(0), summary["merged_count"])

	assert.NotContains(t, stderr, "warning:", "no malformed/incomplete lines -> no warnings")
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

// TestReviewMergeClustersByLocationClass covers the new (location key, class)
// clustering: findings sharing a §-token key and class merge into one cluster
// whose representative is the highest-severity member, annotated with found_by
// and found_by_roles (§8.1, §8.4).
func TestReviewMergeClustersByLocationClass(t *testing.T) {
	dir := t.TempDir()
	f1 := writeFindingsFile(t, dir, "f1.ndjson", []string{
		`{"severity":"high","role":"implementer","class":"dedup-gap","location":"§8.2 detail","finding":"empty key collapses"}`,
		`{"severity":"low","role":"tester","class":"dedup-gap","location":"§8.2 other words","finding":"same cluster paraphrase"}`,
		`{"severity":"medium","role":"architect","class":"attribution","location":"§8.2 yet more","finding":"different class stays apart"}`,
	})
	stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", f1)
	require.Equal(t, 0, code, "merge failed: %s", stderr)

	lines := parseNDJSON(t, stdout)
	require.Len(t, lines, 2, "§8.2+dedup-gap clusters into one; the attribution class stays apart")

	var clustered map[string]any
	for _, l := range lines {
		if l["class"] == "dedup-gap" {
			clustered = l
		}
	}
	require.NotNil(t, clustered, "the dedup-gap cluster is emitted")
	assert.Equal(t, "high", clustered["severity"], "representative is the highest-severity member")
	assert.Equal(t, "empty key collapses", clustered["finding"], "representative row is emitted verbatim")
	assert.Equal(t, float64(2), clustered["found_by"])
	roles, ok := clustered["found_by_roles"].([]any)
	require.True(t, ok, "found_by_roles is present for a multi-role cluster")
	assert.ElementsMatch(t, []any{"implementer", "tester"}, roles)
}

// TestReviewMergeAbsentClassNotMerged confirms findings without a class are never
// merged: each is its own singleton (§8.3), so an empty class cannot collapse
// unrelated findings even at the same location.
func TestReviewMergeAbsentClassNotMerged(t *testing.T) {
	dir := t.TempDir()
	f1 := writeFindingsFile(t, dir, "f1.ndjson", []string{
		`{"severity":"high","role":"implementer","location":"§8.2","finding":"first no-class finding"}`,
		`{"severity":"high","role":"tester","location":"§8.2","finding":"second no-class finding"}`,
	})
	stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", f1)
	require.Equal(t, 0, code, "merge failed: %s", stderr)

	lines := parseNDJSON(t, stdout)
	assert.Len(t, lines, 2, "absent class -> each finding is its own cluster, never merged")
	for _, l := range lines {
		assert.Equal(t, float64(1), l["found_by"], "each singleton has its one diversity role")
	}
}

// TestReviewMergeAuditRecordUntouched confirms clustering is review-only: tp audit
// --record still counts every non-PASS row, so two FAIL rows sharing a location
// and class remain two findings (§8.1).
func TestReviewMergeAuditRecordUntouched(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
	out, stderr, code := auditRecord(t, dir,
		`{"id":"a","status":"FAIL","class":"security-gap","location":"§3.2"}`+"\n"+
			`{"id":"b","status":"FAIL","class":"security-gap","location":"§3.2"}`+"\n")
	require.Equal(t, 0, code, "record failed: %s", stderr)
	assert.Equal(t, float64(2), out["findings"], "audit --record counts each non-PASS row without clustering")
}

// TestReviewMergeOverlapReport surfaces the per-role overlap report in the --merge
// JSON summary: unique/shared counts, the trim-candidate flag for a role that
// found only shared clusters, and exclusion of the built-in regression role (§8.5).
func TestReviewMergeOverlapReport(t *testing.T) {
	dir := t.TempDir()
	f1 := writeFindingsFile(t, dir, "f1.ndjson", []string{
		`{"severity":"high","role":"implementer","class":"A","location":"§1","finding":"impl unique"}`,
		`{"severity":"high","role":"implementer","class":"B","location":"§2","finding":"impl shared b"}`,
		`{"severity":"low","role":"tester","class":"B","location":"§2 words","finding":"tester shared b"}`,
		`{"severity":"medium","role":"architect","class":"C","location":"§3","finding":"arch shared c"}`,
		`{"severity":"low","role":"tester","class":"C","location":"§3 more","finding":"tester shared c"}`,
		`{"severity":"high","role":"regression","class":"D","location":"§4","finding":"regression only"}`,
	})
	stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", "--json", f1)
	require.Equal(t, 0, code, "merge failed: %s", stderr)

	var summary map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &summary))
	report, ok := summary["overlap_report"].([]any)
	require.True(t, ok, "overlap_report present in --merge JSON")

	byRole := map[string]map[string]any{}
	for _, r := range report {
		m := r.(map[string]any)
		byRole[m["role"].(string)] = m
	}
	// implementer: unique=1 (§1/A), shared=1 (§2/B) -> not a trim candidate.
	assert.Equal(t, float64(1), byRole["implementer"]["unique"])
	assert.Equal(t, float64(1), byRole["implementer"]["shared"])
	assert.Equal(t, false, byRole["implementer"]["trim_candidate"])
	// tester: unique=0, shared=2 (§2/B and §3/C) -> trim candidate.
	assert.Equal(t, float64(0), byRole["tester"]["unique"])
	assert.Equal(t, float64(2), byRole["tester"]["shared"])
	assert.Equal(t, true, byRole["tester"]["trim_candidate"])
	// architect: unique=0, shared=1 -> trim candidate.
	assert.Equal(t, true, byRole["architect"]["trim_candidate"])
	// regression is never in the report.
	assert.NotContains(t, byRole, "regression")
}
