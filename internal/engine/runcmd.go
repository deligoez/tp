package engine

import (
	"context"
	"errors"
	"fmt"
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
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	cmd.Dir = dir
	buf := &tailBuffer{max: runBufferSize}
	cmd.Stdout = buf
	cmd.Stderr = buf

	err := cmd.Run()
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
