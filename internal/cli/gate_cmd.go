package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
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
on a committed tree the gate left unchanged is recorded, so the unit's
following tp done on the same tree with the same gate command does not run
the gate again. SIGINT or SIGTERM kills the gate's processes before tp exits.

Exit 0 when the gate passes.
Exit 4 when it fails, times out or a signal ends it (the slot wait included).
Exit 2 when no quality_gate is set.
Exit 3 when no task file is found or it is another project's.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runGate()
		},
	}
}

func runGate() error {
	taskFilePath, err := engine.DiscoverTaskFile(".", flagFile)
	if err != nil {
		// task-file-hint: no task file was found to read the gate from.
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
	}
	// As tp done: in a unit's worktree the task file is the main tree's, so
	// only a checkout outside one is held to the task file's project.
	if donePendingRoot(taskFilePath) == "" {
		enterTree(taskFilePath, "tp gate")
	}
	wf := engine.EffectiveWorkflowForTaskFile(taskFilePath)
	if wf.QualityGate == "" {
		// Inside a run tp set refuses the gate write (fenceQualityGateSet),
		// so the hint is the escalation a gate change takes.
		hint := "set one with tp set --workflow --project quality_gate=\"<command>\""
		if engine.Unattended() {
			hint = "setting quality_gate is the operator's decision: tp escalate --decision " + engine.EscalateSkipGate + " --evidence <what you found>"
		}
		output.Error(ExitUsage, "no quality_gate is set, so there is no gate to run", hint)
		os.Exit(ExitUsage)
	}
	// The directory rule of a unit's tp done: the task file's project
	// directory, carried into the linked worktree the command runs in. On the
	// main tree it is that directory itself.
	cwd, _ := os.Getwd()
	dir := engine.WorktreeDirFor(gateDir(taskFilePath), cwd)
	// The tree is read before the gate runs, as tp done reads it, and again
	// after: only a pass on a committed tree the gate left as it found it is
	// a fact a close can stand on.
	tree := gatePassTree(taskFilePath)
	taker := unitGateTaker()
	runQualityGateIn(taskFilePath, dir, "", gateRerunHint, taker)
	recorded := false
	if taker.runDir != "" && tree != "" && gatePassTree(taskFilePath) == tree {
		recorded = engine.RecordGatePass(taker.runDir, taker.seq, tree, wf.QualityGate) == nil
	}
	output.Success("gate passed")
	printResultOrExit(map[string]any{"passed": true, "gate_cmd": wf.QualityGate, "dir": dir, "recorded": recorded})
	return nil
}

// gatePassTree is the tree a gate here gates, when its pass may stand for a
// close of that tree: in a unit's worktree, a clean checkout's HEAD tree, as
// the worktree close reads it; on the main tree, HEAD's tree while only tp's
// own paths differ from it, as the main-branch close reads it. "" otherwise.
func gatePassTree(taskFilePath string) string {
	cwd, _ := os.Getwd()
	if donePendingRoot(taskFilePath) != "" {
		return engine.CleanTree(cwd)
	}
	tf, err := model.ReadTaskFile(taskFilePath)
	if err != nil {
		return ""
	}
	specPath, _ := engine.ResolveSpecPath(taskFilePath, tf.Spec)
	return engine.MainGatedTree(engine.CanonicalProjectRoot(filepath.Dir(taskFilePath)), taskFilePath, specPath)
}

// gatePassTime is when this attempt's tp gate passed on the tree a main-tree
// unit's close would gate, with the same gate command: the close stands on
// that pass instead of running the gate again. Only a unit's own close inside
// a run reads it; the driver's main-branch close has its own reuse.
func gatePassTime(taskFilePath string) *time.Time {
	taker := unitGateTaker()
	if taker.runDir == "" || os.Getenv(engine.EnvGateMainSlots) != "" {
		return nil
	}
	gateCmd := engine.EffectiveWorkflowForTaskFile(taskFilePath).QualityGate
	tree := gatePassTree(taskFilePath)
	at, ok := engine.GatePassCovers(taker.runDir, taker.seq, tree, gateCmd)
	if !ok {
		return nil
	}
	output.Notice(fmt.Sprintf("gate reused from tp gate: tree %s passed %q in this attempt", tree, gateCmd))
	return &at
}
