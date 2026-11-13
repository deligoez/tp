package cli_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// layoutGraph is a small synthetic plan: two milestones of one track and a
// triggered one, a plain chain, an any-of group and a cross-milestone edge.
const layoutGraph = `{
  "version": 1,
  "milestones": [["M1", "M2"], ["T1"]],
  "opens_after": {"T1": []},
  "gates": {"M2": {"members": ["s2"]}},
  "epics": [
    {"id": "r1", "title": "R1", "milestone": "M1"},
    {"id": "r2", "title": "R2", "milestone": "M1", "depends_on": [{"id": "r1", "why": "r2 needs r1"}]},
    {"id": "s1", "title": "S1", "milestone": "M2", "depends_on": [{"id": "r2", "why": "s1 needs r2"}]},
    {"id": "s2", "title": "S2", "milestone": "M2", "depends_on": [{"id": "s1", "why": "s2 needs s1"}, {"any_of": ["r1", "t1"], "why": "either backend"}]},
    {"id": "t1", "title": "T1", "milestone": "T1", "depends_on": [{"id": "r1", "why": "t1 needs r1"}]}
  ]
}
`

// TestDashboard_LayoutRefusesAnythingButAClosedView is the layout endpoint's
// refusals (spec/backlog/dashboard-ux.md §2): view is one of a closed set,
// each view takes only its own parameters, each once, the one it needs is
// required and depth is a number: anything else is a usage error (400, code
// 2). A milestone or epic the graph does not hold is looked up, never built
// into a path, and answers as an absent object (404, code 4).
func TestDashboard_LayoutRefusesAnythingButAClosedView(t *testing.T) {
	t.Parallel()
	srv := startDashboard(t, epicGraphDir(t, layoutGraph), freePort(t))
	for _, bad := range []string{
		"/api/layout", "/api/layout?view=all", "/api/layout?view=", "/api/layout?view=milestones&epic=r1",
		"/api/layout?view=milestones&view=scope", "/api/layout?view=scope", "/api/layout?view=scope&milestone=M1&epic=r1",
		"/api/layout?view=scope&milestone=M1&milestone=M2", "/api/layout?view=neighbourhood", "/api/layout?view=neighbourhood&epic=r1&depth=two",
		"/api/layout?view=neighbourhood&epic=r1&depth=", "/api/layout?view=neighbourhood&epic=r1&milestone=M1", "/api/layout?view=milestones&x=1",
	} {
		assert.Equal(t, 2, getDashboardError(t, srv, bad, http.StatusBadRequest).Code, bad)
	}
	for _, absent := range []string{"/api/layout?view=scope&milestone=M9", "/api/layout?view=scope&milestone=..%2F..%2Fetc", "/api/layout?view=neighbourhood&epic=nope", "/api/layout?view=neighbourhood&epic=..%2Fr1"} {
		assert.Equal(t, 4, getDashboardError(t, srv, absent, http.StatusNotFound).Code, absent)
	}
}

// TestDashboard_LayoutCutsALongValueInItsError: an error echoes what the
// query named cut to its first 80 characters, so a long value does not
// come back whole.
func TestDashboard_LayoutCutsALongValueInItsError(t *testing.T) {
	t.Parallel()
	srv := startDashboard(t, epicGraphDir(t, layoutGraph), freePort(t))
	long := strings.Repeat("x", 500)
	for path, status := range map[string]int{
		"/api/layout?view=neighbourhood&epic=" + long: http.StatusNotFound,
		"/api/layout?view=scope&milestone=" + long:    http.StatusNotFound,
		"/api/layout?view=milestones&" + long + "=1":  http.StatusBadRequest,
	} {
		e := getDashboardError(t, srv, path, status)
		assert.Contains(t, e.Error, strings.Repeat("x", 80), path)
		assert.NotContains(t, e.Error, strings.Repeat("x", 81), path)
	}
}

// TestDashboard_LayoutClampsTheNeighbourhoodDepth: depth defaults to 2 and
// is clamped to 1 through 4, the response naming the depth it drew.
func TestDashboard_LayoutClampsTheNeighbourhoodDepth(t *testing.T) {
	t.Parallel()
	srv := startDashboard(t, epicGraphDir(t, layoutGraph), freePort(t))
	for query, want := range map[string]string{"": `"depth": 2`, "&depth=0": `"depth": 1`, "&depth=-3": `"depth": 1`, "&depth=99": `"depth": 4`, "&depth=3": `"depth": 3`,
		"&depth=99999999999999999999": `"depth": 4`, "&depth=-99999999999999999999": `"depth": 1`} {
		status, body := getPage(t, strings.TrimSuffix(srv.url, "/")+"/api/layout?view=neighbourhood&epic=r2"+query)
		require.Equal(t, http.StatusOK, status, body)
		assert.Contains(t, body, want, query)
	}
}

// TestDashboard_LayoutGoldenViews pins the three views on layoutGraph. The
// overview: each milestone with its work epics, those done, the edges into
// it from other milestones' epics (M2's s1 -> r2 and s2's group members r1
// and t1) and whether it is open, an edge from each milestone it opens
// after. M2's closure: tp epic scope's epics banded by milestone in
// dependency order (T1 before M2, whose s2 waits on t1), each node its
// place alone (its milestone in the export, its group on its edges), the
// scope's critical path marked and the group's edges marked. r2's
// neighbourhood at depth 1: one step each way, r2 the centre, the graph's
// critical path marked.
func TestDashboard_LayoutGoldenViews(t *testing.T) {
	t.Parallel()
	srv := startDashboard(t, epicGraphDir(t, layoutGraph), freePort(t))
	for query, want := range map[string]string{
		"view=milestones": `{"view": "milestones", "layers": 2, "packing": "rows", "columns": 2,
			"nodes": [{"id": "M1", "layer": 0, "order": 0, "col": 0, "epics": 2, "done": 0, "inbound": 0, "open": true},
				{"id": "T1", "layer": 0, "order": 1, "col": 1, "epics": 1, "done": 0, "inbound": 1, "open": true},
				{"id": "M2", "layer": 1, "order": 0, "col": 0, "epics": 2, "done": 0, "inbound": 3, "open": false}],
			"edges": [{"from": "M1", "to": "M2", "role": "tree"}]}`,
		"view=scope&milestone=M2": `{"view": "scope", "milestone": "M2", "layers": 5, "packing": "rows", "columns": 1,
			"bands": [{"milestone": "M1", "from_layer": 0, "to_layer": 1}, {"milestone": "T1", "from_layer": 2, "to_layer": 2}, {"milestone": "M2", "from_layer": 3, "to_layer": 4}],
			"nodes": [{"id": "r1", "layer": 0, "order": 0, "col": 0}, {"id": "r2", "layer": 1, "order": 0, "col": 0},
				{"id": "t1", "layer": 2, "order": 0, "col": 0},
				{"id": "s1", "layer": 3, "order": 0, "col": 0}, {"id": "s2", "layer": 4, "order": 0, "col": 0}],
			"edges": [{"from": "r1", "to": "r2", "critical": true, "role": "tree"}, {"from": "r2", "to": "s1", "critical": true, "role": "tree"}, {"from": "s1", "to": "s2", "critical": true, "role": "tree"},
				{"from": "r1", "to": "s2", "any_of": ["r1", "t1"], "role": "cross"}, {"from": "t1", "to": "s2", "any_of": ["r1", "t1"], "role": "cross"}, {"from": "r1", "to": "t1", "role": "tree"}]}`,
		"view=plan&pack=tree": `{"view": "plan", "layers": 5, "packing": "tree", "columns": 2,
			"bands": [{"milestone": "M1", "from_layer": 0, "to_layer": 1}, {"milestone": "T1", "from_layer": 2, "to_layer": 2}, {"milestone": "M2", "from_layer": 3, "to_layer": 4}],
			"nodes": [{"id": "r1", "layer": 0, "order": 0, "col": 0.5, "milestone": "M1"}, {"id": "r2", "layer": 1, "order": 0, "col": 0, "milestone": "M1"},
				{"id": "t1", "layer": 2, "order": 0, "col": 1, "milestone": "T1"},
				{"id": "s1", "layer": 3, "order": 0, "col": 0, "milestone": "M2"}, {"id": "s2", "layer": 4, "order": 0, "col": 0, "milestone": "M2"}],
			"edges": [{"from": "r1", "to": "r2", "critical": true, "role": "tree"}, {"from": "r2", "to": "s1", "critical": true, "role": "tree"}, {"from": "s1", "to": "s2", "critical": true, "role": "tree"},
				{"from": "r1", "to": "s2", "any_of": ["r1", "t1"], "role": "cross"}, {"from": "t1", "to": "s2", "any_of": ["r1", "t1"], "role": "cross"}, {"from": "r1", "to": "t1", "role": "tree"}]}`,
		"view=neighbourhood&epic=r2&depth=1": `{"view": "neighbourhood", "epic": "r2", "depth": 1, "layers": 3, "packing": "rows", "columns": 1,
			"nodes": [{"id": "r1", "layer": 0, "order": 0, "col": 0, "milestone": "M1"}, {"id": "r2", "layer": 1, "order": 0, "col": 0, "milestone": "M1", "centre": true},
				{"id": "s1", "layer": 2, "order": 0, "col": 0, "milestone": "M2"}],
			"edges": [{"from": "r1", "to": "r2", "critical": true, "role": "tree"}, {"from": "r2", "to": "s1", "critical": true, "role": "tree"}]}`,
	} {
		status, body := getPage(t, strings.TrimSuffix(srv.url, "/")+"/api/layout?"+query)
		require.Equal(t, http.StatusOK, status, body)
		assert.JSONEq(t, want, body, query)
	}
}
