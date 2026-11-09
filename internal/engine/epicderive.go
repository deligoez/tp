package engine

import (
	"fmt"

	"github.com/deligoez/tp/internal/model"
)

// An epic's derived status (spec/1.5.0.md §3). None is stored: each is read
// from the graph's claim and done record and from the epic's bound files.
const (
	EpicStatusPlanned    = "planned"
	EpicStatusClaimed    = "claimed"
	EpicStatusSpecced    = "specced"
	EpicStatusInProgress = "in_progress"
	EpicStatusDone       = "done"
)

// EpicState is one epic's derived state.
type EpicState struct {
	Status string
}

// DeriveEpicStates derives every epic of g's state from bound, the index
// EpicBoundFiles returns, so ready, status, export and the critical path all
// read one verdict. An epic with no bound file is done by its done record,
// else claimed by its claim, else planned. An epic with bound files ignores
// both: it is done when every bound file is at the release phase tp resume
// reports, in progress when a task of one is wip or done, and specced
// otherwise. A bound file that cannot be read, or whose review state is
// corrupt, is an error.
func DeriveEpicStates(g *model.EpicGraph, bound map[string][]string) (map[string]EpicState, error) {
	states := make(map[string]EpicState, len(g.Epics))
	for i := range g.Epics {
		e := &g.Epics[i]
		st, err := deriveEpicState(e, bound[e.ID])
		if err != nil {
			return nil, err
		}
		states[e.ID] = st
	}
	return states, nil
}

func deriveEpicState(e *model.Epic, files []string) (EpicState, error) {
	if len(files) == 0 {
		switch {
		case e.Done != nil:
			return EpicState{Status: EpicStatusDone}, nil
		case e.Claimed != nil:
			return EpicState{Status: EpicStatusClaimed}, nil
		default:
			return EpicState{Status: EpicStatusPlanned}, nil
		}
	}
	started, allReleased := false, true
	for _, path := range files {
		phase, fileStarted, err := boundFilePhase(path)
		if err != nil {
			return EpicState{}, err
		}
		started = started || fileStarted
		allReleased = allReleased && phase == PhaseRelease
	}
	switch {
	case allReleased:
		return EpicState{Status: EpicStatusDone}, nil
	case started:
		return EpicState{Status: EpicStatusInProgress}, nil
	default:
		return EpicState{Status: EpicStatusSpecced}, nil
	}
}

// boundFilePhase reads one bound task file and returns the phase tp resume
// reports for it, and whether any of its tasks is wip or done.
func boundFilePhase(path string) (phase string, started bool, err error) {
	tf, err := model.ReadTaskFile(path)
	if err != nil {
		return "", false, fmt.Errorf("bound task file %s: %w", path, err)
	}
	specPath, _ := ResolveSpecPath(path, tf.Spec)
	loops, err := readResumeLoops(path, specPath, tf)
	if err != nil {
		return "", false, err
	}
	for i := range tf.Tasks {
		if s := tf.Tasks[i].Status; s == model.StatusWIP || s == model.StatusDone {
			started = true
		}
	}
	return loops.phase, started, nil
}
