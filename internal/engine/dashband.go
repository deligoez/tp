package engine

import (
	"fmt"

	"github.com/deligoez/tp/internal/model"
)

// What a lifecycle band's phase axis counts.
const (
	BandPhasesOfEpics     = "epics"
	BandPhasesOfTaskFiles = "task_files"
)

// bandStages and bandPhases are the band's two axes in display order: the
// derived epic statuses and the phases tp resume reports, review first.
var (
	bandStages = []string{EpicStatusPlanned, EpicStatusClaimed, EpicStatusSpecced, EpicStatusInProgress, EpicStatusDone}
	bandPhases = []string{PhaseReview, PhaseDecompose, PhaseImplement, PhaseAudit, PhaseRelease}
)

// DashboardBand is the dashboard's lifecycle band (spec/1.8.0.md §6): work
// epics counted per stage, and per spec phase. PhasesOf says what the phase
// axis counts: work epics by their earliest bound file's phase when a graph
// exists, task files when none does. Both maps carry every key of their axis.
type DashboardBand struct {
	Stages   map[string]int `json:"stages"`
	Phases   map[string]int `json:"phases"`
	PhasesOf string         `json:"phases_of"`
}

// LifecycleBand counts the band. With a graph holding epics, each work epic
// counts once under its status in states, and, when epicPhases lists phases
// of its bound files, once under the earliest of them; filePhases is
// ignored, so an unbound task file is not counted. With no graph (nil or
// without epics) no stage counts anything and each of filePhases, one per
// task file, counts under its phase.
func LifecycleBand(g *model.EpicGraph, states map[string]EpicState, epicPhases map[string][]string, filePhases []string) DashboardBand {
	band := DashboardBand{Stages: zeroCounts(bandStages), Phases: zeroCounts(bandPhases), PhasesOf: BandPhasesOfTaskFiles}
	if !bandHasGraph(g) {
		for _, p := range filePhases {
			countKnown(band.Phases, p)
		}
		return band
	}
	band.PhasesOf = BandPhasesOfEpics
	for i := range g.Epics {
		e := &g.Epics[i]
		if !IsWorkEpic(e) {
			continue
		}
		countKnown(band.Stages, states[e.ID].Status)
		if p, ok := earliestPhase(epicPhases[e.ID]); ok {
			band.Phases[p]++
		}
	}
	return band
}

// DashboardBandOf reads the band from the repository discovered from start:
// with a graph, the bound files of its epics and their derived states; with
// none, every task file git sees. A task file that cannot be read is an
// error.
func DashboardBandOf(start string, g *model.EpicGraph) (DashboardBand, error) {
	if !bandHasGraph(g) {
		phases, err := taskFilePhases(repoTaskFilesFrom(start))
		if err != nil {
			return DashboardBand{}, err
		}
		return LifecycleBand(nil, nil, nil, phases), nil
	}
	bound, err := EpicBoundFiles(start)
	if err != nil {
		return DashboardBand{}, err
	}
	states, err := DeriveEpicStates(g, bound)
	if err != nil {
		return DashboardBand{}, err
	}
	epicPhases := make(map[string][]string, len(bound))
	for id, files := range bound {
		if epicPhases[id], err = taskFilePhases(files); err != nil {
			return DashboardBand{}, err
		}
	}
	return LifecycleBand(g, states, epicPhases, nil), nil
}

// bandHasGraph reports whether g is a graph holding epics.
func bandHasGraph(g *model.EpicGraph) bool {
	return g != nil && len(g.Epics) > 0
}

// repoTaskFilesFrom is repoTaskFiles of the repository holding start, none
// outside one.
func repoTaskFilesFrom(start string) []string {
	if root := gitToplevel(start); root != "" {
		return repoTaskFiles(root)
	}
	return nil
}

// taskFilePhases reads the phase tp resume reports for each task file.
func taskFilePhases(paths []string) ([]string, error) {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		tf, err := model.ReadTaskFile(path)
		if err != nil {
			return nil, fmt.Errorf("task file %s: %w", path, err)
		}
		specPath, _ := ResolveSpecPath(path, tf.Spec)
		loops, err := readResumeLoops(path, specPath, tf)
		if err != nil {
			return nil, err
		}
		out = append(out, loops.phase)
	}
	return out, nil
}

// earliestPhase is the first of phases in the band's phase order; ok is
// false when none is a known phase.
func earliestPhase(phases []string) (string, bool) {
	for _, p := range bandPhases {
		for _, q := range phases {
			if p == q {
				return p, true
			}
		}
	}
	return "", false
}

// countKnown counts key when counts already holds it, so a value outside
// the axis adds no key.
func countKnown(counts map[string]int, key string) {
	if _, ok := counts[key]; ok {
		counts[key]++
	}
}

// zeroCounts is a count map holding each key at 0.
func zeroCounts(keys []string) map[string]int {
	m := make(map[string]int, len(keys))
	for _, k := range keys {
		m[k] = 0
	}
	return m
}
