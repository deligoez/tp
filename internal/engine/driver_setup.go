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

// WorktreeSetupError is a unit start that failed in its worktree's setup: the
// command exited non-zero, timed out or left the worktree dirty
// (spec/backlog/worktree-setup.md §1). The run stops on it with
// worktree-setup-failed, after the unit is released and the pool drained.
type WorktreeSetupError struct {
	ID, LogPath, Cause string
}

func (e *WorktreeSetupError) Error() string {
	return fmt.Sprintf("the worktree setup for task %s %s (log: %s)", e.ID, e.Cause, e.LogPath)
}

// worktreeSetupCmd reads worktree_setup_cmd from the main tree's project layer
// as the pool starts. No other layer is consulted: a task file's workflow
// block cannot name a command the driver runs. A value that is not a string
// is an error rather than the warning other fields get: read as unset, it
// would start a pool the operator meant to prepare.
func worktreeSetupCmd(root string) (string, error) {
	pc, warnings, err := LoadProjectConfig(ProjectConfigDir(root))
	if err != nil {
		return "", driverErrorf(err, "read %s", WorktreeSetupCmdField)
	}
	for _, w := range warnings {
		if strings.HasPrefix(w, "workflow."+WorktreeSetupCmdField+":") {
			return "", driverErrorf(errors.New(w), "read %s from %s", WorktreeSetupCmdField, ProjectConfigDir(root))
		}
	}
	if pc.Workflow.WorktreeSetupCmd == nil {
		return "", nil
	}
	return strings.TrimSpace(*pc.Workflow.WorktreeSetupCmd), nil
}

// setupWorktree runs the setup command in one pool worktree, records its wall
// time, and requires the worktree to be clean afterwards, untracked files
// included, so only ignored files may have been written.
func (d *driver) setupWorktree(command string, u unitTree) error {
	logPath := filepath.Join(RunDir(d.opts.Root, d.runID), "setup-"+u.id+".log")
	env := append(os.Environ(),
		"TP_MAIN_ROOT="+d.opts.Root, EnvUnitID+"="+u.id, EnvUnattended+"=1")
	timeout := time.Duration(d.opts.Workflow.EffectiveGateTimeoutSeconds()) * time.Second

	run := d.opts.runSetup
	if run == nil {
		run = runSetupCommand
	}
	before, err := worktreeHead(u.worktree)
	if err != nil {
		return driverErrorf(err, "read HEAD in the worktree of task %q", u.id)
	}
	ctx, endWatch := d.sig.context()
	defer endWatch()
	start := time.Now()
	// The setup runs in the worktree's copy of the project directory, the
	// one TP_MAIN_ROOT names in the main tree, so that in a repository whose
	// project lives below the top level both mean the same relative paths.
	exitCode, cause := run(ctx, command, projectDirIn(d.opts.Root, u.worktree), env, timeout, logPath)
	elapsed := time.Since(start)
	if cause == "" {
		cause = movedHead(u.worktree, before, logPath)
	}
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

// setupWaitDelay bounds the wait after the setup is killed, for a process that
// left its group and still holds something the wait is waiting on.
const setupWaitDelay = 5 * time.Second

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
	setupProcessGroup(cmd)
	cmd.WaitDelay = setupWaitDelay
	runErr := cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, fmt.Sprintf("timed out after %ds", int(timeout.Seconds()))
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return nil, "was interrupted"
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

// worktreeHead is what HEAD names in a pool worktree: its commit and the
// branch it is on. A pool worktree is always on its unit's branch.
func worktreeHead(worktree string) (string, error) {
	commit, err := gitOut(worktree, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	branch, err := gitOut(worktree, "symbolic-ref", "-q", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(commit) + " " + strings.TrimSpace(branch), nil
}

// movedHead returns why the setup changed the worktree's HEAD, or "". A
// commit or a checkout leaves the status clean, and the unit would then start
// from a tree the driver never chose.
func movedHead(worktree, before, logPath string) string {
	after, err := worktreeHead(worktree)
	if err != nil {
		return "could not read the worktree's HEAD: " + err.Error()
	}
	if after == before {
		return ""
	}
	appendLog(logPath, "the setup moved HEAD; it must leave the worktree's commit and branch as it found them:\nbefore: "+before+"\nafter:  "+after+"\n")
	return "moved HEAD"
}

// dirtyWorktree returns why the worktree is not clean after its setup, or "".
// Untracked files are listed explicitly, whatever status.showUntrackedFiles
// the user's configuration sets, and so are submodules, whatever
// diff.ignoreSubmodules does.
func dirtyWorktree(worktree, logPath string) string {
	status, err := gitOut(worktree, "status", "--porcelain", "--untracked-files=normal", "--ignore-submodules=none")
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
