package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// epicBoundFileHint answers a bound task file whose state cannot be read.
const epicBoundFileHint = "fix the bound task file's JSON or its review state, or unbind it with tp set --epic \"\""

// epicGateResult is tp epic gate's payload (spec/1.5.0.md §7).
type epicGateResult struct {
	Milestone        string           `json:"milestone"`
	Green            bool             `json:"green"`
	Members          []epicGateMember `json:"members"`
	Deferrable       []epicGateEpic   `json:"deferrable"`
	Threshold        string           `json:"threshold,omitempty"`
	Met              *model.GateMet   `json:"met,omitempty"`
	ThresholdChanged bool             `json:"threshold_changed,omitempty"`
}

// epicGateMember is one member of the gate: green when it is done.
type epicGateMember struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Status string `json:"status"`
}

// epicGateEpic is one deferrable epic, which never holds the gate.
type epicGateEpic struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func newEpicGateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "gate <milestone>",
		Short: "Report a milestone's gate: each member green or red, the deferrable epics apart, the threshold and its met record; exit 0 only when green",
		Args:  cobra.ExactArgs(1),
		RunE:  runEpicGate,
	}
}

// runEpicGate reads the graph and its bound files, writes nothing, and exits
// 0 when the milestone's gate is green, 1 when it is red, 3 with no readable
// graph or bound file, and 4 for a milestone no track lists.
func runEpicGate(_ *cobra.Command, args []string) error {
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
	states, err := engine.DeriveEpicStates(g, engine.EpicBoundFiles(tpDir))
	if err != nil {
		output.Error(ExitFile, err.Error(), epicBoundFileHint)
		os.Exit(ExitFile)
	}
	m := args[0]
	verdict, tracked := engine.MilestoneGates(g, states)[m]
	if !tracked {
		output.Error(ExitState, "milestone "+m+" is in no track of .tp/epics.json", epicMilestoneNotFoundHint)
		os.Exit(ExitState)
	}
	result := epicGateReport(g, m, states)
	result.Green = verdict.Green
	if err := output.JSON(result); err != nil {
		output.Error(ExitFile, err.Error(), internalEncodeHint)
		os.Exit(ExitFile)
	}
	if !result.Green {
		os.Exit(ExitValidation)
	}
	return nil
}

// epicGateReport lists m's members and deferrable epics with their derived
// status, and its threshold and met record. A threshold that differs from the
// one its met record copied shows as changed; the record stands.
func epicGateReport(g *model.EpicGraph, m string, states map[string]engine.EpicState) epicGateResult {
	gate := g.Gates[m]
	members, deferrable := engine.GateMembers(g, m)
	result := epicGateResult{
		Milestone:  m,
		Members:    make([]epicGateMember, 0, len(members)),
		Deferrable: make([]epicGateEpic, 0, len(deferrable)),
		Threshold:  gate.Threshold,
		Met:        gate.Met,
	}
	for _, id := range members {
		status := states[id].Status
		state := "red"
		if status == engine.EpicStatusDone {
			state = "green"
		}
		result.Members = append(result.Members, epicGateMember{ID: id, State: state, Status: status})
	}
	for _, id := range deferrable {
		result.Deferrable = append(result.Deferrable, epicGateEpic{ID: id, Status: states[id].Status})
	}
	result.ThresholdChanged = gate.Met != nil && gate.Met.Threshold != gate.Threshold
	return result
}
