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
	args := append(strings.Fields(recipe)[1:], "--", "t1 acceptance criteria met")
	_, stderr, code = runTP(t, wt, args...)
	assert.Equal(t, 0, code, stderr)
}
