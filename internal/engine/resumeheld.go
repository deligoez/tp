package engine

import (
	"fmt"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// noReadyTaskBlocker is the no-ready-task blocker for tf: the unfinished
// dependencies holding open tasks back, and the held tasks with what holds
// each, so a gate that is not a task reaches the operator in words.
func noReadyTaskBlocker(tf *model.TaskFile) Blocker {
	held := heldTasks(tf)
	b := Blocker{
		Code:    "no-ready-task",
		Class:   ClassEscalate,
		Message: "no task is ready; open tasks are blocked by unfinished dependencies",
		Data:    map[string]any{"blocked_by": unmetDeps(tf)},
	}
	if len(held) == 0 {
		return b
	}
	parts := make([]string, 0, len(held))
	for _, h := range held {
		parts = append(parts, fmt.Sprintf("%s (%s)", h["id"], h["waiting"]))
	}
	b.Message = "no task is ready; held: " + strings.Join(parts, ", ")
	b.Data["held"] = held
	return b
}

// heldTasks lists tf's held tasks in file order, each as its id and what
// holds it.
func heldTasks(tf *model.TaskFile) []map[string]string {
	held := []map[string]string{}
	for i := range tf.Tasks {
		if tf.Tasks[i].Held() {
			held = append(held, map[string]string{"id": tf.Tasks[i].ID, "waiting": tf.Tasks[i].Waiting})
		}
	}
	return held
}
