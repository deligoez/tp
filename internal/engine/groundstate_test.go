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

