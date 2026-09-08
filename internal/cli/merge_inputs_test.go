package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mergeInputsOf pulls the §8a.4 per-input accounting out of a --merge JSON
// summary, keyed by path so a test asserts on the file it wrote rather than on
// an array position.
func mergeInputsOf(t *testing.T, stdout string) map[string][2]int {
	t.Helper()
	var summary map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &summary), "merge summary must be JSON: %s", stdout)

	raw, ok := summary["inputs"]
	require.True(t, ok, "merge summary must carry inputs (§8a.4): %s", stdout)
	entries, ok := raw.([]any)
	require.True(t, ok, "inputs must be an array, never null (§8a.4): %v", raw)

	byPath := make(map[string][2]int, len(entries))
	for _, e := range entries {
		m, ok := e.(map[string]any)
		require.True(t, ok, "each inputs entry is an object: %v", e)
		path, ok := m["path"].(string)
		require.True(t, ok, "inputs entry needs a path: %v", m)
		parsed, ok := m["parsed"].(float64)
		require.True(t, ok, "inputs entry needs parsed: %v", m)
		skipped, ok := m["skipped"].(float64)
		require.True(t, ok, "inputs entry needs skipped: %v", m)
		byPath[path] = [2]int{int(parsed), int(skipped)}
	}
	return byPath
}

const (
	goodFinding1 = `{"role":"implementer","evidence":"read the cited section","severity":"high","class":"gap","location":"§1","finding":"missing bound","suggestion":"state it"}`
	goodFinding2 = `{"role":"implementer","evidence":"read the cited section","severity":"low","class":"nit","location":"§2","finding":"wording","suggestion":"reword"}`
	goodFinding3 = `{"role":"tester","evidence":"read the cited section","severity":"medium","class":"test","location":"§3","finding":"no boundary test","suggestion":"add one"}`

	goodAuditRow1 = `{"role":"go-safety","item_id":"a","status":"PASS"}`
	goodAuditRow2 = `{"role":"go-safety","item_id":"b","status":"FAIL","finding":"unchecked error"}`
	goodAuditRow3 = `{"role":"spec-coverage","item_id":"c","status":"PASS"}`
)

// TestReviewMerge_InputsReportPerFileCounts covers test 32 for the review
// merge: the payload names every input with its own parsed and skipped counts,
// and a file that lost some lines but kept others still exits 0.
func TestReviewMerge_InputsReportPerFileCounts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	f1 := writeFindingsFile(t, dir, "f1.ndjson", []string{
		goodFinding1,
		`{"severity":"high","finding":"no location at all"}`, // incomplete
		goodFinding2,
		`{not json`, // malformed
	})
	f2 := writeFindingsFile(t, dir, "f2.ndjson", []string{goodFinding3})

	stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", "--json", f1, f2)
	require.Equal(t, 0, code, "an input that parsed at least one line is not a dropped role: %s", stderr)

	inputs := mergeInputsOf(t, stdout)
	assert.Len(t, inputs, 2, "one entry per input file")
	assert.Equal(t, [2]int{2, 2}, inputs[f1], "f1: two findings parsed, two lines skipped")
	assert.Equal(t, [2]int{1, 0}, inputs[f2], "f2: one finding parsed, nothing skipped")
}

// TestReviewMerge_DroppedRoleExitsOne covers test 32's exit rule: a role file
// whose every content line failed to parse is silently absent from the merged
// set, so the merge exits 1 instead of letting --record freeze an undercounted
// round.
func TestReviewMerge_DroppedRoleExitsOne(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	f1 := writeFindingsFile(t, dir, "f1.ndjson", []string{goodFinding1, goodFinding2})
	// The measured failure: a reviewer emitted every line with a trailing comma.
	f2 := writeFindingsFile(t, dir, "f2.ndjson", []string{
		goodFinding3 + `,`,
		`{"role":"tester","evidence":"read the cited section","severity":"low","class":"nit","location":"§4","finding":"x"},`,
	})

	stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", "--json", f1, f2)
	require.Equal(t, 1, code, "an input with content lines and no parsed line exits 1: %s", stderr)
	assert.Contains(t, stderr, f2, "the error names the input that contributed nothing")

	inputs := mergeInputsOf(t, stdout)
	assert.Equal(t, [2]int{2, 0}, inputs[f1])
	assert.Equal(t, [2]int{0, 2}, inputs[f2], "the dropped role reports parsed 0")
}

// TestReviewMerge_SoleMalformedLineExitsOne covers test 49's second half for
// the review merge: one input, one content line, malformed.
func TestReviewMerge_SoleMalformedLineExitsOne(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f1 := writeFindingsFile(t, dir, "f1.ndjson", []string{`not json at all`})

	_, stderr, code := runTPMerge(t, dir, "review", "--merge", "--json", f1)
	assert.Equal(t, 1, code, "a sole malformed content line exits 1: %s", stderr)
}

// TestReviewMerge_BlankAndZeroByteInputsExitZero covers test 49's first half:
// blank and whitespace-only lines are neither parsed nor skipped, and a
// zero-byte file stays the documented way a role reports nothing found — so a
// clean round is unaffected.
func TestReviewMerge_BlankAndZeroByteInputsExitZero(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	blank := filepath.Join(dir, "blank.ndjson")
	require.NoError(t, os.WriteFile(blank, []byte("\n   \n\t\n\n"), 0o600))
	empty := filepath.Join(dir, "empty.ndjson")
	require.NoError(t, os.WriteFile(empty, nil, 0o600))

	stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", "--json", blank, empty)
	require.Equal(t, 0, code, "blank-only and zero-byte inputs are a clean round: %s", stderr)

	inputs := mergeInputsOf(t, stdout)
	assert.Equal(t, [2]int{0, 0}, inputs[blank], "blank lines are neither parsed nor skipped")
	assert.Equal(t, [2]int{0, 0}, inputs[empty])
}

// TestAuditMerge_InputsReportPerFileCounts covers test 32 for the audit merge —
// the same rule applies to both, because an unattended driver reads the exit
// code alone.
func TestAuditMerge_InputsReportPerFileCounts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	f1 := writeFindingsFile(t, dir, "a1.ndjson", []string{
		goodAuditRow1,
		`{"role":"go-safety","item_id":"z"}`, // incomplete: no status
		goodAuditRow2,
	})
	f2 := writeFindingsFile(t, dir, "a2.ndjson", []string{goodAuditRow3})

	stdout, stderr, code := runTPMerge(t, dir, "audit", "--merge", "--json", f1, f2)
	require.Equal(t, 0, code, "an input that parsed at least one row is not a dropped role: %s", stderr)

	inputs := mergeInputsOf(t, stdout)
	assert.Len(t, inputs, 2)
	assert.Equal(t, [2]int{2, 1}, inputs[f1])
	assert.Equal(t, [2]int{1, 0}, inputs[f2])
}

// TestAuditMerge_DroppedRoleExitsOne covers test 32's exit rule for the audit
// merge.
func TestAuditMerge_DroppedRoleExitsOne(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	f1 := writeFindingsFile(t, dir, "a1.ndjson", []string{goodAuditRow1, goodAuditRow2})
	f2 := writeFindingsFile(t, dir, "a2.ndjson", []string{goodAuditRow3 + `,`})

	stdout, stderr, code := runTPMerge(t, dir, "audit", "--merge", "--json", f1, f2)
	require.Equal(t, 1, code, "an input with content rows and no parsed row exits 1: %s", stderr)
	assert.Contains(t, stderr, f2)

	inputs := mergeInputsOf(t, stdout)
	assert.Equal(t, [2]int{2, 0}, inputs[f1])
	assert.Equal(t, [2]int{0, 1}, inputs[f2])
}

// TestAuditMerge_BlankAndZeroByteInputsExitZero covers test 49's first half for
// the audit merge.
func TestAuditMerge_BlankAndZeroByteInputsExitZero(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	blank := filepath.Join(dir, "blank.ndjson")
	require.NoError(t, os.WriteFile(blank, []byte("\n \n"), 0o600))
	empty := filepath.Join(dir, "empty.ndjson")
	require.NoError(t, os.WriteFile(empty, nil, 0o600))

	stdout, stderr, code := runTPMerge(t, dir, "audit", "--merge", "--json", blank, empty)
	require.Equal(t, 0, code, "blank-only and zero-byte inputs are a clean round: %s", stderr)

	inputs := mergeInputsOf(t, stdout)
	assert.Equal(t, [2]int{0, 0}, inputs[blank])
	assert.Equal(t, [2]int{0, 0}, inputs[empty])
}

// TestMerge_RefusedMergeLeavesOutputPathUntouched pins §5 row 10, and replaces
// the rule this test asserted through v1.0.1 — that the exit-1 path is a report
// which still writes -o. It is not: `-o` is the file the next command in a
// review loop reads, so a merge that exits non-zero creates no file at that
// path and modifies no file already there. What an operator reads is still the
// summary on stdout, which the refusal does not withhold.
//
// The mutant is HEAD, which writes the surviving row before it refuses: the
// absent path exists after the first run, and the seed is gone after the second.
func TestMerge_RefusedMergeLeavesOutputPathUntouched(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// The first input's only row omits location, so that input parses no row
	// and the merge refuses at exit 1; the second is legal on all four keys, so
	// there is a surviving row for a non-refusing merge to write.
	droppedInput := writeFindingsFile(t, dir, "dropped.ndjson", []string{
		`{"role":"implementer","severity":"high","finding":"missing bound","evidence":"read the cited section"}`,
	})
	legalInput := writeFindingsFile(t, dir, "legal.ndjson", []string{goodFinding2})
	out := filepath.Join(dir, "merged.ndjson")

	const seed = "SEED LINE THE REFUSED MERGE MUST NOT TOUCH\n"

	// The three runs share one -o path and run in order: absent, then seeded,
	// then seeded again against a missing input.
	t.Run("exit 1 creates no file at an absent -o", func(t *testing.T) {
		stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", "-o", out, droppedInput, legalInput)
		require.Equal(t, 1, code, "the dropped input refuses the merge: %s", stderr)

		// The refusal withholds the file, not the accounting: the summary is
		// still emitted and still counts the surviving row.
		var summary map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &summary), "the refused merge still emits its summary: %s", stdout)
		assert.Equal(t, float64(1), summary["merged_count"], "the surviving role's row is still counted")
		assert.NotContains(t, summary, "output_path", "no file was written, so the summary names no output path")

		_, err := os.Stat(out)
		require.Error(t, err, "a refused merge creates no file at -o")
		assert.True(t, os.IsNotExist(err), "the -o path must still not exist, got: %v", err)
	})

	t.Run("exit 1 leaves a seeded -o byte-identical", func(t *testing.T) {
		require.NoError(t, os.WriteFile(out, []byte(seed), 0o600))
		before, err := os.ReadFile(out)
		require.NoError(t, err)

		_, stderr, code := runTPMerge(t, dir, "review", "--merge", "-o", out, droppedInput, legalInput)
		require.Equal(t, 1, code, "the dropped input refuses the merge: %s", stderr)

		after, err := os.ReadFile(out)
		require.NoError(t, err, "the seeded file is still there")
		assert.Equal(t, string(before), string(after), "a refused merge modifies no file already at -o")
	})

	t.Run("exit 3 leaves a seeded -o byte-identical", func(t *testing.T) {
		require.NoError(t, os.WriteFile(out, []byte(seed), 0o600))
		missing := filepath.Join(dir, "never-written.ndjson")
		require.NoFileExists(t, missing, "the fixture's point is that this input is absent")

		_, stderr, code := runTPMerge(t, dir, "review", "--merge", "-o", out, missing, legalInput)
		require.Equal(t, 3, code, "a missing input refuses at exit 3: %s", stderr)

		after, err := os.ReadFile(out)
		require.NoError(t, err)
		assert.Equal(t, seed, string(after), "non-zero is pinned by more than exit 1")
	})
}

// TestMerge_CleanRoundStillWritesOutput pins the case that would turn §5 row 10
// into a defect. A converged round's inputs hold no content line at all — a
// role that found nothing writes a zero-byte file — and the loop reads the
// zero-byte -o that merge writes as exactly that. The refusal is keyed on a
// dropped input, which neither shape produces, so it must not reach here.
//
// The mutant is a refusal keyed on the merged row count instead: both shapes
// merge zero rows, so both would be refused and a clean round would stop
// converging.
func TestMerge_CleanRoundStillWritesOutput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		content []byte
	}{
		{"two zero-byte inputs", nil},
		{"two blank-line-only inputs", []byte("\n   \n\t\n")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			in1 := filepath.Join(dir, "r1.ndjson")
			in2 := filepath.Join(dir, "r2.ndjson")
			require.NoError(t, os.WriteFile(in1, tc.content, 0o600))
			require.NoError(t, os.WriteFile(in2, tc.content, 0o600))
			out := filepath.Join(dir, "merged.ndjson")

			stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", "-o", out, in1, in2)
			require.Equal(t, 0, code, "a clean round is not a refusal: %s", stderr)

			content, err := os.ReadFile(out)
			require.NoError(t, err, "a clean round still writes -o")
			assert.Empty(t, content, "the loop reads a zero-byte -o as nothing found")

			var summary map[string]any
			require.NoError(t, json.Unmarshal([]byte(stdout), &summary), "summary must be JSON: %s", stdout)
			assert.Equal(t, out, summary["output_path"], "the summary names the file it wrote")
		})
	}
}

// TestMerge_UnwritableOutputLeavesNoPartialFile pins the other half of §5 row
// 10: the -o write itself goes to a temporary path and is renamed on success,
// so a write that fails leaves neither a truncated -o nor the temporary file
// beside it. The fixture makes the rename fail by naming a directory as -o.
//
// The mutant is a write path that leaves its temporary file behind when the
// rename fails: the directory's parent then holds one more entry than it did.
func TestMerge_UnwritableOutputLeavesNoPartialFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	f1 := writeFindingsFile(t, dir, "f1.ndjson", []string{goodFinding1, goodFinding2})
	// -o names a directory, so the rename onto it cannot succeed.
	out := filepath.Join(dir, "merged.ndjson")
	require.NoError(t, os.Mkdir(out, 0o700))

	before, err := os.ReadDir(dir)
	require.NoError(t, err)

	_, stderr, code := runTPMerge(t, dir, "review", "--merge", "-o", out, f1)
	require.Equal(t, 3, code, "an unwritable -o is a file error: %s", stderr)

	after, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Equal(t, len(before), len(after), "no partial or temporary file is left behind: %v", after)

	entries, err := os.ReadDir(out)
	require.NoError(t, err, "the directory named as -o is still a directory")
	assert.Empty(t, entries, "nothing was written inside it either")
}

// TestMerge_OutputGoesThroughATemporaryFile pins the mechanism rather than one
// of its consequences. Leaving no partial file behind is not enough to tell
// temp-and-rename from a plain os.WriteFile — an open that fails truncates
// nothing either — so this fixture separates them where they actually differ:
// the destination directory is not writable while the seeded -o inside it is.
// A temporary file cannot be created there, so the merge fails at exit 3 and
// the seed survives; os.WriteFile would open the existing file and replace it.
//
// The mutant is that os.WriteFile.
func TestMerge_OutputGoesThroughATemporaryFile(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root writes into a directory whose mode forbids it, so the fixture cannot separate the two")
	}
	dir := t.TempDir()

	f1 := writeFindingsFile(t, dir, "f1.ndjson", []string{goodFinding1})

	locked := filepath.Join(dir, "locked")
	require.NoError(t, os.Mkdir(locked, 0o700))
	out := filepath.Join(locked, "merged.ndjson")
	const seed = "SEED LINE IN AN UNWRITABLE DIRECTORY\n"
	require.NoError(t, os.WriteFile(out, []byte(seed), 0o600))

	require.NoError(t, os.Chmod(locked, 0o500))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	// The fixture's point is that the file itself is still writable: only the
	// directory forbids creating a new entry.
	f, err := os.OpenFile(out, os.O_WRONLY, 0o600)
	require.NoError(t, err, "the seeded file must stay writable, or this measures the wrong permission")
	require.NoError(t, f.Close())

	_, stderr, code := runTPMerge(t, dir, "review", "--merge", "-o", out, f1)
	require.Equal(t, 3, code, "a directory that admits no temporary file is a file error: %s", stderr)

	after, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, seed, string(after), "the merge never opened the destination itself")
}
