package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClaudeMDExcerptComesFromTheSpecsRepository keeps the regression the CLI
// test TestAuditPrompts_ProjectContextComesFromTheAuditedRepo guarded before
// spec/backlog/project-layer-from-task-file.md refused auditing another
// checkout's spec: the git-root lookup runs in the spec's directory, never in
// the process's. This test process runs inside tp's repository, whose own
// CLAUDE.md would answer if the lookup ignored that directory.
func TestClaudeMDExcerptComesFromTheSpecsRepository(t *testing.T) {
	t.Parallel()
	audited := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(audited, "spec.md"), []byte("# Spec\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(audited, "CLAUDE.md"),
		[]byte("# Audited\n## Conventions\n- AUDITED REPO CONVENTION\n"), 0o600))
	cmd := exec.Command("git", "init", "-q", ".")
	cmd.Dir = audited
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", out)

	excerpt := claudeMDExcerptFor(filepath.Join(audited, "spec.md"))
	assert.Contains(t, excerpt, "AUDITED REPO CONVENTION", "the excerpt comes from the repository holding the spec")
	assert.NotContains(t, excerpt, "Task Plan", "tp's own CLAUDE.md, the process cwd's, never answers")
}
