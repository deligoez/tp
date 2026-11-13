package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// auditNotices decodes an audit payload's notices array.
func auditNotices(t *testing.T, stdout string) []map[string]any {
	t.Helper()
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	raw, _ := result["notices"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, n := range raw {
		out = append(out, n.(map[string]any))
	}
	return out
}

// noticeOfKind returns the notice of the given kind, or nil.
func noticeOfKind(notices []map[string]any, kind string) map[string]any {
	for _, n := range notices {
		if n["kind"] == kind {
			return n
		}
	}
	return nil
}

// TestAuditCut_BelowTheAutoDetectCapIsReported: decision 3. Fourteen changed
// files sit under the auto-detect cap of fifty, so that cap's notice stays
// quiet, and the code roles' cut to ten used to be named by a prompt heading
// alone while file_summary.truncated read false.
func TestAuditCut_BelowTheAutoDetectCapIsReported(t *testing.T) {
	t.Parallel()
	dir, specPath := auditRepoWithChangedFiles(t, 14)

	stdout, stderr, code := runTP(t, dir, "audit", specPath)
	require.Equal(t, 0, code)
	assert.Contains(t, stderr, "code roles audit 10 of 14 files", "the cut is named on stderr")

	cut := noticeOfKind(auditNotices(t, stdout), "code-files-cut")
	require.NotNil(t, cut, "and in the payload")
	assert.Contains(t, cut["message"], "10 of 14")
	assert.Equal(t, true, auditFileSummary(t, stdout)["truncated"], "truncated no longer reads false over a cut list")
}

// TestAuditCut_NoticeSurvivesQuietAndCompact: --quiet erases stderr and
// --compact drops file_summary; the payload notice survives both.
func TestAuditCut_NoticeSurvivesQuietAndCompact(t *testing.T) {
	t.Parallel()
	dir, specPath := auditRepoWithChangedFiles(t, 14)

	stdout, stderr, code := runTP(t, dir, "audit", specPath, "--quiet", "--compact")
	require.Equal(t, 0, code)
	assert.NotContains(t, stderr, "code roles audit")
	assert.NotNil(t, noticeOfKind(auditNotices(t, stdout), "code-files-cut"))
}

// TestAuditCut_NothingCutNothingSaid: three files, no cut, no notice, and
// truncated stays false.
func TestAuditCut_NothingCutNothingSaid(t *testing.T) {
	t.Parallel()
	dir, specPath := auditRepoWithChangedFiles(t, 3)

	stdout, stderr, code := runTP(t, dir, "audit", specPath)
	require.Equal(t, 0, code)
	assert.NotContains(t, stderr, "code roles audit")
	assert.Empty(t, auditNotices(t, stdout))
	assert.Equal(t, false, auditFileSummary(t, stdout)["truncated"])
}
