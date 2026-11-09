package cli

import (
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// epicReadyResult is tp epic ready's payload (spec/1.5.0.md §7, C1).
type epicReadyResult struct {
	Ready    []epicReadyEpic   `json:"ready"`
	Claimed  []epicClaimedEpic `json:"claimed"`
	Operator epicOperatorWait  `json:"operator"`
}

// epicReadyEpic is one listed epic.
type epicReadyEpic struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Milestone string `json:"milestone"`
}

// epicClaimedEpic is an epic that would be ready but for its claim, with the
// claim's holder.
type epicClaimedEpic struct {
	epicReadyEpic
	By string    `json:"by"`
	At time.Time `json:"at"`
}

// epicOperatorWait is what waits on the operator: conditions to decide and
// gates whose threshold is to be measured.
type epicOperatorWait struct {
	Conditions []epicReadyEpic     `json:"conditions"`
	Gates      []epicThresholdWait `json:"gates"`
}

// epicThresholdWait is one gate whose members are done and whose threshold
// has no met record.
type epicThresholdWait struct {
	Milestone string `json:"milestone"`
	Threshold string `json:"threshold"`
}

func newEpicReadyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ready",
		Short: "List the work epics that can start, the claimed ones with their holder, and what waits on the operator; --all lifts the open-milestone rule",
		Args:  cobra.NoArgs,
		RunE:  runEpicReady,
	}
	cmd.Flags().Bool("all", false, "list epics of milestones that are not open yet too")
	return cmd
}

// runEpicReady reads the graph and its bound files, writes nothing, and exits
// 0, or 3 with no readable graph or bound file.
func runEpicReady(cmd *cobra.Command, _ []string) error {
	all, _ := cmd.Flags().GetBool("all")
	g, states, bound := loadEpicStates()
	readiness := engine.EpicReady(g, states, bound, all)
	if err := output.JSON(epicReadyReport(g, &readiness)); err != nil {
		output.Error(ExitFile, err.Error(), internalEncodeHint)
		os.Exit(ExitFile)
	}
	return nil
}

// epicReadyReport turns r's ids into the payload's entries.
func epicReadyReport(g *model.EpicGraph, r *engine.EpicReadiness) epicReadyResult {
	byID := make(map[string]*model.Epic, len(g.Epics))
	for i := range g.Epics {
		byID[g.Epics[i].ID] = &g.Epics[i]
	}
	entry := func(id string) epicReadyEpic {
		e := byID[id]
		return epicReadyEpic{ID: e.ID, Title: e.Title, Milestone: e.Milestone}
	}
	result := epicReadyResult{
		Ready:    make([]epicReadyEpic, 0, len(r.Ready)),
		Claimed:  make([]epicClaimedEpic, 0, len(r.Claimed)),
		Operator: epicOperatorWait{Conditions: make([]epicReadyEpic, 0, len(r.Conditions)), Gates: make([]epicThresholdWait, 0, len(r.Gates))},
	}
	for _, id := range r.Ready {
		result.Ready = append(result.Ready, entry(id))
	}
	for _, id := range r.Claimed {
		c := byID[id].Claimed
		result.Claimed = append(result.Claimed, epicClaimedEpic{epicReadyEpic: entry(id), By: c.By, At: c.At})
	}
	for _, id := range r.Conditions {
		result.Operator.Conditions = append(result.Operator.Conditions, entry(id))
	}
	for _, m := range r.Gates {
		result.Operator.Gates = append(result.Operator.Gates, epicThresholdWait{Milestone: m, Threshold: g.Gates[m].Threshold})
	}
	return result
}
