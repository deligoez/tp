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

// taskFileExistsHint replaces the exit-3 default on `tp init`'s already-exists
// error. The default ends with "or 'tp init <spec>' to create one", which is
// the command that just refused, for the reason it refused — an agent that
// follows it loops. What the caller actually wants is the existing file.
const taskFileExistsHint = "the task file is already there: run 'tp use <file>' to point at it, or 'tp status' to see what it holds"

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

	// §3: the stat-then-write runs under the task-file write lock, on the same
	// terms as every other write command — including the path tp add --spec
	// reaches by calling runInit before taking its own lock. WriteTaskFile is
	// atomic, but atomicity is not mutual exclusion: unlocked, init could stat
	// a missing target, lose the interval to a concurrent writer, and then
	// overwrite that writer's file with the empty shell. engine.WithFileLock
	// honours the resolved lock_timeout_seconds; a lock held past it returns
	// *LockTimeoutError, which Execute maps to exit 4 (STATE) with a hint
	// naming the lock path and the elapsed wait. The success path is unchanged.
	if lockErr := engine.WithFileLock(taskFilePath, func() error {
		if _, err := os.Stat(taskFilePath); err == nil {
			output.Error(ExitFile, fmt.Sprintf("task file already exists: %s", taskFilePath), taskFileExistsHint)
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
		return nil
	}); lockErr != nil {
		return lockErr
	}

	tpDir := engine.ProjectConfigDir(".")
	if err := os.MkdirAll(tpDir, 0o755); err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}
	if err := engine.EnsureTPGitignore(tpDir); err != nil {
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
	// §3.4: the ejected defaults are a starting point, not a finished corpus.
	// Notice (not Info/Success) so the line survives JSON mode — the piped and
	// agent-driven runs are exactly the ones that adopt the files unread. The
	// wording names no language, so it is identical for every domain.
	output.Notice("note: these roles are starting points; rewrite their focus for your project's stack and conventions.")
	return output.JSON(map[string]any{"ejected": written, "domain": domain})
}
