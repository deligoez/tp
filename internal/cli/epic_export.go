package cli

import (
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// epicExportVersion is the schema version of tp epic export's JSON, the
// contract a view of the plan reads (spec/1.5.0.md §7). Renaming, removing or
// retyping a key bumps it; adding one does not. Version 2: a depends_on
// entry may carry any_of in place of id (spec/1.7.0.md §7). The graph keeps
// its own version, the graph file's.
const epicExportVersion = 2

// epicExportResult is tp epic export's JSON: the graph as stored, and every
// field tp derives from it and from the bound files' and slices' state,
// tp epic status's whole-graph critical_path included.
type epicExportResult struct {
	Version      int                     `json:"version"`
	CriticalPath epicPathReport          `json:"critical_path"`
	Graph        *model.EpicGraph        `json:"graph"`
	Milestones   []epicMilestoneReport   `json:"milestones"`
	Epics        []epicExportEpic        `json:"epics"`
	Slices       []engine.EpicSliceState `json:"slices"`
}

// epicExportEpic is one epic's title, place, derived state and bound files, and
// what it waits on and what waits on it (spec/1.6.0.md F9). Downstream is
// absent on a done or moot epic, which nothing waits on; Actionable is a
// condition's key alone, absent on a moot one, false once the condition is
// done, and NotActionable names why an
// undone condition is not actionable (engine.ConditionsNotActionable), absent
// otherwise. HeldBy lists the undone conditions among its direct
// dependencies, BlockedBy every undone direct dependency, both in depends_on
// order.
type epicExportEpic struct {
	ID            string                 `json:"id"`
	Title         string                 `json:"title"`
	Milestone     string                 `json:"milestone"`
	Condition     bool                   `json:"condition"`
	Status        string                 `json:"status"`
	DoneAt        *time.Time             `json:"done_at"`
	Files         []engine.EpicFileState `json:"files"`
	Downstream    *epicExportDownstream  `json:"downstream,omitempty"`
	Actionable    *bool                  `json:"actionable,omitempty"`
	NotActionable []string               `json:"not_actionable,omitempty"`
	HeldBy        []string               `json:"held_by"`
	BlockedBy     []string               `json:"blocked_by"`
	// HeldByAnyOf and BlockedByAnyOf are epicWaits' groups; Blocked and
	// Held say whether anything holds the epic, so no reader combines lists.
	HeldByAnyOf    [][]string `json:"held_by_any_of"`
	BlockedByAnyOf [][]string `json:"blocked_by_any_of"`
	Blocked        bool       `json:"blocked"`
	Held           bool       `json:"held"`
	// Moot marks a moot epic (engine.MootEpics), which carries it in place
	// of actionable and not_actionable.
	Moot bool `json:"moot,omitempty"`
}

// epicExportDownstream is F1's downstream without dependents_by_milestone.
type epicExportDownstream struct {
	LongestChain   int `json:"longest_chain"`
	Dependents     int `json:"dependents"`
	DependentsOpen int `json:"dependents_open"`
}

func newEpicExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Print the epic graph and every derived field as versioned JSON, or one Markdown table per milestone with --md; writes nothing",
		Args:  cobra.NoArgs,
		RunE:  runEpicExport,
	}
	cmd.Flags().Bool("md", false, "Print one Markdown table per milestone, in track order")
	return cmd
}

// runEpicExport prints the graph as JSON, or with --md as Markdown tables
// (spec/1.3.0.md §6); it is read-only and exits 0, or 3 with no readable
// graph, bound file or review state.
func runEpicExport(cmd *cobra.Command, _ []string) error {
	if md, _ := cmd.Flags().GetBool("md"); md {
		runEpicExportMarkdown()
		return nil
	}
	result, err := epicExport()
	if err != nil {
		// task-file-hint: an unreadable bound task file or review state
		output.Error(ExitFile, err.Error(), epicBoundFileHint)
		os.Exit(ExitFile)
	}
	if err := output.JSON(result); err != nil {
		output.Error(ExitFile, err.Error(), internalEncodeHint)
		os.Exit(ExitFile)
	}
	return nil
}

// runEpicExportMarkdown prints the graph's Markdown tables.
func runEpicExportMarkdown() {
	tpDir := engine.DiscoverTPDir(".")
	if tpDir == "" {
		output.Error(ExitFile, "epic graph not found: no .tp/ directory up to the git boundary", epicGraphFileHint)
		os.Exit(ExitFile)
	}
	g, _, err := engine.LoadEpicGraph(tpDir)
	if err != nil {
		output.Error(ExitFile, err.Error(), epicGraphFileHint)
		os.Exit(ExitFile)
	}
	fmt.Print(engine.EpicGraphMarkdown(g))
}

// epicExport assembles the JSON export, deriving every epic's state once.
func epicExport() (*epicExportResult, error) {
	g, states, bound := loadEpicStates()
	tpDir := engine.DiscoverTPDir(".")
	plan, err := epicPlanStatus(g, states, bound, trackedMilestones(g), false)
	if err != nil {
		return nil, err
	}
	files, err := engine.EpicFileStates(tpDir, bound)
	if err != nil {
		return nil, err
	}
	reviewSlices, err := engine.EpicReviewSlices(tpDir, g, epicReviewPolicy(tpDir))
	if err != nil {
		return nil, err
	}
	epics := epicExportEpics(g, states, bound, files)
	return &epicExportResult{Version: epicExportVersion, CriticalPath: plan.CriticalPath, Graph: g, Milestones: plan.Milestones, Epics: epics, Slices: reviewSlices}, nil
}

// epicExportEpics builds every epic's item from one openness verdict, one
// downstream walk and one actionability answer shared by all of them.
func epicExportEpics(g *model.EpicGraph, states map[string]engine.EpicState, bound map[string][]string, files map[string][]engine.EpicFileState) []epicExportEpic {
	gates := engine.MilestoneGates(g, states)
	ids := make([]string, 0, len(g.Epics))
	for i := range g.Epics {
		ids = append(ids, g.Epics[i].ID)
	}
	downs := engine.EpicDownstreams(g, states, gates, ids)
	notActionable := engine.ConditionsNotActionable(g, states, gates, bound)
	moot := engine.MootEpics(g, states)
	conditions := map[string]bool{}
	for i := range g.Epics {
		conditions[g.Epics[i].ID] = g.Epics[i].Condition
	}
	epics := make([]epicExportEpic, 0, len(g.Epics))
	for i := range g.Epics {
		e := &g.Epics[i]
		item := epicExportEpic{ID: e.ID, Title: e.Title, Milestone: e.Milestone, Condition: e.Condition,
			Status: states[e.ID].Status, DoneAt: states[e.ID].DoneAt, Files: files[e.ID]}
		if item.Files == nil {
			item.Files = []engine.EpicFileState{}
		}
		if d, ok := downs[e.ID]; ok {
			item.Downstream = &epicExportDownstream{LongestChain: d.LongestChain, Dependents: d.Dependents, DependentsOpen: d.DependentsOpen}
		}
		switch {
		case moot[e.ID]:
			item.Moot = true
		case e.Condition:
			reasons, undone := notActionable[e.ID]
			yes := undone && len(reasons) == 0
			item.Actionable, item.NotActionable = &yes, reasons
		}
		waits := epicUndoneDeps(e, states, conditions)
		item.BlockedBy, item.HeldBy, item.BlockedByAnyOf, item.HeldByAnyOf = waits.BlockedBy, waits.HeldBy, waits.BlockedByAnyOf, waits.HeldByAnyOf
		item.Blocked, item.Held = waits.blocked(), waits.held()
		epics = append(epics, item)
	}
	return epics
}

// epicUndoneDeps returns what e's direct dependencies hold it on: each
// undone plain dependency once, in depends_on order, the conditions among
// them, and each unsatisfied group. An id the graph does not hold is not
// done.
func epicUndoneDeps(e *model.Epic, states map[string]engine.EpicState, conditions map[string]bool) epicWaits {
	w := epicWaits{BlockedBy: []string{}, HeldBy: []string{}, BlockedByAnyOf: [][]string{}, HeldByAnyOf: [][]string{}}
	seen := map[string]bool{}
	for _, d := range e.DependsOn {
		if d.IsGroup() {
			w.addGroup(d, states, conditions)
			continue
		}
		if seen[d.ID] || states[d.ID].Status == engine.EpicStatusDone {
			continue
		}
		seen[d.ID] = true
		w.BlockedBy = append(w.BlockedBy, d.ID)
		if conditions[d.ID] {
			w.HeldBy = append(w.HeldBy, d.ID)
		}
	}
	return w
}

// epicWaits is what an epic's direct depends_on entries hold it on
// (spec/1.7.0.md §7): its undone plain dependencies and the conditions among
// them, and each unsatisfied any-of group's members, in held when every
// member is a condition. Every list is in depends_on order.
type epicWaits struct {
	BlockedBy      []string
	HeldBy         []string
	BlockedByAnyOf [][]string
	HeldByAnyOf    [][]string
}

// addGroup adds the group d to w unless one of its members is done.
func (w *epicWaits) addGroup(d model.EpicDep, states map[string]engine.EpicState, conditions map[string]bool) {
	members := d.Members()
	if slices.ContainsFunc(members, func(m string) bool { return states[m].Status == engine.EpicStatusDone }) {
		return
	}
	w.BlockedByAnyOf = append(w.BlockedByAnyOf, members)
	if !slices.ContainsFunc(members, func(m string) bool { return !conditions[m] }) {
		w.HeldByAnyOf = append(w.HeldByAnyOf, members)
	}
}

// blocked reports whether anything holds the epic.
func (w *epicWaits) blocked() bool {
	return len(w.BlockedBy) > 0 || len(w.BlockedByAnyOf) > 0
}

// held reports whether a condition, or a group of conditions, holds the epic.
func (w *epicWaits) held() bool {
	return len(w.HeldBy) > 0 || len(w.HeldByAnyOf) > 0
}
