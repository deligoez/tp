package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeBoundTaskFile writes a minimal task file at rel under root, bound to
// epic when epic is non-empty, and returns its absolute path.
func writeBoundTaskFile(t *testing.T, root, rel, epic string) string {
	t.Helper()
	path := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	body := `{"version":1,"spec":"s.md","tasks":[]}`
	if epic != "" {
		body = `{"version":1,"spec":"s.md","epic":"` + epic + `","tasks":[]}`
	}
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

// boundFilesRepo returns a fresh git repository's root with its symlinks
// resolved, the form git reports it in, so paths compare as equal on macOS
// where the temp directory sits behind /var -> /private/var.
func boundFilesRepo(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	gitInTempRepo(t, root, "init", "-q")
	return root
}

// TestEpicBoundFiles_TrackedAndUntrackedFoundIgnoredNot is spec/1.5.0.md §2:
// a bound file is found whether git tracks it or not, and one the ignore rules
// cover is not. A walk that read only tracked files, or ignored nothing, fails.
func TestEpicBoundFiles_TrackedAndUntrackedFoundIgnoredNot(t *testing.T) {
	root := boundFilesRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte("scratch/\n"), 0o600))
	tracked := writeBoundTaskFile(t, root, "spec/tracked.tasks.json", "b")
	gitInTempRepo(t, root, "add", ".gitignore", "spec/tracked.tasks.json")
	untracked := writeBoundTaskFile(t, root, "spec/nested/untracked.tasks.json", "b")
	writeBoundTaskFile(t, root, "scratch/ignored.tasks.json", "b")

	got, err := EpicBoundFiles(filepath.Join(root, "spec"))
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{tracked, untracked}, got["b"])
}

// TestEpicBoundFiles_IndexedByEpicUnboundInNone: each file lands under the
// epic it names and no other, and a file with no epic is in no epic's list —
// neither under its own key nor under the empty one.
func TestEpicBoundFiles_IndexedByEpicUnboundInNone(t *testing.T) {
	root := boundFilesRepo(t)
	a := writeBoundTaskFile(t, root, "a.tasks.json", "a")
	b1 := writeBoundTaskFile(t, root, "spec/b1.tasks.json", "b")
	b2 := writeBoundTaskFile(t, root, "spec/b2.tasks.json", "b")
	writeBoundTaskFile(t, root, "spec/free.tasks.json", "")

	got, err := EpicBoundFiles(root)
	require.NoError(t, err)

	assert.Equal(t, map[string][]string{"a": {a}, "b": {b1, b2}}, got)
}
