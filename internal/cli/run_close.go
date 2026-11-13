package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/deligoez/tp/internal/engine"
)

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
