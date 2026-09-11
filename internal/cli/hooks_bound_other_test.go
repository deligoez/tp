//go:build !unix

package cli

import (
	"os"
	"os/exec"
)

// The hooks and stubs these tests run are sh scripts, so what follows only
// keeps the package compiling off unix, where there are no process groups to
// kill: the hook itself is the most that can be reached.

func ownProcessGroup(*exec.Cmd) {}

func killHookGroup(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Kill()
}

func processAlive(pid int) bool {
	_, err := os.FindProcess(pid)
	return err == nil
}
