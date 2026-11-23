package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// landHint is how an agent in its own worktree closes a task a coordinator
// claimed on another tree (spec/backlog/worktree-land.md).
// It names the commit tool the brief named for the effective strategy.
func landHint(taskFilePath, id string) string {
	commit := "commit your work with git (git add -A && git commit -m \"<message>\")"
	if resolveEffectiveStrategy(taskFilePath) == engine.CommitStrategyHC {
		commit = "commit your work with hc (hc run)"
	}
	return fmt.Sprintf("in this worktree, %s, then run tp done %s --commit <sha> -- \"<evidence>\", naming every commit; then stop: the coordinator picks the commits and closes the task", commit, id)
}

// landHolder returns the copy, in another worktree, that holds one of ids
// under a spawn's claim this worktree's copy does not carry; "" when none.
// A task tp land already landed or gave back from this worktree stops the
// close here instead (refuseAfterLanding).
func landHolder(taskFilePath string, ids []string) string {
	for _, id := range ids {
		refuseAfterLanding(taskFilePath, id)
		if holder := engine.ClaimHolderCopy(taskFilePath, id); holder != "" {
			return holder
		}
	}
	return ""
}

// refuseAfterLanding exits 4 when tp land has dealt with a request this
// worktree made for id while this worktree's copy still has the task open:
// closing that copy would close a task the coordinator already closed, or
// one it reopened for a fresh agent.
func refuseAfterLanding(taskFilePath, id string) {
	if taskStatusIn(taskFilePath, id) != model.StatusOpen {
		return
	}
	cwd, _ := os.Getwd()
	o := engine.LandOutcomeIn(taskFilePath, id, cwd)
	if o == nil {
		return
	}
	if o.Landed {
		output.Error(ExitState, fmt.Sprintf("task %s is already landed: the coordinator picked this worktree's commits and closed it", id),
			"nothing is left to do for this task: stop")
	} else {
		output.Error(ExitState, fmt.Sprintf("the coordinator could not land task %s from this worktree (%s) and reopened it for a fresh agent", id, o.Error),
			"nothing is left to do in this worktree: stop")
	}
	os.Exit(ExitState)
}

// runDoneLand is tp done by an agent in its own worktree on a task a
// coordinator claimed on another tree: it checks the evidence against the
// claim's copy, the commits against the coordinator's branch, runs the gate
// here on the agent's code, closes nothing, and records a land request that
// tp land in the coordinator's tree picks and closes.
func runDoneLand(holder, taskFilePath string, taskIDs []string, reason string) error {
	if len(taskIDs) != 1 {
		exitUsage("a close in an agent's worktree names one task", landHint(taskFilePath, "<id>"))
	}
	id := taskIDs[0]
	if given := landRefusedFlags(); given != "" {
		exitUsage(fmt.Sprintf("task %s is claimed in %s: %s does not apply to a close in this worktree", id, holder, given), landHint(taskFilePath, id))
	}
	if len(doneCommits) == 0 {
		exitUsage(fmt.Sprintf("task %s is claimed in %s: a close in this worktree needs --commit <sha>", id, holder), landHint(taskFilePath, id))
	}
	tf, err := model.ReadTaskFile(holder)
	if err != nil {
		output.Error(ExitFile, err.Error(), runtimeFailureHint)
		os.Exit(ExitFile)
	}
	if ce := checkDoneTarget(tf, id, reason, "", nil); ce != nil {
		exitDoneCheckError(ce)
	}
	specPath, _ := engine.ResolveSpecPath(holder, tf.Spec)
	worktree, _ := os.Getwd()
	root := engine.CanonicalProjectRoot(filepath.Dir(holder))
	ordered, checkErr := engine.CheckUnitCommits(root, worktree, holder, specPath, doneCommits)
	if checkErr != nil {
		output.Error(ExitValidation, checkErr.Error(), landHint(taskFilePath, id))
		os.Exit(ExitValidation)
	}
	runQualityGateIn(taskFilePath, gateDir(taskFilePath), "",
		"fix the failure in this worktree, commit the fix, and run tp done again naming every commit", mainGateTaker())
	record := &engine.LandRequest{ID: id, Reason: reason, Commits: ordered, Worktree: worktree}
	if task, _, findErr := model.FindTask(tf, id); findErr == nil && task.ClaimedBy != nil {
		record.Spawn = task.ClaimedBy.Spawn
	}
	if writeErr := engine.WriteLandRequest(holder, record); writeErr != nil {
		output.Error(ExitFile, writeErr.Error(), runtimeFailureHint)
		os.Exit(ExitFile)
	}
	output.Success(fmt.Sprintf("land request for %s: the coordinator lands it with tp land", id))
	printResultOrExit(map[string]any{"land_requested": true, "id": id, "commits": ordered, "claimed_in": holder})
	return nil
}

// landRefusedFlags names the given tp done flags a close in an agent's
// worktree does not take, "" when none was given.
func landRefusedFlags() string {
	var given []string
	if doneAutoCommit {
		given = append(given, "--auto-commit")
	}
	if doneCoveredBy != "" {
		given = append(given, "--covered-by")
	}
	if doneSkipGate != "" {
		given = append(given, "--skip-gate")
	}
	return strings.Join(given, ", ")
}

// refuseCommitBesideClaim refuses tp commit on a task a coordinator claimed
// on another tree: the commit would carry a task-file change the coordinator's
// merge meets, and close nothing the coordinator can see.
func refuseCommitBesideClaim(taskFilePath, id string) {
	refuseAfterLanding(taskFilePath, id)
	if holder := engine.ClaimHolderCopy(taskFilePath, id); holder != "" {
		exitUsage(fmt.Sprintf("task %s is claimed in %s: tp commit does not apply in this worktree", id, holder), landHint(taskFilePath, id))
	}
}

// refuseOffBranchClose exits 1 when a close in this tree names a commit its
// branch does not have, for a task a spawn claimed here: that commit is a
// worktree agent's, and it lands through tp land, never by a close here.
func refuseOffBranchClose(taskFilePath string, ids []string) {
	if len(doneCommits) == 0 {
		return
	}
	tf, err := model.ReadTaskFile(taskFilePath)
	if err != nil {
		return
	}
	spawned := false
	for _, id := range ids {
		if task, _, findErr := model.FindTask(tf, id); findErr == nil && task.ClaimedBy != nil && task.ClaimedBy.Spawn != "" {
			spawned = true
		}
	}
	if !spawned {
		return
	}
	dir := filepath.Dir(taskFilePath)
	for _, sha := range doneCommits {
		if !engine.CommitOnHead(dir, sha) {
			output.Error(ExitValidation, fmt.Sprintf("commit %s is not on this tree's branch", sha),
				"an agent in its own worktree runs tp done <id> --commit <sha> there and stops; this tree lands its commits with tp land")
			os.Exit(ExitValidation)
		}
	}
}
