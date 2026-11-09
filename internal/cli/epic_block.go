package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// epicBlockHint answers a task file whose epic is blocked (spec/1.5.0.md §6).
const epicBlockHint = "finish the epics in blocked_by_epics first; a task already wip stays resumable and closable"

// epicBlockedBy returns the dependencies of tf's epic that are not done, nil
// when none blocks it. An unbound file reads nothing and returns nil; a
// binding the graph ignores prints its notice and returns nil. An unreadable
// graph or bound file exits 3.
func epicBlockedBy(tf *model.TaskFile, taskFilePath string) []string {
	return epicBlockedNoticing(tf, taskFilePath, true)
}

// epicBlockedUnderLock is epicBlockedBy for tf, the file re-read under the
// lock after pre, the file read before it: a quality gate that ran in between
// may have bound the file or changed the graph, so the block is judged again
// on tf. The notice pre's check printed is not printed twice: only a binding
// that changed says so again.
func epicBlockedUnderLock(tf, pre *model.TaskFile, taskFilePath string) []string {
	return epicBlockedNoticing(tf, taskFilePath, tf.Epic != pre.Epic)
}

// epicBlockedNoticing is epicBlockedBy, printing an ignored binding's notice
// only when notice is set.
func epicBlockedNoticing(tf *model.TaskFile, taskFilePath string, notice bool) []string {
	if block := epicBlockNoticing(tf, taskFilePath, notice); block != nil {
		return block.BlockedBy
	}
	return nil
}

// epicBlockNoticing is epicBlockedNoticing's block itself, nil when it blocks
// nothing.
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
	if len(block.BlockedBy) == 0 {
		return nil
	}
	return block
}

// epicBlockedMessage says why no task of tf can start.
func epicBlockedMessage(tf *model.TaskFile, blocked []string) string {
	return fmt.Sprintf("epic %s is blocked by %s", tf.Epic, strings.Join(blocked, ", "))
}

// exitIfNextBlocked is tp next's and tp next --peek's block: while tf's epic
// is blocked it reports no task to start, carrying blocked_by_epics, and
// exits 4 as when no task is ready. When every blocker is a condition it
// carries operator, the operator's decision and record of each. A file with
// no open task has nothing to start, so its normal answer (all tasks
// complete) stands.
func exitIfNextBlocked(tf *model.TaskFile, taskFilePath string) {
	if !engine.HasOpenTasks(tf) {
		return
	}
	block := epicBlockNoticing(tf, taskFilePath, true)
	if block == nil {
		return
	}
	result := map[string]any{
		"done":             false,
		"message":          "No task to start: " + epicBlockedMessage(tf, block.BlockedBy) + ".",
		"blocked_by_epics": block.BlockedBy,
	}
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
		map[string]any{"blocked_by_epics": blocked}, epicBlockHint)
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
	printResultOrExit(map[string]any{"ready": []model.Task{}, "blocked_by_epics": blocked})
	if readyFirst {
		os.Exit(ExitState)
	}
	return true
}

// doneBlockedMessage is why tp done refuses task id of tf while tf's epic is
// blocked: closing a task that is not already wip would start it. It is ""
// when blocked is nil, the task is not open, or tf does not hold it.
func doneBlockedMessage(tf *model.TaskFile, id string, blocked []string) string {
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
func refuseBlockedDone(tf *model.TaskFile, id string, blocked []string) {
	msg := doneBlockedMessage(tf, id, blocked)
	if msg == "" {
		return
	}
	output.ErrorExtras(ExitValidation, msg, map[string]any{"blocked_by_epics": blocked}, epicBlockHint)
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
		map[string]any{"blocked_by_epics": blocked}, epicBlockHint)
	os.Exit(ExitValidation)
}

// doneReadyCount is the ready count tp done reports: a blocked file offers
// no task to start, so its open tasks are left out.
func doneReadyCount(ready int, blocked []string) int {
	if blocked != nil {
		return 0
	}
	return ready
}
