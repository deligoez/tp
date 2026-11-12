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
	return processAliveReading(pid, os.ReadFile)
}

// processAliveReading is processAlive with the /proc read handed in, so a test
// can reap the process between the signal probe and the read, the window a
// reaper hits by chance on a loaded machine.
func processAliveReading(pid int, readFile func(string) ([]byte, error)) bool {
	if !signalable(pid) {
		return false
	}
	stat, err := readFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		// No stat file is one of two things: an OS with no /proc (darwin),
		// where the probe above has already answered, or a zombie its reaper
		// collected between that probe and this read. Probing again tells them
		// apart; reading the miss as "running" failed the hook-bound test on a
		// loaded runner, where reaping lags just enough to land in the window.
		return signalable(pid)
	}
	fields := bytes.Fields(stat[bytes.LastIndexByte(stat, ')')+1:])
	return len(fields) == 0 || string(fields[0]) != "Z"
}

// signalable reports whether kill(pid, 0) finds a process, running or zombie.
// EPERM is a process that exists and belongs to someone else.
func signalable(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
