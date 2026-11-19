package cli

import (
	"cmp"
	"fmt"
	"math"
	"slices"

	"github.com/deligoez/tp/internal/dashboard"
	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
)

// tasksLayout is one task file's tasks, every one whatever its status, laid
// out by the forest the plan uses, layered by dependency depth with no
// bands, an edge from each dependency the file holds to its dependent, then
// shelved taller than wide (tasksShelved), which packs the trees whatever
// pack the query names. file is taken only when it is exactly a path tp
// export --list lists, as /api/export takes it, so the query never picks a
// filesystem path.
func tasksLayout(file string) (any, *dashboard.Error) {
	open, _, listed := engine.ListedTaskFile(".", file)
	if !listed {
		return nil, dashboard.NewError(ExitState, fmt.Sprintf("task file %q is not one tp export --list lists", layoutEcho(file)), dashboardUnlistedHint)
	}
	tf, err := model.ReadTaskFile(open)
	if err != nil {
		return nil, dashboard.NewError(ExitFile, err.Error(), dashboardUnlistedHint)
	}
	in := make(map[string]bool, len(tf.Tasks))
	ids := make([]string, 0, len(tf.Tasks))
	for i := range tf.Tasks {
		in[tf.Tasks[i].ID] = true
		ids = append(ids, tf.Tasks[i].ID)
	}
	var edges []layoutEdge
	for i := range tf.Tasks {
		for _, d := range tf.Tasks[i].DependsOn {
			if in[d] {
				edges = append(edges, layoutEdge{From: d, To: tf.Tasks[i].ID})
			}
		}
	}
	out := layoutOf(layoutViewTasks, ids, edges, nil, nil, nil, nil, nil, engine.PackTree)
	tasksShelved(ids, &out)
	return out, nil
}

// tasksShelved shelves out, laid out with tree packing, by
// engine.ShelveForest about the square root of its node count wide, so a
// file with many independent tasks reads down the page rather than across
// it, its nodes listed by layer then column again.
func tasksShelved(ids []string, out *dashboardLayout) {
	f := engine.Forest{Places: make(map[string]engine.ForestPlace, len(out.Nodes)), Roles: make([]engine.EdgeRole, len(out.Edges))}
	for _, n := range out.Nodes {
		f.Places[n.ID] = engine.ForestPlace{Layer: n.Layer, Col: n.Col}
	}
	edges := make([]engine.LayoutEdge, len(out.Edges))
	for i, e := range out.Edges {
		edges[i], f.Roles[i] = engine.LayoutEdge{From: e.From, To: e.To}, e.Role
	}
	s := engine.ShelveForest(ids, edges, f, math.Ceil(math.Sqrt(float64(len(ids)))))
	out.Layers, out.Columns = s.Layers, s.Columns
	for i := range out.Edges {
		out.Edges[i].Role = s.Roles[i]
	}
	for i := range out.Nodes {
		p := s.Places[out.Nodes[i].ID]
		out.Nodes[i].Layer, out.Nodes[i].Col = p.Layer, p.Col
	}
	slices.SortStableFunc(out.Nodes, func(a, b layoutNode) int {
		return cmp.Or(cmp.Compare(a.Layer, b.Layer), cmp.Compare(a.Col, b.Col))
	})
}
