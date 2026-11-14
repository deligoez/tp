package cli

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCheckEnv_SpecOutsideTheRepositoryStaysAbsolute: a relative path is
// given only for a spec under the directory the check runs in; one outside it
// is handed absolute rather than as a ../ climb.
func TestCheckEnv_SpecOutsideTheRepositoryStaysAbsolute(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	outside := filepath.Join(t.TempDir(), "spec.md")
	assert.Equal(t, []string{"TP_SPEC=" + outside}, checkEnv(nil, outside, repo))
	assert.Equal(t, []string{"TP_SPEC=" + filepath.Join("spec", "a.md")},
		checkEnv(nil, filepath.Join(repo, "spec", "a.md"), repo))
}

// TestCheckEnv_NoSpecLeavesTPSpecUnset: with no spec, TP_SPEC is absent, not
// empty, and an inherited one is still dropped, so a check can fail on
// "${TP_SPEC:?}" instead of grading whatever the shell held.
func TestCheckEnv_NoSpecLeavesTPSpecUnset(t *testing.T) {
	t.Parallel()
	got := checkEnv([]string{"HOME=/h", "TP_SPEC=stale.md", "TP_ROUND=9", "TP_SPECIAL=kept"}, "", "")
	assert.Equal(t, []string{"HOME=/h", "TP_SPECIAL=kept"}, got)
}
