package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

var commitFiles string

func newCommitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commit <id> [reason]",
		Short: "Stage, commit with structured message, record SHA on task",
		Long: `Atomic commit for a task. Stages changes, generates a structured commit
message from task metadata, commits, and records the SHA on the task.
Implicitly claims open tasks (open → wip).
Output: {id, sha, message}`,
		Example: `  tp commit auth-model "Model + migration done"
  tp commit auth-model --files "app/Models/*,migrations/*"`,
		Args: cobra.RangeArgs(1, 2),
		RunE: runCommit,
	}
	cmd.Flags().StringVar(&commitFiles, "files", "", "comma-separated file globs to stage (default: all dirty)")
	return cmd
}

func runCommit(_ *cobra.Command, args []string) error {
	taskFilePath, err := engine.DiscoverTaskFile(".", flagFile)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	return engine.WithFileLock(taskFilePath, func() error {
		tf, err := model.ReadTaskFile(taskFilePath)
		if err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}

		task, _, err := model.FindTask(tf, args[0])
		if err != nil {
			output.Error(ExitState, err.Error())
			os.Exit(ExitState)
			return nil
		}

		// Implicit claim
		if task.Status == model.StatusOpen {
			done := make(map[string]bool)
			for i := range tf.Tasks {
				if tf.Tasks[i].Status == model.StatusDone {
					done[tf.Tasks[i].ID] = true
				}
			}
			for _, dep := range task.DependsOn {
				if !done[dep] {
					output.Error(ExitState, fmt.Sprintf("cannot commit: task %s is blocked by %s", task.ID, dep))
					os.Exit(ExitState)
					return nil
				}
			}
			now := time.Now().UTC()
			task.Status = model.StatusWIP
			task.StartedAt = &now
		}

		if task.Status == model.StatusDone {
			output.Error(ExitState, fmt.Sprintf("task %s is already done", task.ID))
			os.Exit(ExitState)
			return nil
		}

		// Resolve reason
		reason := ""
		if len(args) > 1 {
			reason = args[1]
		}

		// Stage
		if err := gitStage(commitFiles); err != nil {
			output.Error(ExitFile, fmt.Sprintf("git stage failed: %v", err))
			os.Exit(ExitFile)
			return nil
		}

		// Check if there's anything to commit
		if !gitHasStagedChanges() {
			output.Error(ExitState, "no changes to commit")
			os.Exit(ExitState)
			return nil
		}

		// Build commit message
		msg := buildCommitMessage(task, reason)

		// Commit
		sha, err := gitCommit(msg)
		if err != nil {
			output.Error(ExitFile, fmt.Sprintf("git commit failed: %v", err))
			os.Exit(ExitFile)
			return nil
		}

		// Record SHA on task
		task.CommitSHA = &sha

		if err := model.WriteTaskFile(taskFilePath, tf); err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}

		return output.JSON(map[string]any{
			"id":      task.ID,
			"sha":     sha,
			"message": strings.SplitN(msg, "\n", 2)[0],
		})
	})
}

func buildCommitMessage(task *model.Task, reason string) string {
	// First line: conventional commit with task ID
	title := fmt.Sprintf("feat(%s): %s", task.ID, task.Title)

	var body strings.Builder
	body.WriteString(title)
	body.WriteString("\n")

	if reason != "" {
		body.WriteString("\n")
		body.WriteString(reason)
		body.WriteString("\n")
	}

	// Trailer
	body.WriteString("\n")
	fmt.Fprintf(&body, "Task: %s\n", task.ID)
	if task.Acceptance != "" {
		fmt.Fprintf(&body, "Acceptance: %s\n", task.Acceptance)
	}

	return body.String()
}

func gitStage(filesFlag string) error {
	if filesFlag == "" {
		return runGit("add", "-A")
	}

	// Stage specific file patterns
	for _, pattern := range strings.Split(filesFlag, ",") {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if err := runGit("add", pattern); err != nil {
			return fmt.Errorf("staging %q: %w", pattern, err)
		}
	}
	return nil
}

func gitHasStagedChanges() bool {
	cmd := exec.Command("git", "diff", "--cached", "--quiet")
	return cmd.Run() != nil // exit 1 = has changes
}

func gitCommit(message string) (string, error) {
	if err := runGit("commit", "-m", message); err != nil {
		return "", err
	}

	// Get the SHA
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get commit SHA: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func runGit(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
