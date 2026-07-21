package cli

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

func newConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Print the resolved effective configuration as JSON",
		RunE:  runConfig,
	}
}

// resolveConfigWorkflow resolves the effective workflow for tp config: the
// project config (from CWD) layered under the active/--file task file's own
// workflow override. A missing task file yields the project layer alone. A
// malformed config aborts with exit 3 and a repair-or-delete hint.
func resolveConfigWorkflow() (model.Workflow, model.WorkflowOverride) {
	var override model.WorkflowOverride
	if taskFilePath, err := engine.DiscoverTaskFile(".", flagFile); err == nil && taskFilePath != "" {
		override, _ = engine.LoadTaskWorkflowOverride(taskFilePath)
	}
	wf, warnings, err := engine.ResolveEffectiveWorkflow(".", override)
	if err != nil {
		var mce *engine.MalformedConfigError
		if errors.As(err, &mce) {
			output.Error(ExitFile, mce.Error(), mce.Hint())
		} else {
			output.Error(ExitFile, err.Error())
		}
		os.Exit(ExitFile)
	}
	for _, w := range warnings {
		output.Info(w)
	}
	return wf, override
}

func runConfig(_ *cobra.Command, _ []string) error {
	wf, _ := resolveConfigWorkflow()
	// The effective workflow as JSON on stdout. --compact is a documented no-op
	// (the output is not task-shaped), so tp config always emits this shape.
	return output.JSON(map[string]any{"workflow": wf})
}
