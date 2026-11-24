package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// runBufferSize bounds combined command output capture to the last 64 KB.
const runBufferSize = 64 * 1024

// RunResult is the outcome of RunCommand.
type RunResult struct {
	Passed   bool
	ExitCode *int // process exit code; nil when the run timed out
	TimedOut bool
	// Interrupted: the caller's context ended the run (RunGateCommand), a
	// signal to tp, before the command finished.
	Interrupted bool
	Message     string   // "timed out after <N>s" on timeout, start error otherwise
	OutputTail  []string // last tailLines lines of combined stdout+stderr
}

// tailBuffer is a bounded io.Writer keeping the most recent max bytes.
type tailBuffer struct {
	mu  sync.Mutex
	buf []byte
	max int
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	if len(b.buf) > b.max {
		b.buf = b.buf[len(b.buf)-b.max:]
	}
	return len(p), nil
}

// tail returns the last n lines of the captured output.
// The leading line may be partial when the 64 KB bound trimmed mid-line.
func (b *tailBuffer) tail(n int) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := strings.TrimRight(string(b.buf), "\n")
	if s == "" || n <= 0 {
		return []string{}
	}
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

// RunCommand executes command through the platform shell (`sh -c` on Unix,
// `cmd /C` on Windows) in dir with the environment inherited from the tp
// process. Combined stdout+stderr is captured in a 64 KB ring buffer and
// OutputTail keeps its last tailLines lines. A run exceeding timeout is
// killed and counts as a failure with a timed-out message.
func RunCommand(command, dir string, timeout time.Duration, tailLines int) RunResult {
	return RunCommandEnv(command, dir, nil, timeout, tailLines)
}

// RunCommandEnv is RunCommand with the child's environment given: env nil
// inherits the tp process's environment, anything else replaces it.
func RunCommandEnv(command, dir string, env []string, timeout time.Duration, tailLines int) RunResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd, buf := shellCommand(ctx, command, dir, env)
	err := cmd.Run()
	return commandResult(ctx, err, buf, timeout, tailLines)
}

// gateWaitDelay bounds how long a gate is waited for once its command has
// exited or been killed, for a descendant that still holds the output pipe.
const gateWaitDelay = time.Second

// RunGateCommand is RunCommandEnv for a quality gate, which holds a gate slot
// while it runs (spec/backlog/gate-slots.md): the command leads a process
// group of its own, and the timeout, or parent ending (tp caught a signal),
// kills the whole group; the result then says TimedOut or Interrupted. A gate
// that exits on its own, pass or fail, has nothing killed: what it left
// running (a build server the next gate uses) runs on, outside the cap, as it
// did before gate slots. WaitDelay bounds the wait for such a leftover that
// still holds the output pipe, so the call returns, and the slot frees, when
// the gate is done. Off unix there is no group and only that bound applies.
// The command never sees the run's slot variables (gateEnv).
func RunGateCommand(parent context.Context, command, dir string, env []string, timeout time.Duration, tailLines int) RunResult {
	if parent.Err() != nil {
		return RunResult{Interrupted: true, Message: "interrupted before the gate started"}
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	cmd, buf := shellCommand(ctx, command, dir, gateEnv(env))
	setupProcessGroup(cmd)
	cmd.WaitDelay = gateWaitDelay
	err := cmd.Run()
	if errors.Is(err, exec.ErrWaitDelay) {
		// The command itself exited 0; only a descendant held the pipe.
		err = nil
	}
	if parent.Err() != nil {
		return RunResult{Interrupted: true, Message: "interrupted while the gate ran", OutputTail: buf.tail(tailLines)}
	}
	return commandResult(ctx, err, buf, timeout, tailLines)
}

// gateEnv is env (nil: the tp process's own) without any of RunEnvVars. A
// gate holds a slot while its command runs, so a tp call inside it that took
// another slot of the same run would wait on its own caller, and at a cap of 1
// never return; and a suite that saw TP_FILE resolved the run's own task file
// and wrote its fixtures into it. Without them the command is a call by hand.
func gateEnv(env []string) []string {
	if env == nil {
		env = os.Environ()
	}
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if !runVar(kv) {
			out = append(out, kv)
		}
	}
	return out
}

func runVar(kv string) bool {
	for _, name := range RunEnvVars {
		if strings.HasPrefix(kv, name+"=") {
			return true
		}
	}
	return false
}

// shellCommand is command under the platform shell in dir, its combined
// output captured in a bounded buffer.
func shellCommand(ctx context.Context, command, dir string, env []string) (*exec.Cmd, *tailBuffer) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	cmd.Dir = dir
	cmd.Env = env
	buf := &tailBuffer{max: runBufferSize}
	cmd.Stdout = buf
	cmd.Stderr = buf
	return cmd, buf
}

// commandResult reads a finished command's outcome.
func commandResult(ctx context.Context, err error, buf *tailBuffer, timeout time.Duration, tailLines int) RunResult {
	res := RunResult{OutputTail: buf.tail(tailLines)}

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		res.TimedOut = true
		res.Message = fmt.Sprintf("timed out after %ds", int(timeout.Seconds()))
		return res
	}

	if err == nil {
		res.Passed = true
		code := 0
		res.ExitCode = &code
		return res
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code := exitErr.ExitCode()
		res.ExitCode = &code
	} else {
		res.Message = err.Error()
	}
	return res
}
