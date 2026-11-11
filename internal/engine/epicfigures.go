package engine

import (
	"math"
	"slices"

	"github.com/deligoez/tp/internal/model"
)

// IsWorkEpic reports whether e counts in a work figure: every epic but a
// condition, which is the operator's precondition rather than delivered work
// (spec/1.8.0.md §4). Every figure below, and the ready list and share done
// of tp epic ready and tp epic status, filter through it, so a condition is
// counted in none of them.
func IsWorkEpic(e *model.Epic) bool {
	return !e.Condition
}

// EpicWorkFigures is the work-epic figures of one milestone, or of the whole
// graph: how many work epics there are, how many are ready (on the ready list
// handed in), blocked by an undone dependency, held (blocked with a condition,
// or a group of conditions, among what holds it, so a subset of Blocked) and
// done, and ShareDone, Done over Work rounded to two places, 0 with no work.
// The lists tp epic status and tp epic export print keep carrying conditions
// with their not_actionable and moot fields; only these figures leave them out.
type EpicWorkFigures struct {
	Work      int
	Ready     int
	Blocked   int
	Held      int
	Done      int
	ShareDone float64
}

// WorkFigures counts the figures of milestone m's work epics, or of every
// epic's when m is empty. states is DeriveEpicStates' result and ready the
// ready list EpicReady returns.
func WorkFigures(g *model.EpicGraph, states map[string]EpicState, ready []string, m string) EpicWorkFigures {
	conditions := make(map[string]bool, len(g.Epics))
	for i := range g.Epics {
		conditions[g.Epics[i].ID] = g.Epics[i].Condition
	}
	var f EpicWorkFigures
	for i := range g.Epics {
		e := &g.Epics[i]
		if !IsWorkEpic(e) || (m != "" && e.Milestone != m) {
			continue
		}
		f.Work++
		switch blocked, held := epicWaitFlags(e, states, conditions); {
		case states[e.ID].Status == EpicStatusDone:
			f.Done++
		case blocked:
			f.Blocked++
			if held {
				f.Held++
			}
		case slices.Contains(ready, e.ID):
			f.Ready++
		}
	}
	if f.Work > 0 {
		f.ShareDone = math.Round(float64(f.Done)/float64(f.Work)*100) / 100
	}
	return f
}

// epicWaitFlags reports whether an undone direct dependency, or a group none
// of whose members is done, holds e, and whether a condition, or a group of
// conditions only, is among them.
func epicWaitFlags(e *model.Epic, states map[string]EpicState, conditions map[string]bool) (blocked, held bool) {
	done := func(id string) bool { return states[id].Status == EpicStatusDone }
	for _, d := range e.DependsOn {
		members := []string{d.ID}
		if d.IsGroup() {
			members = d.Members()
		}
		if !slices.ContainsFunc(members, done) {
			blocked = true
			held = held || !slices.ContainsFunc(members, func(id string) bool { return !conditions[id] })
		}
	}
	return blocked, held
}
