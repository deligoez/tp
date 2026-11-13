package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
)

// claimBatchFunc is how tp run claims an implement batch's tasks: open moves
// to wip under the task file's lock, and a task already wip (an interrupted
// attempt) is left as it is.
func claimBatchFunc(taskFile string) func(ids []string) error {
	return func(ids []string) error {
		return engine.WithFileLock(taskFile, func() error {
			tf, err := model.ReadTaskFile(taskFile)
			if err != nil {
				return err
			}
			now := time.Now().UTC()
			for _, id := range ids {
				task, _, findErr := model.FindTask(tf, id)
				if findErr != nil {
					return findErr
				}
				if task.Status == model.StatusOpen {
					task.Status = model.StatusWIP
					task.StartedAt = &now
					task.DurationSource = model.DurationSourceClaimed
				}
			}
			tf.UpdatedAt = now
			return model.WriteTaskFile(taskFile, tf)
		})
	}
}

// reopenSoloFunc is how tp run reopens a task whose unit failed to integrate:
// back to open, marked solo so it runs alone from then on.
func reopenSoloFunc(taskFile string) func(id string) error {
	return func(id string) error {
		return engine.WithFileLock(taskFile, func() error {
			tf, err := model.ReadTaskFile(taskFile)
			if err != nil {
				return err
			}
			task, _, err := model.FindTask(tf, id)
			if err != nil {
				return err
			}
			task.Status = model.StatusOpen
			task.StartedAt = nil
			task.DurationSource = ""
			task.Solo = true
			tf.UpdatedAt = time.Now().UTC()
			return model.WriteTaskFile(taskFile, tf)
		})
	}
}

// closeTaskFunc is how tp run closes an integrated batch unit
// (spec/backlog/parallel-implement.md §2.4): it runs this binary's own tp done
// in the main tree with the picked shas, so the gate, the evidence check and
// the record are tp done's and nothing is reimplemented. Its output goes to
// logPath; a non-zero exit — a failed gate among them — is the error.
func closeTaskFunc(root, taskFile string) engine.CloseFunc {
	// The child runs in root, so a task file named relative to tp run's own
	// working directory is made absolute first.
	if abs, err := filepath.Abs(taskFile); err == nil {
		taskFile = abs
	}
	return func(id, reason string, commits []string, logPath string) error {
		self, err := os.Executable()
		if err != nil {
			return err
		}
		args := []string{"done", id, reason}
		for _, sha := range commits {
			args = append(args, "--commit", sha)
		}
		log, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}
		defer log.Close()
		cmd := exec.Command(self, args...) //nolint:gosec // tp's own binary, with tp's own arguments
		cmd.Dir = root
		cmd.Env = append(os.Environ(), engine.EnvTaskFile+"="+taskFile)
		cmd.Stdout, cmd.Stderr = log, log
		if runErr := cmd.Run(); runErr != nil {
			return fmt.Errorf("tp done %s: %w (log: %s)", id, runErr, logPath)
		}
		return nil
	}
}
