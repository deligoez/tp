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

// readEpicGraphFile returns dir's .tp/epics.json bytes.
func readEpicGraphFile(t *testing.T, dir string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".tp", "epics.json"))
	require.NoError(t, err)
	return data
}

// TestEpicAdd_EdgeWithoutWhyIsRefusedAndWritesNothing is §9 row 1: a writer
// that validated after writing would leave b in the file.
func TestEpicAdd_EdgeWithoutWhyIsRefusedAndWritesNothing(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{
  "version": 1,
  "milestones": [["A"]],
  "epics": [
    {"id": "a", "title": "A", "milestone": "A"}
  ]
}
`)
	before := readEpicGraphFile(t, dir)

	_, stderr, code := runTP(t, dir, "epic", "add",
		`{"id": "b", "title": "B", "milestone": "A", "depends_on": [{"id": "a"}]}`)
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "missing-why")
	assert.Equal(t, before, readEpicGraphFile(t, dir), "a refused add writes nothing")
}

// TestEpicAdd_PreExistingErrorDoesNotBlockButANewOneDoes is §9 row 2: a
// writer refusing any after-state error would refuse the first add, and one
// comparing by rule alone would accept the second.
func TestEpicAdd_PreExistingErrorDoesNotBlockButANewOneDoes(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{
  "version": 1,
  "milestones": [["A"]],
  "epics": [
    {"id": "a", "title": "A", "milestone": "A", "depends_on": [{"id": "z", "why": "a reads z"}]},
    {"id": "b", "title": "B", "milestone": "A"}
  ]
}
`)
	stdout, stderr, code := runTP(t, dir, "epic", "add",
		`{"id": "c", "title": "C", "milestone": "A", "depends_on": [{"id": "b", "why": "c reads b"}]}`)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out struct {
		File string `json:"file"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	assert.Equal(t, "epics.json", filepath.Base(out.File))
	afterFirst := readEpicGraphFile(t, dir)
	assert.Contains(t, string(afterFirst), `"c reads b"`)

	_, stderr, code = runTP(t, dir, "epic", "add",
		`{"id": "e", "title": "E", "milestone": "A", "depends_on": [{"id": "y", "why": "e reads y"}]}`)
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "dangling-reference")
	assert.Contains(t, stderr, "e, y")
	assert.Equal(t, afterFirst, readEpicGraphFile(t, dir), "a refused add writes nothing")
}

// TestEpicAdd_BulkRowWithAnExistingIDReplacesIt is the second half of §9 row
// 20: refusing the second bulk row on duplicate-name would keep the first title.
func TestEpicAdd_BulkRowWithAnExistingIDReplacesIt(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{
  "version": 1,
  "milestones": [["A"]],
  "epics": []
}
`)
	first := filepath.Join(dir, "first.ndjson")
	second := filepath.Join(dir, "second.ndjson")
	require.NoError(t, os.WriteFile(first, []byte(`{"id": "x", "title": "First", "milestone": "A"}`+"\n"), 0o600))
	require.NoError(t, os.WriteFile(second, []byte(`{"id": "x", "title": "Second", "milestone": "A"}`+"\n"), 0o600))

	for _, bulk := range []string{first, second} {
		_, stderr, code := runTP(t, dir, "epic", "add", "--bulk", bulk)
		require.Equal(t, 0, code, "bulk %s stderr: %s", filepath.Base(bulk), stderr)
	}

	var g struct {
		Epics []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"epics"`
	}
	require.NoError(t, json.Unmarshal(readEpicGraphFile(t, dir), &g))
	require.Len(t, g.Epics, 1)
	assert.Equal(t, "x", g.Epics[0].ID)
	assert.Equal(t, "Second", g.Epics[0].Title)
}

// TestEpicAdd_NamesTheKeysItDropped: a writer carries only the schema into the
// file it writes, so the keys outside it it leaves behind are named in its
// payload; one that dropped them silently would print no dropped_keys.
func TestEpicAdd_NamesTheKeysItDropped(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{
  "version": 1,
  "milestones": [["A"]],
  "owner": "x",
  "gates": {"A": {"members": ["a"], "extra": 1}},
  "epics": [
    {"id": "a", "title": "A", "milestone": "A", "trigger": "y"}
  ]
}
`)
	type payload struct {
		DroppedKeys []string `json:"dropped_keys"`
	}
	stdout, stderr, code := runTP(t, dir, "epic", "add", `{"id": "b", "title": "B", "milestone": "A"}`)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out payload
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	assert.Equal(t, []string{"owner", "gates.A.extra", "epics[0].trigger"}, out.DroppedKeys)
	assert.NotContains(t, string(readEpicGraphFile(t, dir)), "owner")

	stdout, stderr, code = runTP(t, dir, "epic", "add", `{"id": "c", "title": "C", "milestone": "A"}`)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Contains(t, stdout, `"dropped_keys": []`, "nothing dropped is an empty list, not an absent key")
}

// TestEpicAdd_TrailingDataAfterTheEpicIsAUsageError: a decoder reading only
// the first JSON value would add a and silently drop b, through the argument,
// --stdin and a --bulk line alike.
func TestEpicAdd_TrailingDataAfterTheEpicIsAUsageError(t *testing.T) {
	t.Parallel()
	two := `{"id": "a", "title": "A", "milestone": "A"} {"id": "b", "title": "B", "milestone": "A"}`
	for _, via := range []string{"argument", "stdin", "bulk"} {
		t.Run(via, func(t *testing.T) {
			t.Parallel()
			dir := epicGraphDir(t, `{
  "version": 1,
  "milestones": [["A"]],
  "epics": []
}
`)
			before := readEpicGraphFile(t, dir)
			var stderr string
			var code int
			switch via {
			case "argument":
				_, stderr, code = runTP(t, dir, "epic", "add", two)
			case "stdin":
				cmd := exec.Command(binaryPath, "--json", "epic", "add", "--stdin")
				cmd.Dir = dir
				cmd.Env = append(byHandEnv(), "NO_COLOR=1", "TP_HC=0")
				cmd.Stdin = strings.NewReader(two + "\n")
				var errBuf bytes.Buffer
				cmd.Stderr = &errBuf
				err := cmd.Run()
				var exitErr *exec.ExitError
				require.ErrorAs(t, err, &exitErr)
				stderr, code = errBuf.String(), exitErr.ExitCode()
			case "bulk":
				bulk := filepath.Join(dir, "rows.ndjson")
				require.NoError(t, os.WriteFile(bulk, []byte(two+"\n"), 0o600))
				_, stderr, code = runTP(t, dir, "epic", "add", "--bulk", bulk)
			}
			require.Equal(t, 2, code, "stderr: %s", stderr)
			assert.Contains(t, stderr, "invalid epic JSON")
			assert.Equal(t, before, readEpicGraphFile(t, dir), "a refused add writes nothing")
		})
	}
}

// TestEpicAdd_BulkListsARepeatedIDOnce: two rows sharing an id write one
// epic, so a payload echoing every row would list z twice.
func TestEpicAdd_BulkListsARepeatedIDOnce(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{
  "version": 1,
  "milestones": [["A"]],
  "epics": []
}
`)
	bulk := filepath.Join(dir, "rows.ndjson")
	rows := `{"id": "z", "title": "First", "milestone": "A"}` + "\n" +
		`{"id": "y", "title": "Y", "milestone": "A"}` + "\n" +
		`{"id": "z", "title": "Second", "milestone": "A"}` + "\n"
	require.NoError(t, os.WriteFile(bulk, []byte(rows), 0o600))

	stdout, stderr, code := runTP(t, dir, "epic", "add", "--bulk", bulk)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out struct {
		Epics []string `json:"epics"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	assert.Equal(t, []string{"z", "y"}, out.Epics)
}

// TestEpicAdd_BulkMissingFileHintNamesEpicRows: --bulk takes epic rows, so a
// missing file's hint must say so instead of pointing at reviewer/auditor
// NDJSON, the input of the review/audit modes.
func TestEpicAdd_BulkMissingFileHintNamesEpicRows(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{
  "version": 1,
  "milestones": [["A"]],
  "epics": []
}
`)
	_, stderr, code := runTP(t, dir, "epic", "add", "--bulk", filepath.Join(dir, "missing.ndjson"))
	require.Equal(t, 3, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "one epic JSON object per line")
	assert.NotContains(t, stderr, "reviewers/auditors")
}

// TestEpicAdd_TwoInputSourcesAreAUsageError: a reader that picked one source
// would write it and silently drop the other epic, so any two of the
// argument, --stdin and --bulk are refused before anything is written.
func TestEpicAdd_TwoInputSourcesAreAUsageError(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		args func(bulk string) []string
	}{
		{"argument and --bulk", func(bulk string) []string {
			return []string{`{"id": "a", "title": "A", "milestone": "A"}`, "--bulk", bulk}
		}},
		{"--stdin and --bulk", func(bulk string) []string { return []string{"--stdin", "--bulk", bulk} }},
		{"argument and --stdin", func(string) []string {
			return []string{`{"id": "a", "title": "A", "milestone": "A"}`, "--stdin"}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := epicGraphDir(t, `{
  "version": 1,
  "milestones": [["A"]],
  "epics": []
}
`)
			bulk := filepath.Join(dir, "rows.ndjson")
			require.NoError(t, os.WriteFile(bulk, []byte(`{"id": "b", "title": "B", "milestone": "A"}`+"\n"), 0o600))
			before := readEpicGraphFile(t, dir)
			cmd := exec.Command(binaryPath, append([]string{"--json", "epic", "add"}, tc.args(bulk)...)...)
			cmd.Dir = dir
			cmd.Env = append(byHandEnv(), "NO_COLOR=1", "TP_HC=0")
			cmd.Stdin = strings.NewReader(`{"id": "c", "title": "C", "milestone": "A"}` + "\n")
			var errBuf bytes.Buffer
			cmd.Stderr = &errBuf
			code := 0
			if err := cmd.Run(); err != nil {
				var exitErr *exec.ExitError
				require.ErrorAs(t, err, &exitErr)
				code = exitErr.ExitCode()
			}
			require.Equal(t, 2, code, "stderr: %s", errBuf.String())
			assert.Contains(t, errBuf.String(), "cannot be combined with")
			assert.Equal(t, before, readEpicGraphFile(t, dir), "a refused add writes nothing")
		})
	}
}
