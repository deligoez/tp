// testhelpers_test.go holds helpers shared by the internal (package cli) tests.
// Anything used by more than one guard test belongs here rather than in
// whichever test file happened to need it first.

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// repoRoot walks up from the test's working directory to the module root (the
// directory holding go.mod), so a guard test can read a repo-root file such as
// .golangci.yml or README.md regardless of which package directory it runs in.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, parent, dir, "reached the filesystem root without finding go.mod")
		dir = parent
	}
}
