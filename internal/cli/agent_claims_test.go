package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Agent-bound claims (spec/backlog/agent-bound-claims.md): the brief carries
// its task as a marker line, the plugin's PreToolUse(Agent) hook claims it
// for the spawn, and the plugin's SubagentStop hook refuses one stop while a
// task that spawn claimed is neither done nor waiting.

const (
	agentClaimHook = "hooks/pre-tool-use-agent-claim.sh"
	agentStopHook  = "hooks/subagent-stop-claim.sh"
)

// claimsRepo is a git repository holding one committed task file with three
// tasks: t1 and t2 open and ready, t3 depending on t1.
func claimsRepo(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n\nContent.\n"), 0o600))
	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, "init: %s", stderr)
	addTaskWithEstimate(t, dir, "t1", 5)
	addTaskWithEstimate(t, dir, "t2", 5)
	addTask(t, dir, `{"id":"t3","title":"Task t3","depends_on":["t1"],"estimate_minutes":5,"acceptance":"t3 acceptance criteria met","source_sections":["s1"]}`)
	gitIn(t, dir, "init", "-q")
	gitIn(t, dir, "config", "user.email", "test@test.com")
	gitIn(t, dir, "config", "user.name", "Test")
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "seed")
	return dir
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
	return string(out)
}

// claimedBy is the claimed_by object of one task, or nil.
func claimedBy(t *testing.T, dir, id string) map[string]any {
	t.Helper()
	cb, _ := taskState(t, dir, id)["claimed_by"].(map[string]any)
	return cb
}

func TestBriefCarriesItsTaskMarker(t *testing.T) {
	t.Parallel()
	dir := claimsRepo(t)

	stdout, stderr, code := runTP(t, dir, "brief", "t1")
	require.Equal(t, 0, code, stderr)
	var brief map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &brief))
	assert.Equal(t, "tp-task: t1 spec.tasks.json", brief["marker"])

	sub := filepath.Join(dir, "sub")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	stdout, stderr, code = runTP(t, sub, "--file", "../spec.tasks.json", "brief", "t1")
	require.Equal(t, 0, code, stderr)
	require.NoError(t, json.Unmarshal([]byte(stdout), &brief))
	assert.Equal(t, "tp-task: t1 spec.tasks.json", brief["marker"], "the path is relative to the repository root, not the working directory")
}

