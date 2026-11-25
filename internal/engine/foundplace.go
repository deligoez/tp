package engine

import (
	"fmt"
	"slices"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// FoundEdge is one depends_on edge placement adds to a task already in the
// task file: Task comes to depend on DependsOn.
type FoundEdge struct {
	Task      string
	DependsOn string
}

// placeFoundRequests places the batch's blocking requests (spec/1.31.0.md
// §4). A blocking request whose outcome is a task makes its finder depend on
// that task, unless the finder already does; when the edge would close a
// cycle it adds no edge and opens a queue entry naming the cycle, which holds
// the finder. A non-blocking request's edge is the found task's own, set by
// newFoundTask; a request with no task outcome adds none here.
func placeFoundRequests(plan *FoundApplyPlan, reqs []FoundRequest, tasks []model.Task, outcomes map[string]FoundRequest) {
	deps := make(map[string][]string, len(tasks)+len(plan.Tasks))
	for i := range tasks {
		deps[tasks[i].ID] = slices.Clone(tasks[i].DependsOn)
	}
	for i := range plan.Tasks {
		deps[plan.Tasks[i].ID] = slices.Clone(plan.Tasks[i].DependsOn)
	}
	batch := make([]*FoundRequest, 0, len(reqs))
	for i := range reqs {
		if _, ok := outcomes[reqs[i].ID]; ok && reqs[i].State == FoundStateUnapplied {
			batch = append(batch, &reqs[i])
		}
	}
	slices.SortStableFunc(batch, func(a, b *FoundRequest) int { return a.Seq - b.Seq })
	for _, req := range batch {
		finder, outcome := req.Finder.Task, outcomes[req.ID].Outcome
		if _, known := deps[finder]; !req.Blocks || !known || outcome == "" || slices.Contains(deps[finder], outcome) {
			continue
		}
		if path := foundDependencyPath(deps, outcome, finder); path != nil {
			plan.Entries = append(plan.Entries, foundCycleEntry(req, append([]string{finder}, path...)))
			continue
		}
		deps[finder] = append(deps[finder], outcome)
		plan.Edges = append(plan.Edges, FoundEdge{Task: finder, DependsOn: outcome})
	}
}

// foundDependencyPath returns the depends_on path from one task to another,
// both ends included, or nil when to is not reached. Breadth-first, so the
// path named is a shortest one.
func foundDependencyPath(deps map[string][]string, from, to string) []string {
	parent := map[string]string{from: ""}
	queue := []string{from}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if id == to {
			path := []string{}
			for at := id; at != ""; at = parent[at] {
				path = append(path, at)
			}
			slices.Reverse(path)
			return path
		}
		for _, dep := range deps[id] {
			if _, seen := parent[dep]; !seen {
				parent[dep] = id
				queue = append(queue, dep)
			}
		}
	}
	return nil
}

// foundCycleEntry is the open queue entry of a blocking request whose edge
// would close cycle (finder first and last). Like a past-cap entry it carries
// the defect itself, since the request store is not committed.
func foundCycleEntry(req *FoundRequest, cycle []string) FoundQueueEntry {
	finder := cycle[0]
	return FoundQueueEntry{
		State:        FoundQueueOpen,
		Request:      req.ID,
		Location:     req.Location,
		Question:     fmt.Sprintf("%s is blocked on %s, but that edge would close the cycle %s: how should %s proceed?", finder, cycle[1], strings.Join(cycle, " -> "), finder),
		Reproduction: req.Reproduction,
		Expected:     req.Expected,
		Observed:     req.Observed,
		Finders:      []string{finder},
	}
}
