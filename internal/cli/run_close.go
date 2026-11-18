package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
)

// claimTasksFunc is how tp run claims an implement pool's tasks: open moves
// to wip under the task file's lock, and a task already wip (an interrupted
// attempt) is left as it is.
func claimTasksFunc(taskFile string) func(ids []string) error {
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

// releaseTasksFunc is how tp run takes back a pool unit that never spawned,
// its worktree add or setup refused or a stop read before its spawn
// (spec/backlog/rolling-implement.md §2): every task still wip goes back to
// open as the claim found it. Solo is untouched, since no attempt was spent.
func releaseTasksFunc(taskFile string) func(ids []string) error {
	return func(ids []string) error {
		return engine.WithFileLock(taskFile, func() error {
			tf, err := model.ReadTaskFile(taskFile)
			if err != nil {
				return err
			}
			for _, id := range ids {
				task, _, findErr := model.FindTask(tf, id)
				if findErr != nil {
					return findErr
				}
				if task.Status == model.StatusWIP {
					task.Status = model.StatusOpen
					task.StartedAt = nil
					task.DurationSource = ""
				}
			}
			tf.UpdatedAt = time.Now().UTC()
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

// closeTaskFunc is how tp run closes an integrated pool unit
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
		// The reason goes after "--": evidence often opens with a "- "
		// bullet, which the flag parser would otherwise read as a flag.
		args := []string{"done", id}
		for _, sha := range commits {
			args = append(args, "--commit", sha)
		}
		args = append(args, "--", reason)
		log, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}
		defer log.Close()
		cmd := exec.Command(self, args...) //nolint:gosec // tp's own binary, with tp's own arguments
		cmd.Dir = root
		// The integrate log is under the run directory, and so are its gate
		// slots: the close's gate takes the main gate's slot from there.
		cmd.Env = append(os.Environ(), engine.EnvTaskFile+"="+taskFile,
			engine.EnvGateMainSlots+"="+engine.GateSlotsDir(filepath.Dir(logPath)))
		cmd.Stdout, cmd.Stderr = log, log
		runErr := cmd.Run()
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) && exitErr.ExitCode() == ExitUsage {
			// A usage refusal is tp calling itself wrong, not the unit's
			// work failing: it stops the run instead of discarding the unit.
			return &engine.CloseRefusedError{ID: id, LogPath: logPath}
		}
		if runErr != nil {
			return fmt.Errorf("tp done %s: %w (log: %s)", id, runErr, logPath)
		}
		return nil
	}
}
