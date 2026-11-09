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
	Ready    []epicReadyItem   `json:"ready"`
	Claimed  []epicClaimedEpic `json:"claimed"`
	Operator epicOperatorWait  `json:"operator"`
}

// epicReadyEpic is one listed epic.
type epicReadyEpic struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Milestone string `json:"milestone"`
}

// epicReadyItem is one ready epic with what waits on it (spec/1.6.0.md F1).
type epicReadyItem struct {
	epicReadyEpic
	Downstream epicDownstreamCounts `json:"downstream"`
}

// epicDownstreamCounts is engine.EpicDownstream's payload.
type epicDownstreamCounts struct {
	LongestChain          int                       `json:"longest_chain"`
	Dependents            int                       `json:"dependents"`
	DependentsOpen        int                       `json:"dependents_open"`
	DependentsByMilestone []epicMilestoneDependents `json:"dependents_by_milestone"`
}

// epicMilestoneDependents is engine.MilestoneDependents' payload.
type epicMilestoneDependents struct {
	Milestone  string `json:"milestone"`
	Dependents int    `json:"dependents"`
}

// epicDownstreamPayload converts d to its payload.
func epicDownstreamPayload(d engine.EpicDownstream) epicDownstreamCounts {
	by := make([]epicMilestoneDependents, 0, len(d.DependentsByMilestone))
	for _, m := range d.DependentsByMilestone {
		by = append(by, epicMilestoneDependents(m))
	}
	return epicDownstreamCounts{LongestChain: d.LongestChain, Dependents: d.Dependents, DependentsOpen: d.DependentsOpen, DependentsByMilestone: by}
}

// epicClaimedEpic is an epic that would be ready but for its claim, with the
// claim's holder.
type epicClaimedEpic struct {
	epicReadyEpic
	By string    `json:"by"`
	At time.Time `json:"at"`
}

// epicOperatorWait is what waits on the operator: conditions to decide (the
// actionable ones, every one not done with --all) and gates whose threshold
// is to be measured.
type epicOperatorWait struct {
	Conditions []epicOperatorCondition `json:"conditions"`
	Gates      []epicThresholdWait     `json:"gates"`
}

// epicOperatorCondition is one condition with whether deciding it can
// unblock work now (spec/1.6.0.md F3).
type epicOperatorCondition struct {
	epicReadyEpic
	Actionable bool `json:"actionable"`
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
	cmd.Flags().Bool("all", false, "list epics of milestones that are not open yet too, and every condition not done, actionable or not")
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
		Ready:    make([]epicReadyItem, 0, len(r.Ready)),
		Claimed:  make([]epicClaimedEpic, 0, len(r.Claimed)),
		Operator: epicOperatorWait{Conditions: make([]epicOperatorCondition, 0, len(r.Conditions)), Gates: make([]epicThresholdWait, 0, len(r.Gates))},
	}
	for _, id := range r.Ready {
		result.Ready = append(result.Ready, epicReadyItem{epicReadyEpic: entry(id), Downstream: epicDownstreamPayload(r.Downstream[id])})
	}
	for _, id := range r.Claimed {
		c := byID[id].Claimed
		result.Claimed = append(result.Claimed, epicClaimedEpic{epicReadyEpic: entry(id), By: c.By, At: c.At})
	}
	for _, id := range r.Conditions {
		result.Operator.Conditions = append(result.Operator.Conditions, epicOperatorCondition{epicReadyEpic: entry(id), Actionable: r.Actionable[id]})
	}
	for _, m := range r.Gates {
		result.Operator.Gates = append(result.Operator.Gates, epicThresholdWait{Milestone: m, Threshold: g.Gates[m].Threshold})
	}
	return result
}
