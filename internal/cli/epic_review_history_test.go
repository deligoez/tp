package cli_test

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicHistoryDiff is the part of a --history difference these tests read.
type epicHistoryDiff struct {
	UnitsChanged []string `json:"units_changed"`
	WhysChanged  []struct {
		Unit   string `json:"unit"`
		To     string `json:"to"`
		Before string `json:"before"`
		After  string `json:"after"`
	} `json:"whys_changed"`
	FieldsChanged []struct {
		Unit   string `json:"unit"`
		Field  string `json:"field"`
		Before string `json:"before"`
		After  string `json:"after"`
	} `json:"fields_changed"`
	EdgesAdded []json.RawMessage `json:"edges_added"`
	Unchanged  *[]string         `json:"unchanged"`
	HashesOnly bool              `json:"hashes_only"`
}

// epicHistoryPayload is tp epic review <slice> --history's payload as these
// tests read it.
type epicHistoryPayload struct {
	StateDir string `json:"state_dir"`
	Rounds   []struct {
		Round          int                         `json:"round"`
		FindingsByRole map[string][]map[string]any `json:"findings_by_role"`
		DiffFrom       *int                        `json:"diff_from"`
		Diff           *epicHistoryDiff            `json:"diff"`
	} `json:"rounds"`
	Current *struct {
		FromRound int             `json:"from_round"`
		Diff      epicHistoryDiff `json:"diff"`
	} `json:"current"`
}

// snapshotTree maps every file under dir to its content.
func snapshotTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	tree := map[string]string{}
	require.NoError(t, filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		tree[path] = string(data)
		return err
	}))
	return tree
}

// stripSnapshotBlocks rewrites round's snapshot of the only slice under dir
// as one written before blocks were kept: its hashes alone.
func stripSnapshotBlocks(t *testing.T, dir string, round int) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, ".tp", "epic-review", "*", fmt.Sprintf("snapshot-round-%d.json", round)))
	require.NoError(t, err)
	require.Len(t, paths, 1)
	data, err := os.ReadFile(paths[0])
	require.NoError(t, err)
	var snap map[string]any
	require.NoError(t, json.Unmarshal(data, &snap))
	delete(snap, "blocks")
	data, err = json.Marshal(snap)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(paths[0], data, 0o600))
}

// epicHistory runs tp epic review with args and decodes its payload.
func epicHistory(t *testing.T, dir string, args ...string) epicHistoryPayload {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, append([]string{"epic", "review"}, args...)...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out epicHistoryPayload
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	return out
}

// twoRoundSlice records two rounds of the milestone A slice of epicChain,
// editing b's why of b -> c between them and a's title after the second, so
// the history has a why change between the rounds and a field change to the
// current graph. Round 1 holds one finding by a role and one by none.
func twoRoundSlice(t *testing.T) (dir string, slice []string) {
	t.Helper()
	dir = epicGraphDir(t, epicChain)
	slice = []string{"--milestone", "A"}
	runEpicReview(t, dir, slice...)
	recordEpicRound(t, dir, `{"role": "why-auditor", "severity": "low", "finding": "thin why", "location": "epic:b", "evidence": "b needs c"}
{"severity": "low", "finding": "no role", "location": "epic:a", "evidence": "a"}
`, slice...)
	editEpicGraph(t, dir, `"b needs c"`, `"b reads c's schema"`)
	runEpicReview(t, dir, slice...)
	recordEpicRound(t, dir, "", slice...)
	editEpicGraph(t, dir, `"title": "A"`, `"title": "A renamed"`)
	return dir, slice
}

// TestEpicReviewHistory_PrintsEachRoundAndTheCurrentDifference is §11's
// --history: each recorded round in order with its findings by role and its
// difference from the round before — the why of b -> c rewritten between
// rounds 1 and 2, with both texts — then the difference from the last round
// to the current graph, a's title.
func TestEpicReviewHistory_PrintsEachRoundAndTheCurrentDifference(t *testing.T) {
	t.Parallel()
	dir, slice := twoRoundSlice(t)
	h := epicHistory(t, dir, append(slice, "--history")...)

	require.Len(t, h.Rounds, 2)
	assert.Equal(t, 1, h.Rounds[0].Round)
	assert.Nil(t, h.Rounds[0].DiffFrom, "round 1 has no round before")
	assert.Nil(t, h.Rounds[0].Diff)
	require.Len(t, h.Rounds[0].FindingsByRole["why-auditor"], 1)
	assert.Equal(t, "thin why", h.Rounds[0].FindingsByRole["why-auditor"][0]["finding"])
	require.Len(t, h.Rounds[0].FindingsByRole["unattributed"], 1)

	r2 := h.Rounds[1]
	assert.Equal(t, 2, r2.Round)
	require.NotNil(t, r2.DiffFrom)
	assert.Equal(t, 1, *r2.DiffFrom)
	require.NotNil(t, r2.Diff)
	require.Len(t, r2.Diff.WhysChanged, 1)
	assert.Equal(t, "epic:b", r2.Diff.WhysChanged[0].Unit)
	assert.Equal(t, "c", r2.Diff.WhysChanged[0].To)
	assert.Equal(t, "b needs c", r2.Diff.WhysChanged[0].Before)
	assert.Equal(t, "b reads c's schema", r2.Diff.WhysChanged[0].After)
	require.NotNil(t, r2.Diff.Unchanged, "without --compact the unchanged context is kept")
	assert.Contains(t, *r2.Diff.Unchanged, "epic:d")
	assert.Equal(t, map[string][]map[string]any{}, r2.FindingsByRole, "a clean round has no finding")

	require.NotNil(t, h.Current)
	assert.Equal(t, 2, h.Current.FromRound)
	require.Len(t, h.Current.Diff.FieldsChanged, 1)
	assert.Equal(t, "title", h.Current.Diff.FieldsChanged[0].Field)
	assert.Equal(t, "A renamed", h.Current.Diff.FieldsChanged[0].After)
}

// TestEpicReviewHistory_FromToNarrowsToTwoRounds is §11's --from <a> --to <b>:
// the payload holds those two rounds, the second's difference taken from the
// first, and no difference to the current graph.
func TestEpicReviewHistory_FromToNarrowsToTwoRounds(t *testing.T) {
	t.Parallel()
	dir, slice := twoRoundSlice(t)
	h := epicHistory(t, dir, append(slice, "--history", "--from", "1", "--to", "2")...)

	require.Len(t, h.Rounds, 2)
	assert.Equal(t, 1, h.Rounds[0].Round)
	assert.Nil(t, h.Rounds[0].Diff)
	require.NotNil(t, h.Rounds[1].Diff)
	require.Len(t, h.Rounds[1].Diff.WhysChanged, 1)
	assert.Nil(t, h.Current, "a narrowed history leaves the current graph out")

	_, stderr, code := runTP(t, dir, append([]string{"epic", "review"}, append(slice, "--history", "--from", "1", "--to", "5")...)...)
	assert.Equal(t, 4, code, "a round the slice did not record is refused; stderr: %s", stderr)
}

// TestEpicReviewHistory_CompactKeepsEveryChangeAndDropsTheContext is §11's
// --compact: the unchanged context goes, every change stays.
func TestEpicReviewHistory_CompactKeepsEveryChangeAndDropsTheContext(t *testing.T) {
	t.Parallel()
	dir, slice := twoRoundSlice(t)
	h := epicHistory(t, dir, append(slice, "--history", "--compact")...)

	require.Len(t, h.Rounds, 2)
	require.NotNil(t, h.Rounds[1].Diff)
	assert.Nil(t, h.Rounds[1].Diff.Unchanged)
	require.Len(t, h.Rounds[1].Diff.WhysChanged, 1)
	require.NotNil(t, h.Current)
	require.Len(t, h.Current.Diff.FieldsChanged, 1)
}

// TestEpicReviewHistory_ReadOnlyUnattendedAndExits3WithoutState is §11's
// contract: it runs under TP_UNATTENDED, writes nothing to the slice's state,
// and a slice with no state directory exits 3.
func TestEpicReviewHistory_ReadOnlyUnattendedAndExits3WithoutState(t *testing.T) {
	t.Parallel()
	dir, slice := twoRoundSlice(t)
	before := snapshotTree(t, dir)
	stdout, stderr, code := runTPFence(t, dir, true, append([]string{"epic", "review"}, append(slice, "--history")...)...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Contains(t, stdout, `"rounds"`)
	assert.Equal(t, before, snapshotTree(t, dir), "--history writes nothing")

	fresh := epicGraphDir(t, epicChain)
	_, stderr, code = runTP(t, fresh, "epic", "review", "--milestone", "A", "--history")
	require.Equal(t, 3, code, "stderr: %s", stderr)
}

