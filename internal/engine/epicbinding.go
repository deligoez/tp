package engine

import (
	"errors"
	"fmt"
)

// EpicBindingRefusedError reports a task-file binding refused because the
// graph does not hold the epic, the epic is a condition, or it carries a done
// record (spec/1.5.0.md §2); nothing was written.
type EpicBindingRefusedError struct {
	ID     string
	Reason string
}

func (e *EpicBindingRefusedError) Error() string {
	return fmt.Sprintf("cannot bind a task file to epic %q: %s", e.ID, e.Reason)
}

// CheckEpicBinding returns nil when a task file may be bound to epic id of the
// graph in tpDir, an *EpicBindingRefusedError when §2 refuses the binding, and
// the load error of an unreadable or unparsable graph file.
func CheckEpicBinding(tpDir, id string) error {
	g, _, err := LoadEpicGraph(tpDir)
	if errors.Is(err, ErrEpicGraphNotFound) {
		return &EpicBindingRefusedError{ID: id, Reason: "it is not in the graph (no " + EpicGraphFile + ")"}
	}
	if err != nil {
		return err
	}
	for i := range g.Epics {
		e := &g.Epics[i]
		if e.ID != id {
			continue
		}
		switch {
		case e.Condition:
			return &EpicBindingRefusedError{ID: id, Reason: "it is a condition, which the operator decides rather than a task file builds"}
		case e.Done != nil:
			return &EpicBindingRefusedError{ID: id, Reason: "it is done"}
		}
		return nil
	}
	return &EpicBindingRefusedError{ID: id, Reason: "it is not in the graph"}
}
