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

// repoRootDocs are the four repo-root documents the documentation guard tests
// hold to one story: each states part of the audit routing contract (§4), and
// each must carry the per-spec deactivation lever wording. Both guards read the
// same set, so the set lives here rather than being duplicated per guard.
var repoRootDocs = []string{
	"README.md",
	"skills/tp/SKILL.md",
	"skills/tp/REFERENCE.md",
	"CLAUDE.md",
}

// readRepoDoc reads a repo-root-relative document and fails the test when it is
// missing, so a documentation guard does not have to close over its own
// repoRoot result.
func readRepoDoc(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(rel)))
	require.NoError(t, err, "%s must exist at the repo root", rel)
	return string(data)
}
