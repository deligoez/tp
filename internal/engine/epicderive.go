package engine

import (
	"fmt"
	"time"

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

// EpicState is one epic's derived state. DoneAt is set only when Status is
// done: the done record's time for an epic with no bound file, else the time
// of the audit round that most recently brought its last bound file to
// release, so it moves when the epic reopens and is done again. It is nil
// when that round's recorded_at cannot be read.
type EpicState struct {
	Status string
	DoneAt *time.Time
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
			at := e.Done.At
			return EpicState{Status: EpicStatusDone, DoneAt: &at}, nil
		case e.Claimed != nil:
			return EpicState{Status: EpicStatusClaimed}, nil
		default:
			return EpicState{Status: EpicStatusPlanned}, nil
		}
	}
	started, allReleased, timeKnown := false, true, true
	var doneAt time.Time
	for _, path := range files {
		f, err := readBoundFile(path)
		if err != nil {
			return EpicState{}, err
		}
		started = started || f.started
		allReleased = allReleased && f.phase == PhaseRelease
		timeKnown = timeKnown && !f.releasedAt.IsZero()
		if f.releasedAt.After(doneAt) {
			doneAt = f.releasedAt
		}
	}
	switch {
	case allReleased && !timeKnown:
		return EpicState{Status: EpicStatusDone}, nil
	case allReleased:
		return EpicState{Status: EpicStatusDone, DoneAt: &doneAt}, nil
	case started:
		return EpicState{Status: EpicStatusInProgress}, nil
	default:
		return EpicState{Status: EpicStatusSpecced}, nil
	}
}

// boundFile is what one bound task file contributes to its epic's state.
type boundFile struct {
	phase   string // the phase tp resume reports for the file
	started bool   // a task of the file is wip or done
	// releasedAt is the time of the audit round that brought the file to
	// release, its latest one; zero when the file is not at release.
	releasedAt time.Time
}

// readBoundFile reads one bound task file and its review state. A file at
// release was brought there by its latest audit round: rounds recorded after
// convergence end no loop, so none follows it.
func readBoundFile(path string) (boundFile, error) {
	tf, err := model.ReadTaskFile(path)
	if err != nil {
		return boundFile{}, fmt.Errorf("bound task file %s: %w", path, err)
	}
	specPath, _ := ResolveSpecPath(path, tf.Spec)
	loops, err := readResumeLoops(path, specPath, tf)
	if err != nil {
		return boundFile{}, err
	}
	f := boundFile{phase: loops.phase}
	for i := range tf.Tasks {
		if s := tf.Tasks[i].Status; s == model.StatusWIP || s == model.StatusDone {
			f.started = true
		}
	}
	if n := len(loops.auditRounds); f.phase == PhaseRelease && n > 0 {
		// An unreadable recorded_at leaves the time unknown, not the status.
		f.releasedAt, _ = time.Parse(time.RFC3339, loops.auditRounds[n-1].RecordedAt)
	}
	return f, nil
}
