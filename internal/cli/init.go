package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

var (
	initQualityGate    string
	initCommitStrategy string
	initEjectRoles     bool
	initDomain         string
	initForce          bool
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init <spec.md>",
		Short: "Create empty task file for a spec",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runInit,
	}
	cmd.Flags().StringVar(&initQualityGate, "quality-gate", "", "set workflow quality gate")
	cmd.Flags().StringVar(&initCommitStrategy, "commit-strategy", "", "set commit strategy")
	cmd.Flags().BoolVar(&initEjectRoles, "eject-roles", false, "write the default role corpus into .tp/reviewers and .tp/auditors")
	cmd.Flags().StringVar(&initDomain, "domain", "", "domain corpus to eject with --eject-roles (default: software)")
	cmd.Flags().BoolVar(&initForce, "force", false, "with --eject-roles: overwrite existing role files")
	return cmd
}

func runInit(_ *cobra.Command, args []string) error {
	if initEjectRoles {
		return runEjectRoles()
	}
	if len(args) != 1 {
		output.Error(ExitUsage, "spec path required: tp init <spec.md>")
		os.Exit(ExitUsage)
		return nil
	}
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
		Workflow:  model.WorkflowOverride{},
		Coverage: model.Coverage{
			ContextOnly: []string{},
			Unmapped:    []string{},
		},
		Tasks: []model.Task{},
	}

	if initQualityGate != "" {
		qg := initQualityGate
		tf.Workflow.QualityGate = &qg
	}
	if initCommitStrategy != "" {
		cs := initCommitStrategy
		tf.Workflow.CommitStrategy = &cs
	}

	if err := model.WriteTaskFile(taskFilePath, tf); err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	output.Success(fmt.Sprintf("created %s", taskFilePath))
	return output.JSON(map[string]string{"created": taskFilePath})
}

// runEjectRoles writes the selected default role corpus into .tp/reviewers and
// .tp/auditors as editable files (§5.3), so the hidden persona prompts become
// visible artifacts. An unknown --domain is a usage error (exit 2) listing the
// known domains; eject refuses to overwrite an existing role file unless
// --force, which overwrites regardless of the existing file's validity.
func runEjectRoles() error {
	domain := initDomain
	if domain == "" {
		domain = "software"
	}
	if !engine.HasDefaultCorpus(domain) {
		output.Error(ExitUsage, fmt.Sprintf("unknown domain %q (known: %s)", domain, strings.Join(engine.DefaultCorpusDomains(), ", ")))
		os.Exit(ExitUsage)
		return nil
	}

	tpDir := engine.ProjectConfigDir(".")
	written := make([]string, 0)
	for _, phase := range []string{engine.PhaseReviewers, engine.PhaseAuditors} {
		phaseDir := filepath.Join(tpDir, phase)
		if err := os.MkdirAll(phaseDir, 0o755); err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}
		files, err := engine.DefaultCorpusFiles(domain, phase)
		if err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}
		for _, f := range files {
			path := filepath.Join(phaseDir, f.ID+".json")
			if !initForce {
				if _, statErr := os.Stat(path); statErr == nil {
					output.Error(ExitFile, fmt.Sprintf("role file already exists: %s", path), "use --force to overwrite")
					os.Exit(ExitFile)
					return nil
				}
			}
			if err := os.WriteFile(path, f.Data, 0o600); err != nil {
				output.Error(ExitFile, err.Error())
				os.Exit(ExitFile)
				return nil
			}
			written = append(written, path)
		}
	}

	output.Success(fmt.Sprintf("ejected %d role files for domain %q", len(written), domain))
	return output.JSON(map[string]any{"ejected": written, "domain": domain})
}
