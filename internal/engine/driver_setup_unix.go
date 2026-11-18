//go:build unix

package engine

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// setupProcessGroup starts cmd as the leader of a process group of its own and
// makes the context's cancel SIGKILL the whole group. A non-interactive shell
// keeps the children it starts in its group, so a child the setup backgrounded
// dies with it at the timeout instead of outliving the worktree it runs in.
func setupProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
}
