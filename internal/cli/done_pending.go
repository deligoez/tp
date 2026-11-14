package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// pendingCloseHint is how a unit in a worktree closes its task.
const pendingCloseHint = "in a unit's worktree, commit on the unit's branch, then run tp done <id> --commit <sha> -- \"<evidence>\"; the driver integrates and closes it"

// donePendingRoot returns the main tree's project root when the working
// directory is a linked worktree of the task file's repository, else "".
func donePendingRoot(taskFilePath string) string {
	root := engine.CanonicalProjectRoot(filepath.Dir(taskFilePath))
	if engine.LinkedWorktreeOf(".", root) {
		return root
	}
	return ""
}

// runDonePending is tp done in a unit's linked worktree
// (spec/backlog/parallel-implement.md §2.4): it checks the evidence and the
// commits, runs the gate in the worktree, closes nothing, and writes a pending
// close the driver integrates and gates again on the main branch.
func runDonePending(root, taskFilePath string, taskIDs []string, reason string) error {
	switch {
	case len(taskIDs) != 1:
		exitUsage("a close in a unit's worktree names one task", pendingCloseHint)
	case doneAutoCommit || doneCoveredBy != "" || doneSkipGate != "":
		exitUsage("--auto-commit, --covered-by and --skip-gate do not apply in a unit's worktree", pendingCloseHint)
	case len(doneCommits) == 0:
		exitUsage("a close in a unit's worktree needs --commit <sha>", pendingCloseHint)
	}
	id := taskIDs[0]

	tf, err := model.ReadTaskFile(taskFilePath)
	if err != nil {
		// task-file-hint: the task file itself could not be read
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
	}
	task, _, findErr := model.FindTask(tf, id)
	if findErr != nil {
		output.Error(ExitState, findErr.Error())
		os.Exit(ExitState)
	}
	if task.Status != model.StatusWIP {
		output.Error(ExitState, fmt.Sprintf("cannot done: task %s is %s (the driver claims a unit's task)", id, task.Status))
		os.Exit(ExitState)
	}
	if ce := checkDoneTarget(tf, id, reason, "", nil); ce != nil {
		exitDoneCheckError(ce)
	}

	specPath, _ := engine.ResolveSpecPath(taskFilePath, tf.Spec)
	worktree, _ := os.Getwd()
	ordered, checkErr := engine.CheckUnitCommits(root, worktree, taskFilePath, specPath, doneCommits)
	if checkErr != nil {
		output.Error(ExitValidation, checkErr.Error(), pendingCloseHint)
		os.Exit(ExitValidation)
	}

	// The unit gates its own checkout, so a red change is caught before the
	// driver spends a pick and a main-branch gate on it; a red gate exits here
	// and leaves its failure under the main tree's root for the retry's brief.
	runQualityGateIn(taskFilePath, engine.WorktreeDirFor(gateDir(taskFilePath), worktree), root,
		"fix the failure in this worktree, commit the fix, and ask for the close again naming every commit")
	record := &engine.PendingClose{ID: id, Reason: reason, Commits: ordered, Worktree: worktree}
	if writeErr := engine.WritePendingClose(root, taskFilePath, record); writeErr != nil {
		output.Error(ExitFile, writeErr.Error(), runtimeFailureHint)
		os.Exit(ExitFile)
	}
	output.Success(fmt.Sprintf("pending close for %s: the driver integrates it", id))
	printResultOrExit(map[string]any{"pending": true, "id": id, "commits": ordered})
	return nil
}

func exitUsage(msg, hint string) {
	output.Error(ExitUsage, msg, hint)
	os.Exit(ExitUsage)
}
