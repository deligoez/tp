package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// reviewInputWording is what the review/audit hints say about THEIR inputs:
// the reviewers'/auditors' NDJSON, the spec or task file it is not, the
// .tp-review state directory, a finding per line with its note. None of it
// describes anything a tp epic command reads or writes.
var reviewInputWording = []string{"reviewers/auditors", "task file", ".tp-review", "finding per line", "oversized note"}

// chmodForTest sets path's mode and restores a writable one at cleanup, so
// t.TempDir can remove the tree.
func chmodForTest(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	require.NoError(t, os.Chmod(path, mode))
	t.Cleanup(func() { _ = os.Chmod(path, 0o755) }) //nolint:gosec // restores a test directory
}

// recordedEpicRound emits and records one round of --epic b and returns its
// round file.
func recordedEpicRound(t *testing.T, dir string) string {
	t.Helper()
	runEpicReview(t, dir, "--epic", "b")
	recordEpicRound(t, dir, epicHighRow, "--epic", "b")
	files := epicRoundFiles(t, dir)
	require.Len(t, files, 1)
	return files[0]
}

// TestEpicErrorHintsNameTheEpicInput is the class repair for
// refusal-hint-misdiagnoses: every tp epic error path that used to reuse a
// review/audit hint now names the input it actually failed on. Each case is
// one such site; reverting any one to the shared hint fails its row.
func TestEpicErrorHintsNameTheEpicInput(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root ignores the permission bits the write-failure rows rely on")
	}
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, dir string) []string
		code  int
		want  string
	}{
		{"add --bulk line over the read cap", func(t *testing.T, dir string) []string {
			t.Helper()
			bulk := filepath.Join(dir, "epics.ndjson")
			row := `{"id": "z", "title": "` + strings.Repeat("x", 1024*1024+16) + `", "milestone": "A"}` + "\n"
			require.NoError(t, os.WriteFile(bulk, []byte(row), 0o600))
			return []string{"epic", "add", "--bulk", bulk}
		}, 3, "one epic JSON object per line"},
		{"add cannot write the graph", func(t *testing.T, dir string) []string {
			t.Helper()
			chmodForTest(t, filepath.Join(dir, ".tp"), 0o500)
			return []string{"epic", "add", `{"id": "z", "title": "Z", "milestone": "A"}`}
		}, 3, ".tp/epics.json"},
		{"import cannot write the graph", func(t *testing.T, dir string) []string {
			t.Helper()
			payload := filepath.Join(dir, "graph.json")
			require.NoError(t, os.WriteFile(payload, []byte(epicChain), 0o600))
			chmodForTest(t, filepath.Join(dir, ".tp"), 0o500)
			return []string{"epic", "import", payload, "--force"}
		}, 3, ".tp/epics.json"},
		{"review cannot write its snapshot", func(t *testing.T, dir string) []string {
			t.Helper()
			state := filepath.Join(dir, ".tp", "epic-review")
			require.NoError(t, os.Mkdir(state, 0o755))
			chmodForTest(t, state, 0o500)
			return []string{"epic", "review", "--epic", "b"}
		}, 3, ".tp/epic-review/"},
		{"review --record of a missing file", func(t *testing.T, dir string) []string {
			t.Helper()
			runEpicReview(t, dir, "--epic", "b")
			return []string{"epic", "review", "--epic", "b", "--record", filepath.Join(dir, "missing.ndjson")}
		}, 3, "epic:<id>"},
		{"review --record cannot write the round", func(t *testing.T, dir string) []string {
			t.Helper()
			path := epicRecordFile(t, dir, epicHighRow)
			state := filepath.Join(dir, ".tp", "epic-review")
			require.NoError(t, os.Mkdir(state, 0o755))
			chmodForTest(t, state, 0o500)
			return []string{"epic", "review", "--epic", "b", "--record", path}
		}, 3, ".tp/epic-review/"},
		{"review over a corrupt last round", func(t *testing.T, dir string) []string {
			t.Helper()
			require.NoError(t, os.WriteFile(recordedEpicRound(t, dir), []byte("not json\n"), 0o600))
			return []string{"epic", "review", "--epic", "b"}
		}, 3, "recorded epic review round"},
		{"review --status over a corrupt earlier round", func(t *testing.T, dir string) []string {
			t.Helper()
			first := recordedEpicRound(t, dir)
			runEpicReview(t, dir, "--epic", "b")
			recordEpicRound(t, dir, epicHighRow, "--epic", "b")
			require.NoError(t, os.WriteFile(first, []byte("not json\n"), 0o600))
			return []string{"epic", "review", "--epic", "b", "--status"}
		}, 3, "recorded epic review round"},
		{"review --resolve cannot rewrite the round", func(t *testing.T, dir string) []string {
			t.Helper()
			chmodForTest(t, recordedEpicRound(t, dir), 0o400)
			return []string{"epic", "review", "--epic", "b", "--resolve", "0", "fixed"}
		}, 3, "recorded epic review round"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := epicGraphDir(t, epicChain)
			_, stderr, code := runTP(t, dir, tc.setup(t, dir)...)
			require.Equal(t, tc.code, code, "stderr: %s", stderr)
			hint := hintOf(t, stderr)
			assert.Contains(t, hint, tc.want, "the hint names the input this command failed on")
			for _, word := range reviewInputWording {
				assert.NotContains(t, hint, word, "the hint describes a review/audit input, not this one")
			}
		})
	}
}
