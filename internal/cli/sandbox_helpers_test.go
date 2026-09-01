// sandbox_helpers_test.go holds the spec sandbox for the external (package
// cli_test) tests — the ones that invoke the CLI and therefore need a spec
// whose review state lives outside the repository (v0.36.0 §6.1).
//
// Go test files in one directory cannot share helpers across the internal and
// external test packages, so the repo-root walk is written once here and once
// in testhelpers_test.go. That duplication is the language's, not a choice.

package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// repoRootDir walks up from the test's working directory to the module root.
func repoRootDir(t *testing.T) string {
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
// repository's real specs without writing the repository's review state.
//
// The copy is what makes it work rather than a flag: engine.ReviewStateDir
// derives <spec-dir>/.tp-review/<spec-base> from the spec's own path and
// nothing overrides it, so moving the spec moves the state with it. Any
// existing .tp-review/<base> is copied alongside, because a test that needs a
// recorded round needs the rounds that produced it — an empty state directory
// silently changes the panel, since `regression` is appended only from round 2.
//
// --no-state is deliberately not used, here or anywhere in this suite: it
// disables review-state reads as well as writes, and that state is the subject.
func relocatedSpec(t *testing.T, rel string) string {
	t.Helper()
	root := repoRootDir(t)
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

// relocatedSpecAtRoundOne copies a repo spec WITHOUT its review state, so the
// emission is round 1.
//
// Round 1 is the only round that skips the built-in `regression` role, with
// reason `no-baseline` — there is no snapshot-round-0.md to diff against. It is
// what makes §6 property 5's exit-0 case measurable in this repository at all:
// every spec here is many rounds in, and a later round skips nothing, so a
// guard written against the current state would skip itself and assert nothing.
func relocatedSpecAtRoundOne(t *testing.T, rel string) string {
	t.Helper()
	root := repoRootDir(t)
	src := filepath.Join(root, filepath.FromSlash(rel))

	dir := t.TempDir()
	dst := filepath.Join(dir, filepath.Base(src))
	body, err := os.ReadFile(src)
	require.NoError(t, err, "%s must exist at the repo root", rel)
	require.NoError(t, os.WriteFile(dst, body, 0o600))
	return dst
}
