package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDone_CommitFilesKeepANonASCIIName: commit_files stores the path as it
// is, not the C-quoted form git prints for a name outside plain ASCII.
func TestDone_CommitFilesKeepANonASCIIName(t *testing.T) {
	t.Parallel()
	dir := setupCommitProject(t, "t1")
	for _, f := range []string{"ü.go", "plain.go", "with\"quote.go"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, f), []byte("package x\n"), 0o600))
	}
	git := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
		return strings.TrimSpace(string(out))
	}
	git("add", "-A")
	git("commit", "-q", "-m", "files")
	_, stderr, code := runTP(t, dir, "done", "t1", "--gate-passed", "--commit", git("rev-parse", "HEAD"), "--", "t1 acceptance met")
	require.Equal(t, 0, code, "done: %s", stderr)
	files, _ := taskState(t, dir, "t1")["commit_files"].([]any)
	assert.ElementsMatch(t, []any{"ü.go", "plain.go", "with\"quote.go"}, filterTP(files))
}

// filterTP drops tp's own files from a commit_files list.
func filterTP(files []any) []any {
	out := make([]any, 0, len(files))
	for _, f := range files {
		if s, _ := f.(string); !strings.HasPrefix(s, ".tp") && !strings.HasSuffix(s, ".tasks.json") && s != "spec.md" {
			out = append(out, f)
		}
	}
	return out
}
