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

// The close command a pool brief prints for the agent's own worktree names
// the task file, relative to the repository root, so it resolves in a
// worktree of a repository that holds more than one task file.
func TestPoolBriefCloseNamesTheTaskFile(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "other.md"), []byte("# Other\n\nContent.\n"), 0o600))
	_, stderr, code := runTP(t, dir, "init", "other.md")
	require.Equal(t, 0, code, "init: %s", stderr)
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "second task file")

	stdout, stderr, code := runTP(t, dir, "brief", "t1", "--file", "spec.tasks.json")
	require.Equal(t, 0, code, stderr)
	var brief struct {
		ClaimToken string `json:"claim_token"`
		Close      string `json:"close"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &brief))
	require.NotEmpty(t, brief.ClaimToken)
	assert.Contains(t, brief.Close, "tp done <id> --file spec.tasks.json --commit <sha> --claim-token "+brief.ClaimToken+" -- ")

	_, stderr, code = runTP(t, dir, "claim", "t1", "--file", "spec.tasks.json", "--spawn", "toolu_W", "--session", "sessC", "--claim-token", brief.ClaimToken)
	require.Equal(t, 0, code, stderr)
	wt := filepath.Join(t.TempDir(), "wt")
	gitIn(t, dir, "worktree", "add", "-q", "-b", "agent-t1", wt)
	sha := agentCommit(t, wt)

	// The recipe as printed, with its placeholders filled, closes the task.
	recipe := strings.NewReplacer("<id>", "t1", "<sha>", sha).Replace("tp done <id> --file spec.tasks.json --commit <sha> --claim-token " + brief.ClaimToken)
	args := strings.Fields(recipe)[1:]
	args = append(args, "--", "t1 acceptance criteria met")
	_, stderr, code = runTP(t, wt, args...)
	assert.Equal(t, 0, code, stderr)
}

// A brief printed in a unit's worktree names the worktree's absolute path and
// says file tools take absolute paths under it, since a relative one can
// resolve against the main checkout; a pool brief, printed before its
// worktree exists, says how to find that path.
func TestWorktreeBriefSaysToUseAbsolutePaths(t *testing.T) {
	t.Parallel()
	_, worktree, taskFile := pendingProject(t)
	top, err := filepath.EvalSymlinks(worktree)
	require.NoError(t, err)
	stdout, stderr, code := runTPEnv(t, worktree, []string{"TP_FILE=" + taskFile}, "brief", "t1")
	require.Equal(t, 0, code, stderr)
	var brief struct {
		Close string `json:"close"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &brief))
	assert.Contains(t, brief.Close, "Your worktree is "+top+". Give every file tool an absolute path under it")

	dir := claimsRepo(t)
	stdout, stderr, code = runTP(t, dir, "brief", "t1")
	require.Equal(t, 0, code, stderr)
	require.NoError(t, json.Unmarshal([]byte(stdout), &brief))
	assert.Contains(t, brief.Close, "give every file tool an absolute path under the directory git rev-parse --show-toplevel prints there")
}

// TestBrief_AProjectConfigTaskClosesInTheMainTree: a task whose touches name
// .tp/config.json runs in the main tree (spec/backlog/worktree-dispatch.md
// §2.1). Its brief, printed in the coordinator's tree or in a worktree,
// carries no worktree close and says to work and close in the main tree;
// the spawn hook still claims from the coordinator's brief.
func TestBrief_AProjectConfigTaskClosesInTheMainTree(t *testing.T) {
	t.Parallel()
	var brief struct {
		ClaimToken string `json:"claim_token"`
		Close      string `json:"close"`
	}
	dir := claimsRepo(t)
	_, stderr, code := runTP(t, dir, "set", "t1", `touches=[".tp/config.json"]`)
	require.Equal(t, 0, code, stderr)
	stdout, stderr, code := runTP(t, dir, "brief", "t1")
	require.Equal(t, 0, code, stderr)
	require.NoError(t, json.Unmarshal([]byte(stdout), &brief))
	assert.NotEmpty(t, brief.ClaimToken)
	assert.NotContains(t, brief.Close, "worktree of your own")
	assert.NotContains(t, brief.Close, "--claim-token")
	assert.Contains(t, brief.Close, "work in the main tree at "+dir)

	root, worktree, taskFile := pendingProject(t)
	_, stderr, code = runTP(t, root, "set", "t1", `touches=[".tp/"]`)
	require.Equal(t, 0, code, stderr)
	main, err := filepath.EvalSymlinks(root)
	require.NoError(t, err)
	stdout, stderr, code = runTPEnv(t, worktree, []string{"TP_FILE=" + taskFile}, "brief", "t1")
	require.Equal(t, 0, code, stderr)
	require.NoError(t, json.Unmarshal([]byte(stdout), &brief))
	assert.NotContains(t, brief.Close, "You work in a worktree of your own")
	assert.Contains(t, brief.Close, "work in the main tree at "+main)
}
