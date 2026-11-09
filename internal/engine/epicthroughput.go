package engine

import (
	"fmt"
	"sort"

	"github.com/deligoez/tp/internal/model"
)

// EpicWeek is one ISO week's throughput: Week as "2026-W45", Done the count
// of the milestone's epics whose done_at falls in it.
type EpicWeek struct {
	Week string
	Done int
}

// EpicThroughput counts milestone's epics per ISO week of their done_at
// (spec/1.5.0.md §7, spec/1.5.0-corrections.md C4). Only an epic whose status
// is done now counts, so one that reopened counts in no week until it is done
// again, then in that later week; a done epic whose done_at cannot be read
// counts in none. states is DeriveEpicStates' result. Weeks come in order and
// only weeks with a done epic are listed.
func EpicThroughput(g *model.EpicGraph, states map[string]EpicState, milestone string) []EpicWeek {
	counts := map[string]int{}
	for i := range g.Epics {
		e := &g.Epics[i]
		st := states[e.ID]
		if e.Milestone != milestone || st.Status != EpicStatusDone || st.DoneAt == nil {
			continue
		}
		year, week := st.DoneAt.UTC().ISOWeek()
		counts[fmt.Sprintf("%d-W%02d", year, week)]++
	}
	out := make([]EpicWeek, 0, len(counts))
	for week, n := range counts {
		out = append(out, EpicWeek{Week: week, Done: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Week < out[j].Week })
	return out
}
