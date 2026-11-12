package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// epicScopeResult is tp epic scope's payload (spec/backlog/epic-scope.md §1,
// Part A): the engine's scope and the milestone's critical path in tp epic
// status's shape.
type epicScopeResult struct {
	*engine.EpicScope
	CriticalPath epicPathReport `json:"critical_path"`
}

// epicScopeCompact is the payload under --compact, a strict subset of the
// full one: the fingerprint first, the purpose (null when unset), the
// members' mode and count with their list only for an explicit gate, the
// closure's size per milestone, the shrink rows without leaves,
// deferrable_in_closure when not empty, and the critical path by count. It
// drops the closure's epic lists, outside, unneeded_earlier, an implicit
// gate's member list and the path by estimate.
type epicScopeCompact struct {
	Fingerprint     string                      `json:"fingerprint"`
	Milestone       string                      `json:"milestone"`
	Purpose         *string                     `json:"purpose"`
	Members         []string                    `json:"members,omitempty"`
	MembersImplicit bool                        `json:"members_implicit"`
	MemberCount     int                         `json:"member_count"`
	Closure         []epicScopeCompactMilestone `json:"closure"`
	Shrink          []epicScopeCompactShrink    `json:"shrink"`
	Deferrable      []string                    `json:"deferrable_in_closure,omitempty"`
	CriticalPath    epicPathReport              `json:"critical_path"`
}

// epicScopeCompactMilestone is one closure milestone without its epics.
type epicScopeCompactMilestone struct {
	Milestone  string `json:"milestone"`
	Track      *int   `json:"track,omitempty"`
	CrossTrack bool   `json:"cross_track,omitempty"`
	Size       int    `json:"size"`
}

// epicScopeCompactShrink is one shrink row without its leaves.
type epicScopeCompactShrink struct {
	Epic   string   `json:"epic"`
	Member bool     `json:"member"`
	AnyOf  []string `json:"any_of,omitempty"`
	Size   int      `json:"size"`
}

func newEpicScopeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "scope <milestone>",
		Short: "Report what a milestone's gate pulls in; writes nothing",
		Args:  cobra.ExactArgs(1),
		RunE:  runEpicScope,
	}
}

// runEpicScope reads the graph and its bound files, writes nothing, and
// exits 0; 3 with no readable graph or bound file, 4 for a milestone no track
// lists. It is read-only, so TP_UNATTENDED does not fence it.
func runEpicScope(_ *cobra.Command, args []string) error {
	g, states, bound := loadEpicStates()
	m := args[0]
	if _, tracked := engine.MilestoneGates(g, states)[m]; !tracked {
		output.Error(ExitState, engine.NewEpicMilestoneNotFoundError(g, m).Error(), epicMilestoneNotFoundHint)
		os.Exit(ExitState)
	}
	var estimates map[string]int
	if !flagCompact {
		var err error
		if estimates, err = engine.EpicEstimates(g, bound); err != nil {
			// task-file-hint: an unreadable bound task file
			output.Error(ExitFile, err.Error(), epicBoundFileHint)
			os.Exit(ExitFile)
		}
	}
	scope := engine.MilestoneScope(g, states, estimates, m)
	var result any = epicScopeResult{EpicScope: &scope, CriticalPath: epicPathReport{ByCount: scope.CriticalPath.ByCount,
		ByEstimate: &epicEstimateChain{Chain: scope.CriticalPath.ByEstimate, EstimateMinutes: scope.CriticalPath.EstimateMinutes}}}
	if flagCompact {
		result = epicScopeCompactOf(&scope)
	}
	if err := output.JSON(result); err != nil {
		output.Error(ExitFile, err.Error(), internalEncodeHint)
		os.Exit(ExitFile)
	}
	return nil
}

// epicScopeCompactOf projects scope onto its --compact payload, a strict
// subset of the full one.
func epicScopeCompactOf(scope *engine.EpicScope) epicScopeCompact {
	out := epicScopeCompact{
		Fingerprint:     scope.Fingerprint,
		Milestone:       scope.Milestone,
		Purpose:         scope.Purpose,
		MembersImplicit: scope.MembersImplicit,
		MemberCount:     scope.MemberCount,
		Closure:         make([]epicScopeCompactMilestone, 0, len(scope.Closure)),
		Shrink:          make([]epicScopeCompactShrink, 0, len(scope.Shrink)),
		CriticalPath:    epicPathReport{ByCount: scope.CriticalPath.ByCount},
		Deferrable:      scope.DeferrableInClosure,
	}
	if !scope.MembersImplicit {
		out.Members = scope.Members
	}
	for _, m := range scope.Closure {
		out.Closure = append(out.Closure, epicScopeCompactMilestone{Milestone: m.Milestone, Track: m.Track, CrossTrack: m.CrossTrack, Size: m.Size})
	}
	for _, r := range scope.Shrink {
		out.Shrink = append(out.Shrink, epicScopeCompactShrink{Epic: r.Epic, Member: r.Member, AnyOf: r.AnyOf, Size: r.Size})
	}
	return out
}
