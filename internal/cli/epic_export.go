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
// field tp derives from it and from the bound files' and slices' state.
type epicExportResult struct {
	Version    int                     `json:"version"`
	Graph      *model.EpicGraph        `json:"graph"`
	Milestones []epicMilestoneReport   `json:"milestones"`
	Epics      []epicExportEpic        `json:"epics"`
	Slices     []engine.EpicSliceState `json:"slices"`
}

// epicExportEpic is one epic's derived state and its bound files.
type epicExportEpic struct {
	ID     string                 `json:"id"`
	Status string                 `json:"status"`
	DoneAt *time.Time             `json:"done_at"`
	Files  []engine.EpicFileState `json:"files"`
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
	epics := make([]epicExportEpic, 0, len(g.Epics))
	for i := range g.Epics {
		id := g.Epics[i].ID
		epicFiles := files[id]
		if epicFiles == nil {
			epicFiles = []engine.EpicFileState{}
		}
		epics = append(epics, epicExportEpic{ID: id, Status: states[id].Status, DoneAt: states[id].DoneAt, Files: epicFiles})
	}
	return &epicExportResult{Version: epicExportVersion, Graph: g, Milestones: plan.Milestones, Epics: epics, Slices: slices}, nil
}
