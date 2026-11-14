package cli_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// namedAudit emits an audit over n changed files, all named with
// --affected-files.
func namedAudit(t *testing.T, n int, extra ...string) (stdout, stderr string) {
	t.Helper()
	dir, specPath := auditRepoWithChangedFiles(t, n)
	args := []string{"audit", specPath}
	for i := range n {
		args = append(args, "--affected-files", fmt.Sprintf("f%03d.go", i))
	}
	stdout, stderr, code := runTP(t, dir, append(args, extra...)...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	return stdout, stderr
}

// TestAuditNamedList_AboveTheCapSaysItIsGradedInOnePrompt: a named list goes
// whole into every code role's prompt. In a measured trial a role given 121
// files in one prompt graded every one by pattern-scanning and found a fraction
// of what ten-file prompts found on the same files, while `graded N of M` read
// complete. So a named list above the code roles' cap is named, on stderr and
// in notices, and one at the cap is not.
func TestAuditNamedList_AboveTheCapSaysItIsGradedInOnePrompt(t *testing.T) {
	t.Parallel()
	stdout, stderr := namedAudit(t, 11)
	n := noticeOfKind(auditNotices(t, stdout), "named-list-one-prompt")
	require.NotNil(t, n, "an 11-file named list is named")
	assert.Contains(t, n["message"], "11 files")
	assert.Contains(t, stderr, "11 files")

	stdout, _ = namedAudit(t, 10)
	assert.Nil(t, noticeOfKind(auditNotices(t, stdout), "named-list-one-prompt"), "a list at the cap is a normal prompt")
}

// TestAuditNamedList_NoticeSurvivesQuiet: --quiet erases stderr; the payload
// notice stays.
func TestAuditNamedList_NoticeSurvivesQuiet(t *testing.T) {
	t.Parallel()
	stdout, stderr := namedAudit(t, 11, "--quiet")
	assert.NotContains(t, stderr, "named-list")
	assert.NotNil(t, noticeOfKind(auditNotices(t, stdout), "named-list-one-prompt"))
}
