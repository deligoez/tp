package engine

import (
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// The environment §5.2 hands notify_cmd. TP_RUN_ID is the child variable of the
// same name (§3.1.1) rather than a second spelling of it, so an operator's
// notification script and a unit read one key for one fact.
const (
	// EnvStopReason is the §3.4 reason the run stopped for.
	EnvStopReason = "TP_STOP_REASON"
	// EnvEscalationPath is the record the run stopped on, present only on
	// an escalation. Absent — not empty — on every other stop, because a
	// path-shaped variable holding "" names the repository root, and a
	// script testing for the variable's presence is asking whether there is
	// a record to read.
	EnvEscalationPath = "TP_ESCALATION_PATH"
)

// NotifyOutcome is one invocation of §5.2's notify_cmd, as the run reports it.
//
// It is a report about the notification and never about the run: every field
// here is what the operator's own command did, and none of them can reach the
// stop reason. That separation is the whole of §5.2's last sentence — a
// notification that fails is a notification that failed, not a run that ended
// differently.
type NotifyOutcome struct {
	// Cmd is the configured command, as written, so a report names the
	// string an operator can go and fix.
	Cmd string
	// ExitCode is the code the command exited with. It is nil when the
	// command never ran, rather than 0 or -1: no code came back, and
	// inventing one would report a result the process never produced.
	ExitCode *int
	// Err is why the command could not be run at all — a name that is not
	// on disk, a file that is not executable.
	Err error
}

// InvokeNotify runs the operator's notify_cmd for a stop that has already been
// decided, and reports what it did. It returns nil when no command is
// configured, which is a report of no invocation rather than an empty one.
//
// The command is exec'd directly and split on whitespace (§5.2): no shell is
// involved, so a semicolon, a `$VAR` or a glob in the configured string reaches
// the child as literal argument text. That is deliberately less expressive than
// a shell line — an operator who needs one names a script — and it is what
// keeps the value of notify_cmd from being a small language tp would then have
// to define.
//
// Both output streams are discarded. The driver's stdout is the run's own JSON
// payload, and a notification writing into it would corrupt the one report a
// caller parses.
func InvokeNotify(root, command string, reason StopReason, runID, escalationPath string) *NotifyOutcome {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return nil
	}

	cmd := exec.Command(fields[0], fields[1:]...) //nolint:gosec // notify_cmd is the operator's own configured command
	cmd.Dir = root
	cmd.Stdin = nil
	cmd.Stdout, cmd.Stderr = nil, nil
	cmd.Env = notifyEnv(os.Environ(), reason, runID, escalationPath)

	out := &NotifyOutcome{Cmd: command}
	var exitErr *exec.ExitError
	switch err := cmd.Run(); {
	case err == nil:
		code := 0
		out.ExitCode = &code
	case errors.As(err, &exitErr):
		code := exitErr.ExitCode()
		out.ExitCode = &code
	default:
		out.Err = err
	}
	return out
}

// notifyEnv returns the notification's environment: the driver's own, with
// §5.2's variables set over it.
//
// Each of the three is removed from the inherited set before the run's own
// values are appended, so a stale TP_ESCALATION_PATH the driver was started
// with cannot be read as this stop's record. TP_ESCALATION_PATH is then added
// only on an escalation, which is what makes its presence the fact it claims
// to be.
func notifyEnv(parent []string, reason StopReason, runID, escalationPath string) []string {
	env := slices.DeleteFunc(slices.Clone(parent), func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		return key == EnvStopReason || key == EnvRunID || key == EnvEscalationPath
	})
	env = append(env, EnvStopReason+"="+string(reason), EnvRunID+"="+runID)
	if escalationPath != "" {
		env = append(env, EnvEscalationPath+"="+escalationPath)
	}
	return env
}
