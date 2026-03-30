package cli_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary to a temp location
	tmp, err := os.MkdirTemp("", "tp-test-bin-*")
	if err != nil {
		panic(err)
	}

	binaryPath = filepath.Join(tmp, "tp")
	cmd := exec.Command("go", "build", "-o", binaryPath, "../../cmd/tp")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("failed to build tp binary: " + err.Error())
	}

	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

// runTP runs the tp binary with the given args in the given directory.
// It always passes --json for parseable output.
func runTP(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	fullArgs := append([]string{"--json"}, args...)
	cmd := exec.Command(binaryPath, fullArgs...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "NO_COLOR=1")

	outBytes, err := cmd.Output()
	stdout = string(outBytes)

	if exitErr, ok := err.(*exec.ExitError); ok {
		stderr = string(exitErr.Stderr)
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("unexpected error running tp: %v", err)
	}

	return stdout, stderr, exitCode
}

func TestFullLifecycle(t *testing.T) {
	dir := t.TempDir()

	// Create a spec file for init
	specPath := filepath.Join(dir, "spec.md")
	err := os.WriteFile(specPath, []byte("# Test Spec\n\n## Section 1\nDo things.\n"), 0o600)
	require.NoError(t, err)

	// 1. Init
	_, stderr, code := runTP(t, dir, "init", "spec.md")
	assert.Equal(t, 0, code, "init should succeed: %s", stderr)

	// Verify task file was created
	taskFile := filepath.Join(dir, "spec.tasks.json")
	_, err = os.Stat(taskFile)
	require.NoError(t, err, "task file should exist after init")

	// 2. Add a task
	taskJSON := `{"id":"t1","title":"First task","status":"open","depends_on":[],"estimate_minutes":10,"acceptance":"Tests pass","source_sections":["s1"]}`
	_, stderr, code = runTP(t, dir, "add", taskJSON)
	assert.Equal(t, 0, code, "add should succeed: %s", stderr)

	// 3. Ready -- should show t1
	stdout, _, code := runTP(t, dir, "ready")
	assert.Equal(t, 0, code)
	var readyTasks []map[string]any
	err = json.Unmarshal([]byte(stdout), &readyTasks)
	require.NoError(t, err)
	require.Len(t, readyTasks, 1)
	assert.Equal(t, "t1", readyTasks[0]["id"])

	// 4. Claim t1
	stdout, stderr, code = runTP(t, dir, "claim", "t1")
	assert.Equal(t, 0, code, "claim should succeed: %s", stderr)
	var claimed map[string]any
	err = json.Unmarshal([]byte(stdout), &claimed)
	require.NoError(t, err)
	assert.Equal(t, "wip", claimed["status"])

	// 5. Close t1 with reason
	stdout, stderr, code = runTP(t, dir, "close", "t1", "All tests pass and verification is complete")
	assert.Equal(t, 0, code, "close should succeed: %s", stderr)
	var closed map[string]any
	err = json.Unmarshal([]byte(stdout), &closed)
	require.NoError(t, err)
	assert.Equal(t, "done", closed["status"])
	assert.NotNil(t, closed["closed_at"])
	assert.NotNil(t, closed["closed_reason"])

	// 6. Status
	stdout, _, code = runTP(t, dir, "status")
	assert.Equal(t, 0, code)
	var status map[string]any
	err = json.Unmarshal([]byte(stdout), &status)
	require.NoError(t, err)
	assert.Equal(t, float64(1), status["total"])
	assert.Equal(t, float64(1), status["done"])
	assert.Equal(t, float64(100), status["progress_percent"])
}

func TestClaimDoneTask(t *testing.T) {
	dir := t.TempDir()

	// Setup: init + add + claim + close
	specPath := filepath.Join(dir, "spec.md")
	err := os.WriteFile(specPath, []byte("# Spec\n"), 0o600)
	require.NoError(t, err)

	runTP(t, dir, "init", "spec.md")

	taskJSON := `{"id":"t1","title":"Task","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"Done","source_sections":["s1"]}`
	runTP(t, dir, "add", taskJSON)
	runTP(t, dir, "claim", "t1")
	runTP(t, dir, "close", "t1", "Everything is Done and verified completely")

	// Try to claim the done task
	_, stderr, code := runTP(t, dir, "claim", "t1")
	assert.NotEqual(t, 0, code, "claiming a done task should fail")
	assert.Contains(t, stderr, "cannot claim")
}

func TestCloseWithoutReason(t *testing.T) {
	dir := t.TempDir()

	specPath := filepath.Join(dir, "spec.md")
	err := os.WriteFile(specPath, []byte("# Spec\n"), 0o600)
	require.NoError(t, err)

	runTP(t, dir, "init", "spec.md")

	taskJSON := `{"id":"t1","title":"Task","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"Done","source_sections":["s1"]}`
	runTP(t, dir, "add", taskJSON)
	runTP(t, dir, "claim", "t1")

	// Close without providing a reason
	_, stderr, code := runTP(t, dir, "close", "t1")
	assert.NotEqual(t, 0, code, "close without reason should fail")
	assert.Contains(t, stderr, "reason is required")
}
