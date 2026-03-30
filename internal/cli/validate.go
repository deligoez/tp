package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

var validateStrict bool

func newValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate task file structure, atomicity, coverage, and cycles",
		RunE:  runValidate,
	}
	cmd.Flags().BoolVar(&validateStrict, "strict", false, "atomicity violations become errors")
	return cmd
}

func runValidate(_ *cobra.Command, _ []string) error {
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

	if err := output.JSON(result); err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
	}

	if !result.Valid {
		os.Exit(ExitValidation)
	}
	return nil
}
