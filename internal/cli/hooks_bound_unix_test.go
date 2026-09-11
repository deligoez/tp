//go:build unix

package cli

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// ownProcessGroup starts cmd as the leader of a process group of its own. A
// non-interactive shell starts its children in its own group, so every process
// a hook starts, backgrounded or not, stays reachable through the hook's pid.
func ownProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killHookGroup SIGKILLs every process left in the group pid leads. An empty
// group is not an error: it is the case where the hook took everything with it.
func killHookGroup(pid int) error {
	err := syscall.Kill(-pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}

// processAlive reports whether pid names a process that has not exited. A
// zombie answers kill(pid, 0) on Linux and darwin alike, so on Linux, where a
// container's init may never reap the orphan, its /proc state decides.
func processAlive(pid int) bool {
	if err := syscall.Kill(pid, 0); err != nil && !errors.Is(err, syscall.EPERM) {
		return false
	}
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return true
	}
	fields := bytes.Fields(stat[bytes.LastIndexByte(stat, ')')+1:])
	return len(fields) == 0 || string(fields[0]) != "Z"
}
