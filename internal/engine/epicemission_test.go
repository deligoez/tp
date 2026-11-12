package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEpicEmissionChanges_CapsALongList: five units changed read as their
// count and the first three, so a large slice's refusal stays short.
func TestEpicEmissionChanges_CapsALongList(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	units := func(hash string) []EpicReviewUnit {
		var out []EpicReviewUnit
		for _, id := range []string{"u1", "u2", "u3", "u4", "u5"} {
			out = append(out, EpicReviewUnit{Key: "epic:" + id, Hash: hash + id, Block: "epic: " + id + "\n"})
		}
		return out
	}
	require.NoError(t, WriteEpicReviewSnapshot(dir, 1, units("a"), nil))
	changes, err := EpicEmissionChanges(dir, 1, units("b"), nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"units changed: 5 units (epic:u1, epic:u2, epic:u3, …)"}, changes)
}
