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

// TestEpicEmissionChanges_NamesEachKindOfChange compares a stored emission
// with a rebuilt one, one kind of change per case: a role whose file hash
// changed under a stored panel, a block that changed under an unchanged
// hash beside one that did not, a single milestone's purpose, and a single
// unit added. Each case is the smallest that a mutant of its comparison
// (`old.Roles != nil`, `before != now.Blocks[k]`, `len(p) > 0`, `len(l) > 0`)
// turns red.
func TestEpicEmissionChanges_NamesEachKindOfChange(t *testing.T) {
	t.Parallel()
	unit := func(id, hash, block, purpose string) EpicReviewUnit {
		return EpicReviewUnit{Key: "epic:" + id, Hash: hash, Block: block, Milestone: "m0", Purpose: purpose}
	}
	role := func(hash string) []EpicSnapshotRole {
		return []EpicSnapshotRole{{ID: "r1", Hash: hash, OutputPath: "inbox/r1-r1.ndjson"}}
	}
	cases := map[string]struct {
		oldUnits, newUnits []EpicReviewUnit
		oldRoles, newRoles []EpicSnapshotRole
		want               []string
	}{
		"a role file changed under a stored panel": {
			oldUnits: []EpicReviewUnit{unit("a1", "h", "epic: a1\n", "")},
			newUnits: []EpicReviewUnit{unit("a1", "h", "epic: a1\n", "")},
			oldRoles: role("sha256:old"), newRoles: role("sha256:new"),
			want: []string{"role files changed: r1"},
		},
		"a block changed under the same hash": {
			oldUnits: []EpicReviewUnit{unit("a1", "h", "epic: a1\n", ""), unit("b1", "g", "epic: b1\n", "")},
			newUnits: []EpicReviewUnit{unit("a1", "h", "epic: a1\ntitle: renamed\n", ""), unit("b1", "g", "epic: b1\n", "")},
			want:     []string{"units changed: epic:a1"},
		},
		"one milestone's purpose changed": {
			oldUnits: []EpicReviewUnit{unit("a1", "h", "epic: a1\n", "ship it")},
			newUnits: []EpicReviewUnit{unit("a1", "h", "epic: a1\n", "ship it twice")},
			want:     []string{"purposes changed: m0"},
		},
		"one unit added": {
			oldUnits: []EpicReviewUnit{unit("a1", "h", "epic: a1\n", "")},
			newUnits: []EpicReviewUnit{unit("a1", "h", "epic: a1\n", ""), unit("b1", "g", "epic: b1\n", "")},
			want:     []string{"units added: epic:b1"},
		},
	}
	for name, c := range cases {
		dir := t.TempDir()
		require.NoError(t, WriteEpicReviewSnapshot(dir, 1, c.oldUnits, c.oldRoles), name)
		changes, err := EpicEmissionChanges(dir, 1, c.newUnits, c.newRoles)
		require.NoError(t, err, name)
		assert.Equal(t, c.want, changes, name)
	}
}
