package engine

import (
	"fmt"

	"github.com/deligoez/tp/internal/model"
)

// EpicConditionRefusedError reports a write that would make an epic with a
// claim or a bound file a condition (spec/1.5.0.md §5); nothing was written.
type EpicConditionRefusedError struct {
	ID     string
	Reason string
}

func (e *EpicConditionRefusedError) Error() string {
	return fmt.Sprintf("epic %q cannot become a condition: %s", e.ID, e.Reason)
}

// EpicConditions returns the ids of g's condition epics.
func EpicConditions(g *model.EpicGraph) map[string]bool {
	ids := map[string]bool{}
	for i := range g.Epics {
		if g.Epics[i].Condition {
			ids[g.Epics[i].ID] = true
		}
	}
	return ids
}

// RefuseNewConditions returns an *EpicConditionRefusedError for the first
// epic of g that is a condition, was not one of before, and is claimed or has
// a task file bound to it, so a condition never has either (§5). Bound files
// are looked up from start only when such an epic is unclaimed.
func RefuseNewConditions(g *model.EpicGraph, before map[string]bool, start string) error {
	var unclaimed []string
	for i := range g.Epics {
		e := &g.Epics[i]
		if !e.Condition || before[e.ID] {
			continue
		}
		if e.Claimed != nil {
			return &EpicConditionRefusedError{ID: e.ID, Reason: "it is claimed by " + e.Claimed.By}
		}
		unclaimed = append(unclaimed, e.ID)
	}
	if len(unclaimed) == 0 {
		return nil
	}
	bound := EpicBoundFiles(start)
	for _, id := range unclaimed {
		if files := bound[id]; len(files) > 0 {
			return &EpicConditionRefusedError{ID: id, Reason: files[0] + " is bound to it"}
		}
	}
	return nil
}
