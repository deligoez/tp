package cli

import (
	"fmt"
	"os"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// epicBlockHint answers a task file whose epic is blocked (spec/1.5.0.md §6).
const epicBlockHint = "finish the epics in blocked_by_epics, and one member of each group in blocked_by_any_of, first; a task already wip stays resumable and closable"

// epicBlockedBy returns the block of tf's epic, nil when nothing blocks it:
// no plain dependency that is not done and no unsatisfied group. An unbound
// file reads nothing and returns nil; a binding the graph ignores prints its
// notice and returns nil. An unreadable graph or bound file exits 3.
func epicBlockedBy(tf *model.TaskFile, taskFilePath string) *engine.EpicBlock {
	return epicBlockNoticing(tf, taskFilePath, true)
}

// epicBlockedUnderLock is epicBlockedBy for tf, the file re-read under the
// lock after pre, the file read before it: a quality gate that ran in between
// may have bound the file or changed the graph, so the block is judged again
// on tf. The notice pre's check printed is not printed twice: only a binding
// that changed says so again.
func epicBlockedUnderLock(tf, pre *model.TaskFile, taskFilePath string) *engine.EpicBlock {
	return epicBlockNoticing(tf, taskFilePath, tf.Epic != pre.Epic)
}

// epicBlockNoticing is epicBlockedBy, printing an ignored binding's notice
// only when notice is set.
func epicBlockNoticing(tf *model.TaskFile, taskFilePath string, notice bool) *engine.EpicBlock {
	block, err := engine.EpicBlockFor(tf, taskFilePath)
	if err != nil {
		output.Error(ExitFile, err.Error(), epicBoundFileHint)
		os.Exit(ExitFile)
	}
	if block == nil {
		return nil
	}
	if notice && block.Notice != "" {
		output.Notice("notice: " + block.Notice)
	}
	if !block.Blocks() {
		return nil
	}
	return block
}

// epicBlockFields are the fields every payload of a blocked file carries:
// blocked_by_epics, and blocked_by_any_of while a group is unsatisfied
// (spec/1.7.0.md §7).
func epicBlockFields(block *engine.EpicBlock) map[string]any {
	fields := map[string]any{"blocked_by_epics": block.BlockedBy}
	if len(block.BlockedByAnyOf) > 0 {
		fields["blocked_by_any_of"] = block.BlockedByAnyOf
	}
	return fields
}

// epicBlockedMessage says why no task of tf can start.
func epicBlockedMessage(tf *model.TaskFile, block *engine.EpicBlock) string {
	return fmt.Sprintf("epic %s is blocked by %s", tf.Epic, block.Describe())
}

// exitIfNextBlocked is tp next's and tp next --peek's block: while tf's epic
// is blocked it reports no task to start, carrying blocked_by_epics (and
// blocked_by_any_of), and exits 4 as when no task is ready. When the
// operator alone can clear it, it carries operator, the operator's decision,
// the record of each condition and each group's records as choices. A file
// with no open task has nothing to start, so its normal answer (all tasks
// complete) stands.
func exitIfNextBlocked(tf *model.TaskFile, taskFilePath string) {
	if !engine.HasOpenTasks(tf) {
		return
	}
	block := epicBlockNoticing(tf, taskFilePath, true)
	if block == nil {
		return
	}
	result := epicBlockFields(block)
	result["done"] = false
	result["message"] = "No task to start: " + epicBlockedMessage(tf, block) + "."
	if op := block.OperatorAction(); op != nil {
		result["operator"] = op
	}
	printResultOrExit(result)
	os.Exit(ExitState)
}

// printResultOrExit prints a command's JSON result; a stdout that cannot be
// written is a file error and exits 3, as in tp ground
// (TestOneStdoutWriteErrorIsOneClassification).
func printResultOrExit(result any) {
	if err := output.JSON(result); err != nil {
		output.Error(ExitFile, err.Error(), groundStdoutHint)
		os.Exit(ExitFile)
	}
}

// refuseBlockedClaim is tp claim's block: while tf's epic is blocked it
// claims nothing and exits 1, carrying blocked_by_epics. It judges ids, the
// ids tp claim was given, as tp done judges each id: only an open one would
// start, so ids none of which is an open task of tf answer as on an unbound
// file — not open, not found. A nil ids is --all-ready's, which judges every
// task: a file with no open task answers as an unbound one does.
func refuseBlockedClaim(tf *model.TaskFile, taskFilePath string, ids []string) {
	if !claimsAnOpenTask(tf, ids) {
		return
	}
	blocked := epicBlockedBy(tf, taskFilePath)
	if blocked == nil {
		return
	}
	output.ErrorExtras(ExitValidation, "cannot claim: "+epicBlockedMessage(tf, blocked),
		epicBlockFields(blocked), epicBlockHint)
	os.Exit(ExitValidation)
}

// claimsAnOpenTask reports whether any of ids is an open task of tf; a nil
// ids asks whether tf holds any open task at all.
func claimsAnOpenTask(tf *model.TaskFile, ids []string) bool {
	if ids == nil {
		return engine.HasOpenTasks(tf)
	}
	for _, id := range ids {
		if task, _, err := model.FindTask(tf, id); err == nil && task.Status == model.StatusOpen {
			return true
		}
	}
	return false
}

// readyBlocked is tp ready's block: while tf's epic is blocked it lists no
// ready task, in every mode, carrying blocked_by_epics; --first exits 4 as
// when no task is ready. It reports whether it answered. A file with no open
// task has nothing to start and answers as an unbound one does.
func readyBlocked(tf *model.TaskFile, taskFilePath string) bool {
	if !engine.HasOpenTasks(tf) {
		return false
	}
	blocked := epicBlockedBy(tf, taskFilePath)
	if blocked == nil {
		return false
	}
	result := epicBlockFields(blocked)
	result["ready"] = []model.Task{}
	printResultOrExit(result)
	if readyFirst {
		os.Exit(ExitState)
	}
	return true
}

// doneBlockedMessage is why tp done refuses task id of tf while tf's epic is
// blocked: closing a task that is not already wip would start it. It is ""
// when blocked is nil, the task is not open, or tf does not hold it.
func doneBlockedMessage(tf *model.TaskFile, id string, blocked *engine.EpicBlock) string {
	if blocked == nil {
		return ""
	}
	task, _, err := model.FindTask(tf, id)
	if err != nil || task.Status != model.StatusOpen {
		return ""
	}
	return fmt.Sprintf("cannot done: task %s is not wip and %s", id, epicBlockedMessage(tf, blocked))
}

// refuseBlockedDone is tp done's block for one task: while tf's epic is
// blocked it refuses a task that is not already wip, exits 1 and carries
// blocked_by_epics.
func refuseBlockedDone(tf *model.TaskFile, id string, blocked *engine.EpicBlock) {
	msg := doneBlockedMessage(tf, id, blocked)
	if msg == "" {
		return
	}
	output.ErrorExtras(ExitValidation, msg, epicBlockFields(blocked), epicBlockHint)
	os.Exit(ExitValidation)
}

// refuseBlockedCommit is tp commit's block: its implicit claim would start
// task, so while tf's epic is blocked an open task is refused, exit 1 with
// blocked_by_epics, before anything is staged or written. A task that is not
// open reads nothing. tf is the file read under tp commit's lock.
func refuseBlockedCommit(tf *model.TaskFile, task *model.Task, taskFilePath string) {
	if task.Status != model.StatusOpen {
		return
	}
	blocked := epicBlockedBy(tf, taskFilePath)
	if blocked == nil {
		return
	}
	output.ErrorExtras(ExitValidation,
		fmt.Sprintf("cannot commit: task %s is not wip and %s", task.ID, epicBlockedMessage(tf, blocked)),
		epicBlockFields(blocked), epicBlockHint)
	os.Exit(ExitValidation)
}

// doneReadyCount is the ready count tp done reports: a blocked file offers
// no task to start, so its open tasks are left out.
func doneReadyCount(ready int, blocked *engine.EpicBlock) int {
	if blocked != nil {
		return 0
	}
	return ready
}
