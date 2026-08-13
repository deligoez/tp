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

// roleContractDoc is the document that owns the audit routing contract and the
// role-corpus rules. Until v0.34.0 §8.1 the same wording was required in all
// four repo-root documents; §8.1 narrowed that to SKILL.md and REFERENCE.md,
// and the v0.34.0 audit found the pair still duplicated the rules verbatim.
// Giving each fact one home leaves SKILL.md, the document an agent already has
// loaded when it spawns a round, as the owner: REFERENCE.md keeps the
// exhaustive schema detail around them, and README.md's one-time index and
// CLAUDE.md's repo conventions point at the skill. Asserting the wording
// anywhere else would re-create the duplication §8.1 removes. Both guards read
// this constant, so the owner is named once rather than per guard.
const roleContractDoc = "skills/tp/SKILL.md"

// readRepoDoc reads a repo-root-relative document and fails the test when it is
// missing, so a documentation guard does not have to close over its own
// repoRoot result.
func readRepoDoc(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(rel)))
	require.NoError(t, err, "%s must exist at the repo root", rel)
	return string(data)
}
