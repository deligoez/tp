package engine

import (
	"fmt"
	"strings"
	"time"

	"github.com/deligoez/tp/internal/model"
)

// EpicDoneRefusedError reports a done record refused on an epic
// (spec/1.5.0.md §5); nothing was written.
type EpicDoneRefusedError struct {
	ID     string
	Reason string
}

func (e *EpicDoneRefusedError) Error() string {
	return fmt.Sprintf("epic %q cannot be recorded done: %s", e.ID, e.Reason)
}

// RecordEpicDone returns the mutate that writes id's done record at at, on an
// epic with no bound file, condition or not (spec/1.5.0.md §5). It returns an
// *EpicNotFoundError for an id the graph does not hold, and an
// *EpicDoneRefusedError for a condition whose acceptance is empty and for an
// epic a task file found from start binds, whose status is derived.
func RecordEpicDone(id, evidence, start string, at time.Time) func(*model.EpicGraph) error {
	return func(g *model.EpicGraph) error {
		i := epicIndex(g, id)
		if i < 0 {
			return &EpicNotFoundError{ID: id}
		}
		e := &g.Epics[i]
		if e.Condition && strings.TrimSpace(e.Acceptance) == "" {
			return &EpicDoneRefusedError{ID: id, Reason: "it is a condition with an empty acceptance"}
		}
		bound, err := EpicBoundFiles(start)
		if err != nil {
			return err
		}
		if files := bound[id]; len(files) > 0 {
			return &EpicDoneRefusedError{ID: id, Reason: files[0] + " is bound to it, so its status is derived"}
		}
		e.Done = &model.EpicDone{At: at.UTC(), Evidence: evidence}
		return nil
	}
}

// UndoEpicDone returns the mutate that removes id's done record, or an
// *EpicNotFoundError for an id the graph does not hold.
func UndoEpicDone(id string) func(*model.EpicGraph) error {
	return func(g *model.EpicGraph) error {
		i := epicIndex(g, id)
		if i < 0 {
			return &EpicNotFoundError{ID: id}
		}
		g.Epics[i].Done = nil
		return nil
	}
}
