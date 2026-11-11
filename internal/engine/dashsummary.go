package engine

import (
	"fmt"
	"path/filepath"

	"github.com/deligoez/tp/internal/model"
)

// DashboardSummary is the dashboard's summary payload: the figures no command
// prints (spec/1.8.0.md §2). Its four keys are fixed. Rounds holds §5's
// per-round finding counts; Band (§6), ReleaseImpact (§13) and Series (§17)
// are present and empty until the tasks that derive them fill them.
type DashboardSummary struct {
	Rounds        map[string][]SliceRoundCount `json:"rounds"`
	Band          map[string]any               `json:"band"`
	ReleaseImpact map[string]any               `json:"release_impact"`
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
// order. An unreadable round file is an error.
func DashboardSummaryOf(tpDir string, g *model.EpicGraph) (DashboardSummary, error) {
	dirs, err := EpicReviewSliceDirs(tpDir, g)
	if err != nil {
		return DashboardSummary{}, err
	}
	sum := DashboardSummary{
		Rounds:        make(map[string][]SliceRoundCount, len(dirs)),
		Band:          map[string]any{},
		ReleaseImpact: map[string]any{},
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
