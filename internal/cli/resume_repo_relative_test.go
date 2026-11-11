package cli_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repoRelativeRepo is a git repository holding spec/s.md, its adjacent task
// file carrying tasksJSON, and an empty sub/ directory to run commands from.
// It returns the repository root.
func repoRelativeRepo(t *testing.T, tasksJSON string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "spec"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", ".keep"), nil, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec", "s.md"), []byte("# S\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec", "s.tasks.json"),
		[]byte(`{"spec":"s.md","tasks":`+tasksJSON+`}`), 0o600))
	return dir
}

// writeCleanReviewRounds records n clean review rounds over spec/s.md as it
// now stands, so editing the spec afterwards makes the review stale.
func writeCleanReviewRounds(t *testing.T, dir string, n int) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "spec", "s.md"))
	require.NoError(t, err)
	hash := fmt.Sprintf("sha256:%x", sha256.Sum256(data))
	rounds := make([]map[string]any, 0, n)
	for i := range n {
		rounds = append(rounds, map[string]any{"round": i + 1, "clean": true, "spec_hash": hash})
	}
	stateDir := filepath.Join(dir, "spec", ".tp-review", "s")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	out, err := json.Marshal(map[string]any{"spec": "s.md", "review_rounds": rounds, "audit_rounds": []any{}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "state.json"), out, 0o600))
}

// resumeFrom runs tp resume in dir against the task file at file and decodes it.
func resumeFrom(t *testing.T, dir, file string) map[string]any {
	t.Helper()
	out, stderr, code := runTP(t, dir, "--file", file, "resume")
	require.Equal(t, 0, code, "resume failed: %s", stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	return res
}

// resumePathStrings collects every string of a resume payload that names the
// spec: the top-level spec, each blocker's data.spec and message, next_action's
// command, brief_command and summary, and each next unit's brief_command.
func resumePathStrings(res map[string]any) []string {
	var out []string
	add := func(v any) {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	add(res["spec"])
	for _, b := range res["blockers"].([]any) {
		bm := b.(map[string]any)
		add(bm["message"])
		if data, ok := bm["data"].(map[string]any); ok {
			add(data["spec"])
		}
	}
	na := res["next_action"].(map[string]any)
	add(na["command"])
	add(na["brief_command"])
	add(na["summary"])
	for _, u := range res["next_units"].([]any) {
		add(u.(map[string]any)["brief_command"])
	}
	return out
}

// assertNoAbsolutePath fails when any string carries the repository's
// absolute path, in either its given or its symlink-resolved form.
func assertNoAbsolutePath(t *testing.T, dir string, strs []string) {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	for _, s := range strs {
		assert.NotContains(t, s, dir, "no absolute path in %q", s)
		assert.NotContains(t, s, resolved, "no absolute path in %q", s)
	}
}

// TestResume_SpecStaleNamesTheSpecRelativeToTheRepositoryRoot: the spec-stale
// blocker's data.spec, its message and the top-level spec are repo-relative,
// from the root and from a subdirectory alike, and tp export --list's
// escalations carry the same form.
func TestResume_SpecStaleNamesTheSpecRelativeToTheRepositoryRoot(t *testing.T) {
	t.Parallel()
	dir := repoRelativeRepo(t, `[{"id":"t1","title":"T","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
	writeCleanReviewRounds(t, dir, 2)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec", "s.md"), []byte("# S\n\nedited after review\n"), 0o600))
	initGitRepo(t, dir)

	for _, run := range []struct{ cwd, file string }{
		{dir, "spec/s.tasks.json"},
		{filepath.Join(dir, "sub"), "../spec/s.tasks.json"},
	} {
		res := resumeFrom(t, run.cwd, run.file)
		assert.Equal(t, "spec/s.md", res["spec"], "from %s", run.cwd)
		b := blockerByCode(res, "spec-stale")
		require.NotNil(t, b, "the edited spec raises spec-stale")
		assert.Equal(t, res["spec"], b["data"].(map[string]any)["spec"], "data.spec is the top-level spec")
		assert.Contains(t, b["message"], "tp review spec/s.md --record <file>")
		assertNoAbsolutePath(t, dir, resumePathStrings(res))
	}

	_, rows := exportList(t, dir, false)
	require.Len(t, rows, 1)
	require.NotEmpty(t, rows[0].Escalations)
	raw := []string{rows[0].NextStep}
	for _, e := range rows[0].Escalations {
		raw = append(raw, string(e))
		var esc struct {
			Code string         `json:"code"`
			Data map[string]any `json:"data"`
		}
		require.NoError(t, json.Unmarshal(e, &esc))
		if esc.Code == "spec-stale" {
			assert.Equal(t, "spec/s.md", esc.Data["spec"], "the escalation names the spec repo-relative")
		}
	}
	assertNoAbsolutePath(t, dir, raw)
}

// TestResume_NextActionCommandsAreRepoRelativeAndRunFromASubdirectory: in the
// review phase next_action's command and brief_command and every next unit's
// brief_command name the spec relative to the repository root, and the
// commands they name still find the spec when run from a subdirectory.
func TestResume_NextActionCommandsAreRepoRelativeAndRunFromASubdirectory(t *testing.T) {
	t.Parallel()
	dir := repoRelativeRepo(t, `[]`)
	initGitRepo(t, dir)
	sub := filepath.Join(dir, "sub")

	res := resumeFrom(t, sub, "../spec/s.tasks.json")
	assert.Equal(t, "review", res["phase"])
	assert.Equal(t, "spec/s.md", res["spec"])
	cmd, _ := nextAction(res)
	assert.Equal(t, "tp review spec/s.md", cmd)
	assert.NotEmpty(t, res["next_units"], "the review phase names its role units")
	assertNoAbsolutePath(t, dir, resumePathStrings(res))

	for _, args := range [][]string{
		{"review", "spec/s.md", "--status"},
		{"audit", "spec/s.md", "--status"},
	} {
		_, stderr, code := runTP(t, sub, args...)
		assert.Equal(t, 0, code, "%v from a subdirectory: %s", args, stderr)
	}
}
