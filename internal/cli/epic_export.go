package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// epicExportVersion is the schema version of tp epic export's JSON, the
// contract a view of the plan reads (spec/1.5.0.md §7). Renaming, removing or
// retyping a key bumps it; adding one does not.
const epicExportVersion = 1

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
// absent on a done epic, which nothing waits on; Actionable is a condition's
// key alone, false once the condition is done, and NotActionable names why an
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
	slices, err := engine.EpicReviewSlices(tpDir, g, epicReviewPolicy(tpDir))
	if err != nil {
		return nil, err
	}
	epics := epicExportEpics(g, states, bound, files)
	return &epicExportResult{Version: epicExportVersion, CriticalPath: plan.CriticalPath, Graph: g, Milestones: plan.Milestones, Epics: epics, Slices: slices}, nil
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
		if e.Condition {
			reasons, undone := notActionable[e.ID]
			yes := undone && len(reasons) == 0
			item.Actionable, item.NotActionable = &yes, reasons
		}
		item.BlockedBy, item.HeldBy = epicUndoneDeps(e, states, conditions)
		epics = append(epics, item)
	}
	return epics
}

// epicUndoneDeps returns e's undone direct dependencies, each once in
// depends_on order, and the conditions among them. An id the graph does not
// hold is not done.
func epicUndoneDeps(e *model.Epic, states map[string]engine.EpicState, conditions map[string]bool) (blockedBy, heldBy []string) {
	blockedBy, heldBy = []string{}, []string{}
	seen := map[string]bool{}
	for _, d := range e.DependsOn {
		if seen[d.ID] || states[d.ID].Status == engine.EpicStatusDone {
			continue
		}
		seen[d.ID] = true
		blockedBy = append(blockedBy, d.ID)
		if conditions[d.ID] {
			heldBy = append(heldBy, d.ID)
		}
	}
	return blockedBy, heldBy
}
