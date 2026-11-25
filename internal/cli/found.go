package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// foundShapeHint names every field a request can carry, so a refused unit can
// rebuild the call without reading the help.
const foundShapeHint = "file one defect: --location <file:line> --reproduction <command or test>, then either --expected and --observed, or --question with an --alternative per choice"

// foundBlocksHint says who may set --blocks and how a caller names its task.
const foundBlocksHint = "--blocks marks a request the caller's own task cannot pass without, so it needs a wip task the caller works: an implement unit's own, or --task <id>; drop --blocks to file the request without it"

// foundAddFlags are tp found add's inputs: the request's fields as flags.
type foundAddFlags struct {
	location, reproduction, expected, observed, question, task string
	alternatives                                               []string
	blocks                                                     bool
}

func newFoundCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "found",
		Short: "File requests for defects found outside the caller's task",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newFoundAddCmd())
	return cmd
}

func newFoundAddCmd() *cobra.Command {
	var f foundAddFlags
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Record one request for a defect outside the caller's task",
		Long: `Record one request, one defect, in the spec's run state. No task changes.

A request carries a location (file:line) and a reproduction (a command or test
that shows the defect), and either all four Fix fields (--expected and
--observed too) or a --question with its --alternative choices. Anything else
exits 2 and records nothing.

--blocks marks a request the caller's task cannot pass without. It needs a wip
task the caller works: under tp run an implement unit's own task, or the one
--task names.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runFoundAdd(&f)
		},
	}
	cmd.Flags().StringVar(&f.location, "location", "", "where the defect is, as file:line")
	cmd.Flags().StringVar(&f.reproduction, "reproduction", "", "a command or test that shows the defect")
	cmd.Flags().StringVar(&f.expected, "expected", "", "the expected behaviour")
	cmd.Flags().StringVar(&f.observed, "observed", "", "the observed behaviour")
	cmd.Flags().StringVar(&f.question, "question", "", "the choice the request needs, when it is not Fix-shaped")
	// StringArray, not StringSlice: an alternative is prose, and splitting it
	// on commas would turn one alternative into several.
	cmd.Flags().StringArrayVar(&f.alternatives, "alternative", nil, "one way to answer --question (repeatable)")
	cmd.Flags().BoolVar(&f.blocks, "blocks", false, "the caller's task cannot pass without this request")
	cmd.Flags().StringVar(&f.task, "task", "", "the wip task the caller works (an implement unit's own by default)")
	return cmd
}

// runFoundAdd implements tp found add (spec/1.31.0.md §1). It reads the task
// file only to name the finder; the request goes to the cycle's found store.
func runFoundAdd(f *foundAddFlags) error {
	req := &engine.FoundRequest{
		Location:     f.location,
		Reproduction: f.reproduction,
		Expected:     f.expected,
		Observed:     f.observed,
		Question:     f.question,
		Alternatives: f.alternatives,
		Blocks:       f.blocks,
	}
	if msg := engine.FoundRequestShapeError(req); msg != "" {
		return foundRefusal(msg, foundShapeHint)
	}

	taskFilePath, err := discoverWriteTarget()
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}
	finder, err := foundFinderTask(taskFilePath, f.task)
	if err != nil {
		return foundRefusal(err.Error(), foundBlocksHint)
	}
	if f.blocks && finder == "" {
		return foundRefusal("--blocks needs a task the caller works, and none is named", foundBlocksHint)
	}
	req.Finder.Task = finder

	root := engine.CanonicalProjectRoot(filepath.Dir(taskFilePath))
	stored, err := engine.AddFoundRequest(root, taskFilePath, req)
	if err != nil {
		output.Error(ExitFile, "cannot record the request: "+err.Error())
		os.Exit(ExitFile)
		return nil
	}
	return output.JSON(map[string]any{
		"id":     stored.ID,
		"state":  stored.State,
		"blocks": stored.Blocks,
		"finder": stored.Finder,
	})
}

// foundFinderTask returns the task the caller works, or "" when it names none:
// --task, or an implement unit's own task under tp run. A role unit's
// TP_UNIT_ID is a role, not a task, so only the implement kind counts. The task
// must be wip in the task file, which is read and never written.
func foundFinderTask(taskFilePath, flagTask string) (string, error) {
	unitTask := ""
	if engine.UnitKind(os.Getenv(engine.EnvUnitKind)) == engine.UnitImplement {
		unitTask = os.Getenv(engine.EnvUnitID)
	}
	if flagTask != "" && unitTask != "" && flagTask != unitTask {
		return "", fmt.Errorf("--task names %s, but this unit works %s", flagTask, unitTask)
	}
	id := flagTask
	if id == "" {
		id = unitTask
	}
	if id == "" {
		return "", nil
	}
	tf, err := model.ReadTaskFile(taskFilePath)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
	}
	for i := range tf.Tasks {
		if tf.Tasks[i].ID != id {
			continue
		}
		if tf.Tasks[i].Status != model.StatusWIP {
			return "", fmt.Errorf("task %s is %s, not a task the caller works", id, tf.Tasks[i].Status)
		}
		return id, nil
	}
	return "", fmt.Errorf("task %s is not in the task file", id)
}

// foundRefusal emits a refusal and exits 2; see escalateRefusal for why it
// returns at all.
func foundRefusal(msg, hint string) error {
	output.Error(ExitUsage, msg, hint)
	os.Exit(ExitUsage)
	return nil
}
