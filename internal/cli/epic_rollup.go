package cli

import (
	"fmt"

	"github.com/deligoez/tp/internal/model"
)

// epicRollupRow is one epic's estimate-versus-actual roll-up: tp report's
// per-task estimate and actual minutes summed over the tracked tasks of the
// epic's bound files, how many tasks were tracked, and EstimateTotal, the
// estimate of every task of those files whatever its status, so the work
// still ahead of an in-flight epic shows (spec/1.6.0.md B3).
type epicRollupRow struct {
	Epic            string  `json:"epic"`
	EstimateMinutes int     `json:"estimate_minutes"`
	ActualMinutes   float64 `json:"actual_minutes"`
	Tracked         int     `json:"tracked"`
	EstimateTotal   int     `json:"estimate_total"`
}

// epicRollup returns the roll-up of milestone's epics that have a bound file,
// in graph order (spec/1.5.0.md §7). Each file goes through computeReport, so
// a task counts here exactly when tp report tracks it. bound is
// engine.EpicBoundFiles' result; a bound file that cannot be read is an error.
func epicRollup(g *model.EpicGraph, bound map[string][]string, milestone string) ([]epicRollupRow, error) {
	rows := []epicRollupRow{}
	for i := range g.Epics {
		e := &g.Epics[i]
		if e.Milestone != milestone || len(bound[e.ID]) == 0 {
			continue
		}
		row := epicRollupRow{Epic: e.ID}
		for _, path := range bound[e.ID] {
			tf, err := model.ReadTaskFile(path)
			if err != nil {
				return nil, fmt.Errorf("bound task file %s: %w", path, err)
			}
			_, summary := computeReport(tf)
			row.EstimateMinutes += summary.TotalEstimatedMinutes
			row.ActualMinutes += summary.TotalActualMinutes
			row.Tracked += summary.Tracked
			for j := range tf.Tasks {
				row.EstimateTotal += tf.Tasks[j].EstimateMinutes
			}
		}
		row.ActualMinutes = roundTo(row.ActualMinutes, 1)
		rows = append(rows, row)
	}
	return rows, nil
}
