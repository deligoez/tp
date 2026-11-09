package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicExportPayload is the part of tp epic export's JSON these tests read.
type epicExportPayload struct {
	Version int `json:"version"`
	Epics   []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Files  []struct {
			Path  string         `json:"path"`
			Phase string         `json:"phase"`
			Tasks map[string]int `json:"tasks"`
		} `json:"files"`
	} `json:"epics"`
	Slices []struct {
		Key   json.RawMessage `json:"key"`
		Round int             `json:"round"`
		State string          `json:"state"`
	} `json:"slices"`
}

// exportWithBRecorded returns the chain with b bound and one round of the
// --epic b slice recorded.
func exportWithBRecorded(t *testing.T) string {
	t.Helper()
	dir := boundEpicDir(t, epicChain)
	runEpicReview(t, dir, "--epic", "b")
	path := epicRecordFile(t, dir, `{"severity":"low","finding":"f1","location":"epic:b","evidence":"e1"}`+"\n")
	_, stderr, code := runTP(t, dir, "epic", "review", "--epic", "b", "--record", path)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	return dir
}

// runEpicExportJSON runs tp epic export with no format flag and decodes it.
func runEpicExportJSON(t *testing.T, dir string) *epicExportPayload {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "epic", "export")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var p epicExportPayload
	require.NoError(t, json.Unmarshal([]byte(stdout), &p), "stdout: %s", stdout)
	return &p
}

// TestEpicExport_JSONCarriesVersionBoundFileAndSliceKey is §10 row 13: with
// no format flag the export is JSON with a version, b's bound file with its
// phase, and the recorded --epic b slice with its key. A slice listed without
// its key turns it red.
func TestEpicExport_JSONCarriesVersionBoundFileAndSliceKey(t *testing.T) {
	t.Parallel()
	p := runEpicExportJSON(t, exportWithBRecorded(t))

	assert.Equal(t, 1, p.Version)
	require.Len(t, p.Epics, 4)
	b := p.Epics[1]
	assert.Equal(t, "b", b.ID)
	assert.Equal(t, "specced", b.Status)
	require.Len(t, b.Files, 1)
	assert.Equal(t, "b.tasks.json", b.Files[0].Path, "the path is repo-relative")
	assert.Equal(t, "review", b.Files[0].Phase, "the phase tp resume reports")
	assert.Equal(t, map[string]int{"open": 0, "wip": 0, "done": 0}, b.Files[0].Tasks)

	require.Len(t, p.Slices, 1)
	assert.JSONEq(t, `{"kind": "epic", "value": "b"}`, string(p.Slices[0].Key))
	assert.Equal(t, 1, p.Slices[0].Round)
	assert.Equal(t, "open", p.Slices[0].State, "one clean round does not converge the slice")
}
