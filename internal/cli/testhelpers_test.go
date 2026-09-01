// testhelpers_test.go holds helpers shared by the internal (package cli) tests.
// Anything used by more than one guard test belongs here rather than in
// whichever test file happened to need it first.

package cli

import (
	"os"
	"path/filepath"
	"strings"
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

// relocatedSpec copies a repo-root-relative spec into the test's own temporary
// directory and returns the copy's path, so a test can emit from one of this
// repository's real specs without writing the repository's review state
// (v0.36.0 §6.1).
//
// The copy is what makes it work rather than a flag: engine.ReviewStateDir
// derives <spec-dir>/.tp-review/<spec-base> from the spec's own path and
// nothing overrides it, so moving the spec moves the state with it. Any
// existing .tp-review/<base> is copied alongside, because a test that needs a
// recorded round needs the rounds that produced it — an empty state directory
// silently changes the panel (`regression` is appended only from round 2).
//
// --no-state is deliberately not used here or anywhere in this suite: it
// disables review-state reads as well as writes, and that state is the subject.
func relocatedSpec(t *testing.T, rel string) string {
	t.Helper()
	root := repoRoot(t)
	src := filepath.Join(root, filepath.FromSlash(rel))
	base := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))

	dir := t.TempDir()
	dst := filepath.Join(dir, filepath.Base(src))
	body, err := os.ReadFile(src)
	require.NoError(t, err, "%s must exist at the repo root", rel)
	require.NoError(t, os.WriteFile(dst, body, 0o600))

	srcState := filepath.Join(filepath.Dir(src), ".tp-review", base)
	if _, statErr := os.Stat(srcState); statErr == nil {
		copyTree(t, srcState, filepath.Join(dir, ".tp-review", base))
	}
	return dst
}

// copyTree copies a directory's regular files one level deep, which is the
// shape a spec's .tp-review/<base> has: round files and snapshots, no
// subdirectories.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dst, 0o750))
	entries, err := os.ReadDir(src)
	require.NoError(t, err)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		body, readErr := os.ReadFile(filepath.Join(src, e.Name()))
		require.NoError(t, readErr)
		require.NoError(t, os.WriteFile(filepath.Join(dst, e.Name()), body, 0o600))
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
