package cli_test

import (
	"fmt"
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

// TestDone_CommitFilesOfAMergeThatRenamedOnOneSide: a merge's combined
// status carries one letter per parent (RM, MM) and one path; it is not a
// two-path rename, so both paths are kept as written.
func TestDone_CommitFilesOfAMergeThatRenamedOnOneSide(t *testing.T) {
	t.Parallel()
	dir := setupCommitProject(t, "t1")
	git := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
		return strings.TrimSpace(string(out))
	}
	write := func(name, body string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600))
	}
	// Long enough that the merged g.txt still reads as x.txt renamed.
	var lines strings.Builder
	for i := range 30 {
		fmt.Fprintf(&lines, "line %d\n", i)
	}
	write("x.txt", lines.String())
	write("z.txt", "z\n")
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	main := git("rev-parse", "--abbrev-ref", "HEAD")
	git("checkout", "-q", "-b", "side")
	git("mv", "x.txt", "g.txt")
	git("commit", "-q", "-m", "rename")
	git("checkout", "-q", main)
	write("z.txt", "z main\n")
	git("commit", "-q", "-am", "edit z")
	git("merge", "-q", "--no-ff", "--no-commit", "side")
	write("g.txt", lines.String()+"merged\n")
	write("z.txt", "z merged\n")
	git("add", "-A")
	git("commit", "-q", "-m", "merge")

	_, stderr, code := runTP(t, dir, "done", "t1", "--gate-passed", "--commit", git("rev-parse", "HEAD"), "--", "t1 acceptance met")
	require.Equal(t, 0, code, "done: %s", stderr)
	files, _ := taskState(t, dir, "t1")["commit_files"].([]any)
	assert.ElementsMatch(t, []any{"g.txt", "z.txt"}, filterTP(files))
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
