package cli

import (
	"os"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// noTasksYet is the no-tasks-yet answer on tf when it holds no task, nil
// otherwise.
func noTasksYet(tf *model.TaskFile, taskFilePath string) *engine.NoTasksYet {
	if len(tf.Tasks) > 0 {
		return nil
	}
	specPath, _ := engine.ResolveSpecPath(taskFilePath, tf.Spec)
	n := engine.NoTasksYetFor(taskFilePath, specPath, tf)
	return &n
}

// exitIfNoTasks is tp next's and tp next --peek's answer on a file holding no
// task: done false, rather than all tasks complete, with the phase tp resume
// reports, its next step as hint and command, and exit 4 as tp next's other
// no-task answers.
func exitIfNoTasks(tf *model.TaskFile, taskFilePath string) {
	n := noTasksYet(tf, taskFilePath)
	if n == nil {
		return
	}
	result := map[string]any{"done": false, "message": n.Message, "hint": n.Hint}
	if n.Phase != "" {
		result["phase"] = n.Phase
	}
	if n.Command != nil {
		result["command"] = *n.Command
	}
	printResultOrExit(result)
	os.Exit(ExitState)
}

// refuseNoTasks is tp claim --all-ready's and tp ready --first's answer on a
// file holding no task: exit 4, as with no ready task, saying there are no
// tasks yet and the step tp resume names.
func refuseNoTasks(tf *model.TaskFile, taskFilePath string) {
	if n := noTasksYet(tf, taskFilePath); n != nil {
		output.Error(ExitState, n.Message, n.Hint)
		os.Exit(ExitState)
	}
}

// noticeNoTasks is tp ready's answer on a file holding no task: --first
// refuses; the other modes keep their empty answer and say why on stderr.
func noticeNoTasks(tf *model.TaskFile, taskFilePath string) {
	if readyFirst {
		refuseNoTasks(tf, taskFilePath)
		return
	}
	if n := noTasksYet(tf, taskFilePath); n != nil {
		output.Notice("notice: " + n.Message + "; " + n.Hint)
	}
}
