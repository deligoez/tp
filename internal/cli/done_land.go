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
		commit = "commit your work with hc"
	}
	return fmt.Sprintf("in this worktree, %s, then run tp done %s --commit <sha> -- \"<evidence>\", naming every commit; then stop: the coordinator picks the commits and closes the task", commit, id)
}

// landHolder returns the copy, in another worktree, that holds one of ids
// under a spawn's claim this worktree's copy does not carry; "" when none.
// A task tp land already landed or gave back from this worktree stops the
// close here instead (refuseAfterLanding).
// landHolder returns the copy, in another worktree, that holds one of ids
// under a spawn's claim this worktree's copy does not carry; "" when none.
// A task tp land already dealt with from this worktree stops the close here
// instead (claimHolderAfterLanding).
func landHolder(taskFilePath string, ids []string) string {
	for _, id := range ids {
		if holder := claimHolderAfterLanding(taskFilePath, id); holder != "" {
			return holder
		}
	}
	return ""
}

// claimHolderAfterLanding is the one place a command writing a task resolves
// where its claim is held: it exits 4 when tp land already dealt with a
// request this worktree made for id (refuseAfterLanding), and otherwise
// returns the copy in another worktree holding id's claim, "" when none.
func claimHolderAfterLanding(taskFilePath, id string) string {
	refuseAfterLanding(taskFilePath, id)
	return engine.ClaimHolderCopy(taskFilePath, id)
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
		output.Error(ExitState, fmt.Sprintf("the coordinator could not land task %s from this worktree: %s", id, o.Error),
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
	if holder := claimHolderAfterLanding(taskFilePath, id); holder != "" {
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
	for _, id := range ids {
		task, _, findErr := model.FindTask(tf, id)
		if findErr != nil {
			continue
		}
		if msg, hint := offBranchRefusal(filepath.Dir(taskFilePath), task, doneCommits); msg != "" {
			output.Error(ExitValidation, msg, hint)
			os.Exit(ExitValidation)
		}
	}
}

// offBranchRefusal is why a close in dir's tree may not record shas for
// task, with its hint, or "" when it may: a task a spawn claimed here closes
// only on commits this tree's branch has, by id and in a batch alike.
func offBranchRefusal(dir string, task *model.Task, shas []string) (msg, hint string) {
	if task.ClaimedBy == nil || task.ClaimedBy.Spawn == "" {
		return "", ""
	}
	for _, sha := range shas {
		switch {
		case !engine.CommitExists(dir, sha):
			return fmt.Sprintf("commit %s does not exist", sha), "name a commit this tree's branch contains"
		case !engine.CommitOnHead(dir, sha):
			return fmt.Sprintf("commit %s is not on this tree's branch", sha), offBranchHint(dir, task.ID, sha)
		}
	}
	return "", ""
}

// offBranchHint names the way a commit not on this tree's branch reaches it:
// the agent in the worktree holding it asks, and this tree lands the request.
func offBranchHint(dir, id, sha string) string {
	ask := fmt.Sprintf("tp done %s --commit %s -- \"<evidence>\"", id, sha)
	if wt := engine.WorktreeWithCommit(dir, sha); wt != "" {
		ask = fmt.Sprintf("(cd %s && %s)", wt, ask)
	}
	return fmt.Sprintf("the agent in the worktree holding the commit asks for the close there with %s, naming every commit; then run tp land here", ask)
}
