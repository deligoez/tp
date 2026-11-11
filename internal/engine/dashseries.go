package engine

import (
	"fmt"
	"time"

	"github.com/deligoez/tp/internal/model"
)

// SeriesDay is one day of the lifecycle series (spec/1.8.0.md §17): the work
// epics in each stage on that UTC day, and Unknown those a stage whose time
// tp did not record may already hold. The counts sum to the work epics.
type SeriesDay struct {
	Date       string `json:"date"`
	Planned    int    `json:"planned"`
	Claimed    int    `json:"claimed"`
	Specced    int    `json:"specced"`
	InProgress int    `json:"in_progress"`
	Done       int    `json:"done"`
	Unknown    int    `json:"unknown"`
}

// The stages after planned, in lifecycle order, as stageTimes indexes them.
const (
	stageClaimed = iota
	stageSpecced
	stageInProgress
	stageDone
	stageCount
)

// stageTime is when an epic reached one stage: never (absent), at a recorded
// time, or at a time tp did not record (unknown).
type stageTime struct {
	reached bool
	known   bool
	day     string
}

func knownStage(at time.Time) stageTime {
	return stageTime{reached: true, known: true, day: at.UTC().Format(time.DateOnly)}
}

// stageTimes is one work epic's time for each stage after planned.
type stageTimes [stageCount]stageTime

// lifecycleSeries derives the series over g's work epics, from each bound
// file's binding and task start times, each epic's claim and its done time
// in states. It runs from the first recorded event's day to the last's, and
// is empty when tp recorded none. A bound file that cannot be read is an
// error.
func lifecycleSeries(g *model.EpicGraph, states map[string]EpicState, bound map[string][]string) ([]SeriesDay, error) {
	var epics []stageTimes
	first, last := "", ""
	for i := range g.Epics {
		e := &g.Epics[i]
		if !IsWorkEpic(e) {
			continue
		}
		st, err := epicStageTimes(e, states[e.ID], bound[e.ID])
		if err != nil {
			return nil, err
		}
		epics = append(epics, st)
		for _, s := range st {
			if !s.known {
				continue
			}
			if first == "" || s.day < first {
				first = s.day
			}
			if s.day > last {
				last = s.day
			}
		}
	}
	series := []SeriesDay{}
	if first == "" {
		return series, nil
	}
	start, _ := time.Parse(time.DateOnly, first)
	for d := start; d.Format(time.DateOnly) <= last; d = d.AddDate(0, 0, 1) {
		day := SeriesDay{Date: d.Format(time.DateOnly)}
		for i := range epics {
			day.count(epics[i].stageOn(day.Date))
		}
		series = append(series, day)
	}
	return series, nil
}

// epicStageTimes reads when e reached each stage: claimed at its claim,
// specced at the earliest bound_at of its bound files when every one carries
// one (else unknown), in progress at the earliest started_at of their wip or
// done tasks (unknown when none carries one), done at state's done time
// (unknown when the state is done without one).
func epicStageTimes(e *model.Epic, state EpicState, files []string) (stageTimes, error) {
	var st stageTimes
	if e.Claimed != nil {
		st[stageClaimed] = knownStage(e.Claimed.At)
	}
	if len(files) > 0 {
		var err error
		if st[stageSpecced], st[stageInProgress], err = boundStageTimes(files); err != nil {
			return st, err
		}
	}
	if state.Status == EpicStatusDone {
		st[stageDone] = stageTime{reached: true}
		if state.DoneAt != nil {
			st[stageDone] = knownStage(*state.DoneAt)
		}
	}
	return st, nil
}

// boundStageTimes reads an epic's specced and in-progress times from its
// bound files.
func boundStageTimes(files []string) (specced, started stageTime, err error) {
	var boundAt, startedAt time.Time
	specced.reached = true
	allBound := true
	for _, path := range files {
		tf, err := model.ReadTaskFile(path)
		if err != nil {
			return specced, started, fmt.Errorf("bound task file %s: %w", path, err)
		}
		if tf.BoundAt == nil {
			allBound = false
		} else if boundAt.IsZero() || tf.BoundAt.Before(boundAt) {
			boundAt = *tf.BoundAt
		}
		for i := range tf.Tasks {
			task := &tf.Tasks[i]
			if task.Status != model.StatusWIP && task.Status != model.StatusDone {
				continue
			}
			started.reached = true
			if task.StartedAt != nil && (startedAt.IsZero() || task.StartedAt.Before(startedAt)) {
				startedAt = *task.StartedAt
			}
		}
	}
	if allBound {
		specced = knownStage(boundAt)
	}
	if !startedAt.IsZero() {
		started = knownStage(startedAt)
	}
	return specced, started, nil
}

// stageOn is the stage the epic is in on day: the latest one reached by then
// at a recorded time, or -1 for planned; stageCount when a later stage it
// reached at an unrecorded time may already hold it.
func (st *stageTimes) stageOn(day string) int {
	stage := -1
	for i, s := range st {
		if s.known && s.day <= day {
			stage = i
		}
	}
	for i := stage + 1; i < stageCount; i++ {
		if st[i].reached && !st[i].known {
			return stageCount
		}
	}
	return stage
}

// count adds one epic in stage to the day.
func (d *SeriesDay) count(stage int) {
	switch stage {
	case -1:
		d.Planned++
	case stageClaimed:
		d.Claimed++
	case stageSpecced:
		d.Specced++
	case stageInProgress:
		d.InProgress++
	case stageDone:
		d.Done++
	default:
		d.Unknown++
	}
}
