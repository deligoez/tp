package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicLayoutGraph has a main lane A -> B and two milestones, T1 and T2, whose
// opens_after entry is an explicit [].
const epicLayoutGraph = `{"version":1,"milestones":[["A","B"],["T1"],["T2"]],
"opens_after":{"T1":[],"T2":[]},
"epics":[
{"id":"a","title":"a","milestone":"A","root":true},
{"id":"b","title":"b","milestone":"B","depends_on":[{"id":"a","why":"w"}]},
{"id":"t1","title":"t1","milestone":"T1","root":true},
{"id":"t2","title":"t2","milestone":"T2","root":true}]}`

// exportedLayouts runs tp epic export in dir and returns each epic's layout
// raw, by id.
func exportedLayouts(t *testing.T, dir string) map[string]json.RawMessage {
	t.Helper()
	var epics []struct {
		ID     string          `json:"id"`
		Layout json.RawMessage `json:"layout"`
	}
	require.NoError(t, json.Unmarshal(epicExportRaw(t, dir)["epics"], &epics))
	out := map[string]json.RawMessage{}
	for _, e := range epics {
		out[e.ID] = e.Layout
	}
	return out
}

// TestEpicExport_CarriesEachEpicsLayout is spec/1.8.0.md §9: every epic
// carries its lane, the lane's kind, its column and its row; both [] milestones'
// epics are in the triggered lane, and two exports print identical layouts.
func TestEpicExport_CarriesEachEpicsLayout(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicLayoutGraph)
	first := exportedLayouts(t, dir)

	assert.JSONEq(t, `{"lane":0,"lane_kind":"main","column":0,"row":0}`, string(first["a"]))
	assert.JSONEq(t, `{"lane":0,"lane_kind":"main","column":1,"row":0}`, string(first["b"]))
	assert.JSONEq(t, `{"lane":1,"lane_kind":"triggered","column":0,"row":0}`, string(first["t1"]))
	assert.JSONEq(t, `{"lane":1,"lane_kind":"triggered","column":0,"row":0}`, string(first["t2"]))
	assert.Equal(t, first, exportedLayouts(t, dir))
}
