package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
			require.NoError(t, json.Unmarshal([]byte(stderr), &refusal), stderr)
			assert.Contains(t, refusal.Hint, "git add -A && git commit -m")
			assert.Contains(t, refusal.Hint, "tp done t1 --commit <sha> --")
			assert.Contains(t, refusal.Hint, "tp land")

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
	assert.Contains(t, stdout, "tp brief t1")
	assert.NotContains(t, stdout+stderr, "--skip-gate")
	assert.NotContains(t, stdout+stderr, "--force")
	assert.Equal(t, "open", taskState(t, dir, "t1")["status"])
	assert.Equal(t, head, gitRun(t, dir, "rev-parse", "HEAD"), "the pick is taken back")
	stdout, _, _ = runTP(t, dir, "land")
	assert.Contains(t, stdout, `"landed": []`, "the request is gone")
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
