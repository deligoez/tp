package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// downstreamEpic is one epic of the downstream fixtures (spec/1.6.0-checks.md
// slice 1), written to .tp/epics.json as JSON.
type downstreamEpic struct {
	ID         string          `json:"id"`
	Title      string          `json:"title"`
	Milestone  string          `json:"milestone"`
	Root       bool            `json:"root,omitempty"`
	Condition  bool            `json:"condition,omitempty"`
	Acceptance string          `json:"acceptance,omitempty"`
	DependsOn  []downstreamDep `json:"depends_on,omitempty"`
	Done       *downstreamDone `json:"done,omitempty"`
}

// downstreamDep is one depends_on edge.
type downstreamDep struct {
	ID  string `json:"id"`
	Why string `json:"why"`
}

// downstreamDone is a done record.
type downstreamDone struct {
	At       string `json:"at"`
	Evidence string `json:"evidence"`
}

// dsEpic is an epic of milestone m depending on deps.
func dsEpic(id, m string, deps ...string) downstreamEpic {
	e := downstreamEpic{ID: id, Title: id, Milestone: m}
	for _, d := range deps {
		e.DependsOn = append(e.DependsOn, downstreamDep{ID: d, Why: "w"})
	}
	return e
}

// downstreamEpics is the C1.1 graph on track [M1, M2], roots in graph order
// r4, r3, r2, r1: r1 heads a1→…→a5 (a1, a2 in M1, a3–a5 in M2, a5 also
// depending on r1 directly); r2 has the chain c1→c2 and leaves direct leaf
// dependents; r3 has the chain d1→d2 and the leaf d3; r4 has none.
func downstreamEpics(leaves int) []downstreamEpic {
	epics := []downstreamEpic{}
	for _, id := range []string{"r4", "r3", "r2", "r1"} {
		e := dsEpic(id, "M1")
		e.Root = true
		epics = append(epics, e)
	}
	epics = append(epics,
		dsEpic("a1", "M1", "r1"), dsEpic("a2", "M1", "a1"), dsEpic("a3", "M2", "a2"),
		dsEpic("a4", "M2", "a3"), dsEpic("a5", "M2", "a4", "r1"),
		dsEpic("c1", "M1", "r2"), dsEpic("c2", "M1", "c1"),
		dsEpic("d1", "M1", "r3"), dsEpic("d2", "M1", "d1"), dsEpic("d3", "M1", "r3"))
	for i := 1; i <= leaves; i++ {
		epics = append(epics, dsEpic(fmt.Sprintf("l%d", i), "M1", "r2"))
	}
	return epics
}

// downstreamDir writes epics, with gates, to a fresh project and returns it.
func downstreamDir(t *testing.T, epics []downstreamEpic, gates map[string]any) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{"version": 1, "milestones": [][]string{{"M1", "M2"}}, "gates": gates, "epics": epics})
	require.NoError(t, err)
	dir := epicGraphDir(t, "")
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "epics.json"), body, 0o600))
	return dir
}

// epicDownstreamReport is tp epic ready's ready list with each downstream.
type epicDownstreamReport struct {
	Ready []struct {
		ID         string `json:"id"`
		Downstream *struct {
			LongestChain          int            `json:"longest_chain"`
			Dependents            int            `json:"dependents"`
			DependentsOpen        int            `json:"dependents_open"`
			DependentsByMilestone map[string]int `json:"dependents_by_milestone"`
		} `json:"downstream"`
	} `json:"ready"`
}

// readyDownstream runs tp epic ready in dir and returns its ready list.
func readyDownstream(t *testing.T, dir string) *epicDownstreamReport {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "epic", "ready")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var r epicDownstreamReport
	require.NoError(t, json.Unmarshal([]byte(stdout), &r), "stdout: %s", stdout)
	for _, e := range r.Ready {
		require.NotNil(t, e.Downstream, "ready item %s carries downstream", e.ID)
	}
	return &r
}

// ids returns the ready list's ids in order.
func (r *epicDownstreamReport) ids() []string {
	out := []string{}
	for _, e := range r.Ready {
		out = append(out, e.ID)
	}
	return out
}

// item returns the downstream of the ready item id, failing when id is not ready.
func (r *epicDownstreamReport) item(t *testing.T, id string) (chain, dependents, open int, byMilestone map[string]int) {
	t.Helper()
	for _, e := range r.Ready {
		if e.ID == id {
			d := e.Downstream
			return d.LongestChain, d.Dependents, d.DependentsOpen, d.DependentsByMilestone
		}
	}
	require.Failf(t, "not ready", "%s is not in the ready list %v", id, r.ids())
	return 0, 0, 0, nil
}

// TestEpicReady_OrdersReadyItemsByLongestChainThenDependents is C1.1: r1's
// chain of five puts it first, r2's twelve dependents put it before r3's
// three, r4 with none is last; made equal in chain and dependents, r2 and r3
// fall back to graph order. A ready list in graph order turns it red.
func TestEpicReady_OrdersReadyItemsByLongestChainThenDependents(t *testing.T) {
	t.Parallel()
	r := readyDownstream(t, downstreamDir(t, downstreamEpics(10), nil))
	assert.Equal(t, []string{"r1", "r2", "r3", "r4"}, r.ids())
	chain, dependents, _, _ := r.item(t, "r1")
	assert.Equal(t, 5, chain, "r1's chain counts a1..a5, r1 excluded")
	assert.Equal(t, 5, dependents)
	chain, dependents, _, _ = r.item(t, "r2")
	assert.Equal(t, 2, chain)
	assert.Equal(t, 12, dependents)

	r = readyDownstream(t, downstreamDir(t, downstreamEpics(1), nil))
	assert.Equal(t, []string{"r1", "r3", "r2", "r4"}, r.ids(), "equal chain and dependents keep graph order")
}

// TestEpicReady_ADoneEpicCutsTheDownstream is C1.2: with a2 done, r1's chain
// is a1 alone, and a2 plus a3 and a4, reachable only through it, leave
// dependents; a5, also depending on r1 directly, still counts.
func TestEpicReady_ADoneEpicCutsTheDownstream(t *testing.T) {
	t.Parallel()
	epics := downstreamEpics(10)
	for i := range epics {
		if epics[i].ID == "a2" {
			epics[i].Done = &downstreamDone{At: "2026-01-02T03:04:05Z", Evidence: "e"}
		}
	}
	chain, dependents, _, _ := readyDownstream(t, downstreamDir(t, epics, nil)).item(t, "r1")
	assert.Equal(t, 1, chain)
	assert.Equal(t, 2, dependents, "a1 and a5")
}

