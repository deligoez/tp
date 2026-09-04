package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/deligoez/tp/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initLockSetup lays down a bare spec with no task file yet and returns the
// symlink-resolved project dir plus the task-file path tp init will derive from
// the spec. macOS maps /var to /private/var, and this process and the tp
// subprocess must agree on the absolute task-file path or they hash to
// different lock files.
func initLockSetup(t *testing.T) (dir, taskFilePath string) {
	t.Helper()
	dir = t.TempDir()
	realDir, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	dir = realDir

	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("# Init Lock Spec\n## 1. Setup\nSetup section.\n"), 0o600))
	return dir, filepath.Join(dir, "spec.tasks.json")
}

// holdTaskFileLock takes the task-file write lock from this process and returns
// a release func. The lock target need not exist: LockFilePath hashes the
// absolute path, which is exactly why tp init — whose whole point is that the
// file is absent — can be excluded at all.
func holdTaskFileLock(t *testing.T, taskFilePath string) (release func()) {
	t.Helper()
	acquired := make(chan struct{})
	releaseCh := make(chan struct{})
	held := make(chan struct{})
	go func() {
		defer close(held)
		_ = engine.WithFileLock(taskFilePath, func() error {
			close(acquired)
			<-releaseCh
			return nil
		})
	}()
	<-acquired
	var once bool
	return func() {
		if once {
			return
		}
		once = true
		close(releaseCh)
		<-held
	}
}

// TestInit_WaitsForTaskFileWriteLock is the guard for §3's init claim: the
// stat-then-write runs INSIDE engine.WithFileLock. Atomicity is not mutual
// exclusion — unlocked, init statted a missing target and then wrote the empty
// shell over whatever a concurrent writer had put there in between.
//
// The assertion is discriminating rather than probabilistic: this process holds
// the task-file write lock for a fixed window and checks mid-window that init
// has created nothing. An unlocked init writes immediately and fails the
// mid-window check; a locked one parks in the retry backoff and only lands
// after the release.
func TestInit_WaitsForTaskFileWriteLock(t *testing.T) {
	t.Parallel()
	dir, taskFilePath := initLockSetup(t)
	require.NoFileExists(t, taskFilePath, "the spec starts with no task file")

	release := holdTaskFileLock(t, taskFilePath)
	defer release()

	type runResult struct {
		stderr string
		code   int
	}
	done := make(chan runResult, 1)
	go func() {
		cmd := exec.Command(binaryPath, "--json", "init", "spec.md")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "NO_COLOR=1", "TP_HC=0")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		_, err := cmd.Output()
		code := 0
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		}
		done <- runResult{stderr: stderr.String(), code: code}
	}()

	// Mid-window: the lock is still held, so nothing may have been written.
	time.Sleep(400 * time.Millisecond)
	assert.NoFileExists(t, taskFilePath,
		"init must not write the task file while another process holds the write lock")

	release()

	res := <-done
	require.Equal(t, 0, res.code, "init succeeds once the lock is released: %s", res.stderr)
	assert.FileExists(t, taskFilePath, "the init shell lands after the release")
}

// TestInit_LockContentionTimeoutExitsFour is §3's second claim for init: a lock
// held past lock_timeout_seconds fails the way every other write command fails
// it — exit 4 (STATE) carrying LockTimeoutError's message and hint, which name
// the lock path and the elapsed wait.
func TestInit_LockContentionTimeoutExitsFour(t *testing.T) {
	t.Parallel()
	dir, taskFilePath := initLockSetup(t)

	// Shorten the lock timeout to 1s so the test stays fast. The target does
	// not exist yet, so this also pins that init resolves the timeout through
	// the project config rather than falling back to the built-in default.
	_, stderr, code := runTP(t, dir, "set", "--workflow", "--project", "lock_timeout_seconds=1")
	require.Equal(t, 0, code, "set lock_timeout_seconds: %s", stderr)

	release := holdTaskFileLock(t, taskFilePath)
	defer release()

	start := time.Now()
	_, stderr, code = runTP(t, dir, "init", "spec.md")
	elapsed := time.Since(start)
	assert.Equal(t, 4, code, "an init held past lock_timeout_seconds exits 4 (STATE): %s", stderr)

	// The window bounds pin "resolved": the project config says 1s, so init
	// must retry for about that long — not bail out on the first failed
	// TryLock, and not fall back to the 5s built-in default.
	assert.GreaterOrEqual(t, elapsed, 900*time.Millisecond, "init retried for the configured 1s, not less")
	assert.Less(t, elapsed, 4*time.Second, "init used the configured 1s, not the 5s built-in default")

	assertLockTimeoutErrorObject(t, stderr)
	assert.NoFileExists(t, taskFilePath, "a timed-out init writes nothing")
}

// assertLockTimeoutErrorObject checks stderr is the tp error object a
// LockTimeoutError produces: the message names the contention, the lock path
// and the elapsed wait, and the hint names the lock path.
func assertLockTimeoutErrorObject(t *testing.T, stderr string) {
	t.Helper()
	var errObj map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &errObj), "stderr is the tp error object: %s", stderr)
	assert.Contains(t, errObj["error"], "timed out waiting for lock", "error names the contention")
	assert.Contains(t, errObj["error"], filepath.Join(".tp", "locks"), "error names the lock path")
	assert.Contains(t, errObj["error"], "after ", "error names the elapsed wait")
	hint, _ := errObj["hint"].(string)
	assert.Contains(t, hint, filepath.Join(".tp", "locks"), "hint names the lock path")
}

// TestAddSpec_LockContentionTimeoutExitsFour covers the second half of §3's
// init claim: tp add --spec reaches the same write through runInit, which it
// calls BEFORE taking its own lock. That call is the reason locking runInit
// rather than tp init's command body is load-bearing — an add that creates the
// shell must contend for the task-file lock exactly as a bare init does, and
// fail identically: exit 4 (STATE) with LockTimeoutError's message and hint.
func TestAddSpec_LockContentionTimeoutExitsFour(t *testing.T) {
	t.Parallel()
	dir, taskFilePath := initLockSetup(t)

	_, stderr, code := runTP(t, dir, "set", "--workflow", "--project", "lock_timeout_seconds=1")
	require.Equal(t, 0, code, "set lock_timeout_seconds: %s", stderr)

	release := holdTaskFileLock(t, taskFilePath)
	defer release()

	task := `{"id":"t1","title":"T","estimate_minutes":5,"acceptance":"t1 is done","source_sections":["## 1. Setup"],"depends_on":[]}`
	start := time.Now()
	_, stderr, code = runTP(t, dir, "add", "--spec", "spec.md", task)
	elapsed := time.Since(start)
	assert.Equal(t, 4, code, "an add --spec held past lock_timeout_seconds exits 4 (STATE): %s", stderr)
	assert.GreaterOrEqual(t, elapsed, 900*time.Millisecond, "add --spec retried for the configured 1s, not less")
	assert.Less(t, elapsed, 4*time.Second, "add --spec used the configured 1s, not the 5s built-in default")

	assertLockTimeoutErrorObject(t, stderr)
	assert.NoFileExists(t, taskFilePath, "a timed-out add --spec creates no shell")
}
