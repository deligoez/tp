package engine

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var lowerHex = regexp.MustCompile(`^[0-9a-f]+$`)

// TestEpicReviewStateDir_MilestoneAndEpicOfOneNameNeverShare is spec/1.3.0.md
// §9 row 14 with D13: --milestone xyz and --epic xyz get two distinct
// lowercase-hex directories under .tp/epic-review/, neither holding the id.
func TestEpicReviewStateDir_MilestoneAndEpicOfOneNameNeverShare(t *testing.T) {
	tpDir := filepath.Join(t.TempDir(), ".tp")

	milestone := EpicReviewStateDir(tpDir, EpicSliceKey{Milestone: "xyz"})
	epic := EpicReviewStateDir(tpDir, EpicSliceKey{Epic: "xyz"})

	assert.NotEqual(t, milestone, epic)
	for _, dir := range []string{milestone, epic} {
		assert.Equal(t, filepath.Join(tpDir, "epic-review"), filepath.Dir(dir))
		assert.Regexp(t, lowerHex, filepath.Base(dir))
		assert.NotContains(t, dir[len(tpDir):], "xyz")
	}
}

// TestEpicReviewStateDir_PathLikeIDStaysUnderTheStateRoot is §9 row 14's
// second graph: an epic id ../x never reaches the file system, so its
// directory is a direct child of .tp/epic-review/.
func TestEpicReviewStateDir_PathLikeIDStaysUnderTheStateRoot(t *testing.T) {
	tpDir := filepath.Join(t.TempDir(), ".tp")
	root := filepath.Join(tpDir, "epic-review")

	dir := EpicReviewStateDir(tpDir, EpicSliceKey{Epic: "../x"})

	rel, err := filepath.Rel(root, dir)
	require.NoError(t, err)
	assert.False(t, strings.Contains(rel, string(filepath.Separator)) || strings.HasPrefix(rel, ".."), "dir %s escapes %s", dir, root)
	assert.Regexp(t, lowerHex, rel)
}

// TestEpicReviewStateDir_TwoSlicesOfOneKindNeverShare: two epics, and two
// milestones, each get a directory of their own. A key text that dropped the
// value would give every slice of a kind one directory and mix their rounds.
func TestEpicReviewStateDir_TwoSlicesOfOneKindNeverShare(t *testing.T) {
	tpDir := filepath.Join(t.TempDir(), ".tp")
	assert.NotEqual(t, EpicReviewStateDir(tpDir, EpicSliceKey{Epic: "a"}), EpicReviewStateDir(tpDir, EpicSliceKey{Epic: "b"}))
	assert.NotEqual(t, EpicReviewStateDir(tpDir, EpicSliceKey{Milestone: "A"}), EpicReviewStateDir(tpDir, EpicSliceKey{Milestone: "B"}))
}
