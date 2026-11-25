package cli_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests below are spec/backlog/audit-round-ergonomics.md §2.2: --inbox
// <dir> names the directory a round's role files are written to and read
// from, in place of the spec's state directory, so a role told the checkout is
// read-only has somewhere to write. The default stays where it is.

// realPath resolves a directory's symlinks, so a temp dir under /var and its
// /private/var spelling compare equal.
func realPath(t *testing.T, dir string) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	return p
}

// TestNamedInbox_EmissionNamesTheInbox: with --inbox, each emitted prompt's
// output_path is <dir>/<phase>-inbox/round-<N>-<role>.ndjson, absolute even
// when the flag was relative, and the prompt's own line names the same file.
func TestNamedInbox_EmissionNamesTheInbox(t *testing.T) {
	t.Parallel()
	dir, specPath, codePath := emissionFixture(t)
	for _, tc := range []struct {
		phase string
		args  []string
	}{
		{"review", []string{"review", specPath, "--no-state", "--inbox", "outside"}},
		{"audit", []string{"audit", specPath, "--affected-files", codePath, "--inbox", "outside"}},
	} {
		t.Run(tc.phase, func(t *testing.T) {
			prompts := emittedPrompts(t, dir, noRoundDir(), tc.args...)
			require.NotEmpty(t, prompts)
			inbox := filepath.Join(dir, "outside", tc.phase+"-inbox")
			require.DirExists(t, inbox, "the emission makes the inbox a role can write into")
			for _, pm := range prompts {
				got := pm["output_path"].(string)
				assert.True(t, filepath.IsAbs(got), "the path is absolute: %s", got)
				assert.Equal(t, "round-1-"+pm["role"].(string)+".ndjson", filepath.Base(got))
				assert.Equal(t, realPath(t, inbox), realPath(t, filepath.Dir(got)), "the role file is in the named inbox")
				assert.Contains(t, pm["prompt"].(string), "Write this round's findings to: "+got)
			}
		})
	}
}
