package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

var (
	validateStrict  bool
	validateProject bool
)

func newValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate task file structure, atomicity, coverage, and cycles",
		RunE:  runValidate,
	}
	cmd.Flags().BoolVar(&validateStrict, "strict", false, "atomicity violations become errors")
	cmd.Flags().BoolVar(&validateProject, "project", false, "report cross-spec workflow drift from .tp/config.json")
	return cmd
}

func runValidate(_ *cobra.Command, _ []string) error {
	if validateProject {
		return runValidateProject()
	}
	taskFilePath, err := engine.DiscoverTaskFile(".", flagFile)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	tf, err := model.ReadTaskFile(taskFilePath)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	specPath, specExists := engine.ResolveSpecPath(taskFilePath, tf.Spec)
	result := engine.Validate(tf, specPath, specExists, validateStrict)
	// The touches rule reads the effective run_max_parallel, the project layer
	// included, which Validate's task-file-only workflow does not see.
	if extra := engine.TouchesWarnings(tf, engine.EffectiveWorkflowForTaskFile(taskFilePath).RunMaxParallel); len(extra) > 0 {
		result.Findings = append(result.Findings, extra...)
		result.Warnings += len(extra)
		result.Checks["touches"] = fmt.Sprintf("%d warnings", len(extra))
	}

	if err := output.JSON(result); err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
	}

	if !result.Valid {
		os.Exit(ExitValidation)
	}
	return nil
}
