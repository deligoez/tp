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
	block, err := engine.EpicBlockFor(tf, taskFilePath)
	if err != nil {
		output.Error(ExitFile, err.Error(), epicBoundFileHint)
		os.Exit(ExitFile)
	}
	if block == nil {
		return nil
	}
	if block.Notice != "" {
		output.Notice("notice: " + block.Notice)
	}
	if len(block.BlockedBy) == 0 {
		return nil
	}
	return block.BlockedBy
}

// epicBlockedMessage says why no task of tf can start.
func epicBlockedMessage(tf *model.TaskFile, blocked []string) string {
	return fmt.Sprintf("epic %s is blocked by %s", tf.Epic, strings.Join(blocked, ", "))
}

// exitIfNextBlocked is tp next's and tp next --peek's block: while tf's epic
// is blocked it reports no task to start, carrying blocked_by_epics, and
// exits 4 as when no task is ready.
func exitIfNextBlocked(tf *model.TaskFile, taskFilePath string) {
	blocked := epicBlockedBy(tf, taskFilePath)
	if blocked == nil {
		return
	}
	_ = output.JSON(map[string]any{
		"done":             false,
		"message":          "No task to start: " + epicBlockedMessage(tf, blocked) + ".",
		"blocked_by_epics": blocked,
	})
	os.Exit(ExitState)
}

// refuseBlockedClaim is tp claim's block: while tf's epic is blocked it
// claims nothing and exits 1, carrying blocked_by_epics.
func refuseBlockedClaim(tf *model.TaskFile, taskFilePath string) {
	blocked := epicBlockedBy(tf, taskFilePath)
	if blocked == nil {
		return
	}
	output.ErrorExtras(ExitValidation, "cannot claim: "+epicBlockedMessage(tf, blocked),
		map[string]any{"blocked_by_epics": blocked}, epicBlockHint)
	os.Exit(ExitValidation)
}

// readyBlocked is tp ready's block: while tf's epic is blocked it lists no
// ready task, in every mode, carrying blocked_by_epics; --first exits 4 as
// when no task is ready. It reports whether it answered.
func readyBlocked(tf *model.TaskFile, taskFilePath string) bool {
	blocked := epicBlockedBy(tf, taskFilePath)
	if blocked == nil {
		return false
	}
	_ = output.JSON(map[string]any{"ready": []model.Task{}, "blocked_by_epics": blocked})
	if readyFirst {
		os.Exit(ExitState)
	}
	return true
}
