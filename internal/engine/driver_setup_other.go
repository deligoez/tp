//go:build !unix

package engine

import "os/exec"

// setupProcessGroup leaves cmd as it is off unix, where there is no process
// group to kill: the context's cancel kills the command itself, and WaitDelay
// bounds the wait for anything it started.
func setupProcessGroup(*exec.Cmd) {}

// killProcessGroup has no group to kill off unix.
func killProcessGroup(*exec.Cmd) {}
