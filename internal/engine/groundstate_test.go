package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// groundArtifactNames are the three files a ground round writes, named by
// §7.3: emit writes the snapshot and the floor it derived from it, --record
// writes the round file beside them. The tests below assert on these names
// rather than on the prefix strings, so a prefix list that stops matching a
// real artifact fails even when its literals still read correctly.
var groundArtifactNames = []string{
	"snapshot-ground-round-1.md",
	"floor-ground-round-1.txt",
	"ground-round-1.ndjson",
}

// TestGroundArtifactsAreTheirOwnPrefixList guards §7.3: ground artifacts go in
// groundFilePrefixes and join neither existing list. The assertion is made
// through hasAnyPrefixed on a real filename rather than by restating the three
// lists, so it also fails on any *other* entry that would make a review or
// audit predicate match a ground artifact — "ground-" in roundFilePrefixes,
// say, or "snapshot-ground-" in snapshotPrefixes. Those are the additions the
// acceptance forbids; an addition matching nothing of ground's is not.
func TestGroundArtifactsAreTheirOwnPrefixList(t *testing.T) {
	for _, name := range groundArtifactNames {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600))

			assert.True(t, hasAnyPrefixed(dir, groundFilePrefixes),
				"groundFilePrefixes must match the ground artifact %s", name)
			assert.False(t, hasAnyPrefixed(dir, roundFilePrefixes),
				"roundFilePrefixes must not match the ground artifact %s", name)
			assert.False(t, hasAnyPrefixed(dir, snapshotPrefixes),
				"snapshotPrefixes must not match the ground artifact %s", name)
		})
	}
}

// TestGroundOnlyStateDirIsInvisibleToReviewState is §11 row 11. A ground-only
// state directory — the ground round and its emitted artifacts, no state.json —
// is the state §7.3 mandates, and grounding precedes review, so it is what
// every spec looks like before its first review round. If either predicate saw
// it, LoadReviewState would find state artifacts with no index and return
// StateCorruptError{MissingIndex: true}, and every command that loads state for
// that spec would abort.
func TestGroundOnlyStateDirIsInvisibleToReviewState(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))

	stateDir := ReviewStateDir(specPath)
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	for _, name := range groundArtifactNames {
		require.NoError(t, os.WriteFile(filepath.Join(stateDir, name), []byte("x"), 0o600))
	}
	require.NoFileExists(t, filepath.Join(stateDir, stateFileName),
		"the fixture is only ground-only if it holds no index")

	assert.False(t, hasRecordedRoundFiles(stateDir),
		"a ground round is not a recorded review or audit round")
	assert.False(t, hasStateArtifacts(stateDir),
		"a ground-only directory holds no review state artifact")

	st, err := LoadReviewState(specPath)
	require.NoError(t, err, "a ground-only directory must not read as corrupt state")
	assert.Nil(t, st, "no state.json means no state, not an empty one")
}
