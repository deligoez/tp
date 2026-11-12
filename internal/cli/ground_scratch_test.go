package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests below are spec/backlog/ground-scratch-in-state-dir.md: the file a
// ground unit writes lives in the spec's own state directory, so its identity
// comes from location rather than from a name two specs can share.

// TestSameBaseSpecsInTwoDirectoriesGetTwoScratchFiles is the collision the
// decision closes: x/a.md and y/a.md grounded from one working directory named
// one ground-a-r1.ndjson, and the second unit's rows overwrote the first's.
func TestSameBaseSpecsInTwoDirectoriesGetTwoScratchFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, sub := range []string{"x", "y"} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, sub), 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(dir, sub, "a.md"), []byte(groundFixtureSpec), 0o600))
	}

	pathX, promptX := groundEmitSpec(t, dir, filepath.Join("x", "a.md"))
	pathY, promptY := groundEmitSpec(t, dir, filepath.Join("y", "a.md"))
	assert.Equal(t, filepath.Join("x", ".tp-review", "a", "ground-inbox", "round-1.ndjson"), pathX,
		"the scratch file sits in the spec's own state directory, apart from the recorded rounds")
	assert.Equal(t, filepath.Join("y", ".tp-review", "a", "ground-inbox", "round-1.ndjson"), pathY)
	assert.Contains(t, promptX, "Write this round's rows to: "+pathX)
	assert.Contains(t, promptY, "Write this round's rows to: "+pathY)
}

// TestGroundRecordRemovesTheScratchFile: the in-flight file sits inside the
// repository now, so --record removes it once the round is on disk, and leaves
// the state directory holding the round's three artifacts and nothing else.
func TestGroundRecordRemovesTheScratchFile(t *testing.T) {
	t.Parallel()
	dir := writeGroundFixture(t)
	out := groundEmit(t, dir)
	scratch, ok := out["output_path"].(string)
	require.True(t, ok)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, filepath.Dir(scratch)), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, scratch),
		[]byte(groundRecordRow(1)+"\n"+groundRecordRow(2)+"\n"), 0o600))

	_, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", scratch)
	require.Equal(t, 0, code, "stderr: %s", stderr)

	assert.NoFileExists(t, filepath.Join(dir, scratch), "--record removes the scratch file it recorded")
	assert.Equal(t, []string{
		"floor-ground-round-1.txt",
		"ground-round-1.ndjson",
		"snapshot-ground-round-1.md",
	}, stateDirNames(t, dir), "and the emptied inbox with it")
}

// TestGroundRecordRefusesTheOldScratchPath: there is no fallback read of the
// old cwd-relative name, because that name is the one two specs share. A round
// whose rows sit there is refused, and the refusal names the file and where to
// move it, recording nothing.
func TestGroundRecordRefusesTheOldScratchPath(t *testing.T) {
	t.Parallel()
	dir := writeGroundFixture(t)
	out := groundEmit(t, dir)
	scratch, ok := out["output_path"].(string)
	require.True(t, ok)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ground-spec-r1.ndjson"),
		[]byte(groundRecordRow(1)+"\n"), 0o600))

	_, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", "ground-spec-r1.ndjson")
	assert.Equal(t, 3, code, "stderr: %s", stderr)
	envelope := groundErrorEnvelope(t, stderr)
	assert.Contains(t, envelope["error"], "ground-spec-r1.ndjson", "the refusal names the old file")
	assert.Contains(t, envelope["hint"], scratch, "and says where to move it")
	assert.NotContains(t, stateDirNames(t, dir), "ground-round-1.ndjson", "nothing is recorded")
}
