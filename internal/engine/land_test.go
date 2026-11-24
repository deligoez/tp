package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tp land refuses a task file of another project, so a project below the
// worktree root is entered first.
func TestLandCommandEntersAProjectBelowTheWorktreeRoot(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, "proj", ".tp"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".tp"), 0o750))

	assert.Equal(t, "(cd proj && tp land --file spec.tasks.json)", landCommand(home, "proj/spec.tasks.json"))
	assert.Equal(t, "tp land --file spec.tasks.json", landCommand(home, "spec.tasks.json"))
}

// A hint run from the repository root names a task file by its path from
// there, also for a project below it.
func TestRepoRelativePathIsFromTheWorktreeRoot(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	cmd := exec.Command("git", "init", "-q", home)
	require.NoError(t, cmd.Run())
	require.NoError(t, os.MkdirAll(filepath.Join(home, "proj", ".tp"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(home, "proj", "spec.tasks.json"), []byte("{}"), 0o600))

	assert.Equal(t, "proj/spec.tasks.json", RepoRelativePath(filepath.Join(home, "proj", "spec.tasks.json")))
}

// tp land removes only the request it read: one the agent wrote while the
// landing ran stays for the next tp land.
func TestRemovingALandRequestKeepsOneWrittenSince(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(at string) {
		t.Helper()
		body := `{"id":"t1","reason":"r","commits":["abc"],"worktree":"/w","at":"` + at + `"}`
		require.NoError(t, os.WriteFile(filepath.Join(dir, "t1.json"), []byte(body), 0o600))
	}
	write("1")
	read := LandRequest{ID: "t1", At: "1"}
	write("2")

	require.NoError(t, RemoveLandRequest(dir, &read))
	assert.FileExists(t, filepath.Join(dir, "t1.json"), "the later request is kept")

	later := LandRequest{ID: "t1", At: "2"}
	require.NoError(t, RemoveLandRequest(dir, &later))
	assert.NoFileExists(t, filepath.Join(dir, "t1.json"))
}
