package engine

import (
	"sort"

	"github.com/deligoez/tp/internal/model"
)

// TopoSort returns tasks in dependency-safe execution order.
// WIP tasks appear first (resume support). Done tasks are excluded.
func TopoSort(tasks []model.Task) []model.Task {
	// Separate WIP and non-done tasks
	remaining := make(map[string]*model.Task)
	var wipTasks []model.Task

	for i := range tasks {
		if tasks[i].Status == model.StatusDone {
			continue
		}
		if tasks[i].Status == model.StatusWIP {
			wipTasks = append(wipTasks, tasks[i])
		}
		remaining[tasks[i].ID] = &tasks[i]
	}

	// Kahn's algorithm for topological sort
	// Build in-degree map (only counting deps within remaining set)
	inDegree := make(map[string]int)
	dependents := make(map[string][]string) // dep -> tasks that depend on it

	for id, t := range remaining {
		if _, exists := inDegree[id]; !exists {
			inDegree[id] = 0
		}
		for _, dep := range t.DependsOn {
			if _, inRemaining := remaining[dep]; inRemaining {
				inDegree[id]++
				dependents[dep] = append(dependents[dep], id)
			}
		}
	}

	// Start with zero in-degree nodes
	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	// Sort queue for deterministic output (alphabetical)
	sort.Strings(queue)

	var sorted []model.Task
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]

		t := remaining[id]
		// Skip WIP tasks here - they go first
		if t.Status != model.StatusWIP {
			sorted = append(sorted, *t)
		}

		for _, dep := range dependents[id] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
				sort.Strings(queue) // maintain deterministic order
			}
		}
	}

	// WIP first, then topologically sorted open tasks
	result := make([]model.Task, 0, len(wipTasks)+len(sorted))
	result = append(result, wipTasks...)
	result = append(result, sorted...)
	return result
}
