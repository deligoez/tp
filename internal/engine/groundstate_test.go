package engine

import (
	"os"
	"path/filepath"
	"slices"
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

// groundRoundFixture writes the named files into the review state directory of
// a fresh spec and returns the spec's path. The files hold no meaningful
// content: numbering is a filename question (§7.3), so a fixture that had to
// write valid rounds would be testing something else.
func groundRoundFixture(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))

	stateDir := ReviewStateDir(specPath)
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	for _, name := range names {
		require.NoError(t, os.WriteFile(filepath.Join(stateDir, name), []byte("{}\n"), 0o600))
	}
	return specPath
}

// TestNextGroundRoundPreservesAGap is §11 row 13. A directory holding rounds 1
// and 3 yields 4, never 2 and never 3: the missing round is a deleted or lost
// artifact, and refilling its number would make two different rounds share an
// identifier.
//
// The fixture's discriminating property is asserted rather than assumed. Two
// round files plus one is 3, so this fixture separates the shipped rule from
// the count-plus-one mutant — a fixture of rounds 1 and 2 would score both
// readings identically and prove nothing.
func TestNextGroundRoundPreservesAGap(t *testing.T) {
	names := []string{"ground-round-1.ndjson", "ground-round-3.ndjson"}
	require.NotEqual(t, 4, len(names)+1,
		"the fixture must separate highest-plus-one from count-plus-one")

	n, err := NextGroundRound(groundRoundFixture(t, names...))
	require.NoError(t, err)
	assert.Equal(t, 4, n)
}

// TestNextGroundRoundOnASpecWithNoStateDirectoryIsOne is the first run of the
// ordering this release advocates (§11 row 12): grounding precedes review, so
// the first `tp ground` on a spec meets no state directory at all.
func TestNextGroundRoundOnASpecWithNoStateDirectoryIsOne(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))
	require.NoDirExists(t, ReviewStateDir(specPath),
		"the fixture is only a first run while the directory is absent")

	n, err := NextGroundRound(specPath)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
}

// TestNextGroundRoundComparesNumbersNotFilenames pins that the highest round is
// found by integer comparison. os.ReadDir returns entries sorted by filename
// and "ground-round-10" sorts before "ground-round-2", so an implementation
// that keeps the last match it walks past answers 3 here and collides with the
// existing round 10. The sort order is the property that makes this fixture
// discriminating, so it is asserted.
func TestNextGroundRoundComparesNumbersNotFilenames(t *testing.T) {
	names := []string{"ground-round-2.ndjson", "ground-round-10.ndjson"}
	sorted := slices.Clone(names)
	slices.Sort(sorted)
	require.Equal(t, "ground-round-10.ndjson", sorted[0],
		"the fixture only discriminates while 10 sorts before 2")

	n, err := NextGroundRound(groundRoundFixture(t, names...))
	require.NoError(t, err)
	assert.Equal(t, 11, n)
}

// TestNextGroundRoundIgnoresACrashLeftoverTemp: a crash mid-record leaves
// writeFileAtomic's unique sibling temp behind — "<name>.<random>.tmp" — and
// that round was never recorded. Counting it would skip a number, which is the
// same reason hasAnyPrefixed skips .tmp.
func TestNextGroundRoundIgnoresACrashLeftoverTemp(t *testing.T) {
	specPath := groundRoundFixture(t,
		"ground-round-1.ndjson",
		"ground-round-2.ndjson.2841913.tmp")

	n, err := NextGroundRound(specPath)
	require.NoError(t, err)
	assert.Equal(t, 2, n, "an unfinished write is not a recorded round")
}
