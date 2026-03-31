package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

var (
	initQualityGate    string
	initCommitStrategy string
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init <spec.md>",
		Short: "Create empty task file for a spec",
		Args:  cobra.ExactArgs(1),
		RunE:  runInit,
	}
	cmd.Flags().StringVar(&initQualityGate, "quality-gate", "", "set workflow quality gate")
	cmd.Flags().StringVar(&initCommitStrategy, "commit-strategy", "", "set commit strategy")
	return cmd
}

func runInit(_ *cobra.Command, args []string) error {
	specPath := args[0]

	// Derive task file path
	base := strings.TrimSuffix(filepath.Base(specPath), filepath.Ext(specPath))
	dir := filepath.Dir(specPath)
	taskFilePath := filepath.Join(dir, base+".tasks.json")

	if _, err := os.Stat(taskFilePath); err == nil {
		output.Error(ExitFile, fmt.Sprintf("task file already exists: %s", taskFilePath))
		os.Exit(ExitFile)
		return nil
	}

	now := time.Now().UTC()
	tf := &model.TaskFile{
		Version:   1,
		Spec:      specPath,
		CreatedAt: now,
		UpdatedAt: now,
		Workflow:  model.Workflow{},
		Coverage: model.Coverage{
			ContextOnly: []string{},
			Unmapped:    []string{},
		},
		Tasks: []model.Task{},
	}

	if initQualityGate != "" {
		tf.Workflow.QualityGate = initQualityGate
	}
	if initCommitStrategy != "" {
		tf.Workflow.CommitStrategy = initCommitStrategy
	}

	if err := model.WriteTaskFile(taskFilePath, tf); err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	output.Success(fmt.Sprintf("created %s", taskFilePath))
	return output.JSON(map[string]string{"created": taskFilePath})
}
