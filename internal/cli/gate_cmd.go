package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// gateRerunHint is what a red tp gate tells its caller.
const gateRerunHint = "fix the failure, commit the fix, and run tp gate again"

func newGateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "gate",
		Short: "Run the quality gate on this checkout, closing nothing",
		Long: `Run the resolved quality_gate on the current checkout and report the result.
Nothing is closed and no task changes.

In a unit's worktree the gate runs in the worktree, as that unit's tp done
would run it. Inside tp run the gate first takes one of the run's gate slots
(run_max_parallel_gates); outside a run it runs at once. A pass inside a run
on a clean checkout is recorded, so the unit's following tp done on the same
tree with the same gate command does not run the gate again.

Exit 0 when the gate passes, 4 when it fails, 2 when no quality_gate is set.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runGate()
		},
	}
}

func runGate() error {
	taskFilePath, err := engine.DiscoverTaskFile(".", flagFile)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
	}
	wf := engine.EffectiveWorkflowForTaskFile(taskFilePath)
	if wf.QualityGate == "" {
		output.Error(ExitUsage, "no quality_gate is set, so there is no gate to run",
			"set one with tp set --workflow --project quality_gate=\"<command>\"")
		os.Exit(ExitUsage)
	}
	// The directory rule of a unit's tp done: the task file's project
	// directory, carried into the linked worktree the command runs in. On the
	// main tree it is that directory itself.
	cwd, _ := os.Getwd()
	dir := engine.WorktreeDirFor(gateDir(taskFilePath), cwd)
	// The tree is read before the gate runs, as tp done reads it: only a
	// clean checkout's pass is a fact about the committed tree.
	tree := engine.CleanTree(cwd)
	taker := unitGateTaker()
	runQualityGateIn(taskFilePath, dir, "", gateRerunHint, taker)
	recorded := false
	if taker.runDir != "" && tree != "" {
		recorded = engine.RecordGatePass(taker.runDir, taker.seq, tree, wf.QualityGate) == nil
	}
	output.Success("gate passed")
	printResultOrExit(map[string]any{"passed": true, "gate_cmd": wf.QualityGate, "dir": dir, "recorded": recorded})
	return nil
}
