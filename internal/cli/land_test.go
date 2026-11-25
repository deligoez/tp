package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A worktree agent's close lands through the coordinator
// (spec/backlog/worktree-land.md): the agent records a land request, and
// tp land in the coordinator's tree picks its commits and closes the task.

// landRepo is claimsRepo (t3 depends on t1) with a project gate that fails
// without agent.txt, committed, and t1 claimed for spawn toolu_W on the
// coordinator's tree. It returns the coordinator's tree and the agent's
// worktree, cut from HEAD, where t1 is still open.
func landRepo(t *testing.T, strategy string) (dir, wt string) {
	t.Helper()
	dir = claimsRepo(t)
	_, stderr, code := runTP(t, dir, "set", "--workflow", "--project", "quality_gate=test -f agent.txt")
	require.Equal(t, 0, code, stderr)
	if strategy != "" {
		_, stderr, code = runTP(t, dir, "set", "--workflow", "--project", "commit_strategy="+strategy)
		require.Equal(t, 0, code, stderr)
	}
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "gate")
	spawnClaim(t, dir, "t1", "toolu_W", "sessC")
	wt = filepath.Join(t.TempDir(), "wt")
	gitIn(t, dir, "worktree", "add", "-q", "-b", "agent-t1", wt)
	return dir, wt
}

// agentCommit writes agent.txt in the worktree and commits it with git, as
// the land hint says, returning the commit.
func agentCommit(t *testing.T, wt string) string {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(wt, "agent.txt"), []byte("work\n"), 0o600))
	gitIn(t, wt, "add", "-A")
	gitIn(t, wt, "commit", "-q", "-m", "t1 work")
	return strings.TrimSpace(gitRun(t, wt, "rev-parse", "HEAD"))
}

// lockAgentWorktree locks the agent's worktree wt, as a harness does while
// its agent still runs, so tp land keeps it for the test to run tp in.
func lockAgentWorktree(t *testing.T, dir, wt string) {
	t.Helper()
	gitIn(t, dir, "worktree", "lock", "--reason", "agent still running", wt)
}

func isReady(t *testing.T, dir, id string) bool {
	t.Helper()
	stdout, _, _ := runTP(t, dir, "ready")
	return strings.Contains(stdout, `"id": "`+id+`"`)
}

func TestAWorktreeAgentsCloseLandsThroughTheCoordinator(t *testing.T) {
	t.Parallel()
	for _, strategy := range []string{"builtin", "hc"} {
		t.Run(strategy, func(t *testing.T) {
			t.Parallel()
			dir, wt := landRepo(t, strategy)

			// The brief's own close recipe is refused with the commands to run.
			_, stderr, code := runTP(t, wt, "commit", "t1", "t1 work")
			require.Equal(t, 2, code, stderr)
			var refusal struct {
				Hint string `json:"hint"`
			}
			// The refusal is the last line; a notice may precede it.
			lines := strings.Split(strings.TrimSpace(stderr), "\n")
			require.NoError(t, json.Unmarshal([]byte(lines[len(lines)-1]), &refusal), stderr)
			if strategy == "hc" {
				assert.Contains(t, refusal.Hint, "commit your work with hc", "the hint follows the strategy the brief named")
			} else {
				assert.Contains(t, refusal.Hint, "git add -A && git commit -m")
			}
			assert.Contains(t, refusal.Hint, "tp done t1 --commit <sha> --")
			assert.Contains(t, refusal.Hint, "then stop")

			sha := agentCommit(t, wt)
			stdout, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
			require.Equal(t, 0, code, stderr)
			assert.Contains(t, stdout, `"land_requested": true`)
			assert.Empty(t, gitRun(t, wt, "status", "--porcelain"), "the agent's branch carries no task-file change")
			assert.Equal(t, "wip", taskState(t, dir, "t1")["status"], "nothing is closed before landing")
			assert.False(t, isReady(t, dir, "t3"), "t3 waits until t1's code is on the coordinator's branch")

			stdout, stderr, code = runTP(t, dir, "claims", "--spawn", "toolu_W")
			require.Equal(t, 0, code, stderr)
			assert.Contains(t, stdout, `"unaccounted":[]`, "a land request accounts for the claim")

			// The coordinator is told to land, though its agent still runs.
			p := pool(t, dir, "--session", "sessC", "--live", "toolu_W")
			assert.Equal(t, []string{"t1"}, idsOf(p.Landing))
			assert.True(t, p.Ask)
			assert.Contains(t, p.Reason, "`tp land --file spec.tasks.json`")

			lockAgentWorktree(t, dir, wt)
			stdout, stderr, code = runTP(t, dir, "land")
			require.Equal(t, 0, code, stderr)
			var res struct {
				Landed []string `json:"landed"`
			}
			require.NoError(t, json.Unmarshal([]byte(stdout), &res), stdout)
			assert.Equal(t, []string{"t1"}, res.Landed)
			assert.Equal(t, "done", taskState(t, dir, "t1")["status"])
			assert.FileExists(t, filepath.Join(dir, "agent.txt"), "the agent's commit is on the coordinator's branch")
			assert.True(t, isReady(t, dir, "t3"))

			_, stderr, code = runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
			assert.Equal(t, 4, code, stderr)
			assert.Contains(t, stderr, "already landed")
		})
	}
}

func TestLandReopensATaskWhosePickConflicts(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	sha := agentCommit(t, wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	// The coordinator's branch gets its own agent.txt meanwhile.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "agent.txt"), []byte("other\n"), 0o600))
	gitIn(t, dir, "add", "agent.txt")
	gitIn(t, dir, "commit", "-q", "-m", "other")
	head := gitRun(t, dir, "rev-parse", "HEAD")

	stdout, stderr, code := runTP(t, dir, "land")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, `"reopened"`)
	assert.Contains(t, stdout, "tp brief t1 --file spec.tasks.json")
	assert.NotContains(t, stdout+stderr, "--skip-gate")
	assert.NotContains(t, stdout+stderr, "--force")
	assert.Equal(t, "open", taskState(t, dir, "t1")["status"])
	assert.Equal(t, head, gitRun(t, dir, "rev-parse", "HEAD~1"), "the pick is taken back")
	assert.Equal(t, "spec.tasks.json", gitRun(t, dir, "diff", "--name-only", "HEAD~1", "HEAD"), "only the reopen is committed")
	assert.Contains(t, stdout, "conflicts with this branch in agent.txt")
	stdout, _, _ = runTP(t, dir, "land")
	assert.Contains(t, stdout, `"landed": []`, "the request is gone")

	// The agent, still running, cannot close the reopened task on its copy.
	_, stderr, code = runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	assert.Equal(t, 4, code, stderr)
	assert.Contains(t, stderr, "could not land task t1")
	assert.Empty(t, gitRun(t, wt, "status", "--porcelain"), "the agent's copy is not closed")
}

// A pick that conflicts keeps the agent's work (spec/backlog/worktree-dispatch.md
// §2.2): the reopened task names the agent's branch as its conflict_branch,
// the branch stays, and the next agent's brief carries it, so the conflict is
// resolved rather than the task redone.
func TestLandKeepsTheWorkOfAPickThatConflicts(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	sha := agentCommit(t, wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "agent.txt"), []byte("other\n"), 0o600))
	gitIn(t, dir, "add", "agent.txt")
	gitIn(t, dir, "commit", "-q", "-m", "other")

	stdout, stderr, code := runTP(t, dir, "land")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, "agent-t1", "the reopened item names the branch the work stays on")
	assert.Equal(t, "agent-t1", taskState(t, dir, "t1")["conflict_branch"])
	assert.NotEmpty(t, gitRun(t, dir, "branch", "--list", "agent-t1"), "the branch is kept")
	assert.Equal(t, "agent-t1", committedTaskField(t, dir, "conflict_branch"), "the reopen commit records it")

	stdout, stderr, code = runTP(t, dir, "brief", "t1")
	require.Equal(t, 0, code, stderr)
	var brief struct {
		Task struct {
			ConflictBranch string `json:"conflict_branch"`
		} `json:"task"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &brief), stdout)
	assert.Equal(t, "agent-t1", brief.Task.ConflictBranch, "the brief's Earlier work section names it")
}

// A tp land that crashed after its pick left the work on HEAD under a new
// sha; the rerun must close on it, not refuse the redundant pick forever.
func TestLandClosesOverAPickACrashLeftBehind(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	sha := agentCommit(t, wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	// The coordinator's HEAD moves first, so the pick gets a sha of its own.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "unrelated.txt"), []byte("x\n"), 0o600))
	gitIn(t, dir, "add", "unrelated.txt")
	gitIn(t, dir, "commit", "-q", "-m", "unrelated")
	gitIn(t, dir, "cherry-pick", sha)
	require.NotEqual(t, sha, strings.TrimSpace(gitRun(t, dir, "rev-parse", "HEAD")))

	stdout, stderr, code := runTP(t, dir, "land")
	require.Equal(t, 0, code, stdout+stderr)
	assert.Contains(t, stdout, `"landed": [
    "t1"`)
	assert.Equal(t, "done", taskState(t, dir, "t1")["status"])
	// The task records the commit that carries the work, not an empty one.
	shas, _ := taskState(t, dir, "t1")["commit_shas"].([]any)
	require.Len(t, shas, 1)
	assert.Contains(t, gitRun(t, dir, "show", "--name-only", "--format=", shas[0].(string)), "agent.txt")
}

// An agent's merge commit cannot be picked: the landing ends, it does not
// wait forever.
func TestLandReopensATaskWhoseCommitIsAMerge(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	gitIn(t, wt, "checkout", "-q", "-b", "side")
	side := agentCommit(t, wt)
	gitIn(t, wt, "checkout", "-q", "agent-t1")
	require.NoError(t, os.WriteFile(filepath.Join(wt, "more.txt"), []byte("more\n"), 0o600))
	gitIn(t, wt, "add", "-A")
	gitIn(t, wt, "commit", "-q", "-m", "more")
	more := strings.TrimSpace(gitRun(t, wt, "rev-parse", "HEAD"))
	gitIn(t, wt, "merge", "-q", "--no-ff", "-m", "merge side", "side")
	merge := strings.TrimSpace(gitRun(t, wt, "rev-parse", "HEAD"))
	_, stderr, code := runTP(t, wt, "done", "t1", "--commit", side, "--commit", more, "--commit", merge, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)

	stdout, stderr, code := runTP(t, dir, "land")
	require.Equal(t, 0, code, stdout+stderr)
	assert.Contains(t, stdout, `"reopened"`)
	assert.Contains(t, stdout, "is a merge")
	assert.Equal(t, "open", taskState(t, dir, "t1")["status"])
}

// A command the SubagentStop reason prints, run in the coordinator's tree,
// must not close a spawn's claim on a commit its branch does not have.
func TestACoordinatorCloseRefusesACommitItsBranchLacks(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	sha := agentCommit(t, wt)
	// The coordinator's gate passes without the agent's code.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "agent.txt"), []byte("local\n"), 0o600))

	_, stderr, code := runTP(t, dir, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	assert.Equal(t, 1, code, stderr)
	assert.Contains(t, stderr, "not on this tree's branch")
	// The hint names the worktree holding the commit and the agent's command.
	assert.Contains(t, stderr, filepath.Base(wt))
	assert.Contains(t, stderr, "tp done t1 --commit "+sha)
	assert.Equal(t, "wip", taskState(t, dir, "t1")["status"])

	_, stderr, code = runTP(t, dir, "done", "t1", "--commit", "deadbeef", "--", "t1 acceptance criteria met")
	assert.Equal(t, 1, code, stderr)
	assert.Contains(t, stderr, "does not exist")
}

// A request lands only in the tree whose copy holds the claim it was made
// for, not in a third tree that has the task wip on a claim of its own.
func TestLandRefusesATreeWithoutTheRequestsClaim(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	sha := agentCommit(t, wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	other := filepath.Join(t.TempDir(), "other")
	gitIn(t, dir, "worktree", "add", "-q", "-b", "other", other)
	_, stderr, code = runTP(t, other, "claim", "t1")
	require.Equal(t, 0, code, stderr)

	_, stderr, code = runTP(t, other, "land")
	assert.Equal(t, 2, code, stderr)
	assert.NoFileExists(t, filepath.Join(other, "agent.txt"))

	stdout, stderr, code := runTP(t, dir, "land")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, `"landed": [
    "t1"`)
}

func TestLandKeepsTheCoordinatorsStagedChange(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	sha := agentCommit(t, wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	// A staged file the agent's commit does not touch: git refuses a pick
	// over a dirty index all the same.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "other.txt"), []byte("mine\n"), 0o600))
	gitIn(t, dir, "add", "other.txt")

	stdout, _, code := runTP(t, dir, "land")
	assert.Equal(t, 4, code, stdout)
	assert.Contains(t, stdout, "other.txt")
	assert.Equal(t, "wip", taskState(t, dir, "t1")["status"], "a blocked landing reopens nothing")
	assert.Equal(t, "other.txt", strings.TrimSpace(gitRun(t, dir, "diff", "--cached", "--name-only")), "the staged change stays staged")

	gitIn(t, dir, "commit", "-q", "-m", "mine")
	stdout, stderr, code = runTP(t, dir, "land")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, `"landed": [
    "t1"`)
}

func TestLandRefusesToRunInAnAgentsWorktree(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	sha := agentCommit(t, wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)

	_, stderr, code = runTP(t, wt, "land")
	assert.Equal(t, 2, code, stderr)
	assert.Contains(t, stderr, "holds the claims")

	stdout, stderr, code := runTP(t, dir, "land")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, `"landed": [
    "t1"`, "the request survived the refused run")
}

func TestLandReportsWhyTheGateFailed(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	tokenClaim(t, dir, "toolu_W", "tokA0001")
	sha := agentCommit(t, wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--claim-token", "tokA0001", "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, dir, "set", "--workflow", "--project", "quality_gate=test -f never.txt")
	require.Equal(t, 0, code, stderr)

	stdout, stderr, code := runTP(t, dir, "land")
	require.Equal(t, 0, code, stderr)
	var res struct {
		Reopened []struct {
			Error string `json:"error"`
			Hint  string `json:"hint"`
			Log   string `json:"log"`
		} `json:"reopened"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &res), stdout)
	require.Len(t, res.Reopened, 1, stdout)
	assert.Contains(t, res.Reopened[0].Error, "gate", "the child's own error, not its exit status")
	assert.NotContains(t, res.Reopened[0].Error, "exit status")
	assert.NotContains(t, stdout, "--skip-gate")
	require.NotEmpty(t, res.Reopened[0].Log, stdout)
	log, err := os.ReadFile(res.Reopened[0].Log)
	require.NoError(t, err)
	assert.NotContains(t, string(log), "--skip-gate", "the close's own log offers no skip either")
	assert.Equal(t, map[string]any{"retired": []any{"tokA0001"}}, claimedBy(t, dir, "t1"),
		"the reopen retires the failed agent's token, so its brief cannot spawn again")
}

// A worktree left over from an earlier landing has the task done; a close
// in another tree, where the task is open again, closes there.
func TestACloseBesideALandedWorktreeClosesItsOwnCopy(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	sha := agentCommit(t, wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, dir, "land")
	require.Equal(t, 0, code, stderr)
	gitIn(t, dir, "worktree", "add", "-q", "-b", "later", filepath.Join(t.TempDir(), "later"))

	_, stderr, code = runTP(t, dir, "reopen", "t1")
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, dir, "claim", "t1")
	require.Equal(t, 0, code, stderr)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "agent.txt"), []byte("again\n"), 0o600))
	stdout, stderr, code := runTP(t, dir, "done", "t1", "--auto-commit", "--", "t1 acceptance criteria met again")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, `"closed"`)
	assert.Equal(t, "done", taskState(t, dir, "t1")["status"])
}

func TestLandIsBlockedByTheCoordinatorsOwnChanges(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	sha := agentCommit(t, wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "agent.txt"), []byte("mine\n"), 0o600))

	stdout, _, code := runTP(t, dir, "land")
	assert.Equal(t, 4, code)
	assert.Contains(t, stdout, "agent.txt")
	assert.Contains(t, stdout, "run tp land again")
	assert.Equal(t, "wip", taskState(t, dir, "t1")["status"], "a blocked landing reopens nothing")
}

// A pick that fails for this tree's own reasons (here a held index lock) is
// blocked and keeps the task, not reopened: the agent's work is fine.
func TestLandIsBlockedByAHeldIndexLock(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	sha := agentCommit(t, wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	lock := filepath.Join(dir, ".git", "index.lock")
	require.NoError(t, os.WriteFile(lock, nil, 0o600))

	stdout, _, code := runTP(t, dir, "land")
	assert.Equal(t, 4, code, stdout)
	assert.Contains(t, stdout, `"blocked"`)
	assert.Equal(t, "wip", taskState(t, dir, "t1")["status"], "a blocked landing reopens nothing")

	require.NoError(t, os.Remove(lock))
	stdout, stderr, code = runTP(t, dir, "land")
	require.Equal(t, 0, code, stdout+stderr)
	assert.Equal(t, "done", taskState(t, dir, "t1")["status"])
}

// An index lock another git process holds for a moment is waited out: the
// land retries the pick and lands, rather than reporting the task blocked.
func TestLandWaitsOutABrieflyHeldIndexLock(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	sha := agentCommit(t, wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	lock := filepath.Join(dir, ".git", "index.lock")
	require.NoError(t, os.WriteFile(lock, nil, 0o600))
	released := make(chan struct{})
	go func() {
		defer close(released)
		time.Sleep(300 * time.Millisecond)
		_ = os.Remove(lock)
	}()

	stdout, stderr, code := runTP(t, dir, "land")
	<-released
	require.Equal(t, 0, code, stdout+stderr)
	assert.Contains(t, stdout, `"landed": [
    "t1"
  ]`)
	assert.Equal(t, "done", taskState(t, dir, "t1")["status"])
}

// A cherry-pick left paused in this tree blocks the landing with the command
// that clears it.
func TestLandIsBlockedByACherryPickInProgress(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	sha := agentCommit(t, wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	head := strings.TrimSpace(gitRun(t, dir, "rev-parse", "HEAD"))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git", "CHERRY_PICK_HEAD"), []byte(head+"\n"), 0o600))

	stdout, _, code := runTP(t, dir, "land")
	assert.Equal(t, 4, code, stdout)
	assert.Contains(t, stdout, "git cherry-pick --abort")
	assert.Equal(t, "wip", taskState(t, dir, "t1")["status"])
}

// A change this branch already carries inside a larger commit lands as that
// commit rather than reopening the task.
func TestLandClosesAChangeTheBranchCarriesInsideALargerCommit(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	sha := agentCommit(t, wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "agent.txt"), []byte("work\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "other.txt"), []byte("other\n"), 0o600))
	gitIn(t, dir, "add", "agent.txt", "other.txt")
	gitIn(t, dir, "commit", "-q", "-m", "squashed")
	squashed := strings.TrimSpace(gitRun(t, dir, "rev-parse", "HEAD"))

	stdout, stderr, code := runTP(t, dir, "land")
	require.Equal(t, 0, code, stdout+stderr)
	assert.Equal(t, "done", taskState(t, dir, "t1")["status"], stdout)
	shas, _ := taskState(t, dir, "t1")["commit_shas"].([]any)
	require.Len(t, shas, 1)
	assert.Equal(t, squashed, shas[0], "the task records the commit that already carries the work")
}

// An agent whose landing failed cannot ask again over the fresh agent the
// coordinator spawned for the reopened task: the fresh request lands.
func TestAFailedAgentsSecondCloseLeavesTheFreshAgentsRequest(t *testing.T) {
	t.Parallel()
	dir, wtA := landRepo(t, "")
	shaA := agentCommit(t, wtA)
	_, stderr, code := runTP(t, wtA, "done", "t1", "--commit", shaA, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "agent.txt"), []byte("other\n"), 0o600))
	gitIn(t, dir, "add", "agent.txt")
	gitIn(t, dir, "commit", "-q", "-m", "other")
	stdout, stderr, code := runTP(t, dir, "land")
	require.Equal(t, 0, code, stderr)
	require.Contains(t, stdout, `"reopened"`)

	spawnClaim(t, dir, "t1", "toolu_B", "sessC")
	wtB := filepath.Join(t.TempDir(), "wtB")
	gitIn(t, dir, "worktree", "add", "-q", "-b", "agent-t1-b", wtB)
	require.NoError(t, os.WriteFile(filepath.Join(wtB, "agent.txt"), []byte("fresh\n"), 0o600))
	gitIn(t, wtB, "add", "-A")
	gitIn(t, wtB, "commit", "-q", "-m", "t1 fresh work")
	shaB := strings.TrimSpace(gitRun(t, wtB, "rev-parse", "HEAD"))
	_, stderr, code = runTP(t, wtB, "done", "t1", "--commit", shaB, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)

	_, stderr, code = runTP(t, wtA, "done", "t1", "--commit", shaA, "--", "t1 acceptance criteria met")
	assert.Equal(t, 4, code, "the first agent is told its landing failed: %s", stderr)

	stdout, stderr, code = runTP(t, dir, "land")
	require.Equal(t, 0, code, stdout+stderr)
	assert.Equal(t, "done", taskState(t, dir, "t1")["status"], stdout)
	got, err := os.ReadFile(filepath.Join(dir, "agent.txt"))
	require.NoError(t, err)
	assert.Equal(t, "fresh\n", string(got), "the fresh agent's commit is the one landed")
}

// After a takeover, the old agent's late close does not replace the fresh
// agent's waiting request: the request is refused, naming the other
// worktree, and the fresh agent's commit is the one landed.
func TestALateCloseAfterATakeoverLeavesTheFreshRequest(t *testing.T) {
	t.Parallel()
	dir, wtA := landRepo(t, "")
	shaA := agentCommit(t, wtA)
	_, stderr, code := runTP(t, dir, "claim", "t1", "--takeover")
	require.Equal(t, 0, code, stderr)
	spawnClaim(t, dir, "t1", "toolu_B", "sessC")
	wtB := filepath.Join(t.TempDir(), "wtB")
	gitIn(t, dir, "worktree", "add", "-q", "-b", "agent-t1-b", wtB)
	require.NoError(t, os.WriteFile(filepath.Join(wtB, "agent.txt"), []byte("fresh\n"), 0o600))
	gitIn(t, wtB, "add", "-A")
	gitIn(t, wtB, "commit", "-q", "-m", "t1 fresh work")
	shaB := strings.TrimSpace(gitRun(t, wtB, "rev-parse", "HEAD"))
	_, stderr, code = runTP(t, wtB, "done", "t1", "--commit", shaB, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)

	_, stderr, code = runTP(t, wtA, "done", "t1", "--commit", shaA, "--", "t1 acceptance criteria met")
	assert.Equal(t, 4, code, "the late close is refused: %s", stderr)
	assert.Contains(t, stderr, filepath.Base(wtB), "the refusal names the worktree whose request waits")

	stdout, stderr, code := runTP(t, dir, "land")
	require.Equal(t, 0, code, stdout+stderr)
	got, err := os.ReadFile(filepath.Join(dir, "agent.txt"))
	require.NoError(t, err)
	assert.Equal(t, "fresh\n", string(got), "the fresh agent's commit is the one landed")
}

// committedClaimRepo is a coordinator tree whose t1 claim, made under
// token, is committed before the agent's worktree is added: the worktree's
// own copy then has t1 wip under that claim too.
func committedClaimRepo(t *testing.T, token string) (dir, wt string) {
	t.Helper()
	dir = claimsRepo(t)
	_, stderr, code := runTP(t, dir, "set", "--workflow", "--project", "quality_gate=test -f agent.txt")
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, dir, "claim", "t1", "--spawn", "toolu_W", "--session", "sessC", "--claim-token", token)
	require.Equal(t, 0, code, stderr)
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "claim t1")
	wt = filepath.Join(t.TempDir(), "wt")
	gitIn(t, dir, "worktree", "add", "-q", "-b", "agent-t1", wt)
	return dir, wt
}

// A claim committed with the task file reaches the agent's worktree, where
// t1 is wip as well: the close still lands through the coordinator.
func TestACommittedClaimStillLandsThroughTheCoordinator(t *testing.T) {
	t.Parallel()
	dir, wt := committedClaimRepo(t, "tokA0001")
	sha := agentCommit(t, wt)
	stdout, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--claim-token", "tokA0001", "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, `"land_requested": true`, "the close is a land request, not a close of the worktree's copy")

	stdout, stderr, code = runTP(t, dir, "land")
	require.Equal(t, 0, code, stdout+stderr)
	assert.Equal(t, "done", taskState(t, dir, "t1")["status"], stdout)
}

// Under a committed claim, a close in the agent's worktree after tp land
// picked it is refused, as under an uncommitted one.
func TestACommittedClaimsCloseAfterLandingIsRefused(t *testing.T) {
	t.Parallel()
	dir, wt := committedClaimRepo(t, "tokA0001")
	sha := agentCommit(t, wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--claim-token", "tokA0001", "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	lockAgentWorktree(t, dir, wt)
	stdout, stderr, code := runTP(t, dir, "land")
	require.Equal(t, 0, code, stdout+stderr)

	_, stderr, code = runTP(t, wt, "done", "t1", "--commit", sha, "--claim-token", "tokA0001", "--", "t1 acceptance criteria met")
	assert.Equal(t, 4, code, "the task is landed: %s", stderr)
}

// A coordinator working in a linked worktree writes its own claim's fields
// in its own copy, even when the main worktree carries the same committed
// claim.
func TestALinkedCoordinatorsOwnWriteStaysInItsTree(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)
	coord := filepath.Join(t.TempDir(), "coord")
	gitIn(t, dir, "worktree", "add", "-q", "-b", "coord", coord)
	_, stderr, code := runTP(t, coord, "claim", "t1", "--spawn", "toolu_W", "--session", "sessC", "--claim-token", "tokA0001")
	require.Equal(t, 0, code, stderr)
	gitIn(t, coord, "commit", "-q", "-am", "claim t1")
	gitIn(t, dir, "merge", "-q", "--ff-only", "coord")

	_, stderr, code = runTP(t, coord, "set", "t1", "waiting=a long run")
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, "a long run", taskState(t, coord, "t1")["waiting"], "written in the coordinator's own copy")
	assert.Nil(t, taskState(t, dir, "t1")["waiting"], "not in the main worktree's")
}

// After a takeover the worktree's committed copy names the old token, while
// the coordinator's claim names the fresh one: the coordinator's claim is
// the one the close is checked against.
func TestACommittedClaimIsCheckedAgainstTheCoordinatorsClaim(t *testing.T) {
	t.Parallel()
	dir, _ := committedClaimRepo(t, "tokA0001")
	_, stderr, code := runTP(t, dir, "claim", "t1", "--takeover")
	require.Equal(t, 0, code, stderr)
	tokenClaim(t, dir, "toolu_B", "tokB0002")
	wtB, shaB := freshAgent(t, dir)

	_, stderr, code = runTP(t, wtB, "done", "t1", "--commit", shaB, "--claim-token", "tokA0001", "--", "t1 acceptance criteria met")
	assert.Equal(t, 4, code, "the replaced agent's token is refused: %s", stderr)
	stdout, stderr, code := runTP(t, wtB, "done", "t1", "--commit", shaB, "--claim-token", "tokB0002", "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, `"land_requested": true`)
	landsFresh(t, dir)
}

// tokenClaim binds t1's claim in dir to spawn under a claim token, as the
// spawn hook does from a brief that carries one.
func tokenClaim(t *testing.T, dir, spawn, token string) {
	t.Helper()
	_, stderr, code := runTP(t, dir, "claim", "t1", "--spawn", spawn, "--session", "sessC", "--claim-token", token)
	require.Equal(t, 0, code, stderr)
}

// freshAgent is the agent a takeover spawns: its own worktree and commit.
func freshAgent(t *testing.T, dir string) (wt, sha string) {
	t.Helper()
	wt = filepath.Join(t.TempDir(), "wtB")
	gitIn(t, dir, "worktree", "add", "-q", "-b", "agent-t1-b", wt)
	require.NoError(t, os.WriteFile(filepath.Join(wt, "agent.txt"), []byte("fresh\n"), 0o600))
	gitIn(t, wt, "add", "-A")
	gitIn(t, wt, "commit", "-q", "-m", "t1 fresh work")
	return wt, strings.TrimSpace(gitRun(t, wt, "rev-parse", "HEAD"))
}

// landsFresh runs tp land in dir and asserts the fresh agent's work landed.
func landsFresh(t *testing.T, dir string) {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "land")
	require.Equal(t, 0, code, stdout+stderr)
	got, err := os.ReadFile(filepath.Join(dir, "agent.txt"))
	require.NoError(t, err)
	assert.Equal(t, "fresh\n", string(got), "the fresh agent's commit is the one landed")
}

// The agent a takeover replaced asks after the fresh agent's claim: its
// token is not the claim's, so it is refused before its gate and the fresh
// agent's close goes through.
func TestAReplacedAgentCannotCloseUnderTheFreshClaim(t *testing.T) {
	t.Parallel()
	dir, wtA := landRepo(t, "")
	tokenClaim(t, dir, "toolu_W", "tokA0001")
	shaA := agentCommit(t, wtA)
	_, stderr, code := runTP(t, dir, "claim", "t1", "--takeover")
	require.Equal(t, 0, code, stderr)
	tokenClaim(t, dir, "toolu_B", "tokB0002")

	_, stderr, code = runTP(t, wtA, "done", "t1", "--commit", shaA, "--claim-token", "tokA0001", "--", "t1 acceptance criteria met")
	assert.Equal(t, 4, code, "the replaced agent is refused: %s", stderr)
	assert.NotContains(t, stderr, "tokB0002", "the refusal does not print the claim's token")

	wtB, shaB := freshAgent(t, dir)
	_, stderr, code = runTP(t, wtB, "done", "t1", "--commit", shaB, "--claim-token", "tokB0002", "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	landsFresh(t, dir)
}

// The agent a takeover replaced asks before the fresh agent is spawned: no
// tree holds the claim then, so its close writes no land request, and the
// fresh agent spawned after it is the one that lands.
func TestAReplacedAgentsCloseBeforeTheFreshSpawnDoesNotLand(t *testing.T) {
	t.Parallel()
	dir, wtA := landRepo(t, "")
	tokenClaim(t, dir, "toolu_W", "tokA0001")
	shaA := agentCommit(t, wtA)
	_, stderr, code := runTP(t, dir, "claim", "t1", "--takeover")
	require.Equal(t, 0, code, stderr)
	_, _, _ = runTP(t, wtA, "done", "t1", "--commit", shaA, "--claim-token", "tokA0001", "--", "t1 acceptance criteria met")

	tokenClaim(t, dir, "toolu_B", "tokB0002")
	wtB, shaB := freshAgent(t, dir)
	_, stderr, code = runTP(t, wtB, "done", "t1", "--commit", shaB, "--claim-token", "tokB0002", "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	landsFresh(t, dir)
}

// A fresh agent claimed without a token (a hook older than the claim
// token) does not let the replaced agent's retired token close under it.
func TestARetiredTokenIsRefusedUnderATokenlessClaim(t *testing.T) {
	t.Parallel()
	dir, wtA := landRepo(t, "")
	tokenClaim(t, dir, "toolu_W", "tokA0001")
	shaA := agentCommit(t, wtA)
	_, stderr, code := runTP(t, dir, "claim", "t1", "--takeover")
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, dir, "claim", "t1", "--spawn", "toolu_B", "--session", "sessC")
	require.Equal(t, 0, code, stderr)

	_, stderr, code = runTP(t, wtA, "done", "t1", "--commit", shaA, "--claim-token", "tokA0001", "--", "t1 acceptance criteria met")
	assert.Equal(t, 4, code, "a retired token is refused: %s", stderr)

	wtB, shaB := freshAgent(t, dir)
	_, stderr, code = runTP(t, wtB, "done", "t1", "--commit", shaB, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	landsFresh(t, dir)
}

// The agent a takeover replaced asked before the takeover: its waiting
// request does not block the fresh agent, and it does not land.
func TestAReplacedAgentsEarlierRequestDoesNotBlockTheFreshAgent(t *testing.T) {
	t.Parallel()
	dir, wtA := landRepo(t, "")
	tokenClaim(t, dir, "toolu_W", "tokA0001")
	shaA := agentCommit(t, wtA)
	_, stderr, code := runTP(t, wtA, "done", "t1", "--commit", shaA, "--claim-token", "tokA0001", "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, dir, "claim", "t1", "--takeover")
	require.Equal(t, 0, code, stderr)
	tokenClaim(t, dir, "toolu_B", "tokB0002")

	wtB, shaB := freshAgent(t, dir)
	_, stderr, code = runTP(t, wtB, "done", "t1", "--commit", shaB, "--claim-token", "tokB0002", "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, "a request made under the replaced claim does not block: %s", stderr)
	landsFresh(t, dir)
}

// A takeover while the replaced agent's gate runs: the claim is checked again
// after the gate, so the replaced agent writes no request over the fresh one.
func TestATakeoverDuringTheGateRefusesTheReplacedAgent(t *testing.T) {
	t.Parallel()
	dir, _ := landRepo(t, "")
	once := filepath.Join(t.TempDir(), "once")
	script := fmt.Sprintf("test -f agent.txt || exit 1\n[ -f %q ] && exit 0\ntouch %q\ncd %q || exit 1\n%q claim t1 --takeover >/dev/null && %q claim t1 --spawn toolu_B --session sessC --claim-token tokB0002 >/dev/null\n",
		once, once, dir, binaryPath, binaryPath)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gate.sh"), []byte(script), 0o600))
	_, stderr, code := runTP(t, dir, "set", "--workflow", "--project", "quality_gate=sh gate.sh")
	require.Equal(t, 0, code, stderr)
	// Only the gate: the claim stays an uncommitted change of this tree.
	gitIn(t, dir, "add", "gate.sh", ".tp")
	gitIn(t, dir, "commit", "-q", "-m", "takeover gate")
	tokenClaim(t, dir, "toolu_W", "tokA0001")
	wtA := filepath.Join(t.TempDir(), "wtA")
	gitIn(t, dir, "worktree", "add", "-q", "-b", "agent-t1-a", wtA)
	shaA := agentCommit(t, wtA)

	_, stderr, code = runTP(t, wtA, "done", "t1", "--commit", shaA, "--claim-token", "tokA0001", "--", "t1 acceptance criteria met")
	assert.Equal(t, 4, code, "the claim moved during the gate: %s", stderr)

	wtB, shaB := freshAgent(t, dir)
	_, stderr, code = runTP(t, wtB, "done", "t1", "--commit", shaB, "--claim-token", "tokB0002", "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	landsFresh(t, dir)
}

// A claim that carries a token takes a close only with that token; the
// refusal names neither token.
func TestACloseWithoutTheClaimTokenIsRefused(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	tokenClaim(t, dir, "toolu_W", "tokA0001")
	sha := agentCommit(t, wt)
	for want, extra := range map[int][]string{2: nil, 4: {"--claim-token", "tokZ9999"}} {
		args := append([]string{"done", "t1", "--commit", sha}, extra...)
		_, stderr, code := runTP(t, wt, append(args, "--", "t1 acceptance criteria met")...)
		assert.Equal(t, want, code, "%v: %s", extra, stderr)
		assert.NotContains(t, stderr, "tokA0001")
	}
	_, stderr, code := runTP(t, wt, "done", "t1", "--", "t1 acceptance criteria met")
	assert.Equal(t, 2, code, stderr)
	var refusal struct{ Hint string }
	require.NoError(t, json.Unmarshal([]byte(stderr), &refusal))
	assert.Contains(t, refusal.Hint, "--claim-token <the token your brief names>", "the land hint names the flag, not the token")
	_, stderr, code = runTP(t, wt, "done", "t1", "--commit", sha, "--claim-token", "tokA0001", "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
}

// A request whose task was closed without its commits is not dropped
// silently: tp land names the commits, and the agent is told they did not
// land instead of closing its own copy.
func TestLandTellsTheAgentWhenItsRequestIsDropped(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	sha := agentCommit(t, wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	// The coordinator closes t1 by hand with its own work.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "agent.txt"), []byte("mine\n"), 0o600))
	_, stderr, code = runTP(t, dir, "done", "t1", "--auto-commit", "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)

	stdout, stderr, code := runTP(t, dir, "land")
	require.Equal(t, 0, code, stdout+stderr)
	var res struct {
		Unlanded []struct {
			ID   string `json:"id"`
			Hint string `json:"hint"`
		} `json:"unlanded"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &res), stdout)
	require.Len(t, res.Unlanded, 1, stdout)
	assert.Equal(t, "t1", res.Unlanded[0].ID)
	assert.Contains(t, res.Unlanded[0].Hint, "git cherry-pick "+sha)

	_, stderr, code = runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	assert.Equal(t, 4, code, stderr)
	assert.Contains(t, stderr, "could not land task t1")
	assert.Equal(t, "open", taskState(t, wt, "t1")["status"], "the agent's copy is not closed")
}

// A landed task is done in the coordinator's tree; a write to the agent's
// copy of it is refused as tp done's is.
// A request whose task is already done with its change on this branch, as a
// crash between the close and its outcome leaves it, is dropped as landed:
// nothing is named unlanded, and the agent is told it landed.
func TestLandDropsARequestItsBranchAlreadyCarriesAsLanded(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	sha := agentCommit(t, wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	// A commit of the coordinator's own first, so the pick is a new sha.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "other.txt"), []byte("other\n"), 0o600))
	gitIn(t, dir, "add", "other.txt")
	gitIn(t, dir, "commit", "-q", "-m", "other")
	gitIn(t, dir, "cherry-pick", sha)
	picked := strings.TrimSpace(gitRun(t, dir, "rev-parse", "HEAD"))
	require.NotEqual(t, sha, picked)
	_, stderr, code = runTP(t, dir, "done", "t1", "--commit", picked, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)

	stdout, stderr, code := runTP(t, dir, "land")
	require.Equal(t, 0, code, stdout+stderr)
	assert.Contains(t, stdout, `"unlanded": []`, "the branch carries the commit under another sha")

	_, stderr, code = runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	assert.Equal(t, 4, code, stderr)
	assert.Contains(t, stderr, "already landed")
}

func TestASetInTheAgentsWorktreeAfterLandingIsRefused(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	sha := agentCommit(t, wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	lockAgentWorktree(t, dir, wt)
	_, stderr, code = runTP(t, dir, "land")
	require.Equal(t, 0, code, stderr)

	for _, kv := range []string{"waiting=a long run", "title=Renamed"} {
		_, stderr, code = runTP(t, wt, "set", "t1", kv)
		assert.Equal(t, 4, code, "%s: %s", kv, stderr)
		assert.Contains(t, stderr, "already landed", kv)
	}
	assert.Empty(t, gitRun(t, wt, "status", "--porcelain"), "the agent's copy is untouched")
}

// Every command that writes one task refuses the agent's copy of a task tp
// land dealt with, as tp done does. tp add writes a new task and tp reopen
// only a done one, so neither touches the open copy; tp set --workflow and
// tp set --epic write no task.
func TestEveryTaskWriterRefusesALandedCopy(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	sha := agentCommit(t, wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	lockAgentWorktree(t, dir, wt)
	_, stderr, code = runTP(t, dir, "land")
	require.Equal(t, 0, code, stderr)

	batch := filepath.Join(t.TempDir(), "done.ndjson")
	require.NoError(t, os.WriteFile(batch, []byte(`{"id":"t1","reason":"t1 acceptance criteria met"}`+"\n"), 0o600))
	bulk := filepath.Join(t.TempDir(), "set.ndjson")
	require.NoError(t, os.WriteFile(bulk, []byte(`{"id":"t1","field":"title","value":"Renamed"}`+"\n"), 0o600))
	writers := map[string][]string{
		"claim":     {"claim", "t1"},
		"close":     {"close", "t1", "t1 acceptance criteria met"},
		"remove":    {"remove", "t1"},
		"done":      {"done", "t1", "--auto-commit", "--", "t1 acceptance criteria met"},
		"done bulk": {"done", "--batch", batch},
		"commit":    {"commit", "t1", "t1 acceptance criteria met"},
		"set":       {"set", "t1", "waiting=a long run"},
		"set bulk":  {"set", "--bulk", bulk},
	}
	for name, args := range writers {
		stdout, stderr, code := runTP(t, wt, args...)
		assert.Equal(t, 4, code, "%s: %s%s", name, stdout, stderr)
		assert.Contains(t, stdout+stderr, "already landed", name)
		assert.Empty(t, gitRun(t, wt, "status", "--porcelain"), "%s leaves the agent's copy untouched", name)
		// A writer that slipped through must not hide the next one's result.
		gitIn(t, wt, "checkout", "-q", "--", ".")
	}
}

// A batch row is refused a commit this tree's branch lacks, as a close by id
// is: the commit is a worktree agent's, and it lands through tp land.
func TestABatchCloseRefusesACommitItsBranchLacks(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	sha := agentCommit(t, wt)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "agent.txt"), []byte("local\n"), 0o600))
	batch := filepath.Join(t.TempDir(), "batch.ndjson")
	require.NoError(t, os.WriteFile(batch,
		[]byte(`{"id":"t1","reason":"t1 acceptance criteria met","commit_shas":["`+sha+`"]}`+"\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "done", "--batch", batch)
	assert.NotEqual(t, 0, code, stdout+stderr)
	assert.Contains(t, stdout+stderr, "not on this tree's branch")
	assert.Contains(t, stdout+stderr, "tp done t1 --commit "+sha)
	assert.Equal(t, "wip", taskState(t, dir, "t1")["status"])
}

// landOne has the agent in wt commit its work and request t1's land, then
// runs tp land in dir, returning its output.
func landOne(t *testing.T, dir, wt string) string {
	t.Helper()
	sha := agentCommit(t, wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	stdout, stderr, code := runTP(t, dir, "land")
	require.Equal(t, 0, code, stdout+stderr)
	return stdout
}

// Under builtin, land commits the task file itself: the close leaves no
// change behind for the coordinator to commit.
func TestLandCommitsTheTaskFileUnderBuiltin(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "builtin")
	stdout := landOne(t, dir, wt)
	assert.Contains(t, stdout, `"commit": "`, stdout)
	assert.Empty(t, gitRun(t, dir, "status", "--porcelain", "--", "spec.tasks.json"), "the task file is committed")
	assert.Equal(t, "done", committedTaskField(t, dir, "status"))
}

// Under hc the land commits the task file through hc: a stand-in hc first on
// PATH makes the commit its plan names, and the close leaves nothing behind.
func TestLandUnderHCCommitsTheTaskFileThroughHC(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "hc")
	bin := t.TempDir()
	script := "#!/bin/sh\n[ \"$1\" = run ] || exit 9\ncat > /dev/null\ngit commit -q -m land -- spec.tasks.json\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "hc"), []byte(script), 0o755)) //nolint:gosec // an executable stand-in
	sha := agentCommit(t, wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	stdout, stderr, code := runTPEnv(t, dir, []string{"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH")}, "land")
	require.Equal(t, 0, code, stdout+stderr)
	assert.Contains(t, stdout, `"commit": "`, stdout)
	assert.NotContains(t, stdout, `"task_file_changed"`)
	assert.Empty(t, gitRun(t, dir, "status", "--porcelain", "--", "spec.tasks.json"))
	assert.Equal(t, "done", committedTaskField(t, dir, "status"))
}

// One tp land that closes two tasks makes one task-file commit beside the
// two picks, not one per task.
func TestALandOfTwoTasksMakesOneTaskFileCommit(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)
	_, stderr, code := runTP(t, dir, "set", "--workflow", "--project", "quality_gate=true")
	require.Equal(t, 0, code, stderr)
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "gate")
	for _, id := range []string{"t1", "t2"} {
		spawnClaim(t, dir, id, "toolu_"+id, "sessC")
		wt := filepath.Join(t.TempDir(), id)
		gitIn(t, dir, "worktree", "add", "-q", "-b", "agent-"+id, wt)
		require.NoError(t, os.WriteFile(filepath.Join(wt, id+".txt"), []byte(id+"\n"), 0o600))
		gitIn(t, wt, "add", "-A")
		gitIn(t, wt, "commit", "-q", "-m", id+" work")
		sha := strings.TrimSpace(gitRun(t, wt, "rev-parse", "HEAD"))
		_, stderr, code = runTP(t, wt, "done", id, "--commit", sha, "--", id+" acceptance criteria met")
		require.Equal(t, 0, code, stderr)
	}
	before := strings.TrimSpace(gitRun(t, dir, "rev-parse", "HEAD"))

	stdout, stderr, code := runTP(t, dir, "land")
	require.Equal(t, 0, code, stdout+stderr)
	assert.Equal(t, "3", strings.TrimSpace(gitRun(t, dir, "rev-list", "--count", before+"..HEAD")), "two picks and one task-file commit")
	assert.Empty(t, gitRun(t, dir, "status", "--porcelain", "--", "spec.tasks.json"))
}

// landPrunes is the worktree part of tp land's output.
type landPrunes struct {
	Pruned []struct {
		ID       string `json:"id"`
		Worktree string `json:"worktree"`
		Branch   string `json:"branch"`
	} `json:"pruned"`
	Kept []struct {
		ID       string `json:"id"`
		Worktree string `json:"worktree"`
		Branch   string `json:"branch"`
		Reason   string `json:"reason"`
	} `json:"kept"`
}

// A landed worktree with nothing left in it goes, and its branch with it.
func TestLandRemovesACleanLandedWorktreeAndItsBranch(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	stdout := landOne(t, dir, wt)
	var res landPrunes
	require.NoError(t, json.Unmarshal([]byte(stdout), &res), stdout)
	require.Len(t, res.Pruned, 1, stdout)
	assert.Equal(t, "t1", res.Pruned[0].ID)
	assert.Equal(t, "agent-t1", res.Pruned[0].Branch)
	assert.NoDirExists(t, wt)
	assert.Empty(t, gitRun(t, dir, "branch", "--list", "agent-t1"), "the branch is deleted")
}

// A landed worktree that still holds an uncommitted change is kept, with
// the reason, and so is its branch.
func TestLandKeepsADirtyLandedWorktree(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	sha := agentCommit(t, wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	require.NoError(t, os.WriteFile(filepath.Join(wt, "notes.txt"), []byte("left over\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "land")
	require.Equal(t, 0, code, stdout+stderr)
	var res landPrunes
	require.NoError(t, json.Unmarshal([]byte(stdout), &res), stdout)
	assert.Empty(t, res.Pruned, stdout)
	require.Len(t, res.Kept, 1, stdout)
	assert.Equal(t, "t1", res.Kept[0].ID)
	assert.Contains(t, res.Kept[0].Reason, "uncommitted")
	assert.DirExists(t, wt)
	assert.NotEmpty(t, gitRun(t, dir, "branch", "--list", "agent-t1"))
}

// A landed worktree whose branch carries a commit the land did not pick is
// kept, and so is the branch: deleting it would leave that commit with no ref.
func TestLandKeepsABranchWithACommitItDidNotPick(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	sha := agentCommit(t, wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	require.NoError(t, os.WriteFile(filepath.Join(wt, "later.txt"), []byte("follow-up\n"), 0o600))
	gitRun(t, wt, "add", "later.txt")
	gitRun(t, wt, "commit", "-q", "-m", "follow-up")

	stdout, stderr, code := runTP(t, dir, "land")
	require.Equal(t, 0, code, stdout+stderr)
	var res landPrunes
	require.NoError(t, json.Unmarshal([]byte(stdout), &res), stdout)
	assert.Empty(t, res.Pruned, stdout)
	require.Len(t, res.Kept, 1, stdout)
	assert.Contains(t, res.Kept[0].Reason, "which the land did not pick")
	assert.NotEmpty(t, gitRun(t, dir, "branch", "--list", "agent-t1"))
}

// A landed worktree a process holds locked is kept, with the reason.
func TestLandKeepsALockedLandedWorktree(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	sha := agentCommit(t, wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	gitIn(t, dir, "worktree", "lock", "--reason", "claude agent agent-t1 (pid 4242)", wt)

	stdout, stderr, code := runTP(t, dir, "land")
	require.Equal(t, 0, code, stdout+stderr)
	var res landPrunes
	require.NoError(t, json.Unmarshal([]byte(stdout), &res), stdout)
	assert.Empty(t, res.Pruned, stdout)
	require.Len(t, res.Kept, 1, stdout)
	assert.Contains(t, res.Kept[0].Reason, "locked")
	assert.Contains(t, res.Kept[0].Reason, "pid 4242", "git's lock reason is named")
	assert.DirExists(t, wt)
	assert.NotEmpty(t, gitRun(t, dir, "branch", "--list", "agent-t1"))
}

// A landed worktree whose branch a task records as its conflict_branch is
// kept, branch and all: that branch may hold the only copy of the work the
// task's next agent resolves (spec/backlog/worktree-dispatch.md §2.2).
func TestLandNeverPrunesARecordedConflictBranch(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	setConflictBranch(t, dir, "t3", "agent-t1")
	stdout := landOne(t, dir, wt)
	var res landPrunes
	require.NoError(t, json.Unmarshal([]byte(stdout), &res), stdout)
	assert.Empty(t, res.Pruned, stdout)
	require.Len(t, res.Kept, 1, stdout)
	assert.Contains(t, res.Kept[0].Reason, "t3", "the reason names the task that records the branch")
	assert.DirExists(t, wt)
	assert.NotEmpty(t, gitRun(t, dir, "branch", "--list", "agent-t1"))
}

// conflictThenResolve lands t1 into a conflict, so it reopens with
// conflict_branch agent-t1 and the old worktree wt left in place, then has a
// fresh agent claim t1 in its own worktree, cherry-pick the old branch as its
// brief says, resolve the conflict and request the land.
func conflictThenResolve(t *testing.T, dir, wt string) {
	t.Helper()
	sha := agentCommit(t, wt)
	_, stderr, code := runTP(t, wt, "done", "t1", "--commit", sha, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "agent.txt"), []byte("other\n"), 0o600))
	gitIn(t, dir, "add", "agent.txt")
	gitIn(t, dir, "commit", "-q", "-m", "other")
	stdout, stderr, code := runTP(t, dir, "land")
	require.Equal(t, 0, code, stdout+stderr)
	require.Equal(t, "agent-t1", taskState(t, dir, "t1")["conflict_branch"])

	spawnClaim(t, dir, "t1", "toolu_R", "sessC")
	wtB := filepath.Join(t.TempDir(), "wtB")
	gitIn(t, dir, "worktree", "add", "-q", "-b", "agent-t1-b", wtB)
	pick := exec.Command("git", "cherry-pick", "HEAD..agent-t1")
	pick.Dir = wtB
	out, err := pick.CombinedOutput()
	require.Error(t, err, "the pick conflicts: %s", out)
	require.NoError(t, os.WriteFile(filepath.Join(wtB, "agent.txt"), []byte("resolved\n"), 0o600))
	gitIn(t, wtB, "add", "agent.txt")
	gitIn(t, wtB, "-c", "core.editor=true", "cherry-pick", "--continue")
	resolved := gitRun(t, wtB, "rev-parse", "HEAD")
	_, stderr, code = runTP(t, wtB, "done", "t1", "--commit", resolved, "--", "t1 acceptance criteria met")
	require.Equal(t, 0, code, stderr)
}

// A conflict branch whose work a fresh agent cherry-picked with a resolution
// goes once the task lands: every commit on it has a landed commit with its
// author, author date and message, so its worktree and branch are removed.
func TestLandPrunesAConflictBranchItsResolutionCarries(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	conflictThenResolve(t, dir, wt)

	stdout, stderr, code := runTP(t, dir, "land")
	require.Equal(t, 0, code, stdout+stderr)
	var res landPrunes
	require.NoError(t, json.Unmarshal([]byte(stdout), &res), stdout)
	branches := []string{}
	for _, p := range res.Pruned {
		branches = append(branches, p.Branch)
	}
	assert.ElementsMatch(t, []string{"agent-t1-b", "agent-t1"}, branches, stdout)
	assert.Empty(t, res.Kept, stdout)
	assert.NoDirExists(t, wt, "the old worktree is removed")
	assert.Empty(t, gitRun(t, dir, "branch", "--list", "agent-t1"), "the conflict branch is deleted")
}

// A conflict branch holding a commit the resolution did not carry is kept
// once the task lands, and tp land names the branch and the commit.
func TestLandKeepsAConflictBranchWithAnUnpickedCommit(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	conflictThenResolve(t, dir, wt)
	require.NoError(t, os.WriteFile(filepath.Join(wt, "late.txt"), []byte("late\n"), 0o600))
	gitIn(t, wt, "add", "late.txt")
	gitIn(t, wt, "commit", "-q", "-m", "late work")
	late := gitRun(t, wt, "rev-parse", "HEAD")

	stdout, stderr, code := runTP(t, dir, "land")
	require.Equal(t, 0, code, stdout+stderr)
	var res landPrunes
	require.NoError(t, json.Unmarshal([]byte(stdout), &res), stdout)
	require.Len(t, res.Kept, 1, stdout)
	assert.Equal(t, "t1", res.Kept[0].ID)
	assert.Equal(t, "agent-t1", res.Kept[0].Branch)
	assert.Contains(t, res.Kept[0].Reason, late)
	assert.DirExists(t, wt)
	assert.NotEmpty(t, gitRun(t, dir, "branch", "--list", "agent-t1"), "the conflict branch is kept")
}

// A conflict branch amended in its old worktree after the fresh agent picked
// it is kept once the task lands: the amend keeps the author, author date
// and message a cherry-pick keeps, but moves the branch off the tip recorded
// with the conflict, and the amended commit holds work no land picked.
func TestLandKeepsAConflictBranchAmendedAfterThePick(t *testing.T) {
	t.Parallel()
	dir, wt := landRepo(t, "")
	conflictThenResolve(t, dir, wt)
	require.NoError(t, os.WriteFile(filepath.Join(wt, "precious.txt"), []byte("precious\n"), 0o600))
	gitIn(t, wt, "add", "precious.txt")
	gitIn(t, wt, "commit", "-q", "--amend", "--no-edit")
	amended := gitRun(t, wt, "rev-parse", "HEAD")

	stdout, stderr, code := runTP(t, dir, "land")
	require.Equal(t, 0, code, stdout+stderr)
	var res landPrunes
	require.NoError(t, json.Unmarshal([]byte(stdout), &res), stdout)
	require.Len(t, res.Kept, 1, stdout)
	assert.Equal(t, "agent-t1", res.Kept[0].Branch)
	assert.Contains(t, res.Kept[0].Reason, amended, "the reason names the tip the branch moved to")
	assert.FileExists(t, filepath.Join(wt, "precious.txt"), "the amended work stays")
	assert.NotEmpty(t, gitRun(t, dir, "branch", "--list", "agent-t1"), "the conflict branch is kept")
}
