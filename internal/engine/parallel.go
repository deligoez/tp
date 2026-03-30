package engine

import "github.com/deligoez/tp/internal/model"

// ParallelLevel represents a group of tasks that can run in parallel.
type ParallelLevel struct {
	Level             int      `json:"level"`
	Tasks             []string `json:"tasks"`
	BottleneckMinutes int      `json:"bottleneck_minutes"`
}

// ComputeParallelismLevels assigns tasks to parallelism levels.
// Level 0: no deps. Level N: all deps in levels 0..N-1.
// Only includes open and wip tasks (done tasks excluded).
func ComputeParallelismLevels(tasks []model.Task) []ParallelLevel {
	// Filter to remaining tasks
	remaining := make(map[string]model.Task)
	for i := range tasks {
		if tasks[i].Status != model.StatusDone {
			remaining[tasks[i].ID] = tasks[i]
		}
	}

	if len(remaining) == 0 {
		return nil
	}

	// Assign levels via iterative fixed-point
	levels := make(map[string]int)
	changed := true
	for changed {
		changed = false
		for id := range remaining {
			t := remaining[id]
			if _, assigned := levels[id]; assigned {
				continue
			}

			maxDepLevel := -1
			allResolved := true
			for _, dep := range t.DependsOn {
				if _, isDone := remaining[dep]; !isDone {
					// Dep is done, counts as resolved at level -1
					continue
				}
				if depLevel, ok := levels[dep]; ok {
					if depLevel > maxDepLevel {
						maxDepLevel = depLevel
					}
				} else {
					allResolved = false
					break
				}
			}

			if allResolved {
				levels[id] = maxDepLevel + 1
				changed = true
			}
		}
	}

	// Group by level
	levelGroups := make(map[int][]string)
	maxLevel := 0
	for id, level := range levels {
		levelGroups[level] = append(levelGroups[level], id)
		if level > maxLevel {
			maxLevel = level
		}
	}

	// Build result
	result := make([]ParallelLevel, 0, maxLevel+1)
	for l := 0; l <= maxLevel; l++ {
		taskIDs := levelGroups[l]
		if len(taskIDs) == 0 {
			continue
		}

		// Find bottleneck (max estimate in this level)
		bottleneck := 0
		for _, id := range taskIDs {
			if t, ok := remaining[id]; ok {
				if t.EstimateMinutes > bottleneck {
					bottleneck = t.EstimateMinutes
				}
			}
		}

		result = append(result, ParallelLevel{
			Level:             l,
			Tasks:             taskIDs,
			BottleneckMinutes: bottleneck,
		})
	}

	return result
}
