package cli

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// epicPlanResult is tp epic status's payload (spec/1.5.0.md §7, C1): the
// whole graph's critical path (spec/1.6.0.md F1), by count only, then one
// report per tracked milestone, in track order.
type epicPlanResult struct {
	CriticalPath epicPathReport        `json:"critical_path"`
	Milestones   []epicMilestoneReport `json:"milestones"`
}

// epicMilestoneReport is one milestone's state. Throughput, Rollup and the
// critical path are nil under --compact and then left out: the top-level
// critical_path carries the chain once.
type epicMilestoneReport struct {
	Milestone    string            `json:"milestone"`
	Open         bool              `json:"open"`
	OpensAfter   []string          `json:"opens_after"`
	WaitingOn    []string          `json:"waiting_on"`
	Counts       map[string]int    `json:"counts"`
	ShareDone    float64           `json:"share_done"`
	Gate         epicGateResult    `json:"gate"`
	Ready        []epicReadyItem   `json:"ready"`
	Claimed      []epicClaimedEpic `json:"claimed"`
	Blocked      []epicBlockedEpic `json:"blocked"`
	Operator     epicOperatorWait  `json:"operator"`
	CriticalPath *epicPathReport   `json:"critical_path,omitempty"`
	Throughput   *[]epicWeekRow    `json:"throughput,omitempty"`
	Rollup       *[]epicRollupRow  `json:"rollup,omitempty"`
}

// epicBlockedEpic is a not-done epic with a dependency that is not done,
// naming each such dependency, and the conditions among them, as export's
// epics do: held_by alone tells a wait on the operator from one on work. A
// blocked condition also carries not_actionable, why it is not actionable
// (engine.ConditionsNotActionable).
type epicBlockedEpic struct {
	epicReadyEpic
	HeldBy        []string `json:"held_by"`
	BlockedBy     []string `json:"blocked_by"`
	NotActionable []string `json:"not_actionable,omitempty"`
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
		Short: "Report the whole graph's critical_path, then each milestone: open, opens_after and waiting_on, counts and share done, gate, ready by downstream rank, blocked with held_by and blocked_by, operator waits, claim holders, critical path, throughput, roll-up; writes nothing",
		Args:  cobra.NoArgs,
		RunE:  runEpicStatus,
	}
	cmd.Flags().String("milestone", "", "report only this milestone")
	cmd.Flags().Bool("open", false, "report only open milestones (the whole graph's critical_path is unchanged)")
	return cmd
}

// runEpicStatus reads the graph and its bound files, derives every epic's
// state once, writes nothing, and exits 0; 3 with no readable graph or bound
// file, 4 for a --milestone no track lists, or a closed one with --open.
// --open keeps the open milestones only (spec/1.6.0.md F7). --compact
// leaves out each milestone's critical path, throughput and the roll-up.
func runEpicStatus(cmd *cobra.Command, _ []string) error {
	only, _ := cmd.Flags().GetString("milestone")
	g, states, bound := loadEpicStates()
	milestones := trackedMilestones(g)
	if only != "" {
		if !slices.Contains(milestones, only) {
			output.Error(ExitState, engine.NewEpicMilestoneNotFoundError(g, only).Error(), epicMilestoneNotFoundHint)
			os.Exit(ExitState)
		}
		milestones = []string{only}
	}
	if open, _ := cmd.Flags().GetBool("open"); open {
		milestones = openMilestonesOrExit(g, states, milestones, only)
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

// openMilestonesOrExit keeps the open milestones of milestones. A milestone
// --milestone named that is closed exits 4 saying what it waits on: an empty
// report would read as a milestone with nothing in it.
func openMilestonesOrExit(g *model.EpicGraph, states map[string]engine.EpicState, milestones []string, only string) []string {
	gates := engine.MilestoneGates(g, states)
	if only != "" && !gates[only].Open {
		waits := engine.MilestoneWaitingOn(engine.EffectiveOpensAfter(g)[only], gates)
		output.Error(ExitState, fmt.Sprintf("milestone %q is closed: it waits on %s", only, strings.Join(waits, ", ")),
			"drop --open to report it, or tp epic status --open for the open milestones")
		os.Exit(ExitState)
	}
	return slices.DeleteFunc(milestones, func(m string) bool { return !gates[m].Open })
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
	after     map[string][]string
	ready     epicReadyResult
	estimates map[string]int
	// conditions marks each epic id that is a condition.
	conditions map[string]bool
	// notActionable holds each undone condition's reasons it is not
	// actionable, empty for an actionable one.
	notActionable map[string][]string
	compact       bool
}

// epicPlanStatus reports each of milestones over states, DeriveEpicStates'
// result, and bound, EpicBoundFiles'. A bound file that cannot be read is an
// error.
func epicPlanStatus(g *model.EpicGraph, states map[string]engine.EpicState, bound map[string][]string, milestones []string, compact bool) (epicPlanResult, error) {
	readiness := engine.EpicReady(g, states, bound, false)
	p := epicPlan{g: g, states: states, bound: bound, gates: engine.MilestoneGates(g, states), after: engine.EffectiveOpensAfter(g), ready: epicReadyReport(g, &readiness, compact), conditions: map[string]bool{}, notActionable: readiness.NotActionable, compact: compact}
	for i := range g.Epics {
		p.conditions[g.Epics[i].ID] = g.Epics[i].Condition
	}
	if !compact {
		estimates, err := engine.EpicEstimates(g, bound)
		if err != nil {
			return epicPlanResult{}, err
		}
		p.estimates = estimates
	}
	out := epicPlanResult{
		CriticalPath: epicPathReport{ByCount: engine.GraphCriticalPath(g, states)},
		Milestones:   make([]epicMilestoneReport, 0, len(milestones)),
	}
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
		Milestone:  m,
		Open:       p.gates[m].Open,
		OpensAfter: p.after[m],
		WaitingOn:  engine.MilestoneWaitingOn(p.after[m], p.gates),
		Gate:       gate,
		Ready:      inMilestone(p.ready.Ready, m, func(e epicReadyItem) string { return e.Milestone }),
		Operator: epicOperatorWait{
			Conditions: inMilestone(p.ready.Operator.Conditions, m, func(e epicOperatorCondition) string { return e.Milestone }),
			Gates:      inMilestone(p.ready.Operator.Gates, m, func(w epicThresholdWait) string { return w.Milestone }),
		},
	}
	rep.Counts, rep.ShareDone = p.counts(m)
	rep.Claimed, rep.Blocked = p.claimedAndBlocked(m)
	if p.compact {
		return rep, nil
	}
	path := engine.CriticalPath(p.g, p.states, p.estimates, m)
	rep.CriticalPath = &epicPathReport{ByCount: path.ByCount,
		ByEstimate: &epicEstimateChain{Chain: path.ByEstimate, EstimateMinutes: path.EstimateMinutes}}
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
		if by, held := epicUndoneDeps(e, p.states, p.conditions); len(by) > 0 {
			blocked = append(blocked, epicBlockedEpic{epicReadyEpic: entry, HeldBy: held, BlockedBy: by, NotActionable: p.notActionable[e.ID]})
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
