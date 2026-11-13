package cli

import (
	"cmp"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"

	"github.com/deligoez/tp/internal/dashboard"
	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
)

// The layout endpoint's views (spec/backlog/dashboard-ux.md §2), a closed set,
// each with the parameters it takes beside view; the first one it lists is
// required, except for milestones, which takes none.
const (
	layoutViewPlan          = "plan"
	layoutViewMilestones    = "milestones"
	layoutViewScope         = "scope"
	layoutViewNeighbourhood = "neighbourhood"
)

// layoutPackParam is the parameter every view takes beside its own: how the
// forest's columns are packed, rows (the default) or tree.
const layoutPackParam = "pack"

var layoutViewParams = map[string][]string{
	layoutViewPlan:          {},
	layoutViewMilestones:    {},
	layoutViewScope:         {"milestone"},
	layoutViewNeighbourhood: {"epic", "depth"},
}

// The neighbourhood's depth: its default, and the bounds it is clamped to.
const (
	layoutDefaultDepth = 2
	layoutMinDepth     = 1
	layoutMaxDepth     = 4
)

// dashboardLayoutHint answers a /api/layout query the endpoint refuses.
const dashboardLayoutHint = "name one view: ?view=plan, ?view=milestones, ?view=scope&milestone=<m>, or ?view=neighbourhood&epic=<id>[&depth=<n>], depth clamped to 1-4, each with an optional &pack=rows|tree"

// dashboardLayoutAbsentHint answers a milestone or epic the graph does not
// hold.
const dashboardLayoutAbsentHint = "pass a milestone a track lists, or an epic id, exactly as tp epic export prints it"

// layoutEchoMax is how many characters of a query's value an error echoes.
const layoutEchoMax = 80

// layoutEcho is s cut to its first layoutEchoMax characters, an ellipsis
// marking the cut, so an error never echoes a long value whole.
func layoutEcho(s string) string {
	if r := []rune(s); len(r) > layoutEchoMax {
		return string(r[:layoutEchoMax]) + "…"
	}
	return s
}

// layoutQuery is a /api/layout query the endpoint accepts.
type layoutQuery struct {
	view      string
	milestone string
	epic      string
	depth     int
	pack      engine.ForestPacking
}

// dashboardLayout is /api/layout's payload: the view and what it is about,
// the number of layers, the milestone bands of a scope, and the nodes in
// layer then order with the edges between them, a dependency's edge from it
// to its dependent.
type dashboardLayout struct {
	View      string              `json:"view"`
	Milestone string              `json:"milestone,omitempty"`
	Epic      string              `json:"epic,omitempty"`
	Depth     int                 `json:"depth,omitempty"`
	Layers    int                 `json:"layers"`
	Packing   string              `json:"packing"`
	Columns   float64             `json:"columns"`
	Bands     []engine.LayoutBand `json:"bands,omitempty"`
	Nodes     []layoutNode        `json:"nodes"`
	Edges     []layoutEdge        `json:"edges"`
}

// layoutNode is one node and its place. A neighbourhood's epic node names
// its milestone, which the view has no band for, and its centre is marked;
// a scope's carries only whether it is deferrable, its milestone being in
// the export and its any-of group on its edges. A milestone node carries its work
// epics, those done, the dependency edges into it from other milestones'
// epics (an any-of member one each) and whether it is open.
type layoutNode struct {
	ID string `json:"id"`
	engine.LayoutPlace
	Col        float64 `json:"col"`
	Milestone  string  `json:"milestone,omitempty"`
	Deferrable bool    `json:"deferrable,omitempty"`
	Centre     bool    `json:"centre,omitempty"`
	Epics      *int    `json:"epics,omitempty"`
	Done       *int    `json:"done,omitempty"`
	Inbound    *int    `json:"inbound,omitempty"`
	Open       *bool   `json:"open,omitempty"`
}

// layoutEdge is one edge: from the dependency to its dependent, or from a
// milestone to one that opens after it. Critical marks an edge of the
// critical path drawn; AnyOf names the group of an edge from an any-of
// member.
type layoutEdge struct {
	From     string          `json:"from"`
	To       string          `json:"to"`
	Critical bool            `json:"critical,omitempty"`
	AnyOf    []string        `json:"any_of,omitempty"`
	Role     engine.EdgeRole `json:"role"`
}

// Layout answers /api/layout: the view the query names, laid out by
// engine.LayeredLayout over the graph and its derived states.
func (dashboardAPI) Layout(q url.Values) (any, *dashboard.Error) {
	lq, derr := dashboardLayoutQuery(q)
	if derr != nil {
		return nil, derr
	}
	g, states, _, rerr := readEpicStates()
	if rerr != nil {
		return nil, runErrorOf(rerr)
	}
	switch lq.view {
	case layoutViewScope:
		if _, tracked := engine.MilestoneGates(g, states)[lq.milestone]; !tracked {
			return nil, dashboard.NewError(ExitState, fmt.Sprintf("milestone %q is not one a track lists", layoutEcho(lq.milestone)), dashboardLayoutAbsentHint)
		}
		return scopeLayout(g, states, lq.milestone, lq.pack), nil
	case layoutViewNeighbourhood:
		if !slices.ContainsFunc(g.Epics, func(e model.Epic) bool { return e.ID == lq.epic }) {
			return nil, dashboard.NewError(ExitState, fmt.Sprintf("epic %q is not in the graph", layoutEcho(lq.epic)), dashboardLayoutAbsentHint)
		}
		return neighbourhoodLayout(g, states, lq.epic, lq.depth, lq.pack), nil
	case layoutViewPlan:
		return planLayout(g, states, lq.pack), nil
	default:
		return milestonesLayout(g, states, lq.pack), nil
	}
}

// dashboardLayoutQuery reads q under the endpoint's rules: view once and one
// of the closed set, no parameter its view does not take, each at most once,
// the one it needs present and not empty, and depth a number, clamped.
func dashboardLayoutQuery(q url.Values) (layoutQuery, *dashboard.Error) {
	bad := func(msg string) (layoutQuery, *dashboard.Error) {
		return layoutQuery{}, dashboard.NewError(ExitUsage, msg, dashboardLayoutHint)
	}
	view := q.Get("view")
	params, known := layoutViewParams[view]
	if len(q["view"]) != 1 || !known {
		return bad("/api/layout takes view once: plan, milestones, scope or neighbourhood")
	}
	for key, values := range q {
		if key != "view" && key != layoutPackParam && !slices.Contains(params, key) {
			return bad(fmt.Sprintf("view %s takes no %s", view, layoutEcho(key)))
		}
		if len(values) != 1 {
			return bad(layoutEcho(key) + " is given more than once")
		}
	}
	if len(params) > 0 && q.Get(params[0]) == "" {
		return bad(fmt.Sprintf("view %s needs %s", view, params[0]))
	}
	lq := layoutQuery{view: view, milestone: q.Get("milestone"), epic: q.Get("epic"), pack: engine.PackRows}
	switch p := engine.ForestPacking(q.Get(layoutPackParam)); p {
	case "", engine.PackRows:
	case engine.PackTree:
		lq.pack = p
	default:
		return bad("pack is rows or tree")
	}
	if view == layoutViewNeighbourhood {
		lq.depth = layoutDefaultDepth
		if q.Has("depth") {
			// ParseInt saturates an integer out of int64's range to the
			// bound of its sign, which the clamp then takes.
			d, err := strconv.ParseInt(q.Get("depth"), 10, 64)
			if err != nil && !errors.Is(err, strconv.ErrRange) {
				return bad("depth is not an integer")
			}
			lq.depth = int(min(layoutMaxDepth, max(layoutMinDepth, d)))
		}
	}
	return lq, nil
}

// milestonesLayout is the overview: every tracked milestone, an edge from
// each milestone it opens after, and its counts.
func milestonesLayout(g *model.EpicGraph, states map[string]engine.EpicState, pack engine.ForestPacking) dashboardLayout {
	ids := trackedMilestones(g)
	after := engine.EffectiveOpensAfter(g)
	var edges []layoutEdge
	for _, m := range ids {
		for _, p := range after[m] {
			edges = append(edges, layoutEdge{From: p, To: m})
		}
	}
	inbound := milestoneInbound(g)
	gates := engine.MilestoneGates(g, states)
	out := layoutOf(layoutViewMilestones, ids, edges, nil, nil, nil, pack)
	for i := range out.Nodes {
		m := out.Nodes[i].ID
		f := engine.WorkFigures(g, states, nil, m)
		open, in := gates[m].Open, inbound[m]
		out.Nodes[i].Epics, out.Nodes[i].Done, out.Nodes[i].Inbound, out.Nodes[i].Open = &f.Work, &f.Done, &in, &open
	}
	return out
}

// milestoneInbound counts, per milestone, the dependency edges its epics
// have on other milestones' epics, an any-of member one each.
func milestoneInbound(g *model.EpicGraph) map[string]int {
	milestoneOf := map[string]string{}
	for i := range g.Epics {
		milestoneOf[g.Epics[i].ID] = g.Epics[i].Milestone
	}
	out := map[string]int{}
	for i := range g.Epics {
		e := &g.Epics[i]
		for _, d := range e.DependsOn {
			for _, id := range d.Names() {
				if m, ok := milestoneOf[id]; ok && m != e.Milestone {
					out[e.Milestone]++
				}
			}
		}
	}
	return out
}

// scopeLayout is a gate's closure as tp epic scope derives it, banded by
// milestone, its critical path marked.
func scopeLayout(g *model.EpicGraph, states map[string]engine.EpicState, milestone string, pack engine.ForestPacking) dashboardLayout {
	scope := engine.MilestoneScope(g, states, nil, milestone)
	in := map[string]bool{}
	bandOf := map[string]string{}
	marks := map[string]engine.ScopeEpic{}
	var bands []string
	for _, m := range scope.Closure {
		bands = append(bands, m.Milestone)
		for _, e := range m.Epics {
			in[e.ID], bandOf[e.ID], marks[e.ID] = true, m.Milestone, e
		}
	}
	ids, edges := epicEdges(g, in, scope.CriticalPath.ByCount)
	out := layoutOf(layoutViewScope, ids, edges, bands, bandOf, trackGroups(g), pack)
	out.Milestone = milestone
	for i := range out.Nodes {
		out.Nodes[i].Deferrable = marks[out.Nodes[i].ID].Deferrable
	}
	return out
}

// neighbourhoodLayout is epic with what it depends on and what depends on
// it, depth steps each way, the graph's critical path marked.
func neighbourhoodLayout(g *model.EpicGraph, states map[string]engine.EpicState, epic string, depth int, pack engine.ForestPacking) dashboardLayout {
	deps, dependents := map[string][]string{}, map[string][]string{}
	milestoneOf := map[string]string{}
	for i := range g.Epics {
		e := &g.Epics[i]
		milestoneOf[e.ID] = e.Milestone
		for _, d := range e.DependsOn {
			for _, id := range d.Names() {
				deps[e.ID] = append(deps[e.ID], id)
				dependents[id] = append(dependents[id], e.ID)
			}
		}
	}
	in := map[string]bool{epic: true}
	walkSteps(epic, deps, depth, in)
	walkSteps(epic, dependents, depth, in)
	ids, edges := epicEdges(g, in, engine.GraphCriticalPath(g, states))
	out := layoutOf(layoutViewNeighbourhood, ids, edges, nil, nil, nil, pack)
	out.Epic, out.Depth = epic, depth
	for i := range out.Nodes {
		n := &out.Nodes[i]
		n.Milestone, n.Centre = milestoneOf[n.ID], n.ID == epic
	}
	return out
}

// walkSteps adds to in every id next reaches from start in at most depth
// steps.
func walkSteps(start string, next map[string][]string, depth int, in map[string]bool) {
	front := []string{start}
	for range depth {
		var reached []string
		for _, id := range front {
			for _, n := range next[id] {
				if !in[n] {
					in[n] = true
					reached = append(reached, n)
				}
			}
		}
		front = reached
	}
}

// epicEdges lists the epics in in, in graph order, and the edges between
// them: a plain edge, or one per any-of member carrying its group, each
// marked critical when its two ends follow each other on path.
func epicEdges(g *model.EpicGraph, in map[string]bool, path []string) ([]string, []layoutEdge) {
	onPath := map[[2]string]bool{}
	for i := 1; i < len(path); i++ {
		onPath[[2]string{path[i-1], path[i]}], onPath[[2]string{path[i], path[i-1]}] = true, true
	}
	critical := func(a, b string) bool { return onPath[[2]string{a, b}] }
	var ids []string
	var edges []layoutEdge
	for i := range g.Epics {
		e := &g.Epics[i]
		if !in[e.ID] {
			continue
		}
		ids = append(ids, e.ID)
		for _, d := range e.DependsOn {
			group := d.Members()
			for _, id := range d.Names() {
				if in[id] {
					edges = append(edges, layoutEdge{From: id, To: e.ID, Critical: critical(id, e.ID), AnyOf: group})
				}
			}
		}
	}
	return ids, edges
}

// layoutOf lays ids and edges out, bands and bandOf as engine.LayeredLayout
// takes them, and lists the nodes by layer, then order.
// layoutOf lays ids and edges out, bands and bandOf as engine.LayeredLayout
// takes them, groupOf and pack as engine.ForestLayout does, and lists the
// nodes by layer, then column, each with its barycenter order and its
// forest column, each edge with its role.
func layoutOf(view string, ids []string, edges []layoutEdge, bands []string, bandOf, groupOf map[string]string, pack engine.ForestPacking) dashboardLayout {
	in := make([]engine.LayoutEdge, len(edges))
	for i, e := range edges {
		in[i] = engine.LayoutEdge{From: e.From, To: e.To}
	}
	l := engine.LayeredLayout(ids, in, bands, bandOf)
	f := engine.ForestLayout(ids, in, bands, bandOf, groupOf, pack)
	out := dashboardLayout{View: view, Layers: l.Layers, Packing: string(pack), Columns: f.Columns, Bands: l.Bands, Nodes: make([]layoutNode, 0, len(ids)), Edges: edges}
	if out.Edges == nil {
		out.Edges = []layoutEdge{}
	}
	for i := range out.Edges {
		out.Edges[i].Role = f.Roles[i]
	}
	for _, id := range ids {
		out.Nodes = append(out.Nodes, layoutNode{ID: id, LayoutPlace: l.Places[id], Col: f.Places[id].Col})
	}
	slices.SortFunc(out.Nodes, func(a, b layoutNode) int {
		return cmp.Or(cmp.Compare(a.Layer, b.Layer), cmp.Compare(a.Col, b.Col), cmp.Compare(a.Order, b.Order))
	})
	return out
}

// planLayout is the whole graph: every epic, banded by milestone in track
// order, its roots grouped by track, the graph's critical path marked.
func planLayout(g *model.EpicGraph, states map[string]engine.EpicState, pack engine.ForestPacking) dashboardLayout {
	in := map[string]bool{}
	bandOf := map[string]string{}
	for i := range g.Epics {
		in[g.Epics[i].ID], bandOf[g.Epics[i].ID] = true, g.Epics[i].Milestone
	}
	ids, edges := epicEdges(g, in, engine.GraphCriticalPath(g, states))
	out := layoutOf(layoutViewPlan, ids, edges, trackedMilestones(g), bandOf, trackGroups(g), pack)
	for i := range out.Nodes {
		out.Nodes[i].Milestone = bandOf[out.Nodes[i].ID]
	}
	return out
}

// trackGroups maps each epic to the track its milestone is first listed in,
// so the forest packs each track's trees side by side.
func trackGroups(g *model.EpicGraph) map[string]string {
	track := map[string]string{}
	for t, ms := range g.Milestones {
		for _, m := range ms {
			if _, seen := track[m]; !seen {
				track[m] = strconv.Itoa(t)
			}
		}
	}
	out := make(map[string]string, len(g.Epics))
	for i := range g.Epics {
		out[g.Epics[i].ID] = track[g.Epics[i].Milestone]
	}
	return out
}
