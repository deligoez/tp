package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pointerPhrase is the part of the write notice that names the reason the file
// was chosen. Every assertion on the notice keys on it, so a warning that
// merely mentions .tp/local.json for another reason (a dangling or absolute
// pointer) cannot satisfy one.
const pointerPhrase = "active pointer"

// requirePayloadFile decodes a write command's JSON payload and requires its
// "file" key to name want.
func requirePayloadFile(t *testing.T, label, stdout, want string) {
	t.Helper()
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload), "%s: payload is not one JSON object: %s", label, stdout)
	assert.Equal(t, want, payload["file"], "%s: the payload must name the task file it wrote: %s", label, stdout)
}

// pointerNoticeLines returns the stderr lines that attribute a write to the
// active pointer.
func pointerNoticeLines(stderr string) []string {
	lines := make([]string, 0)
	for line := range strings.SplitSeq(stderr, "\n") {
		if strings.Contains(line, pointerPhrase) {
			lines = append(lines, line)
		}
	}
	return lines
}

func writeTargetTask(id string) string {
	return `{"id":"` + id + `","title":"Task ` + id + `","estimate_minutes":5,"acceptance":"Tests pass","source_sections":["## 1. One"],"depends_on":[]}`
}

func writeTargetSpec(t *testing.T, dir, name string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("# "+name+"\n\n## 1. One\nText.\n"), 0o600))
}

// TestActivePointerWriteNamesItsFile is the BUGS P1 reproduction, row 1 of
// spec/backlog/a-task-file-write-names-its-target.md §8: a pointer left behind
// by `tp use a.tasks.json` survived `tp init b.md`, and the next `tp remove a1`
// rewrote a.tasks.json with exit 0, a payload of {"removed":"a1"} and nothing
// on stderr. The write must now name its file in the payload and, because the
// pointer chose it while another task file is in reach, say so on stderr;
// init and import warn where the pointer stops matching the file they made.
func TestActivePointerWriteNamesItsFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTargetSpec(t, dir, "a.md")
	writeTargetSpec(t, dir, "b.md")
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o755))
	initGitRepo(t, dir)

	stdout, stderr, code := runTP(t, dir, "init", "a.md")
	require.Equal(t, 0, code, stderr)
	requirePayloadFile(t, "init a.md", stdout, "a.tasks.json")

	stdout, stderr, code = runTP(t, dir, "add", writeTargetTask("a1"))
	require.Equal(t, 0, code, stderr)
	requirePayloadFile(t, "add a1", stdout, "a.tasks.json")
	_, stderr, code = runTP(t, dir, "add", writeTargetTask("a2"))
	require.Equal(t, 0, code, stderr)

	_, stderr, code = runTP(t, dir, "use", "a.tasks.json")
	require.Equal(t, 0, code, stderr)

	// The pointer chose the file, but it is the only task file: no notice.
	stdout, stderr, code = runTP(t, dir, "set", "a1", "title=A one renamed")
	require.Equal(t, 0, code, stderr)
	requirePayloadFile(t, "set a1 (single task file)", stdout, "a.tasks.json")
	assert.Empty(t, pointerNoticeLines(stderr), "a pointer write with no other task file in reach needs no notice: %s", stderr)

	// init of another spec while the pointer names a.tasks.json warns.
	stdout, stderr, code = runTP(t, dir, "init", "b.md")
	require.Equal(t, 0, code, stderr)
	requirePayloadFile(t, "init b.md", stdout, "b.tasks.json")
	initWarn := pointerNoticeLines(stderr)
	require.Len(t, initWarn, 1, "init of another spec must warn once that the pointer names a different file: %q", stderr)
	assert.Contains(t, initWarn[0], "a.tasks.json")
	assert.Contains(t, initWarn[0], "b.tasks.json")
	assert.Contains(t, initWarn[0], "tp use")

	// The reproduction: the write lands in a.tasks.json and now says so twice.
	stdout, stderr, code = runTP(t, dir, "remove", "a1")
	require.Equal(t, 0, code, stderr)
	requirePayloadFile(t, "remove a1", stdout, "a.tasks.json")
	notice := pointerNoticeLines(stderr)
	require.Len(t, notice, 1, "a pointer-resolved write beside another task file must print exactly one notice: %q", stderr)
	assert.Contains(t, notice[0], "a.tasks.json", "the notice must name the file written")
	assert.Contains(t, notice[0], ".tp/local.json", "the notice must name the pointer as the reason")
	assert.Contains(t, notice[0], "b.tasks.json", "the notice must name the other candidate")

	// Row 3: from a subdirectory, "file" is relative to where the command ran
	// and works as --file's argument from there. The pointer's stored value
	// ("a.tasks.json", project-root-relative) would miss from sub/.
	fromSub := filepath.Join(dir, "sub")
	stdout, stderr, code = runTP(t, fromSub, "set", "a2", "title=A two renamed")
	require.Equal(t, 0, code, stderr)
	subFile := filepath.Join("..", "a.tasks.json")
	requirePayloadFile(t, "set a2 from sub/", stdout, subFile)
	assert.Len(t, pointerNoticeLines(stderr), 1, "the pointer still chose the file from sub/: %q", stderr)
	stdout, stderr, code = runTP(t, fromSub, "--file", subFile, "set", "a2", "title=A two again")
	require.Equal(t, 0, code, "the payload's file must work as --file from the same directory: %s", stderr)
	requirePayloadFile(t, "--file ../a.tasks.json set a2 from sub/", stdout, subFile)

	// An explicit --file is not the pointer's choice: no notice.
	stdout, stderr, code = runTP(t, dir, "--file", "b.tasks.json", "add", writeTargetTask("b1"))
	require.Equal(t, 0, code, stderr)
	requirePayloadFile(t, "--file b.tasks.json add b1", stdout, "b.tasks.json")
	assert.Empty(t, pointerNoticeLines(stderr), "--file outranks the pointer, so the pointer is not the reason: %s", stderr)

	// import of another spec while the pointer names a.tasks.json warns too.
	writeTargetSpec(t, dir, "c.md")
	importDoc := `{"version":1,"spec":"c.md","tasks":[{"id":"c1","title":"C one","status":"open","estimate_minutes":5,"acceptance":"Tests pass","source_sections":["## 1. One"],"depends_on":[]}]}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c-import.json"), []byte(importDoc), 0o600))
	stdout, stderr, code = runTP(t, dir, "import", "c-import.json")
	require.Equal(t, 0, code, stderr)
	requirePayloadFile(t, "import c-import.json", stdout, "c.tasks.json")
	importWarn := pointerNoticeLines(stderr)
	require.Len(t, importWarn, 1, "import of another spec must warn once that the pointer names a different file: %q", stderr)
	assert.Contains(t, importWarn[0], "a.tasks.json")
	assert.Contains(t, importWarn[0], "c.tasks.json")

	// Neither warning moved the pointer.
	stdout, _, code = runTP(t, dir, "use")
	require.Equal(t, 0, code)
	assert.Contains(t, stdout, "a.tasks.json", "init and import warn; they do not move the pointer")
}

// TestEveryTaskFileWriteNamesItsFile is row 2 of the same spec: every command
// that writes a task file, run with --file spec.tasks.json while the pointer
// names alpha.tasks.json, names spec.tasks.json in its payload, prints no
// pointer notice (--file outranks the pointer), and leaves alpha untouched. A
// "file" filled from the pointer rather than from the path written fails it.
func TestEveryTaskFileWriteNamesItsFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTargetSpec(t, dir, "spec.md")
	writeTargetSpec(t, dir, "alpha.md")
	for _, spec := range []string{"alpha.md", "spec.md"} {
		_, stderr, code := runTP(t, dir, "init", spec)
		require.Equal(t, 0, code, stderr)
		assert.Empty(t, pointerNoticeLines(stderr), "init %s with no pointer set warns about nothing: %s", spec, stderr)
	}
	_, stderr, code := runTP(t, dir, "use", "alpha.tasks.json")
	require.Equal(t, 0, code, stderr)

	// Row 6's other quiet case: import into the very file the pointer names.
	alphaDoc := `{"version":1,"spec":"alpha.md","tasks":[{"id":"a1","title":"Alpha one","status":"open","estimate_minutes":5,"acceptance":"Tests pass","source_sections":["## 1. One"],"depends_on":[]}]}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "alpha-import.json"), []byte(alphaDoc), 0o600))
	stdout, stderr, code := runTP(t, dir, "import", "alpha-import.json")
	require.Equal(t, 0, code, stderr)
	requirePayloadFile(t, "import alpha-import.json", stdout, "alpha.tasks.json")
	assert.Empty(t, pointerNoticeLines(stderr), "import into the file the pointer names warns about nothing: %s", stderr)

	initGitRepo(t, dir)
	alphaBefore, err := os.ReadFile(filepath.Join(dir, "alpha.tasks.json"))
	require.NoError(t, err)

	const reason = "All tests pass and verification is complete"
	bulk := filepath.Join(dir, "bulk.ndjson")
	require.NoError(t, os.WriteFile(bulk, []byte(`{"id":"t2","field":"title","value":"Renamed"}`+"\n"), 0o600))
	batch := filepath.Join(dir, "batch.ndjson")
	require.NoError(t, os.WriteFile(batch, []byte(`{"id":"t4","reason":"`+reason+`"}`+"\n"), 0o600))

	steps := []struct {
		label string
		args  []string
		setup func()
	}{
		{label: "add", args: []string{"add", writeTargetTask("t1")}},
		{label: "add t2", args: []string{"add", writeTargetTask("t2")}},
		{label: "add t3", args: []string{"add", writeTargetTask("t3")}},
		{label: "add t4", args: []string{"add", writeTargetTask("t4")}},
		{label: "add t5", args: []string{"add", writeTargetTask("t5")}},
		{label: "add t6", args: []string{"add", writeTargetTask("t6")}},
		{label: "set", args: []string{"set", "t1", "title=Renamed"}},
		{label: "set --bulk", args: []string{"set", "--bulk", bulk}},
		{label: "set --workflow", args: []string{"set", "--workflow", "review_clean_rounds=2"}},
		{label: "claim (single)", args: []string{"claim", "t1"}},
		{label: "claim (batch)", args: []string{"claim", "t2", "t3"}},
		{label: "close", args: []string{"close", "t1", reason}},
		{label: "reopen", args: []string{"reopen", "t1"}},
		{label: "done (single)", args: []string{"done", "t1", reason}},
		{label: "done (multi)", args: []string{"done", "t2", "t3", reason}},
		{label: "done --batch", args: []string{"done", "--batch", batch}},
		{label: "remove", args: []string{"remove", "t5"}},
		{label: "next", args: []string{"next"}},
		{label: "commit", args: []string{"commit", "t6", reason}, setup: func() {
			require.NoError(t, os.WriteFile(filepath.Join(dir, "impl.go"), []byte("package impl\n"), 0o600))
		}},
	}
	for _, s := range steps {
		if s.setup != nil {
			s.setup()
		}
		stdout, stderr, code := runTP(t, dir, append([]string{"--file", "spec.tasks.json"}, s.args...)...)
		require.Equal(t, 0, code, "%s: %s", s.label, stderr)
		requirePayloadFile(t, s.label, stdout, "spec.tasks.json")
		assert.Empty(t, pointerNoticeLines(stderr), "%s: --file is not the pointer: %s", s.label, stderr)
	}

	alphaAfter, err := os.ReadFile(filepath.Join(dir, "alpha.tasks.json"))
	require.NoError(t, err)
	assert.Equal(t, string(alphaBefore), string(alphaAfter), "every write went to --file's target, none to the pointer's")
}
