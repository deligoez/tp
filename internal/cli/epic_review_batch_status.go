package cli

import (
	"cmp"
	"os"
	"path/filepath"
	"slices"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// epicBatchStatusRow is one slice of a batch --status (spec/1.7.0.md §5): a
// one-slice --status cut to what a row needs, with the loop state tp epic
// export reports.
type epicBatchStatusRow struct {
	Key         *engine.EpicSliceKeyRecord `json:"key"`
	Round       int                        `json:"round"`
	State       string                     `json:"state"`
	Open        map[string]int             `json:"open"`
	Grantable   bool                       `json:"grantable"`
	MaxRounds   *int                       `json:"max_rounds"`
	GrantsSpent int                        `json:"grants_spent"`
	NextAction  string                     `json:"next_action"`
}

// epicBatchStatusResult is a batch --status's payload.
type epicBatchStatusResult struct {
	Slices []epicBatchStatusRow `json:"slices"`
}

// runEpicReviewBatchStatus prints one row per slice: under --all every slice
// directory under .tp/epic-review/, under --milestones each listed milestone.
// With check it exits 1 unless every slice's own --status --check would exit
// 0: its loop done and every mechanical check passing. A slice the graph no
// longer holds gets a row; it fails the check unless it is done, since a done
// one is a legacy slice tp reports needs no action.
func runEpicReviewBatchStatus(scope epicBatchScope, check bool) {
	eg := loadEpicReviewGraph()
	policy := epicReviewPolicy(eg.tpDir)
	checksPass := epicChecksPass(eg.checks)
	dirs := epicBatchStatusDirs(&eg, scope)
	result := epicBatchStatusResult{Slices: make([]epicBatchStatusRow, 0, len(dirs))}
	pass := checksPass
	for _, d := range dirs {
		row, done := epicBatchStatusRowOf(&eg, policy, d, checksPass)
		pass = pass && done
		result.Slices = append(result.Slices, row)
	}
	if err := output.JSON(result); err != nil {
		output.Error(ExitFile, err.Error(), internalEncodeHint)
		os.Exit(ExitFile)
	}
	if check && !pass {
		os.Exit(ExitValidation)
	}
}

// epicBatchStatusDirs names the slices a batch --status or --report reads:
// every slice directory under --all, in track order, each listed
// milestone's otherwise.
func epicBatchStatusDirs(eg *epicReviewGraph, scope epicBatchScope) []engine.EpicSliceDir {
	if scope.all {
		dirs, err := engine.EpicReviewSliceDirs(eg.tpDir, eg.g)
		if err != nil {
			output.Error(ExitFile, err.Error(), epicRoundFileHint)
			os.Exit(ExitFile)
		}
		sortEpicSliceDirsByTrack(dirs, eg.g)
		return dirs
	}
	names := epicBatchMilestones(eg, scope)
	dirs := make([]engine.EpicSliceDir, 0, len(names))
	for _, m := range names {
		key := engine.EpicSliceKey{Milestone: m}
		rec := engine.EpicSliceKeyRecordOf(key)
		dirs = append(dirs, engine.EpicSliceDir{Path: engine.EpicReviewStateDir(eg.tpDir, key), Key: &rec})
	}
	return dirs
}

// sortEpicSliceDirsByTrack orders dirs as the graph's tracks list their
// milestones: each milestone slice at its milestone's place, then every
// other slice (an epic or epics slice, a milestone no track holds, a
// directory with no key) in the name order it arrived in.
func sortEpicSliceDirsByTrack(dirs []engine.EpicSliceDir, g *model.EpicGraph) {
	order := slices.Concat(g.Milestones...)
	rank := func(d engine.EpicSliceDir) int {
		if d.Key != nil && d.Key.Kind == "milestone" {
			if i := slices.Index(order, d.Key.Value); i >= 0 {
				return i
			}
		}
		return len(order)
	}
	slices.SortStableFunc(dirs, func(a, b engine.EpicSliceDir) int { return cmp.Compare(rank(a), rank(b)) })
}

// epicBatchStatusRowOf grades one slice and reports whether its loop is done.
// An unreadable recorded round exits 3, as a one-slice --status does.
func epicBatchStatusRowOf(eg *epicReviewGraph, policy engine.EpicReviewPolicy, d engine.EpicSliceDir, checksPass bool) (epicBatchStatusRow, bool) {
	if d.Key == nil {
		return epicUnheldSliceRow(eg, d, policy, "the slice directory .tp/epic-review/"+d.Name+" records no key and its rounds name none")
	}
	key := d.Key.SliceKey()
	run, rerr := prepareEpicReviewRun(eg, key)
	if rerr != nil && rerr.code != ExitState {
		exitEpicRunError(rerr)
	}
	if rerr != nil {
		return epicUnheldSliceRow(eg, d, policy, rerr.msg)
	}
	st := epicBatchLoop(run.stateDir, run.all, policy)
	return epicBatchStatusRow{
		Key:         d.Key,
		Round:       st.Round,
		State:       engine.EpicSliceLoop(&st, true).State,
		Open:        st.Open,
		Grantable:   st.Grantable,
		MaxRounds:   epicMaxRoundsOut(st.MaxRounds),
		GrantsSpent: st.GrantsSpent,
		NextAction:  epicStatusNextAction(key, &st, checksPass),
	}, st.Done
}

// epicUnheldSliceRow is the row of a slice whose units cannot be derived: its
// key is unknown or names what the graph no longer holds. It is graded
// without its staleness, as tp epic export grades it, and nothing runs on it;
// a done one is a legacy slice whose directory is safe to delete, and counts
// as done for the batch --check (v1.7.0 §5), which the second result reports.
func epicUnheldSliceRow(eg *epicReviewGraph, d engine.EpicSliceDir, policy engine.EpicReviewPolicy, why string) (epicBatchStatusRow, bool) {
	st := epicBatchLoop(d.Path, nil, policy)
	next := why + ": nothing runs on this slice until the graph holds it"
	if st.Done {
		path := d.Path
		if rel, err := filepath.Rel(filepath.Dir(eg.tpDir), d.Path); err == nil {
			path = rel
		}
		next = "legacy slice, done; no action (safe to delete " + path + ")"
	}
	return epicBatchStatusRow{
		Key:         d.Key,
		Round:       st.Round,
		State:       engine.EpicSliceLoop(&st, false).State,
		Open:        st.Open,
		MaxRounds:   epicMaxRoundsOut(st.MaxRounds),
		GrantsSpent: st.GrantsSpent,
		NextAction:  next,
	}, st.Done
}

// epicBatchLoop reads one slice's loop verdict, exiting 3 on unreadable state.
func epicBatchLoop(stateDir string, units []engine.EpicReviewUnit, policy engine.EpicReviewPolicy) engine.EpicReviewStatus {
	st, err := engine.EpicReviewLoopStatus(stateDir, units, policy)
	if err != nil {
		output.Error(ExitFile, err.Error(), epicRoundFileHint)
		os.Exit(ExitFile)
	}
	return st
}
