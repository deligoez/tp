package cli_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAuditOfAForeignSpecIsRefusedBeforeChangeDetection: with no
// --affected-files, tp audit detects changed files from git, and in a clean
// foreign tree that detection exits 4, "no changed files". The cross-tree
// refusal comes first, so the answer is the refusal (exit 3), which names
// where to run the audit, not a report about this tree's changes
// (spec/backlog/project-layer-from-task-file.md).
func TestAuditOfAForeignSpecIsRefusedBeforeChangeDetection(t *testing.T) {
	t.Parallel()
	here, there, _ := twoProjects(t)
	initGitRepo(t, there)
	initGitRepo(t, here)

	_, stderr, code := runTP(t, here, "audit", filepath.Join(there, "spec.md"))
	assert.Equal(t, 3, code, "the refusal, not the change detection, answers: %s", stderr)
	assert.Contains(t, stderr, "is not this project's")
	assert.NotContains(t, stderr, "no changed files")
}
