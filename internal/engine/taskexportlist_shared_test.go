package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// perRowTaskFileRow is a list row derived as TaskFileListOf derived it
// before the list shared its reads: the row's state from epicFileState and
// its resume from AssembleResumeLive, each reading the tree, the loops and
// the unit branches for itself.
func perRowTaskFileRow(t *testing.T, start, shown, path string) (TaskFileRow, *model.TaskFile) {
	t.Helper()
	row := TaskFileRow{Path: shown}
	tf, err := model.ReadTaskFile(path)
	require.NoError(t, err)
	specPath, _ := ResolveSpecPath(path, tf.Spec)
	st, err := epicFileState(path)
	require.NoError(t, err)
	resume, err := AssembleResumeLive(start, path, specPath, tf)
	require.NoError(t, err)
	row.readable = true
	row.Title = specTitle(specPath)
	if tf.Epic != "" {
		row.Epic = &tf.Epic
	}
	row.Phase, row.Tasks, row.Review, row.Audit = resume.Phase, st.Tasks, &st.Review, &st.Audit
	row.NextStep = resume.NextAction.Summary
	row.ContentHash = contentHash(path, specPath)
	row.UpdatedAt = taskFileUpdatedAt(tf, specPath)
	row.Escalations = []Blocker{}
	for _, b := range resume.Blockers {
		if b.Class == ClassEscalate {
			row.Escalations = append(row.Escalations, b)
		}
	}
	return row, tf
}

// TestTaskFileList_SharedReadsPrintTheBytesPerRowReadsPrint: tp export
// --list runs git status once and reads each file's loops once for all its
// rows; its JSON is byte for byte the list derived row by row, each row
// reading everything itself, over a tree with an unexplained change and
// task files in two phases.
func TestTaskFileList_SharedReadsPrintTheBytesPerRowReadsPrint(t *testing.T) {
	root, _, _, _ := batchProject(t, rollingTasks(1, "alpha", "beta"))
	tasks := rollingTasks(1, "alpha", "beta")
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.tasks.json"), []byte(tasks), 0o600))
	done := strings.ReplaceAll(tasks, `"status":"open"`, `"status":"done"`)
	require.NoError(t, os.WriteFile(filepath.Join(root, "b.tasks.json"), []byte(done), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "stray.txt"), []byte("unexplained\n"), 0o600))

	got := TaskFileListOf(root)
	require.Len(t, got.Files, 3, "the project's s.tasks.json, a.tasks.json and b.tasks.json")

	want := TaskFileList{Version: TaskExportVersion, Files: []TaskFileRow{}}
	bound := map[string][]string{}
	for _, path := range repoTaskFiles(gitToplevel(root)) {
		row, tf := perRowTaskFileRow(t, root, repoRelativeTo(gitToplevel(root), path), path)
		if tf.Epic != "" {
			bound[tf.Epic] = append(bound[tf.Epic], path)
		}
		want.Files = append(want.Files, row)
	}
	fillEpicStates(root, want.Files, bound)
	sort.SliceStable(want.Files, func(i, j int) bool { return taskFileRowLess(&want.Files[i], &want.Files[j]) })

	gotJSON, err := json.MarshalIndent(got, "", "  ")
	require.NoError(t, err)
	wantJSON, err := json.MarshalIndent(want, "", "  ")
	require.NoError(t, err)
	assert.Equal(t, string(wantJSON), string(gotJSON))
	assert.NotEqual(t, got.Files[0].Phase, got.Files[2].Phase, "the fixture holds two phases")
}
