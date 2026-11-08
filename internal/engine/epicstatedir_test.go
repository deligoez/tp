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

