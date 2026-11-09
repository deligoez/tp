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

// epicFileTask is one open task, the shape every --epic fixture file holds.
const epicFileTask = `{"id":"t1","title":"T","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}`

// epicFileRepo is a git repository whose graph holds the root epics b and
// u, with x.tasks.json bound to no epic, so plain discovery is ambiguous.
func epicFileRepo(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "epics.json"), []byte(`{"version": 1, "milestones": [["A"]], "epics": [
    {"id": "b", "title": "B", "milestone": "A", "root": true},
    {"id": "u", "title": "U", "milestone": "A", "root": true}
  ]}`), 0o600))
	writeEpicFile(t, dir, "x", "", "open")
	return dir
}

// writeEpicFile writes <base>.md and <base>.tasks.json bound to epic (none
// when ""), holding one task of status.
func writeEpicFile(t *testing.T, dir, base, epic, status string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, base+".md"), []byte("# "+base+"\n"), 0o600))
	task := epicFileTask
	if status != "open" {
		task = fmt.Sprintf(`{"id":"t1","title":"T","status":%q,"depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}`, status)
	}
	binding := ""
	if epic != "" {
		binding = fmt.Sprintf(`"epic":%q,`, epic)
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, base+".tasks.json"),
		[]byte(`{"version":1,"spec":"`+base+`.md",`+binding+`"tasks":[`+task+`]}`), 0o600))
}

// TestEpicFlag_TheOnlyBoundFileIsUsed is F8: b binds b1.tasks.json alone, so
// tp ready --epic b reads it although two task files make discovery ambiguous.
func TestEpicFlag_TheOnlyBoundFileIsUsed(t *testing.T) {
	t.Parallel()
	dir := epicFileRepo(t)
	writeEpicFile(t, dir, "b1", "b", "open")
	initGitRepo(t, dir)

	stdout, stderr, code := runTP(t, dir, "ready", "--epic", "b", "--ids")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Contains(t, stdout, "t1")
}

// releaseEpicFile brings <base>.tasks.json, whose tasks are done, to the
// release phase: two clean review and two clean audit rounds on its spec.
func releaseEpicFile(t *testing.T, dir, base string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, base+".md"))
	require.NoError(t, err)
	hash := fmt.Sprintf("sha256:%x", sha256.Sum256(data))
	rounds := []map[string]any{}
	for i := range 2 {
		rounds = append(rounds, map[string]any{"round": i + 1, "clean": true, "spec_hash": hash, "recorded_at": "2026-01-01T00:00:00Z"})
	}
	stateDir := filepath.Join(dir, ".tp-review", base)
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	out, err := json.Marshal(map[string]any{"spec": base + ".md", "review_rounds": rounds, "audit_rounds": rounds})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "state.json"), out, 0o600))
}

// TestEpicFlag_TheOnlyFileNotAtReleaseIsUsed is F8: b1 is at release, so tp
// resume --epic b resumes b2, the only bound file still in flight.
func TestEpicFlag_TheOnlyFileNotAtReleaseIsUsed(t *testing.T) {
	t.Parallel()
	dir := epicFileRepo(t)
	writeEpicFile(t, dir, "b1", "b", "done")
	releaseEpicFile(t, dir, "b1")
	writeEpicFile(t, dir, "b2", "b", "open")
	initGitRepo(t, dir)

	stdout, stderr, code := runTP(t, dir, "resume", "--epic", "b")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var res struct {
		Phase string `json:"phase"`
		Spec  string `json:"spec"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &res), "stdout: %s", stdout)
	assert.Equal(t, "implement", res.Phase)
	assert.Equal(t, "b2.md", filepath.Base(res.Spec))
}

// TestEpicFlag_TwoFilesInFlightExit4NamingBoth is F8: neither b1 nor b2 is at
// release, so tp next --epic b cannot choose and names both.
func TestEpicFlag_TwoFilesInFlightExit4NamingBoth(t *testing.T) {
	t.Parallel()
	dir := epicFileRepo(t)
	writeEpicFile(t, dir, "b1", "b", "open")
	writeEpicFile(t, dir, "b2", "b", "open")
	initGitRepo(t, dir)

	_, stderr, code := runTP(t, dir, "next", "--peek", "--epic", "b")
	require.Equal(t, 4, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "b1.tasks.json")
	assert.Contains(t, stderr, "b2.tasks.json")
}

// TestEpicFlag_NoBoundFileExits4 is F8: u binds no file, so --epic u exits 4
// on each of the three commands.
func TestEpicFlag_NoBoundFileExits4(t *testing.T) {
	t.Parallel()
	dir := epicFileRepo(t)
	initGitRepo(t, dir)

	for _, args := range [][]string{{"resume"}, {"next", "--peek"}, {"ready"}} {
		_, stderr, code := runTP(t, dir, append(args, "--epic", "u")...)
		require.Equal(t, 4, code, "%v stderr: %s", args, stderr)
		assert.Contains(t, stderr, `epic \"u\" has no bound task file`, "%v", args)
	}
}

// TestEpicFlag_WithFileIsAUsageError is F8: --epic picks the file --file
// names, so passing both exits 2.
func TestEpicFlag_WithFileIsAUsageError(t *testing.T) {
	t.Parallel()
	dir := epicFileRepo(t)
	writeEpicFile(t, dir, "b1", "b", "open")
	initGitRepo(t, dir)

	_, stderr, code := runTP(t, dir, "ready", "--epic", "b", "--file", "x.tasks.json")
	require.Equal(t, 2, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "--epic and --file are mutually exclusive")
}

// symlinkedEpicRepo is epicFileRepo with b1.tasks.json bound to b, reached
// through a symlink to the repository, the way macOS reaches /var and /tmp.
func symlinkedEpicRepo(t *testing.T) string {
	t.Helper()
	dir := epicFileRepo(t)
	writeEpicFile(t, dir, "b1", "b", "open")
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o755))
	initGitRepo(t, dir)
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(dir, link))
	return link
}

// epicFileLabel runs tp next --peek --epic b from cwd, its PWD the symlinked
// path a shell would export, and returns the payload's "file".
func epicFileLabel(t *testing.T, cwd string) string {
	t.Helper()
	stdout, stderr, code := runTPEnv(t, cwd, []string{"PWD=" + cwd}, "next", "--peek", "--epic", "b")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var res struct {
		File string `json:"file"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &res), "stdout: %s", stdout)
	return res.File
}

// TestEpicFlag_FileLabelRoundTripsFromASymlinkedCwd: the "file" tp prints for
// --epic, run from a symlinked cwd, is the path --file accepts from there.
func TestEpicFlag_FileLabelRoundTripsFromASymlinkedCwd(t *testing.T) {
	t.Parallel()
	link := symlinkedEpicRepo(t)

	file := epicFileLabel(t, link)
	assert.Equal(t, "b1.tasks.json", file)
	_, stderr, code := runTPEnv(t, link, []string{"PWD=" + link}, "claim", "t1", "--file", file)
	assert.Equal(t, 0, code, "stderr: %s", stderr)
}

// TestEpicFlag_FileLabelIsRelativeToASubdirectory: from a subdirectory of a
// symlinked cwd the label climbs to the file, and --file accepts it there.
func TestEpicFlag_FileLabelIsRelativeToASubdirectory(t *testing.T) {
	t.Parallel()
	sub := filepath.Join(symlinkedEpicRepo(t), "sub")

	file := epicFileLabel(t, sub)
	assert.Equal(t, filepath.Join("..", "b1.tasks.json"), file)
	_, stderr, code := runTPEnv(t, sub, []string{"PWD=" + sub}, "claim", "t1", "--file", file)
	assert.Equal(t, 0, code, "stderr: %s", stderr)
}
