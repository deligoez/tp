package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// snapshotUnits builds the units of a slice over epics, as an emission does.
func snapshotUnits(epics ...model.Epic) []EpicReviewUnit {
	byID := map[string]*model.Epic{}
	for i := range epics {
		byID[epics[i].ID] = &epics[i]
	}
	units := make([]EpicReviewUnit, 0, len(epics))
	for i := range epics {
		units = append(units, epicUnit(&epics[i], byID))
	}
	return units
}

// TestEpicSnapshotDiff_ListsUnitEdgeWhyAndFieldChanges is spec/1.8.0.md §11's
// difference and §18 row 9's diff: between two recorded rounds it lists the
// units added and removed, the edges added and removed, and each why and
// field changed with both texts. A diff over the hashes alone sees only that
// b changed, and turns it red.
func TestEpicSnapshotDiff_ListsUnitEdgeWhyAndFieldChanges(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	round1 := snapshotUnits(
		model.Epic{ID: "a", Title: "A", Milestone: "M"},
		model.Epic{ID: "b", Title: "B", Milestone: "M", DependsOn: []model.EpicDep{
			{ID: "a", Why: "b reads what a writes"},
			{ID: "c", Why: "b needs c's schema"},
		}},
		model.Epic{ID: "c", Title: "C", Milestone: "M"},
	)
	round2 := snapshotUnits(
		model.Epic{ID: "a", Title: "A", Milestone: "M"},
		model.Epic{ID: "b", Title: "B renamed", Milestone: "M", DependsOn: []model.EpicDep{
			{ID: "a", Why: "b replays a's journal"},
			{ID: "d", Why: "b ships after d"},
		}},
		model.Epic{ID: "d", Title: "D", Milestone: "M"},
	)
	require.NoError(t, WriteEpicReviewSnapshot(dir, 1, round1))
	require.NoError(t, WriteEpicReviewSnapshot(dir, 2, round2))

	diff, err := EpicReviewRoundsDiff(dir, 1, 2)
	require.NoError(t, err)

	assert.False(t, diff.HashesOnly)
	assert.Equal(t, []string{"epic:d"}, diff.UnitsAdded)
	assert.Equal(t, []string{"epic:c"}, diff.UnitsRemoved)
	assert.Equal(t, []EpicEdgeChange{{Unit: "epic:b", To: "d", Why: "b ships after d"}}, diff.EdgesAdded)
	assert.Equal(t, []EpicEdgeChange{{Unit: "epic:b", To: "c", Why: "b needs c's schema"}}, diff.EdgesRemoved)
	assert.Equal(t, []EpicWhyChange{{Unit: "epic:b", To: "a", Before: "b reads what a writes", After: "b replays a's journal"}}, diff.WhysChanged)
	assert.Equal(t, []EpicFieldChange{{Unit: "epic:b", Field: "title", Before: "B", After: "B renamed"}}, diff.FieldsChanged)
}

// TestEpicSnapshotDiff_AWhyEditIsAWhyChangeWithBothTexts is §18 row 9 alone:
// a why edited between two recorded rounds, nothing else, reads as one why
// change carrying both texts and no other change.
func TestEpicSnapshotDiff_AWhyEditIsAWhyChangeWithBothTexts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := model.Epic{ID: "a", Title: "A", Milestone: "M"}
	before := model.Epic{ID: "b", Title: "B", Milestone: "M", DependsOn: []model.EpicDep{{ID: "a", Why: "old why"}}}
	after := before
	after.DependsOn = []model.EpicDep{{ID: "a", Why: "new why"}}
	require.NoError(t, WriteEpicReviewSnapshot(dir, 1, snapshotUnits(a, before)))
	require.NoError(t, WriteEpicReviewSnapshot(dir, 2, snapshotUnits(a, after)))

	diff, err := EpicReviewRoundsDiff(dir, 1, 2)
	require.NoError(t, err)

	assert.Equal(t, []EpicWhyChange{{Unit: "epic:b", To: "a", Before: "old why", After: "new why"}}, diff.WhysChanged)
	assert.Empty(t, diff.EdgesAdded)
	assert.Empty(t, diff.EdgesRemoved)
	assert.Empty(t, diff.FieldsChanged)
	assert.Empty(t, diff.UnitsAdded)
	assert.Empty(t, diff.UnitsRemoved)
}

// TestEpicSnapshotDiff_AHashOnlySnapshotCarriesNoTextAndSaysSo is the
// pre-block snapshot: a round snapshotted before blocks were kept holds only
// hashes, so its diff names the units added, removed and changed, carries no
// edge, why or field text, and sets HashesOnly.
func TestEpicSnapshotDiff_AHashOnlySnapshotCarriesNoTextAndSaysSo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, epicSnapshotFile(1)),
		[]byte(`{"round": 1, "units": {"epic:a": "h1", "epic:b": "h2", "epic:c": "h3"}}`+"\n"), 0o600))
	require.NoError(t, WriteEpicReviewSnapshot(dir, 2, snapshotUnits(
		model.Epic{ID: "a", Title: "A", Milestone: "M"},
		model.Epic{ID: "b", Title: "B", Milestone: "M", DependsOn: []model.EpicDep{{ID: "a", Why: "why"}}},
		model.Epic{ID: "d", Title: "D", Milestone: "M"},
	)))

	diff, err := EpicReviewRoundsDiff(dir, 1, 2)
	require.NoError(t, err)

	assert.True(t, diff.HashesOnly)
	assert.Equal(t, []string{"epic:d"}, diff.UnitsAdded)
	assert.Equal(t, []string{"epic:c"}, diff.UnitsRemoved)
	assert.Equal(t, []string{"epic:a", "epic:b"}, diff.UnitsChanged)
	assert.Empty(t, diff.EdgesAdded)
	assert.Empty(t, diff.EdgesRemoved)
	assert.Empty(t, diff.WhysChanged)
	assert.Empty(t, diff.FieldsChanged)
}

// TestEpicSnapshotDiff_AFieldAddedReadsAsAChangeFromEmpty: round 2's block of
// a carries an acceptance round 1's did not, so the later block holds more
// fields than the earlier; the diff lists the acceptance as changed from "".
func TestEpicSnapshotDiff_AFieldAddedReadsAsAChangeFromEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, WriteEpicReviewSnapshot(dir, 1, snapshotUnits(model.Epic{ID: "a", Title: "A", Milestone: "M"})))
	require.NoError(t, WriteEpicReviewSnapshot(dir, 2, snapshotUnits(model.Epic{ID: "a", Title: "A", Milestone: "M", Acceptance: "a ships"})))

	diff, err := EpicReviewRoundsDiff(dir, 1, 2)
	require.NoError(t, err)
	assert.Equal(t, []EpicFieldChange{{Unit: "epic:a", Field: "acceptance", Before: "", After: "a ships"}}, diff.FieldsChanged)
}
