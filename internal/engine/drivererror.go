package engine

import "fmt"

// DriverError is §3.4's driver-error: a failure the driver itself could not
// recover from — a runner that will not exec, a run directory or a run state
// file it cannot write.
//
// It is a named type because the two things that must agree about it live in
// different packages. The driver records StopDriverError for it, and the CLI
// maps it to exit 4; a bare error could carry neither decision, and a run that
// died because its harness is missing would report as the code-1 default that
// blames the task file.
//
// It is deliberately kept apart from a failed attempt (§3.4). The unit never
// ran, so charging it a retry would spend a budget the unit never got to use,
// and the run state would carry an exit code no child produced.
type DriverError struct {
	// Op is what the driver was doing, in the message's own words —
	// "create run directory", "spawn implement unit \"alpha\"".
	Op string
	// Err is the underlying cause, kept so the operator sees the real
	// filesystem or exec error rather than only the classification.
	Err error
}

func (e *DriverError) Error() string {
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *DriverError) Unwrap() error { return e.Err }

// Hint is the actionable hint surfaced to the agent alongside exit 4. It is one
// string for every driver error because the answer is always the same shape:
// the run's own environment is not in the state it needs, and no retry of the
// same command changes that until it is.
func (e *DriverError) Hint() string {
	return "the driver could not spawn a unit or write its own state — check that workflow.runner names an " +
		"executable on PATH and that .tp/ under the repository root is writable, then run 'tp run' again"
}

// driverErrorf builds a DriverError whose Op is formatted from the driver's own
// description of what it was doing.
func driverErrorf(err error, format string, args ...any) *DriverError {
	return &DriverError{Op: fmt.Sprintf(format, args...), Err: err}
}
