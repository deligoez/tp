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

// physicalPath resolves path through every symlink, so a test compares the
// file tp names with the file the test expects whatever spelling of the
// temporary directory either side used (/var is /private/var on macOS).
func physicalPath(t *testing.T, path string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(path)
	require.NoError(t, err, "%s must exist", path)
	return real
}

// TestEpicReview_RoleOutputPathIsTheSliceInbox is v1.7.0 §1: outside a run a
// role's output path is absolute and lies in the slice's inbox,
// <state dir>/inbox/<role>-r<N>.ndjson, and the prompt names that same path.
// The cwd-relative epic-review-<hash>-r<N>-<role>.ndjson it replaces fails on
// every assertion: it is relative and lies in the working directory.
func TestEpicReview_RoleOutputPathIsTheSliceInbox(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)

	out := runEpicReview(t, dir, "--epic", "b")
	require.NotEmpty(t, out.Prompts)
	inbox := filepath.Join(dir, out.StateDir, "inbox")
	for _, p := range out.Prompts {
		assert.True(t, filepath.IsAbs(p.OutputPath), "%s: the output path is absolute: %s", p.Role, p.OutputPath)
		assert.Equal(t, p.Role+"-r1.ndjson", filepath.Base(p.OutputPath), "%s: the file is <role>-r<N>.ndjson", p.Role)
		assert.Equal(t, physicalPath(t, inbox), physicalPath(t, filepath.Dir(p.OutputPath)),
			"%s: the file lies in the slice's inbox, which the emission creates", p.Role)
		assert.Equal(t, 2, strings.Count(p.Prompt, p.OutputPath), "%s: the prompt names the output path in its instruction and its reminder", p.Role)
	}
}

