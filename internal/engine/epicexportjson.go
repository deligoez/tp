package engine

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/deligoez/tp/internal/model"
)

// The states tp epic export reports for a loop (spec/1.5.0.md §7).
const (
	// LoopStateNone: no round recorded or emitted yet.
	LoopStateNone = "none"
	// LoopStateOpen: rounds recorded, the loop neither done nor at its cap.
	LoopStateOpen = "open"
	// LoopStateStale: what the last round read has changed since.
	LoopStateStale = "stale"
	// LoopStateCapReached: the round cap is reached and the loop is not done.
	LoopStateCapReached = "cap_reached"
	// LoopStateInFlight: an epic review round is emitted and not recorded.
	LoopStateInFlight = "in_flight"
	// LoopStateDone: the loop converged, or is done at its cap.
	LoopStateDone = "done"
	// LoopStateComplete: the latest ground round has every emitted unit
	// dispositioned.
	LoopStateComplete = "complete"
)

// EpicLoop is one loop's latest round and its state.
type EpicLoop struct {
	Round int    `json:"round"`
	State string `json:"state"`
}

// EpicFileState is one bound task file as tp epic export reports it: its
// repo-relative path, the phase tp resume reports, its tasks counted by
// status, and the review, audit and ground loops of its spec.
type EpicFileState struct {
	Path   string         `json:"path"`
	Phase  string         `json:"phase"`
	Tasks  map[string]int `json:"tasks"`
	Review EpicLoop       `json:"review"`
	Audit  EpicLoop       `json:"audit"`
	Ground EpicLoop       `json:"ground"`
}

// EpicFileStates reports every file of bound, EpicBoundFiles(start)'s result,
// keyed by epic in bound's order. A file or review state that cannot be read
// is an error.
func EpicFileStates(start string, bound map[string][]string) (map[string][]EpicFileState, error) {
	root := gitToplevel(start)
	out := make(map[string][]EpicFileState, len(bound))
	for id, files := range bound {
		for _, path := range files {
			st, err := epicFileState(path)
			if err != nil {
				return nil, err
			}
			st.Path = filepath.ToSlash(path)
			if rel, err := filepath.Rel(root, path); root != "" && err == nil {
				st.Path = filepath.ToSlash(rel)
			}
			out[id] = append(out[id], st)
		}
	}
	return out, nil
}

// epicFileState reads one bound file's loops the way tp resume does.
func epicFileState(path string) (EpicFileState, error) {
	tf, err := model.ReadTaskFile(path)
	if err != nil {
		return EpicFileState{}, fmt.Errorf("bound task file %s: %w", path, err)
	}
	specPath, _ := ResolveSpecPath(path, tf.Spec)
	loops, err := readResumeLoops(path, specPath, tf)
	if err != nil {
		return EpicFileState{}, err
	}
	ground, err := epicGroundLoop(specPath)
	if err != nil {
		return EpicFileState{}, err
	}
	tasks := map[string]int{model.StatusOpen: 0, model.StatusWIP: 0, model.StatusDone: 0}
	for i := range tf.Tasks {
		tasks[tf.Tasks[i].Status]++
	}
	return EpicFileState{
		Phase:  loops.phase,
		Tasks:  tasks,
		Review: specLoop(len(loops.reviewRounds), loops.reviewDone, loops.reviewStale),
		Audit:  specLoop(len(loops.auditRounds), loops.auditDone, false),
		Ground: ground,
	}, nil
}

// specLoop grades a spec's review or audit loop from its recorded rounds and
// the verdict tp resume reads.
func specLoop(rounds int, done LoopDone, stale bool) EpicLoop {
	l := EpicLoop{Round: rounds}
	switch {
	case rounds == 0:
		l.State = LoopStateNone
	case done.Done:
		l.State = LoopStateDone
	case stale:
		l.State = LoopStateStale
	case done.CapReached:
		l.State = LoopStateCapReached
	default:
		l.State = LoopStateOpen
	}
	return l
}

// epicGroundLoop grades a spec's ground loop from its latest emitted round,
// as tp ground --status reads it.
func epicGroundLoop(specPath string) (EpicLoop, error) {
	st, err := LatestGroundStatus(specPath)
	if errors.Is(err, ErrNoGroundEmission) {
		return EpicLoop{State: LoopStateNone}, nil
	}
	if err != nil {
		return EpicLoop{}, err
	}
	if st.Coverage.Dispositioned == st.Coverage.Emitted {
		return EpicLoop{Round: st.Round, State: LoopStateComplete}, nil
	}
	return EpicLoop{Round: st.Round, State: LoopStateOpen}, nil
}

// EpicSliceState is one epic review slice as tp epic export lists it: its
// state directory's name under .tp/epic-review/, its key (key.json's, else
// the one its recorded rounds name; nil when neither says), and its loop.
type EpicSliceState struct {
	Dir string              `json:"dir"`
	Key *EpicSliceKeyRecord `json:"key"`
	EpicLoop
}

// EpicReviewSlices lists every slice state directory under tpDir, in name
// order, graded under p as tp epic review --status grades it. A slice whose
// key is unknown, or names what the graph no longer holds, is graded without
// its staleness, which needs the slice's current units.
func EpicReviewSlices(tpDir string, g *model.EpicGraph, p EpicReviewPolicy) ([]EpicSliceState, error) {
	root := filepath.Join(tpDir, EpicReviewStateRoot)
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return []EpicSliceState{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []EpicSliceState{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		key, err := ReadEpicSliceKey(dir)
		if err != nil {
			return nil, err
		}
		if key == nil {
			key = deriveEpicSliceKey(dir, g)
		}
		units, known := epicSliceUnits(g, key)
		st, err := EpicReviewLoopStatus(dir, units, p)
		if err != nil {
			return nil, err
		}
		out = append(out, EpicSliceState{Dir: entry.Name(), Key: key, EpicLoop: sliceLoop(&st, known)})
	}
	return out, nil
}

// epicSliceUnits returns the current units of the slice key names, and false
// when the key is unknown or names what the graph no longer holds.
func epicSliceUnits(g *model.EpicGraph, key *EpicSliceKeyRecord) ([]EpicReviewUnit, bool) {
	if key == nil {
		return nil, false
	}
	k := EpicSliceKey{Epic: key.Value}
	switch key.Kind {
	case "epics":
		k = EpicSliceKey{Epics: key.IDs, Name: key.Name}
	case "milestone":
		k = EpicSliceKey{Milestone: key.Value}
	}
	slice, err := EpicReviewSlice(g, k)
	if err != nil {
		return nil, false
	}
	return slice.Units, true
}

// sliceLoop names a slice's loop state; staleness counts only when known.
func sliceLoop(st *EpicReviewStatus, known bool) EpicLoop {
	l := EpicLoop{Round: st.Round}
	switch {
	case st.Done:
		l.State = LoopStateDone
	case st.InFlightRound > 0:
		l.State = LoopStateInFlight
	case st.Round == 0:
		l.State = LoopStateNone
	case known && st.Stale:
		l.State = LoopStateStale
	case st.CapReached:
		l.State = LoopStateCapReached
	default:
		l.State = LoopStateOpen
	}
	return l
}
