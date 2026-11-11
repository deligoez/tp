package engine

import (
	"fmt"
	"path/filepath"

	"github.com/deligoez/tp/internal/model"
)

// DashboardSummary is the dashboard's summary payload: the figures no command
// prints (spec/1.8.0.md §2). Its four keys are fixed. Rounds holds §5's
// per-round finding counts, Band §6's lifecycle band and ReleaseImpact §13's
// impact of releasing each spec, keyed by its task file's repository path;
// Series (§17) is present and empty until the task that derives it fills it.
type DashboardSummary struct {
	Rounds        map[string][]SliceRoundCount `json:"rounds"`
	Band          DashboardBand                `json:"band"`
	ReleaseImpact map[string]ReleaseImpact     `json:"release_impact"`
	Series        []any                        `json:"series"`
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
// order; and, from the repository holding tpDir, read once, the lifecycle
// band and each task file's release impact. An unreadable round file or task
// file is an error.
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
	}
	for _, d := range dirs {
		rounds, err := sliceRoundCounts(d.Path)
		if err != nil {
			return DashboardSummary{}, err
		}
		sum.Rounds[d.Name] = rounds
	}
	return sum, nil
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
