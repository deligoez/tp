package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// landHint is how an agent in its own worktree closes a task a coordinator
// claimed on another tree (spec/backlog/worktree-land.md).
func landHint(id string) string {
	return fmt.Sprintf("in this worktree, commit your work with git (git add -A && git commit -m \"<message>\"), then run tp done %s --commit <sha> -- \"<evidence>\", naming every commit; the coordinator lands it with tp land", id)
}

// landHolder returns the copy, in another worktree, that holds one of ids
// under a spawn's claim this worktree's copy does not carry; "" when none.
func landHolder(taskFilePath string, ids []string) string {
	for _, id := range ids {
		if holder := engine.ClaimHolderCopy(taskFilePath, id); holder != "" {
			return holder
		}
	}
	return ""
}

// runDoneLand is tp done by an agent in its own worktree on a task a
// coordinator claimed on another tree: it checks the evidence against the
// claim's copy, the commits against the coordinator's branch, runs the gate
// here on the agent's code, closes nothing, and records a land request that
// tp land in the coordinator's tree picks and closes.
func runDoneLand(holder, taskFilePath string, taskIDs []string, reason string) error {
	switch {
	case len(taskIDs) != 1:
		exitUsage("a close in an agent's worktree names one task", landHint("<id>"))
	case doneAutoCommit || doneCoveredBy != "" || doneSkipGate != "":
		exitUsage(fmt.Sprintf("task %s is claimed in %s: --auto-commit, --covered-by and --skip-gate do not apply to a close in this worktree", taskIDs[0], holder), landHint(taskIDs[0]))
	case len(doneCommits) == 0:
		exitUsage(fmt.Sprintf("task %s is claimed in %s: a close in this worktree needs --commit <sha>", taskIDs[0], holder), landHint(taskIDs[0]))
	}
	id := taskIDs[0]
	tf, err := model.ReadTaskFile(holder)
	if err != nil {
		output.Error(ExitFile, err.Error(), runtimeFailureHint)
		os.Exit(ExitFile)
	}
	if task, _, findErr := model.FindTask(tf, id); findErr == nil && task.Status == model.StatusDone {
		output.Error(ExitState, fmt.Sprintf("task %s is already landed and closed in %s", id, holder))
		os.Exit(ExitState)
	}
	if ce := checkDoneTarget(tf, id, reason, "", nil); ce != nil {
		exitDoneCheckError(ce)
	}
	specPath, _ := engine.ResolveSpecPath(holder, tf.Spec)
	worktree, _ := os.Getwd()
	root := engine.CanonicalProjectRoot(filepath.Dir(holder))
	ordered, checkErr := engine.CheckUnitCommits(root, worktree, holder, specPath, doneCommits)
	if checkErr != nil {
		output.Error(ExitValidation, checkErr.Error(), landHint(id))
		os.Exit(ExitValidation)
	}
	runQualityGateIn(taskFilePath, gateDir(taskFilePath), "",
		"fix the failure in this worktree, commit the fix, and run tp done again naming every commit", mainGateTaker())
	record := &engine.LandRequest{ID: id, Reason: reason, Commits: ordered, Worktree: worktree}
	if writeErr := engine.WriteLandRequest(holder, record); writeErr != nil {
		output.Error(ExitFile, writeErr.Error(), runtimeFailureHint)
		os.Exit(ExitFile)
	}
	output.Success(fmt.Sprintf("land request for %s: the coordinator lands it with tp land", id))
	printResultOrExit(map[string]any{"land_requested": true, "id": id, "commits": ordered, "claimed_in": holder})
	return nil
}

// refuseCommitBesideClaim refuses tp commit on a task a coordinator claimed
// on another tree: the commit would carry a task-file change the coordinator's
// merge meets, and close nothing the coordinator can see.
func refuseCommitBesideClaim(taskFilePath, id string) {
	if holder := engine.ClaimHolderCopy(taskFilePath, id); holder != "" {
		exitUsage(fmt.Sprintf("task %s is claimed in %s: tp commit does not apply in this worktree", id, holder), landHint(id))
	}
}
