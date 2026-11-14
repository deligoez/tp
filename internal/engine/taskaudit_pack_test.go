package engine

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// files returns n distinct paths under dir.
func files(dir string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("%s/f%02d.go", dir, i)
	}
	return out
}

func cand(id, closed string, fs ...string) TaskAuditCandidate {
	return TaskAuditCandidate{ID: id, ClosedAt: closed, Roles: []string{"go-safety"}, Files: fs}
}

// batchIDs is each batch's member ids, piece-qualified when a task is cut.
func batchIDs(bs []TaskAuditBatch) [][]string {
	out := make([][]string, len(bs))
	for i, b := range bs {
		for _, m := range b.Members {
			id := m.ID
			if m.Pieces > 1 {
				id = fmt.Sprintf("%s#%d", id, m.Piece)
			}
			out[i] = append(out[i], id)
		}
	}
	return out
}

// TestPackTaskAudits_PacksInCloseOrderUpToTenFiles: tasks join a batch in
// close order until the next would push it past ten files.
func TestPackTaskAudits_PacksInCloseOrderUpToTenFiles(t *testing.T) {
	t.Parallel()
	cs := []TaskAuditCandidate{
		cand("c", "2026-01-03T00:00:00Z", files("c", 4)...),
		cand("a", "2026-01-01T00:00:00Z", files("a", 3)...),
		cand("b", "2026-01-02T00:00:00Z", files("b", 3)...),
		cand("d", "2026-01-04T00:00:00Z", files("d", 2)...),
	}
	bs := PackTaskAudits(cs)
	assert.Equal(t, [][]string{{"a", "b", "c"}, {"d"}}, batchIDs(bs))
	assert.Len(t, bs[0].Files(), 10)
	assert.True(t, bs[0].Full, "exactly ten files is full")
	assert.False(t, bs[1].Full)
}

// TestPackTaskAudits_OverflowClosesTheBatch: a batch the next task would
// overflow is full even below ten files.
func TestPackTaskAudits_OverflowClosesTheBatch(t *testing.T) {
	t.Parallel()
	bs := PackTaskAudits([]TaskAuditCandidate{
		cand("a", "2026-01-01T00:00:00Z", files("a", 6)...),
		cand("b", "2026-01-02T00:00:00Z", files("b", 5)...),
	})
	require.Equal(t, [][]string{{"a"}, {"b"}}, batchIDs(bs))
	assert.True(t, bs[0].Full)
	assert.False(t, bs[1].Full)
}

// TestPackTaskAudits_TieOnCloseTimeBreaksByID: equal close times order by
// task id.
func TestPackTaskAudits_TieOnCloseTimeBreaksByID(t *testing.T) {
	t.Parallel()
	bs := PackTaskAudits([]TaskAuditCandidate{
		cand("z", "2026-01-01T00:00:00Z", "z.go"),
		cand("m", "2026-01-01T00:00:00Z", "m.go"),
	})
	assert.Equal(t, [][]string{{"m", "z"}}, batchIDs(bs))
}

// TestPackTaskAudits_CutsALargeTaskIntoSortedSlices: a task over ten files
// is cut into sorted slices of ten, each its own batch.
func TestPackTaskAudits_CutsALargeTaskIntoSortedSlices(t *testing.T) {
	t.Parallel()
	big := files("big", 23)
	big[0], big[22] = big[22], big[0]
	bs := PackTaskAudits([]TaskAuditCandidate{
		cand("a", "2026-01-01T00:00:00Z", "a.go"),
		cand("big", "2026-01-02T00:00:00Z", big...),
		cand("c", "2026-01-03T00:00:00Z", "c.go"),
	})
	require.Equal(t, [][]string{{"a"}, {"big#0"}, {"big#1"}, {"big#2", "c"}}, batchIDs(bs))
	assert.Equal(t, []bool{true, true, true, false}, []bool{bs[0].Full, bs[1].Full, bs[2].Full, bs[3].Full})
	assert.Equal(t, files("big", 10), bs[1].Files(), "a slice is ten of the sorted paths")
	assert.Equal(t, append(files("big", 23)[20:], "c.go"), bs[3].Files())
}

// TestPackTaskAudits_IsPrefixStable: a later close never changes an earlier
// batch.
func TestPackTaskAudits_IsPrefixStable(t *testing.T) {
	t.Parallel()
	base := []TaskAuditCandidate{
		cand("a", "2026-01-01T00:00:00Z", files("a", 4)...),
		cand("b", "2026-01-02T00:00:00Z", files("b", 4)...),
		cand("c", "2026-01-03T00:00:00Z", files("c", 4)...),
	}
	before := PackTaskAudits(base)
	after := PackTaskAudits(append(base, cand("d", "2026-01-04T00:00:00Z", files("d", 9)...)))
	require.GreaterOrEqual(t, len(after), len(before))
	for i := range before {
		if before[i].Full {
			assert.Equal(t, before[i], after[i], "full batch %d changed", i)
		}
	}
	assert.Equal(t, batchIDs(before)[0], batchIDs(after)[0])
}

// TestPackTaskAudits_ATaskWithoutFilesStillJoinsABatch: a task whose close
// left no auditable file still owes its roles, so it joins a batch with no
// files of its own and that batch's record satisfies it.
func TestPackTaskAudits_ATaskWithoutFilesStillJoinsABatch(t *testing.T) {
	t.Parallel()
	bs := PackTaskAudits([]TaskAuditCandidate{cand("a", "2026-01-01T00:00:00Z"), cand("b", "2026-01-02T00:00:00Z", "b.go")})
	assert.Equal(t, [][]string{{"a", "b"}}, batchIDs(bs))
	assert.Equal(t, []string{"b.go"}, bs[0].Files())
}
