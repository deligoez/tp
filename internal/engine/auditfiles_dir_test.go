package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// TestGitTaskFileMappingResolvesInTheAuditedRepo keeps the regression the CLI
// test of the same name guarded before spec/backlog/project-layer-from-task-
// file.md refused auditing another checkout's spec: GitTaskFileMapping
// resolves each task's commit in dir, never in the process's own repository.
// This test process runs inside tp's repository, which knows nothing of the
// commit made below, so a lookup that ignored dir maps the task to no file.
func TestGitTaskFileMappingResolvesInTheAuditedRepo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	git := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", ".")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "auth_helper.go"), []byte("package main\n"), 0o600))
	git("add", "auth_helper.go")
	git("commit", "-qm", "add auth helper")
	sha := git("rev-parse", "HEAD")

	tasks := []model.Task{{ID: "t1", CommitSHAs: []string{sha}}}
	got := GitTaskFileMapping(dir, tasks, []string{"auth_helper.go"})
	assert.Equal(t, map[string][]string{"auth_helper.go": {"t1"}}, got)
}
