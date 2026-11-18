package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// WorktreeSetupError is a batch start that failed in a worktree's setup: the
// command exited non-zero, timed out or left the worktree dirty
// (spec/backlog/worktree-setup.md §1). The run stops on it with
// worktree-setup-failed, after the batch is unwound.
type WorktreeSetupError struct {
	ID, LogPath, Cause string
}

func (e *WorktreeSetupError) Error() string {
	return fmt.Sprintf("the worktree setup for task %s %s (log: %s)", e.ID, e.Cause, e.LogPath)
}

// worktreeSetupCmd reads worktree_setup_cmd from the main tree's project layer
// as the batch starts. No other layer is consulted: a task file's workflow
// block cannot name a command the driver runs.
func worktreeSetupCmd(root string) (string, error) {
	pc, _, err := LoadProjectConfig(ProjectConfigDir(root))
	if err != nil {
		return "", driverErrorf(err, "read %s", WorktreeSetupCmdField)
	}
	if pc.Workflow.WorktreeSetupCmd == nil {
		return "", nil
	}
	return strings.TrimSpace(*pc.Workflow.WorktreeSetupCmd), nil
}

// setupWorktree runs the setup command in one batch worktree, records its wall
// time, and requires the worktree to be clean afterwards, untracked files
// included, so only ignored files may have been written.
func (d *driver) setupWorktree(command string, u batchUnit) error {
	logPath := filepath.Join(RunDir(d.opts.Root, d.runID), "setup-"+u.id+".log")
	env := append(os.Environ(),
		"TP_MAIN_ROOT="+d.opts.Root, EnvUnitID+"="+u.id, EnvUnattended+"=1")
	timeout := time.Duration(d.opts.Workflow.EffectiveGateTimeoutSeconds()) * time.Second

	run := d.opts.runSetup
	if run == nil {
		run = runSetupCommand
	}
	start := time.Now()
	exitCode, cause := run(context.Background(), command, u.worktree, env, timeout, logPath)
	elapsed := time.Since(start)
	if cause == "" {
		cause = dirtyWorktree(u.worktree, logPath)
	}
	appendLog(logPath, fmt.Sprintf("worktree_setup_cmd wall time: %.3fs\n", elapsed.Seconds()))
	if err := d.rec.RecordSetup(&RunSetupRow{
		ID: u.id, ExitCode: exitCode, DurationSeconds: elapsed.Seconds(), LogPath: logPath,
	}); err != nil {
		return driverErrorf(err, "record the worktree setup of task %q", u.id)
	}
	if cause != "" {
		return &WorktreeSetupError{ID: u.id, LogPath: logPath, Cause: cause}
	}
	return nil
}

// setupRunner is runSetupCommand's shape, so the driver can be handed another.
type setupRunner func(ctx context.Context, command, dir string, env []string, timeout time.Duration, logPath string) (exitCode *int, cause string)

// runSetupCommand runs command through the platform shell in dir with stdin
// closed and stdout and stderr written straight to logPath. A file rather than
// a pipe is what lets the wait end at the timeout even when a grandchild still
// holds the output open. It returns the exit code (nil when the command timed
// out or never started) and, on failure, why.
func runSetupCommand(parent context.Context, command, dir string, env []string, timeout time.Duration, logPath string) (exitCode *int, cause string) {
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec // the run directory is tp's own
	if err != nil {
		return nil, "could not open its log: " + err.Error()
	}
	defer func() { _ = log.Close() }()

	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr = dir, env, log, log
	runErr := cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, fmt.Sprintf("timed out after %ds", int(timeout.Seconds()))
	}
	var exitErr *exec.ExitError
	switch {
	case runErr == nil:
		code := 0
		return &code, ""
	case errors.As(runErr, &exitErr):
		code := exitErr.ExitCode()
		return &code, fmt.Sprintf("exited %d", code)
	default:
		return nil, "could not start: " + runErr.Error()
	}
}

// dirtyWorktree returns why the worktree is not clean after its setup, or "".
// Untracked files are listed explicitly, whatever status.showUntrackedFiles
// the user's configuration sets.
func dirtyWorktree(worktree, logPath string) string {
	status, err := gitOut(worktree, "status", "--porcelain", "--untracked-files=normal")
	if err != nil {
		return "could not read the worktree's status: " + err.Error()
	}
	if status == "" {
		return ""
	}
	appendLog(logPath, "the setup left the worktree dirty; only ignored files may be written:\n"+status+"\n")
	return "left the worktree dirty"
}

// appendLog appends text to the setup's log, best effort: the log is a report,
// and a write that fails must not change what the setup ended as.
func appendLog(logPath, text string) {
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // the run directory is tp's own
	if err != nil {
		return
	}
	_, _ = f.WriteString(text)
	_ = f.Close()
}

// unwindBatch takes back a batch that failed to start: every worktree and
// tp-unit branch it added is removed — failed, the one whose add failed, may
// have left both — and every claimed task goes back to open. No attempt was
// spent, so nothing is marked solo.
func (d *driver) unwindBatch(ids []string, added map[string]batchUnit, failed *batchUnit) error {
	for _, u := range sortedBatch(added) {
		removeUnitWorktree(d.opts.Root, u.worktree, u.branch)
	}
	if failed != nil {
		removeUnitWorktree(d.opts.Root, failed.worktree, failed.branch)
	}
	_, _ = gitOut(d.opts.Root, "worktree", "prune")
	if d.opts.ReleaseTasks == nil {
		return driverErrorf(errors.New("no release function"), "release the implement batch")
	}
	if err := d.opts.ReleaseTasks(ids); err != nil {
		return driverErrorf(err, "release the implement batch")
	}
	return nil
}
