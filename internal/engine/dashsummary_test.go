package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// writeSliceRound writes round N's findings into a slice state directory
// named dir under tpDir.
func writeSliceRound(t *testing.T, tpDir, dir string, round int, ndjson string) {
	t.Helper()
	stateDir := filepath.Join(tpDir, EpicReviewStateRoot, dir)
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, epicRoundFile(round)), []byte(ndjson), 0o644))
}

// jsonKeys returns an object's keys, sorted.
func jsonKeys(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &m), "raw: %s", raw)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// TestDashboardSummary_ShapeIsPinned is spec/1.8.0.md §2's summary shape: the
// four top-level keys rounds, band, release_impact and series, each per-round
// entry's round, total and by_role, and the band's stages, phases and
// phases_of with every key of both axes, and a release impact entry's frees
// and gates with each gate move's keys. series is present and empty here; the
// task that fills it extends this test.
func TestDashboardSummary_ShapeIsPinned(t *testing.T) {
	t.Parallel()
	tpDir := t.TempDir()
	writeSliceRound(t, tpDir, "s1", 1, `{"role":"why-auditor","severity":"high"}`+"\n")
	sum, err := DashboardSummaryOf(tpDir, &model.EpicGraph{})
	require.NoError(t, err)
	data, err := json.Marshal(sum)
	require.NoError(t, err)

	assert.Equal(t, []string{"band", "release_impact", "rounds", "series"}, jsonKeys(t, data))
	var top struct {
		Rounds map[string][]json.RawMessage `json:"rounds"`
		Band   struct {
			Stages json.RawMessage `json:"stages"`
			Phases json.RawMessage `json:"phases"`
		} `json:"band"`
		ReleaseImpact json.RawMessage `json:"release_impact"`
		Series        json.RawMessage `json:"series"`
	}
	require.NoError(t, json.Unmarshal(data, &top))
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	require.Len(t, top.Rounds["s1"], 1)
	assert.Equal(t, []string{"by_role", "round", "total"}, jsonKeys(t, top.Rounds["s1"][0]))
	assert.Equal(t, []string{"phases", "phases_of", "stages"}, jsonKeys(t, raw["band"]))
	assert.Equal(t, []string{"claimed", "done", "in_progress", "planned", "specced"}, jsonKeys(t, top.Band.Stages))
	assert.Equal(t, []string{"audit", "decompose", "implement", "release", "review"}, jsonKeys(t, top.Band.Phases))
	assert.JSONEq(t, `{}`, string(top.ReleaseImpact), "outside a repository no task file has an impact")
	g, states := releaseImpactGraph()
	entry, err := json.Marshal(SpecReleaseImpact(g, states, "a"))
	require.NoError(t, err)
	assert.Equal(t, []string{"frees", "gates"}, jsonKeys(t, entry))
	var impact struct {
		Gates []json.RawMessage `json:"gates"`
	}
	require.NoError(t, json.Unmarshal(entry, &impact))
	require.Len(t, impact.Gates, 1)
	assert.Equal(t, []string{"members", "members_done", "members_done_after", "milestone"}, jsonKeys(t, impact.Gates[0]))
	assert.JSONEq(t, `[]`, string(top.Series))
}

// TestDashboardSummary_RoundCountsByRoleSumToEachTotal is §5's acceptance: a
// slice with two recorded rounds carries two per-round entries in round order,
// each counting every finding of its round, disposed ones too, by role, and
// the by-role counts sum to the round's total; a finding naming no role is
// unattributed. A count of only the open findings, or rounds out of order,
// fails it.
func TestDashboardSummary_RoundCountsByRoleSumToEachTotal(t *testing.T) {
	t.Parallel()
	tpDir := t.TempDir()
	writeSliceRound(t, tpDir, "s1", 1, `{"role":"why-auditor","severity":"high"}
{"role":"why-auditor","severity":"low","resolved":{"status":"wontfix","evidence":"e"}}
{"role":"phase-economist","severity":"medium"}
`)
	writeSliceRound(t, tpDir, "s1", 2, `{"severity":"low"}`+"\n")
	writeSliceRound(t, tpDir, "s2", 1, "")

	sum, err := DashboardSummaryOf(tpDir, &model.EpicGraph{})
	require.NoError(t, err)
	assert.Equal(t, []SliceRoundCount{
		{Round: 1, Total: 3, ByRole: map[string]int{"why-auditor": 2, "phase-economist": 1}},
		{Round: 2, Total: 1, ByRole: map[string]int{EpicRoleUnattributed: 1}},
	}, sum.Rounds["s1"])
	assert.Equal(t, []SliceRoundCount{{Round: 1, Total: 0, ByRole: map[string]int{}}}, sum.Rounds["s2"])
	for dir, rounds := range sum.Rounds {
		for _, r := range rounds {
			n := 0
			for _, c := range r.ByRole {
				n += c
			}
			assert.Equal(t, r.Total, n, "%s round %d: by-role counts sum to the total", dir, r.Round)
		}
	}
}
