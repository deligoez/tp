package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

var importForce bool

func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import a complete task file (validates, then writes)",
		Args:  cobra.ExactArgs(1),
		RunE:  runImport,
	}
	cmd.Flags().BoolVar(&importForce, "force", false, "overwrite existing task file")
	return cmd
}

func runImport(_ *cobra.Command, args []string) error {
	tf, err := model.ReadTaskFile(args[0])
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	// Derive target path from spec field
	base := strings.TrimSuffix(filepath.Base(tf.Spec), filepath.Ext(tf.Spec))
	dir := filepath.Dir(tf.Spec)
	targetPath := filepath.Join(dir, base+".tasks.json")

	// Check if exists
	if _, err := os.Stat(targetPath); err == nil && !importForce {
		output.Error(ExitFile, fmt.Sprintf("task file already exists: %s (use --force to overwrite)", targetPath))
		os.Exit(ExitFile)
		return nil
	}

	// Auto-fill coverage if empty and spec exists
	specPath, specExists := engine.ResolveSpecPath(targetPath, tf.Spec)
	if specExists && tf.Coverage.TotalSections == 0 {
		engine.AutoFillCoverage(tf, specPath)
	}

	// Validate (strict atomicity unless --force)
	result := engine.Validate(tf, specPath, specExists, !importForce)
	if !result.Valid {
		if err := output.JSON(result); err != nil {
			output.Error(ExitFile, err.Error())
		}
		output.Error(ExitValidation, "import failed: validation errors found")
		os.Exit(ExitValidation)
		return nil
	}

	if err := model.WriteTaskFile(targetPath, tf); err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	output.Success(fmt.Sprintf("imported %d tasks to %s", len(tf.Tasks), targetPath))
	return output.JSON(map[string]any{"imported": len(tf.Tasks), "path": targetPath})
}
