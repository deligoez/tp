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

const importLockTaskFile = `{
	"version": 1,
	"spec": "spec.md",
	"workflow": {},
	"coverage": {"total_sections": 0, "mapped_sections": 0, "context_only": [], "unmapped": []},
	"tasks": [
		{"id":"t1","title":"Imported","estimate_minutes":5,"acceptance":"import is done","source_sections":["## 1. Setup"],"depends_on":[]}
	]
}`

// importLockSetup lays down an init shell plus an import document and returns
// the symlink-resolved project dir (macOS maps /var to /private/var, and this
// process and the tp subprocess must agree on the absolute task-file path or
// they hash to different lock files).
func importLockSetup(t *testing.T) (dir, importPath, taskFilePath string) {
	t.Helper()
	dir = t.TempDir()
	realDir, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	dir = realDir

	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("# Import Lock Spec\n## 1. Setup\nSetup section.\n"), 0o600))
	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, "init failed: %s", stderr)

	importPath = filepath.Join(dir, "import.json")
	require.NoError(t, os.WriteFile(importPath, []byte(importLockTaskFile), 0o600))
	return dir, importPath, filepath.Join(dir, "spec.tasks.json")
}

func importLockTaskCount(t *testing.T, taskFilePath string) int {
	t.Helper()
	data, err := os.ReadFile(taskFilePath)
	require.NoError(t, err)
	var tf struct {
		Tasks []json.RawMessage `json:"tasks"`
	}
	require.NoError(t, json.Unmarshal(data, &tf))
	return len(tf.Tasks)
}

// TestImport_WaitsForTaskFileWriteLock is the guard for §3's first claim: the
// import read-modify-write runs INSIDE engine.WithFileLock. Atomicity is not
// mutual exclusion — before this, WriteTaskFile's atomic rename kept the file
// well-formed while an unlocked import still clobbered a concurrent add.
//
// The assertion is discriminating rather than probabilistic: this process holds
// the task-file write lock for a fixed window and checks mid-window that the
// import has written nothing. An unlocked import writes immediately and fails
// the mid-window count; a locked one parks in the retry backoff and only lands
// after the release.
func TestImport_WaitsForTaskFileWriteLock(t *testing.T) {
	dir, importPath, taskFilePath := importLockSetup(t)
	require.Equal(t, 0, importLockTaskCount(t, taskFilePath), "init shell starts with no tasks")

	acquired := make(chan struct{})
	release := make(chan struct{})
	held := make(chan struct{})
	go func() {
		defer close(held)
		_ = engine.WithFileLock(taskFilePath, func() error {
			close(acquired)
			<-release
			return nil
		})
	}()
	<-acquired

	type runResult struct {
		stderr string
		code   int
	}
	done := make(chan runResult, 1)
	go func() {
		cmd := exec.Command(binaryPath, "--json", "import", importPath)
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
	assert.Equal(t, 0, importLockTaskCount(t, taskFilePath),
		"import must not write the task file while another process holds the write lock")

	close(release)
	<-held

	res := <-done
	require.Equal(t, 0, res.code, "import succeeds once the lock is released: %s", res.stderr)
	assert.Equal(t, 1, importLockTaskCount(t, taskFilePath), "the imported task lands after the release")
}

