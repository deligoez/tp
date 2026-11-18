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
	Passed     bool
	ExitCode   *int // process exit code; nil when the run timed out
	TimedOut   bool
	Message    string   // "timed out after <N>s" on timeout, start error otherwise
	OutputTail []string // last tailLines lines of combined stdout+stderr
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
// group of its own, the timeout kills the whole group, and once the command
// has exited whatever it left in the group is killed too. WaitDelay bounds the
// wait for a descendant that left the group and keeps the pipe open. So the
// call returns when the gate is done, and the slot frees then, not when the
// last descendant closes the pipe; nothing the gate started runs on outside
// the cap. The command never sees the run's slot variables (gateEnv).
func RunGateCommand(command, dir string, env []string, timeout time.Duration, tailLines int) RunResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd, buf := shellCommand(ctx, command, dir, gateEnv(env))
	setupProcessGroup(cmd)
	cmd.WaitDelay = gateWaitDelay
	err := cmd.Run()
	killProcessGroup(cmd)
	if errors.Is(err, exec.ErrWaitDelay) {
		// The command itself exited 0; only a descendant held the pipe.
		err = nil
	}
	return commandResult(ctx, err, buf, timeout, tailLines)
}

// gateSlotVars are what makes a tp call take one of the run's gate slots.
var gateSlotVars = []string{EnvRunDir, EnvUnitSeq, EnvGateMainSlots, EnvGateMainTimes}

// gateEnv is env (nil: the tp process's own) without the run's slot
// variables. A gate holds a slot while its command runs, so a tp call inside
// it that took another slot of the same run would wait on its own caller, and
// at a cap of 1 never return; without them it is a call by hand.
func gateEnv(env []string) []string {
	if env == nil {
		env = os.Environ()
	}
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if !slotVar(kv) {
			out = append(out, kv)
		}
	}
	return out
}

func slotVar(kv string) bool {
	for _, name := range gateSlotVars {
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
