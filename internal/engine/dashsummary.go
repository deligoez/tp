package engine

import (
	"fmt"
	"path/filepath"

	"github.com/deligoez/tp/internal/model"
)

// DashboardSummary is the dashboard's summary payload: the figures no command
// prints (spec/1.8.0.md §2). Rounds holds §5's per-round finding counts, Band
// §6's lifecycle band and ReleaseImpact §13's impact of releasing each spec,
// keyed by its task file's repository path; Series (§17) is present and empty
// until the task that derives it fills it; Plan is §4's KPI figures and now
// lists, empty with no graph.
type DashboardSummary struct {
	Rounds        map[string][]SliceRoundCount `json:"rounds"`
	Band          DashboardBand                `json:"band"`
	ReleaseImpact map[string]ReleaseImpact     `json:"release_impact"`
	Series        []any                        `json:"series"`
	Plan          DashboardPlan                `json:"plan"`
}

// SliceRoundCount is one recorded round of a slice: its finding count, total
// and by role, every finding counted whatever its disposition, and a finding
// naming no role under EpicRoleUnattributed, so ByRole sums to Total.
type SliceRoundCount struct {
	Round  int            `json:"round"`
	Total  int            `json:"total"`
	ByRole map[string]int `json:"by_role"`
}

// DashboardSummaryOf derives the summary from tpDir's epic review state: for
// every slice state directory, keyed by its directory name as tp epic
// export's slices[].dir names it, each recorded round's counts in round
// order; from the repository holding tpDir, read once, the lifecycle band and
// each task file's release impact; and, with a graph, the plan. An unreadable
// round file, task file or project config is an error.
func DashboardSummaryOf(tpDir string, g *model.EpicGraph) (DashboardSummary, error) {
	dirs, err := EpicReviewSliceDirs(tpDir, g)
	if err != nil {
		return DashboardSummary{}, err
	}
	repo, err := readDashRepo(filepath.Dir(tpDir), g)
	if err != nil {
		return DashboardSummary{}, err
	}
	sum := DashboardSummary{
		Rounds:        make(map[string][]SliceRoundCount, len(dirs)),
		Band:          repo.band(g),
		ReleaseImpact: repo.releaseImpacts(g),
		Series:        []any{},
		Plan:          emptyDashboardPlan(),
	}
	for _, d := range dirs {
		rounds, err := sliceRoundCounts(d.Path)
		if err != nil {
			return DashboardSummary{}, err
		}
		sum.Rounds[d.Name] = rounds
	}
	if bandHasGraph(g) {
		if sum.Plan, err = repo.plan(tpDir, g); err != nil {
			return DashboardSummary{}, err
		}
	}
	return sum, nil
}

// plan derives r's plan against g, reading the project's placeholder pattern
// (the default when it cannot be read, as the read commands fall back) and
// the slices' review policy from tpDir.
func (r *dashRepo) plan(tpDir string, g *model.EpicGraph) (DashboardPlan, error) {
	matcher, err := ProjectPlaceholderMatcher(tpDir)
	if err != nil {
		matcher, _ = NewPlaceholderMatcher(DefaultThresholdPlaceholder)
	}
	pc, _, err := LoadProjectConfig(tpDir)
	if err != nil {
		return DashboardPlan{}, err
	}
	wf := ResolveWorkflowLayers(&model.WorkflowOverride{}, &pc.Workflow)
	policy := EpicReviewPolicy{CleanRounds: wf.ReviewCleanRounds, MaxRounds: wf.ReviewMaxRounds, ConvergeOn: wf.ReviewConvergeOn}
	reviewSlices, err := EpicReviewSlices(tpDir, g, policy)
	if err != nil {
		return DashboardPlan{}, err
	}
	return DashboardPlanOf(g, r.states, r.bound, matcher, reviewSlices), nil
}

// sliceRoundCounts counts each recorded round of the slice at stateDir.
func sliceRoundCounts(stateDir string) ([]SliceRoundCount, error) {
	last := EpicReviewNextRound(stateDir) - 1
	out := make([]SliceRoundCount, 0, last)
	for r := 1; r <= last; r++ {
		path := filepath.Join(stateDir, epicRoundFile(r))
		rows, ok := readNDJSONRows(path)
		if !ok {
			return nil, fmt.Errorf("epic review round file %s cannot be read as NDJSON", path)
		}
		c := SliceRoundCount{Round: r, Total: len(rows), ByRole: map[string]int{}}
		for _, row := range rows {
			role, _ := row["role"].(string)
			if role == "" {
				role = EpicRoleUnattributed
			}
			c.ByRole[role]++
		}
		out = append(out, c)
	}
	return out, nil
}
