package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

type statsResult struct {
	Tasks         taskCounts         `json:"tasks"`
	Estimates     estimateStats      `json:"estimates"`
	Tags          []tagStats         `json:"tags"`
	Dependencies  depStats           `json:"dependencies"`
	Parallelism   parallelismStats   `json:"parallelism"`
	PlanHeuristic *planHeuristic     `json:"plan_size_heuristic,omitempty"`
}

type taskCounts struct {
	Total int `json:"total"`
	Open  int `json:"open"`
	WIP   int `json:"wip"`
	Done  int `json:"done"`
}

type estimateStats struct {
	Min   int `json:"min"`
	Avg   int `json:"avg"`
	Max   int `json:"max"`
	Total int `json:"total"`
}

type tagStats struct {
	Tag          string `json:"tag"`
	Count        int    `json:"count"`
	TotalMinutes int    `json:"total_minutes"`
}

type depStats struct {
	WithDeps    int `json:"with_deps"`
	Independent int `json:"independent"`
	CrossTag    int `json:"cross_tag"`
}

type parallelismStats struct {
	Levels               int                    `json:"levels"`
	MaxParallel          int                    `json:"max_parallel"`
	CriticalPathMinutes  int                    `json:"critical_path_minutes"`
	TotalEstimateMinutes int                    `json:"total_estimate_minutes"`
	Speedup              float64                `json:"speedup"`
	PerLevel             []engine.ParallelLevel `json:"per_level"`
}

type planHeuristic struct {
	SpecLines     int    `json:"spec_lines"`
	ExpectedTasks string `json:"expected_tasks"`
	ActualTasks   int    `json:"actual_tasks"`
	Assessment    string `json:"assessment"`
}

func newStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Estimates, parallelism, per-tag breakdown",
		RunE:  runStats,
	}
}

func runStats(_ *cobra.Command, _ []string) error {
	taskFilePath, err := engine.DiscoverTaskFile(".", flagFile)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	tf, err := model.ReadTaskFile(taskFilePath)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	result := statsResult{}

	// Task counts
	for i := range tf.Tasks {
		result.Tasks.Total++
		switch tf.Tasks[i].Status {
		case model.StatusOpen:
			result.Tasks.Open++
		case model.StatusWIP:
			result.Tasks.WIP++
		case model.StatusDone:
			result.Tasks.Done++
		}
	}

	// Estimates
	if len(tf.Tasks) > 0 {
		result.Estimates.Min = tf.Tasks[0].EstimateMinutes
		for i := range tf.Tasks {
			e := tf.Tasks[i].EstimateMinutes
			result.Estimates.Total += e
			if e < result.Estimates.Min {
				result.Estimates.Min = e
			}
			if e > result.Estimates.Max {
				result.Estimates.Max = e
			}
		}
		result.Estimates.Avg = result.Estimates.Total / len(tf.Tasks)
	}

	// Tags
	tagMap := make(map[string]*tagStats)
	for i := range tf.Tasks {
		for _, tag := range tf.Tasks[i].Tags {
			if _, ok := tagMap[tag]; !ok {
				tagMap[tag] = &tagStats{Tag: tag}
			}
			tagMap[tag].Count++
			tagMap[tag].TotalMinutes += tf.Tasks[i].EstimateMinutes
		}
	}
	for _, ts := range tagMap {
		result.Tags = append(result.Tags, *ts)
	}

	// Dependencies
	for i := range tf.Tasks {
		if len(tf.Tasks[i].DependsOn) > 0 {
			result.Dependencies.WithDeps++
		} else {
			result.Dependencies.Independent++
		}
	}

	// Parallelism
	levels := engine.ComputeParallelismLevels(tf.Tasks)
	result.Parallelism.Levels = len(levels)
	result.Parallelism.PerLevel = levels

	totalEst := 0
	criticalPath := 0
	maxParallel := 0
	for _, l := range levels {
		if len(l.Tasks) > maxParallel {
			maxParallel = len(l.Tasks)
		}
		criticalPath += l.BottleneckMinutes
		for _, id := range l.Tasks {
			for j := range tf.Tasks {
				if tf.Tasks[j].ID == id {
					totalEst += tf.Tasks[j].EstimateMinutes
				}
			}
		}
	}
	result.Parallelism.MaxParallel = maxParallel
	result.Parallelism.CriticalPathMinutes = criticalPath
	result.Parallelism.TotalEstimateMinutes = totalEst
	if criticalPath > 0 {
		result.Parallelism.Speedup = float64(totalEst) / float64(criticalPath)
	}

	return output.JSON(result)
}
