package engine

import (
	"fmt"

	"github.com/deligoez/tp/internal/model"
)

// implementGuidance extends the implement phase's guidance line with what a
// reader after a reset cannot see from the units alone: what a wip task the
// next unit resumes is waiting on, and that only a chain is left when
// run_max_parallel would run more units side by side. The line survives
// --compact, so both reach the SessionStart orientation.
func implementGuidance(base string, tf *model.TaskFile, runMaxParallel int, units []NextUnit) string {
	out := base
	if w := waitingNote(tf, units); w != "" {
		out += "; " + w
	}
	if c := chainNote(tf, runMaxParallel); c != "" {
		out += "; " + c
	}
	return out
}

// waitingNote names the first implement unit whose wip task says what it is
// waiting on.
func waitingNote(tf *model.TaskFile, units []NextUnit) string {
	for i := range units {
		if units[i].Kind != UnitImplement {
			continue
		}
		t, _, err := model.FindTask(tf, units[i].ID)
		if err != nil || t.Status != model.StatusWIP || t.Waiting == "" {
			continue
		}
		return fmt.Sprintf("task %s is wip and waiting on: %s; check that before redoing its work", t.ID, t.Waiting)
	}
	return ""
}

// chainNote says when the remaining tasks form one chain, so a parallel cap
// above 1 runs a single unit at a time until the end.
func chainNote(tf *model.TaskFile, runMaxParallel int) string {
	if runMaxParallel <= 1 {
		return ""
	}
	levels := ComputeParallelismLevels(tf.Tasks)
	if len(levels) < 2 {
		return ""
	}
	for i := range levels {
		if len(levels[i].Tasks) > 1 {
			return ""
		}
	}
	return fmt.Sprintf("only a chain of %d tasks remains, so run_max_parallel %d runs one unit at a time", len(levels), runMaxParallel)
}
