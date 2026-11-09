package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestEpicThroughput_CountsEpicsDoneNowInTheirISOWeek is spec/1.5.0.md §7
// with spec/1.5.0-corrections.md C4: throughput counts the epics whose status
// is done now, in the ISO week of their done_at. c reopened after it was done
// in week 44, so it counts in no week until it is done again; d is done in
// milestone B; 2027-01-01 falls in ISO week 2026-W53. A count that read
// done_at alone would list a week 44.
func TestEpicThroughput_CountsEpicsDoneNowInTheirISOWeek(t *testing.T) {
	g := epicChainGraph()
	g.Epics[3].Milestone = "B"
	at := func(y int, m time.Month, d int) *time.Time {
		ts := time.Date(y, m, d, 12, 0, 0, 0, time.UTC)
		return &ts
	}
	states := map[string]EpicState{
		"a": {Status: EpicStatusDone, DoneAt: at(2027, time.January, 1)},
		"b": {Status: EpicStatusDone, DoneAt: at(2026, time.November, 3)},
		"c": {Status: EpicStatusInProgress, DoneAt: at(2026, time.October, 28)},
		"d": {Status: EpicStatusDone, DoneAt: at(2026, time.November, 4)},
	}

	assert.Equal(t, []EpicWeek{{Week: "2026-W45", Done: 1}, {Week: "2026-W53", Done: 1}},
		EpicThroughput(g, states, "A"))

	states["c"] = EpicState{Status: EpicStatusDone, DoneAt: at(2026, time.November, 6)}
	assert.Equal(t, []EpicWeek{{Week: "2026-W45", Done: 2}, {Week: "2026-W53", Done: 1}},
		EpicThroughput(g, states, "A"), "done again, c counts in its later week")
	assert.Equal(t, []EpicWeek{{Week: "2026-W45", Done: 1}}, EpicThroughput(g, states, "B"))
	assert.Empty(t, EpicThroughput(g, states, "L1"))
}
