package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// longFindingLine returns one NDJSON review finding whose finding text is n
// bytes long — the shape that used to divide the readers: --resolve rewrote it
// at a 1MB cap while --merge and --report read it at bufio's 64KB default.
func longFindingLine(n int) string {
	return fmt.Sprintf(`{"severity":"high","category":"correctness","location":"## API","finding":%q}`,
		strings.Repeat("x", n))
}

// TestReviewVerifyUnreadableFindingsFailsLoudly: readVerifyFindings answered
// every os.ReadFile error with an empty set, and the upstream guard rejects
// only a missing path. `tp review --verify --findings <unreadable>` therefore
// exited 0 with empty stderr and told the verifier "Previous review rounds
// produced 0 findings … If verifier finds 0 issues, review is complete" — the
// convergence signal, from a file tp never read. The same file through
// mustParseFindingsFile already aborted; both --findings consumers now do.
func TestReviewVerifyUnreadableFindingsFailsLoudly(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a 0o000 file, so the open never fails")
	}
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n\n## Section\n\nContent.\n"), 0o600))

	findingsPath := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(findingsPath,
		[]byte(`{"severity":"critical","category":"correctness","location":"## A","finding":"missing invariant"}`+"\n"), 0o600))
	require.NoError(t, os.Chmod(findingsPath, 0o000))
	t.Cleanup(func() { _ = os.Chmod(findingsPath, 0o600) })

	stdout, stderr, code := runTP(t, dir, "review", "--verify", "--findings", findingsPath, specPath)
	assert.Equal(t, 3, code, "an unreadable findings file is a file error, not a silent empty set")
	assert.Contains(t, stderr, findingsPath, "stderr names the file tp could not read")
	assert.NotContains(t, stdout, "produced 0 findings", "no verifier prompt is emitted from findings tp never read")
}

// TestReviewVerifyFindingsDirectoryFailsLoudly: a directory passes the
// existence guard and fails the read, which is exactly the arm that used to
// come back empty — `--findings .tp-review/0.33.0` instead of the file inside
// it is the likelier operator slip of the two.
func TestReviewVerifyFindingsDirectoryFailsLoudly(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n\n## Section\n\nContent.\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", "--verify", "--findings", dir, specPath)
	assert.Equal(t, 3, code, "a directory handed to --findings is a file error, not zero findings")
	assert.Contains(t, stderr, dir, "stderr names the path tp could not read")
	assert.NotContains(t, stdout, "produced 0 findings", "no verifier prompt is emitted from a path tp never read")
}

// TestReviewVerifyOverLongLineFailsLoudly: the second half of the same swallow.
// A line past the read cap left the earlier findings in place and dropped the
// rest, so a three-finding file reported one — a smaller set reaching the same
// "review is complete" instruction, with a warning as its only signal.
func TestReviewVerifyOverLongLineFailsLoudly(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n\n## Section\n\nContent.\n"), 0o600))

	findingsPath := filepath.Join(dir, "findings.ndjson")
	content := longFindingLine(64) + "\n" + longFindingLine(2*1024*1024) + "\n" + longFindingLine(64) + "\n"
	require.NoError(t, os.WriteFile(findingsPath, []byte(content), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", "--verify", "--findings", findingsPath, specPath)
	assert.Equal(t, 3, code, "a truncated read is a file error, not a smaller findings set")
	assert.NotContains(t, stdout, "produced 1 findings", "the prompt must not report the rows read before the cap")

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
	hint, _ := payload["hint"].(string)
	assert.Contains(t, hint, "1MB", "the hint names the cap that was exceeded")
	assert.NotContains(t, hint, "check the path", "the path was fine — the line was too long")
}

// TestNDJSONLineCapIsUniform: --resolve rewrote findings at a 1MB cap while
// --merge and --report read them at bufio's 64KB default, so a findings file
// tp had just written could not be read back by the next mode in its own loop.
// One 200KB finding travels the whole chain.
func TestNDJSONLineCapIsUniform(t *testing.T) {
	dir := t.TempDir()
	r1 := filepath.Join(dir, "r1.ndjson")
	require.NoError(t, os.WriteFile(r1, []byte(longFindingLine(200*1024)+"\n"), 0o600))

	_, stderr, code := runTP(t, dir, "review", "--merge", r1)
	assert.Equal(t, 0, code, "--merge reads a 200KB finding: %s", stderr)

	_, stderr, code = runTP(t, dir, "review", "--report", r1)
	assert.Equal(t, 0, code, "--report reads the same file --merge just read: %s", stderr)

	_, stderr, code = runTP(t, dir, "review", "--resolve", r1, "0", "fixed", "done")
	assert.Equal(t, 0, code, "--resolve reads it too: %s", stderr)
}

// TestReviewResolveOverLongLineHintNamesTheCap covers the arm the first version
// of this guard left open: --resolve was exercised only UNDER the cap, and past
// it both --resolve and --resolve-all reported the read failure through a bare
// exit 3 that named neither the file nor the cap — it inherited the code-3
// default's task-file advice, for a command that takes no task file.
func TestReviewResolveOverLongLineHintNamesTheCap(t *testing.T) {
	dir := t.TempDir()

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"resolve", []string{"--resolve", "", "0", "fixed", "done"}},
		{"resolve-all", []string{"--resolve-all", "", "fixed", "done"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.name+".ndjson")
			require.NoError(t, os.WriteFile(path, []byte(longFindingLine(2*1024*1024)+"\n"), 0o600))

			args := append([]string{"review"}, tc.args...)
			args[2] = path
			_, stderr, code := runTP(t, dir, args...)
			require.Equal(t, 3, code, "a line past the cap aborts rather than rewriting a partial set")

			var payload map[string]any
			require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
			assert.Contains(t, payload["error"], path, "the error names the file it could not update")
			hint, _ := payload["hint"].(string)
			assert.Contains(t, hint, "1MB", "the hint names the cap that was exceeded")
			assert.NotContains(t, hint, "tp use", "task-file advice repairs nothing here")
		})
	}
}

// TestReviewMergeOverLongLineHintNamesTheCap: past the shared cap the abort is
// right — a swallowed read here records a clean round — but it used to attach
// the path hint, telling the operator to check a path that was never wrong.
func TestReviewMergeOverLongLineHintNamesTheCap(t *testing.T) {
	dir := t.TempDir()
	r1 := filepath.Join(dir, "r1.ndjson")
	require.NoError(t, os.WriteFile(r1, []byte(longFindingLine(2*1024*1024)+"\n"), 0o600))

	_, stderr, code := runTP(t, dir, "review", "--merge", r1)
	require.Equal(t, 3, code, "a line past the cap aborts rather than merging a partial set")

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
	hint, _ := payload["hint"].(string)
	assert.Contains(t, hint, "1MB", "the hint names the cap that was exceeded")
	assert.NotContains(t, hint, "check the path", "the path was fine — the line was too long")
}

// TestReviewReportInputErrorContract pins --report's four input outcomes, which
// shipped with no test at all. §10.5 of spec/0.16.0-review-orchestration.md
// fixes the two usage rows at exit 2; what was wrong was their hint, which was
// the code-2 default ("see tp --help") and repairs neither.
func TestReviewReportInputErrorContract(t *testing.T) {
	dir := t.TempDir()

	t.Run("no arguments", func(t *testing.T) {
		_, stderr, code := runTP(t, dir, "review", "--report")
		require.Equal(t, 2, code, "nothing named is a usage error (§10.5)")
		assert.Contains(t, hintOf(t, stderr), "*.ndjson", "the hint names what --report takes")
	})

	t.Run("empty directory", func(t *testing.T) {
		empty := t.TempDir()
		_, stderr, code := runTP(t, dir, "review", "--report", empty)
		require.Equal(t, 2, code, "a readable directory with nothing to report on is a usage error (§10.5)")
		assert.Contains(t, hintOf(t, stderr), "*.ndjson", "the hint names what the directory should hold")
	})

	t.Run("missing file", func(t *testing.T) {
		_, stderr, code := runTP(t, dir, "review", "--report", filepath.Join(dir, "nope.ndjson"))
		require.Equal(t, 3, code, "a path tp cannot find is a file error")
		assert.Contains(t, hintOf(t, stderr), "check the path", "the hint points at the path")
	})

	t.Run("unreadable directory", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads a 0o000 directory, so the scan never fails")
		}
		locked := t.TempDir()
		require.NoError(t, os.Chmod(locked, 0o000))
		t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })

		_, stderr, code := runTP(t, dir, "review", "--report", locked)
		require.Equal(t, 3, code, "a directory tp cannot read is a file error")
		assert.Contains(t, hintOf(t, stderr), "check the path", "the hint points at the path")
	})
}

// TestReviewSpecInlineOverLongLineTruncationIsVisible: readSpecContent scanned
// lines at a 64KB cap and warned when one exceeded it, returning the lines read
// so far — so --spec-inline emitted a spec whose tail was absent with nothing
// in the prompt saying so. Reading the whole file the way the --diff-from
// branch does leaves only the documented specContentCap truncation, which says
// where it stopped.
func TestReviewSpecInlineOverLongLineTruncationIsVisible(t *testing.T) {
	dir := t.TempDir()
	goPath := filepath.Join(dir, "g.go")
	require.NoError(t, os.WriteFile(goPath, []byte("package main\n"), 0o600))

	specPath := filepath.Join(dir, "spec.md")
	spec := "# Spec\n\n## 1. Alpha\n\n" + strings.Repeat("x", 70*1024) + "\n\n## 2. Omega\n\nTail section.\n"
	require.NoError(t, os.WriteFile(specPath, []byte(spec), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", specPath,
		"--perspective", "code-audit", "--spec-inline", "--affected-files", goPath)
	require.Equal(t, 0, code, "an over-long spec line is not a failure: %s", stderr)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	prompts, ok := result["prompts"].([]any)
	require.True(t, ok, "code-audit emits a prompt")
	require.NotEmpty(t, prompts)
	prompt := prompts[0].(map[string]any)["prompt"].(string)
	assert.Contains(t, prompt, "truncated at", "the prompt says where the spec stopped rather than stopping silently")
}

// hintOf pulls the hint out of tp's JSON error envelope.
func hintOf(t *testing.T, stderr string) string {
	t.Helper()
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &payload), "stderr is tp's JSON error envelope")
	hint, _ := payload["hint"].(string)
	return hint
}
