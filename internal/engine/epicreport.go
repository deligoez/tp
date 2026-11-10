package engine

import (
	"cmp"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
)

// EpicClassSlice is one slice a finding class was raised on, by its key, and
// the recorded rounds of that slice that raised it, ascending.
type EpicClassSlice struct {
	Key    *EpicSliceKeyRecord `json:"key"`
	Rounds []int               `json:"rounds"`
}

// EpicClassRaised is one finding class across slices (spec/1.7.0.md §3): the
// distinct epics its rows were raised on, sorted, and the slices and rounds
// that raised it, in slice order. An epic two overlapping slices both raised
// it on is one epic.
type EpicClassRaised struct {
	Class  string           `json:"class"`
	Epics  []string         `json:"epics"`
	Slices []EpicClassSlice `json:"slices"`
}

// epicClassTally accumulates one class while the slices are read in order.
type epicClassTally struct {
	raised  EpicClassRaised
	epics   map[string]bool
	lastDir string
}

// add counts one row of class raised in round of the slice d on location.
func (t *epicClassTally) add(d *EpicSliceDir, round int, location string) {
	if id, ok := strings.CutPrefix(location, "epic:"); ok {
		t.epics[id] = true
	}
	if t.lastDir != d.Path {
		t.lastDir = d.Path
		t.raised.Slices = append(t.raised.Slices, EpicClassSlice{Key: d.Key})
	}
	s := &t.raised.Slices[len(t.raised.Slices)-1]
	if n := len(s.Rounds); n == 0 || s.Rounds[n-1] != round {
		s.Rounds = append(s.Rounds, round)
	}
}

// tallyEpicRound counts each classed row of round r of the slice d.
func tallyEpicRound(tallies map[string]*epicClassTally, d *EpicSliceDir, r int, rows []map[string]any) {
	for _, row := range rows {
		class, _ := row["class"].(string)
		if class == "" {
			continue
		}
		t := tallies[class]
		if t == nil {
			t = &epicClassTally{raised: EpicClassRaised{Class: class}, epics: map[string]bool{}}
			tallies[class] = t
		}
		location, _ := row["location"].(string)
		t.add(d, r, location)
	}
}

// EpicReviewClasses reads every recorded round of dirs and lists each class
// their rows carry, the class raised on the most distinct epics first, ties by
// class name. A row with no class counts in none, a location outside an epic
// (a gate unit) names no epic, and a round file tp cannot read is an error.
func EpicReviewClasses(dirs []EpicSliceDir) ([]EpicClassRaised, error) {
	tallies := map[string]*epicClassTally{}
	for i := range dirs {
		d := &dirs[i]
		for r := 1; r < EpicReviewNextRound(d.Path); r++ {
			path := filepath.Join(d.Path, epicRoundFile(r))
			rows, ok := readNDJSONRows(path)
			if !ok {
				return nil, fmt.Errorf("epic review round file %s cannot be read as NDJSON", path)
			}
			tallyEpicRound(tallies, d, r, rows)
		}
	}
	out := make([]EpicClassRaised, 0, len(tallies))
	for _, t := range tallies {
		t.raised.Epics = slices.Sorted(maps.Keys(t.epics))
		out = append(out, t.raised)
	}
	slices.SortFunc(out, func(a, b EpicClassRaised) int {
		return cmp.Or(cmp.Compare(len(b.Epics), len(a.Epics)), cmp.Compare(a.Class, b.Class))
	})
	return out, nil
}
