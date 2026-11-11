package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// exportListRow is one row of tp export --list as these tests read it.
type exportListRow struct {
	Path      string         `json:"path"`
	Title     string         `json:"title"`
	Epic      *string        `json:"epic"`
	EpicState *string        `json:"epic_state"`
	Phase     string         `json:"phase"`
	Tasks     map[string]int `json:"tasks"`
	Review    *struct {
		Round int    `json:"round"`
		State string `json:"state"`
	} `json:"review"`
	Audit *struct {
		Round int    `json:"round"`
		State string `json:"state"`
	} `json:"audit"`
	NextStep    string            `json:"next_step"`
	Escalations []json.RawMessage `json:"escalations"`
	Error       string            `json:"error"`
}

// exportList runs tp export --list in dir, requires exit 0, and decodes it.
func exportList(t *testing.T, dir string, unattended bool) (version int, rows []exportListRow) {
	t.Helper()
	stdout, stderr, code := runTPFence(t, dir, unattended, "export", "--list")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out struct {
		Version int             `json:"version"`
		Files   []exportListRow `json:"files"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	require.NotNil(t, out.Files, "files is a list, never null: %s", stdout)
	return out.Version, out.Files
}

// resumePhase is the phase tp resume reports for the task file at path.
func resumePhase(t *testing.T, dir, path string) string {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "--file", path, "resume")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out struct {
		Phase string `json:"phase"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	return out.Phase
}

// TestExportList_OneRowPerTaskFileUnderItsResumePhaseInPhaseOrder is §16's
// --list and §12's acceptance on a synthetic repository: a task file in
// implement and one in review each print one row, under the phase tp resume
// reports for it, the review row first, with title, tasks and loops.
func TestExportList_OneRowPerTaskFileUnderItsResumePhaseInPhaseOrder(t *testing.T) {
	t.Parallel()
	dir := exportRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "other.md"), []byte("# Other spec\n\n## 1. A\n\na.\n"), 0o600))
	_, stderr, code := runTP(t, dir, "init", "other.md")
	require.Equal(t, 0, code, stderr)

	version, rows := exportList(t, dir, false)
	assert.Equal(t, 1, version)
	require.Len(t, rows, 2)
	assert.Equal(t, []string{"other.tasks.json", "spec.tasks.json"}, []string{rows[0].Path, rows[1].Path})
	for _, r := range rows {
		assert.Equal(t, resumePhase(t, dir, r.Path), r.Phase, r.Path)
		assert.Nil(t, r.Epic, r.Path)
		assert.Nil(t, r.EpicState, r.Path)
		assert.NotEmpty(t, r.NextStep, r.Path)
		assert.Empty(t, r.Escalations, r.Path)
		assert.Empty(t, r.Error, r.Path)
	}
	assert.Equal(t, "review", rows[0].Phase)
	assert.Equal(t, "Other spec", rows[0].Title)
	assert.Equal(t, "implement", rows[1].Phase)
	assert.Equal(t, "S", rows[1].Title)
	assert.Equal(t, map[string]int{"open": 1, "wip": 0, "done": 0}, rows[1].Tasks)
	require.NotNil(t, rows[1].Review)
	assert.Equal(t, 2, rows[1].Review.Round)
	require.NotNil(t, rows[1].Audit)
	assert.Equal(t, 0, rows[1].Audit.Round)
}

// TestExportList_KeepsTheReadableRowAndMarksTheMalformedOne is §18 row 10:
// two task files, one malformed — the list exits 0, keeps the readable row
// whole and gives the malformed one a row carrying its error.
func TestExportList_KeepsTheReadableRowAndMarksTheMalformedOne(t *testing.T) {
	t.Parallel()
	dir := exportRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.tasks.json"), []byte("{not json"), 0o600))

	_, rows := exportList(t, dir, false)
	require.Len(t, rows, 2)
	byPath := map[string]exportListRow{}
	for _, r := range rows {
		byPath[r.Path] = r
	}
	good, bad := byPath["spec.tasks.json"], byPath["bad.tasks.json"]
	assert.Equal(t, "implement", good.Phase)
	assert.Empty(t, good.Error)
	assert.NotEmpty(t, bad.Error)
	assert.Contains(t, bad.Error, "bad.tasks.json")
	assert.Empty(t, bad.Phase)
}

// TestExportList_EmptyWithoutATaskFileAndRunsUnattended is §16's --list
// contract: no task file exits 0 with an empty list, and it runs under
// TP_UNATTENDED.
func TestExportList_EmptyWithoutATaskFileAndRunsUnattended(t *testing.T) {
	t.Parallel()
	empty := t.TempDir()
	initGitRepo(t, empty)
	_, rows := exportList(t, empty, false)
	assert.Empty(t, rows)
	_, rows = exportList(t, t.TempDir(), true)
	assert.Empty(t, rows)

	_, rows = exportList(t, exportRepo(t), true)
	assert.Len(t, rows, 1)
}
