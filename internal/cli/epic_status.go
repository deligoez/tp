package cli

import (
	"os"
	"slices"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// epicPlanResult is tp epic status's payload (spec/1.5.0.md §7, C1): one
// report per tracked milestone, in track order.
type epicPlanResult struct {
	Milestones []epicMilestoneReport `json:"milestones"`
}

// epicMilestoneReport is one milestone's state. Throughput, Rollup and the
// critical path's estimate chain are nil under --compact and then left out.
type epicMilestoneReport struct {
	Milestone    string            `json:"milestone"`
	Open         bool              `json:"open"`
	Counts       map[string]int    `json:"counts"`
	ShareDone    float64           `json:"share_done"`
	Gate         epicGateResult    `json:"gate"`
	Ready        []epicReadyEpic   `json:"ready"`
	Claimed      []epicClaimedEpic `json:"claimed"`
	Blocked      []epicBlockedEpic `json:"blocked"`
	Operator     epicOperatorWait  `json:"operator"`
	CriticalPath epicPathReport    `json:"critical_path"`
	Throughput   *[]epicWeekRow    `json:"throughput,omitempty"`
	Rollup       *[]epicRollupRow  `json:"rollup,omitempty"`
}

// epicBlockedEpic is a not-done epic with a dependency that is not done,
// naming each such dependency.
type epicBlockedEpic struct {
	epicReadyEpic
	BlockedBy []string `json:"blocked_by"`
}

// epicPathReport is the milestone's critical path: the longest chain by
// count, and the heaviest by summed estimate unless --compact drops it.
type epicPathReport struct {
	ByCount    []string           `json:"by_count"`
	ByEstimate *epicEstimateChain `json:"by_estimate,omitempty"`
}

// epicEstimateChain is the heaviest chain by summed estimate; Chain is null
// when an epic on some chain has no estimate.
type epicEstimateChain struct {
	Chain           []string `json:"chain"`
	EstimateMinutes int      `json:"estimate_minutes"`
}

// epicWeekRow is one ISO week's count of epics done.
type epicWeekRow struct {
	Week string `json:"week"`
	Done int    `json:"done"`
}

func newEpicStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Report each milestone: open, counts and share done, gate, ready, blocked with blockers, operator waits, claim holders, critical path, throughput, roll-up; writes nothing",
		Args:  cobra.NoArgs,
		RunE:  runEpicStatus,
	}
	cmd.Flags().String("milestone", "", "report only this milestone")
	return cmd
}

// runEpicStatus reads the graph and its bound files, derives every epic's
// state once, writes nothing, and exits 0; 3 with no readable graph or bound
// file, 4 for a --milestone no track lists. --compact leaves out the estimate
// chain, throughput and the roll-up.
func runEpicStatus(cmd *cobra.Command, _ []string) error {
	only, _ := cmd.Flags().GetString("milestone")
	g, states, bound := loadEpicStates()
	milestones := trackedMilestones(g)
	if only != "" {
		if !slices.Contains(milestones, only) {
			output.Error(ExitState, "milestone "+only+" is in no track of .tp/epics.json", epicMilestoneNotFoundHint)
			os.Exit(ExitState)
		}
		milestones = []string{only}
	}
	result, err := epicPlanStatus(g, states, bound, milestones, flagCompact)
	if err != nil {
		// task-file-hint: an unreadable bound task file
		output.Error(ExitFile, err.Error(), epicBoundFileHint)
		os.Exit(ExitFile)
	}
	if err := output.JSON(result); err != nil {
		output.Error(ExitFile, err.Error(), internalEncodeHint)
		os.Exit(ExitFile)
	}
	return nil
}

// trackedMilestones lists every milestone a track names once, in track order.
func trackedMilestones(g *model.EpicGraph) []string {
	out := []string{}
	for _, track := range g.Milestones {
		for _, m := range track {
			if !slices.Contains(out, m) {
				out = append(out, m)
			}
		}
	}
	return out
}

// epicPlan is what every milestone's report reads, derived once per call.
type epicPlan struct {
	g         *model.EpicGraph
	states    map[string]engine.EpicState
	bound     map[string][]string
	gates     map[string]engine.MilestoneGate
	ready     epicReadyResult
	estimates map[string]int
	compact   bool
}

// epicPlanStatus reports each of milestones over states, DeriveEpicStates'
// result, and bound, EpicBoundFiles'. A bound file that cannot be read is an
// error.
func epicPlanStatus(g *model.EpicGraph, states map[string]engine.EpicState, bound map[string][]string, milestones []string, compact bool) (epicPlanResult, error) {
	readiness := engine.EpicReady(g, states, bound, false)
	p := epicPlan{g: g, states: states, bound: bound, gates: engine.MilestoneGates(g, states), ready: epicReadyReport(g, &readiness), compact: compact}
	if !compact {
		estimates, err := engine.EpicEstimates(g, bound)
		if err != nil {
			return epicPlanResult{}, err
		}
		p.estimates = estimates
	}
	out := epicPlanResult{Milestones: make([]epicMilestoneReport, 0, len(milestones))}
	for _, m := range milestones {
		rep, err := p.milestone(m)
		if err != nil {
			return epicPlanResult{}, err
		}
		out.Milestones = append(out.Milestones, rep)
	}
	return out, nil
}

// milestone builds m's report.
func (p *epicPlan) milestone(m string) (epicMilestoneReport, error) {
	gate := epicGateReport(p.g, m, p.states)
	gate.Green = p.gates[m].Green
	rep := epicMilestoneReport{
		Milestone: m,
		Open:      p.gates[m].Open,
		Gate:      gate,
		Ready:     inMilestone(p.ready.Ready, m, func(e epicReadyEpic) string { return e.Milestone }),
		Operator: epicOperatorWait{
			Conditions: inMilestone(p.ready.Operator.Conditions, m, func(e epicReadyEpic) string { return e.Milestone }),
			Gates:      inMilestone(p.ready.Operator.Gates, m, func(w epicThresholdWait) string { return w.Milestone }),
		},
	}
	rep.Counts, rep.ShareDone = p.counts(m)
	rep.Claimed, rep.Blocked = p.claimedAndBlocked(m)
	path := engine.CriticalPath(p.g, p.states, p.estimates, m)
	rep.CriticalPath = epicPathReport{ByCount: path.ByCount}
	if p.compact {
		return rep, nil
	}
	rep.CriticalPath.ByEstimate = &epicEstimateChain{Chain: path.ByEstimate, EstimateMinutes: path.EstimateMinutes}
	weeks := []epicWeekRow{}
	for _, w := range engine.EpicThroughput(p.g, p.states, m) {
		weeks = append(weeks, epicWeekRow{Week: w.Week, Done: w.Done})
	}
	rollup, err := epicRollup(p.g, p.bound, m)
	if err != nil {
		return epicMilestoneReport{}, err
	}
	rep.Throughput, rep.Rollup = &weeks, &rollup
	return rep, nil
}

// epicCountConditionsDone is the counts key of m's done conditions, reported
// apart from the status counts (spec/1.6.0.md B2).
const epicCountConditionsDone = "conditions_done"

// counts returns m's work epics counted by derived status, every status
// present, and its done conditions under conditions_done, apart: a condition
// is the operator's precondition, not delivered work. shareDone is the share of
// the work epics done, rounded to two places; 0 for a milestone with none.
func (p *epicPlan) counts(m string) (counts map[string]int, shareDone float64) {
	counts = map[string]int{
		engine.EpicStatusPlanned: 0, engine.EpicStatusClaimed: 0, engine.EpicStatusSpecced: 0,
		engine.EpicStatusInProgress: 0, engine.EpicStatusDone: 0, epicCountConditionsDone: 0,
	}
	total := 0
	for i := range p.g.Epics {
		e := &p.g.Epics[i]
		status := p.states[e.ID].Status
		switch {
		case e.Milestone != m:
		case e.Condition:
			if status == engine.EpicStatusDone {
				counts[epicCountConditionsDone]++
			}
		default:
			counts[status]++
			total++
		}
	}
	if total == 0 {
		return counts, 0
	}
	return counts, roundTo(float64(counts[engine.EpicStatusDone])/float64(total), 2)
}

// claimedAndBlocked lists m's epics whose derived status is claimed, each
// with its holder, and its not-done epics with a dependency that is not done,
// each with those dependencies, in graph order.
func (p *epicPlan) claimedAndBlocked(m string) (claimed []epicClaimedEpic, blocked []epicBlockedEpic) {
	claimed, blocked = []epicClaimedEpic{}, []epicBlockedEpic{}
	for i := range p.g.Epics {
		e := &p.g.Epics[i]
		if e.Milestone != m {
			continue
		}
		entry := epicReadyEpic{ID: e.ID, Title: e.Title, Milestone: e.Milestone}
		status := p.states[e.ID].Status
		if status == engine.EpicStatusClaimed {
			claimed = append(claimed, epicClaimedEpic{epicReadyEpic: entry, By: e.Claimed.By, At: e.Claimed.At})
		}
		if status == engine.EpicStatusDone {
			continue
		}
		var by []string
		for _, dep := range e.DependsOn {
			if p.states[dep.ID].Status != engine.EpicStatusDone {
				by = append(by, dep.ID)
			}
		}
		if len(by) > 0 {
			blocked = append(blocked, epicBlockedEpic{epicReadyEpic: entry, BlockedBy: by})
		}
	}
	return claimed, blocked
}

// inMilestone keeps the items of milestone m, in their order.
func inMilestone[T any](items []T, m string, milestoneOf func(T) string) []T {
	out := make([]T, 0, len(items))
	for _, it := range items {
		if milestoneOf(it) == m {
			out = append(out, it)
		}
	}
	return out
}
