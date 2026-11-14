package engine

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTaskExport_NeverPrintsNullForAnEmptyList is §6.2 of the export
// contract: a task without tags, depends_on or source_sections, and a section
// without a paragraph, print [] rather than null.
func TestTaskExport_NeverPrintsNullForAnEmptyList(t *testing.T) {
	tasks := `{"spec":"s.md","tasks":[{"id":"t1","title":"T1","status":"open","estimate_minutes":5,"acceptance":"a1"}]}`
	dir, specPath, tfp := setupResumeProject(t, tasks)
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n\n## 1. Empty\n\n## 2. Full\n\nPara.\n"), 0o600))

	exp, err := TaskExportOf(dir, tfp)
	require.NoError(t, err)
	data, err := json.Marshal(exp)
	require.NoError(t, err)
	var out struct {
		Tasks    []map[string]json.RawMessage `json:"tasks"`
		Sections []map[string]json.RawMessage `json:"sections"`
	}
	require.NoError(t, json.Unmarshal(data, &out))
	require.Len(t, out.Tasks, 1)
	for _, key := range []string{"tags", "depends_on", "source_sections"} {
		assert.JSONEq(t, `[]`, string(out.Tasks[0][key]), key)
	}
	require.NotEmpty(t, out.Sections)
	for _, s := range out.Sections {
		assert.NotEqual(t, "null", string(s["paragraphs"]), string(s["anchor"]))
	}
}

// TestTaskExport_UnparsableGroundRoundNamesItsFile is the hint's promise:
// "repair the file the message names" needs the message to name the file.
func TestTaskExport_UnparsableGroundRoundNamesItsFile(t *testing.T) {
	for name, tc := range map[string]struct {
		file string
		path func(string) string
	}{
		"rows":  {"ground-round-1.ndjson", func(s string) string { return GroundRoundPath(s, 1) }},
		"floor": {"floor-ground-round-1.txt", func(s string) string { return GroundFloorPath(s, 1) }},
	} {
		t.Run(name, func(t *testing.T) {
			dir, specPath, tfp := setupResumeProject(t, exportTasksJSON)
			require.NoError(t, os.WriteFile(specPath, []byte(exportSpec), 0o600))
			exportLoopsFixture(t, specPath)
			writeStateFile(t, specPath, tc.file, "not json\n")

			_, err := TaskExportOf(dir, tfp)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.path(specPath))
		})
	}
}
