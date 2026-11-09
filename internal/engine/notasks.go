package engine

import (
	"fmt"

	"github.com/deligoez/tp/internal/model"
)

// NoTasksYet is what tp next, tp ready and tp claim --all-ready say about a
// task file that holds no task: not that every task is complete, but that
// there are none yet, the phase tp resume reports for the file, and the step
// that phase takes next. Phase is "" and Command nil when the review state
// cannot be read; Hint then points at tp resume, which says why.
type NoTasksYet struct {
	Message string
	Phase   string
	Hint    string
	Command *string
}

// NoTasksYetFor reads tp resume's phase and next action for tf, a task file
// holding no task, through the same loops tp resume reads.
func NoTasksYetFor(taskFilePath, specPath string, tf *model.TaskFile) NoTasksYet {
	const empty = "no tasks yet: the task file holds no task"
	loops, err := readResumeLoops(taskFilePath, specPath, tf)
	if err != nil {
		return NoTasksYet{Message: empty, Hint: "run tp resume for the next step"}
	}
	next := BuildNextAction(loops.phase, specPath, tf, loops.st, len(loops.wf.Checks) > 0)
	hint := next.Summary
	if next.Command != nil {
		hint += ": " + *next.Command
	}
	return NoTasksYet{
		Message: fmt.Sprintf("%s; tp resume reports phase %s", empty, loops.phase),
		Phase:   loops.phase,
		Hint:    hint,
		Command: next.Command,
	}
}
