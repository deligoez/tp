package engine

import (
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// deriveEpicSliceKey returns the key of a slice state directory that holds
// no key file, from the units its recorded rounds name (each row's location
// and each round's snapshot): the candidate key whose hash is the
// directory's name, so the answer is exact or nil. A named set's name is in
// no round, so such a slice stays nil. It reads only.
func deriveEpicSliceKey(stateDir string, g *model.EpicGraph) *EpicSliceKeyRecord {
	want := filepath.Base(stateDir)
	for _, key := range epicSliceKeyCandidates(g, epicRecordedUnitKeys(stateDir)) {
		if epicHash(epicSliceKeyText(key)) == want {
			rec := epicSliceKeyRecord(key)
			return &rec
		}
	}
	return nil
}

// epicRecordedUnitKeys returns the distinct unit keys the slice's recorded
// rounds name, sorted: their rows' locations and their snapshots' units. An
// unreadable file names none.
func epicRecordedUnitKeys(stateDir string) []string {
	seen := map[string]bool{}
	for r := 1; r < EpicReviewNextRound(stateDir); r++ {
		rows, _ := readNDJSONRows(filepath.Join(stateDir, epicRoundFile(r)))
		for _, row := range rows {
			if loc, ok := row["location"].(string); ok && loc != "" {
				seen[loc] = true
			}
		}
		if snap, err := readEpicSnapshotFile(stateDir, r); err == nil {
			for key := range snap.Units {
				seen[key] = true
			}
		}
	}
	return slices.Sorted(maps.Keys(seen))
}

// epicSliceKeyCandidates lists the keys the unit keys can belong to: each
// epic alone, each gate's milestone, each named epic's milestone in g, and
// the unnamed set of every named epic.
func epicSliceKeyCandidates(g *model.EpicGraph, units []string) []EpicSliceKey {
	byID := epicMilestones(g)
	var out, milestones []EpicSliceKey
	var ids []string
	for _, u := range units {
		if id, ok := epicUnitID(u); ok {
			ids = append(ids, id)
			out = append(out, EpicSliceKey{Epic: id})
			if m, known := byID[id]; known {
				milestones = append(milestones, EpicSliceKey{Milestone: m})
			}
		} else if m, ok := strings.CutPrefix(u, "gate:"); ok {
			milestones = append(milestones, EpicSliceKey{Milestone: m})
		}
	}
	out = append(out, milestones...)
	if len(ids) > 0 {
		out = append(out, EpicSliceKey{Epics: ids})
	}
	return out
}
