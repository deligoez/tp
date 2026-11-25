package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
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

// TestLandUndo_AbortsAPickLeftInProgress: a landing taken back over a pick
// still stopped on a conflict aborts it first, since reset --keep refuses an
// unmerged index; the branch is back at pre with no cherry-pick in progress.
func TestLandUndo_AbortsAPickLeftInProgress(t *testing.T) {
	root, _, _, _ := batchProject(t, wipAlpha)
	pre := conflictingPickInProgress(t, root)

	require.NoError(t, LandUndo(root, pre))
	assert.Equal(t, pre, gitIn(t, root, "rev-parse", "HEAD"))
	op, _ := gitOperationInProgress(root)
	assert.Empty(t, op)
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

// A request tp land cannot remove is reported, not passed over as removed.
func TestRemovingALandRequestReportsAFailedRemove(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	body := `{"id":"t1","reason":"r","commits":["abc"],"worktree":"/w","at":"1"}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "t1.json"), []byte(body), 0o600))
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	assert.Error(t, RemoveLandRequest(dir, &LandRequest{ID: "t1", At: "1"}))
}

// A request is made under a claim only with that claim's spawn and claim
// token, each where the request carries one: the same spawn rebound under
// another brief's token does not land the old request.
func TestALandRequestIsMadeUnderItsSpawnAndToken(t *testing.T) {
	t.Parallel()
	claim := &model.ClaimedBy{Spawn: "s1", Token: "tokA"}
	assert.True(t, (&LandRequest{Spawn: "s1", Token: "tokA"}).MadeUnder(claim))
	assert.True(t, (&LandRequest{Spawn: "s1"}).MadeUnder(claim), "a request without a token")
	assert.True(t, (&LandRequest{}).MadeUnder(nil), "no claim, no stamp")
	assert.False(t, (&LandRequest{Spawn: "s1", Token: "tokB"}).MadeUnder(claim), "another token")
	assert.False(t, (&LandRequest{Spawn: "s2", Token: "tokA"}).MadeUnder(claim), "another spawn")
	assert.False(t, (&LandRequest{Spawn: "s1"}).MadeUnder(nil), "a claim gone")
}
