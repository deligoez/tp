package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// milestoneStateExportGraph is spec/1.8.0.md §6's deferrable fixture: M1's
// gate has three members a, f, c, two of them done, and a fourth, deferrable
// member d, done too; its threshold TBD is a placeholder under the default
// pattern. M2, after M1 in its track, has no gate entry.
const milestoneStateExportGraph = `{
  "version": 1,
  "milestones": [["M1", "M2"]],
  "gates": {"M1": {"members": ["a", "f", "c"], "deferrable": ["d"], "threshold": "TBD"}},
  "epics": [
    {"id": "a", "title": "A", "milestone": "M1", "root": true, "done": {"at": "2026-01-03T03:04:05Z", "evidence": "shipped"}},
    {"id": "f", "title": "F", "milestone": "M1", "root": true, "done": {"at": "2026-01-03T03:04:05Z", "evidence": "shipped"}},
    {"id": "c", "title": "C", "milestone": "M1", "root": true},
    {"id": "d", "title": "D", "milestone": "M1", "root": true, "done": {"at": "2026-01-03T03:04:05Z", "evidence": "shipped"}},
    {"id": "e", "title": "E", "milestone": "M2", "root": true}
  ]
}
`

// exportMilestoneState is one milestones[] entry's state as the export
// prints it.
type exportMilestoneState struct {
	Milestone string `json:"milestone"`
	State     *struct {
		MembersDone int    `json:"members_done"`
		Members     int    `json:"members"`
		Threshold   string `json:"threshold"`
	} `json:"state"`
}

// TestEpicExport_CarriesEachMilestonesState is spec/1.8.0.md §6: the export
// carries every milestone's state, the deferrable fixture's gate reading 2 of
// 3 with a placeholder threshold, with --compact as without it. The mutant
// counting deferrable members reads 3 of 4 and turns it red.
func TestEpicExport_CarriesEachMilestonesState(t *testing.T) {
	t.Parallel()
	dir := boundEpicDir(t, milestoneStateExportGraph)

	for _, args := range [][]string{{"epic", "export"}, {"epic", "export", "--compact"}} {
		stdout, stderr, code := runTP(t, dir, args...)
		require.Equal(t, 0, code, "stderr: %s", stderr)
		var p struct {
			Milestones []exportMilestoneState `json:"milestones"`
		}
		require.NoError(t, json.Unmarshal([]byte(stdout), &p), "stdout: %s", stdout)
		require.Len(t, p.Milestones, 2, "%v", args)
		for _, m := range p.Milestones {
			require.NotNil(t, m.State, "%v: %s carries state", args, m.Milestone)
		}
		first, second := p.Milestones[0], p.Milestones[1]
		assert.Equal(t, "M1", first.Milestone)
		assert.Equal(t, [2]int{2, 3}, [2]int{first.State.MembersDone, first.State.Members}, "%v: 2 of 3, d left out", args)
		assert.Equal(t, "placeholder", first.State.Threshold, "%v", args)
		assert.Equal(t, "absent", second.State.Threshold, "%v: M2 has no gate entry", args)
	}
}
