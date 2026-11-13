package engine

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The rows spec/backlog/carried-rows-block.md's decision lines are tested
// over: one finding at §2, open and later fixed, and a second finding at the
// same location.
const (
	carriedOpen    = `{"severity":"high","category":"clarity","location":"§2","finding":"Rule 1 is ambiguous"}`
	carriedFixed   = `{"severity":"high","category":"clarity","location":"§2","finding":"Rule 1 is ambiguous","resolved":{"status":"fixed","evidence":"reworded"}}`
	carriedOtherAt = `{"severity":"high","category":"clarity","location":"§2","finding":"Rule 2 contradicts rule 1"}`
)

// TestReviewCarried_AnUnresolvedHighBlocksPastEmptyRounds: the BUGS.md
// sequence at the engine, uncapped: two empty rounds after an open high leave
// the loop open, with the row named as what holds it.
func TestReviewCarried_AnUnresolvedHighBlocksPastEmptyRounds(t *testing.T) {
	t.Parallel()
	spec := filepath.Join(t.TempDir(), "spec.md")
	st := &ReviewState{ReviewRounds: loopRounds(t, spec, []string{carriedOpen}, []string{}, []string{})}

	d := ReviewLoopDone(spec, st, 2, 0, "h", ReviewConvergeOnBlocking)
	assert.False(t, d.Done)
	require.Len(t, d.CarriedBlocking, 1)
	assert.Equal(t, 1, d.CarriedBlocking[0].Round)
	assert.Equal(t, 0, d.CarriedBlocking[0].Index)
	assert.False(t, ReviewConverged(spec, st, 2, "h", ReviewConvergeOnBlocking))
}

