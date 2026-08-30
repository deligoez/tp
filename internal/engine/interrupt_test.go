package engine

import (
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/fakerunner"
)

// driverSignalCases are the two signals §3.4 records as `interrupted`. Both are
// driven, rather than one standing in for the other, because they are two
// separate registrations and a driver that watched only os.Interrupt would pass
// every assertion here under SIGINT and die outright under SIGTERM.
var driverSignalCases = []os.Signal{os.Interrupt, syscall.SIGTERM}

// guardSignal keeps this signal's default disposition off for the duration of
// one sub-test.
//
// The guard is the test's own channel, not the driver's: signal.Notify delivers
// a copy to every registered channel, so the driver still sees the signal, and
// the driver's own registration is still the only thing that can turn it into a
// stop reason. What the guard buys is the failure mode — a regression that
// dropped the driver's watch would otherwise kill the test binary with the
// signal's default disposition and take every other test in the package with
// it, reporting a process death where an assertion failure is what a reader
// needs.
func guardSignal(t *testing.T, sig os.Signal) {
	t.Helper()
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, sig)
	t.Cleanup(func() { signal.Stop(ch) })
}

// signalSelf delivers sig to this process, which is the driver's process: the
// loop under test runs on a goroutine of this same binary, so the operator's
// signal and the test's are the same event.
func signalSelf(t *testing.T, sig os.Signal) {
	t.Helper()
	proc, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)
	require.NoError(t, proc.Signal(sig))
}

// awaitStop takes the run's result, failing rather than hanging if the signal
// did not end it. A driver that ignores the signal does not fail an assertion —
// it simply never returns — so the bound is the assertion.
func awaitStop(t *testing.T, done <-chan DriverResult) DriverResult {
	t.Helper()
	select {
	case res := <-done:
		return res
	case <-time.After(90 * time.Second):
		t.Fatal("the driver did not stop after the signal")
		return DriverResult{}
	}
}

// §3.4 test 41: SIGINT and SIGTERM stop the run with `interrupted`, letting
// in-flight children finish and spawning no new unit.
//
// The signal is delivered for real, to this process, while three role children
// are asleep inside one iteration. That is the whole point of the arrangement:
// the claim is about *when* the driver notices, and a test that called the
// watch directly could not tell a driver that waits for its children from one
// that abandons them, nor a driver that stops from one that spawns the record
// unit the next iteration owes.
func TestRunDriver_SignalStopsTheRunWithInterrupted(t *testing.T) {
	for _, sig := range driverSignalCases {
		t.Run(sig.String(), func(t *testing.T) {
			guardSignal(t, sig)
			root, spec, taskFile, records := seamProject(t, `{"spec":"s.md","tasks":[]}`)
			t.Setenv(fakerunner.EnvDurable, "1")
			t.Setenv(fakerunner.EnvSleepMS, "1000")

			done := driveInBackground(t, root, spec, taskFile, driverWorkflow())
			awaitFirstChild(t, records)
			signalSelf(t, sig)
			res := awaitStop(t, done)

			assert.Equal(t, StopInterrupted, res.StopReason)

			// The panel was mid-flight when the signal arrived, and every one
			// of its children still wrote its own record: none was killed, and
			// the driver waited for all three.
			// A run that carried on would have spawned this round's record
			// unit next, so a fourth claimed slot is exactly "a new unit was
			// spawned after the signal", and a claimed slot with no record is
			// exactly "a child was killed". One assertion rules out both.
			recs := invocations(t, records)
			assert.Equal(t, len(recs), claimedInvocations(t, records),
				"no unit was spawned after the signal and no in-flight child was killed")
			require.Len(t, recs, 3, "the in-flight children finished")
			for i := range recs {
				assert.Equal(t, 0, recs[i].ExitCode, "each child exited on its own terms")
			}

			// §3.4: the run state is written before the driver exits, so an
			// operator who signalled a backgrounded run finds the reason on
			// disk rather than only in an exit code.
			st, err := ReadRunState(root, taskFile)
			require.NoError(t, err)
			require.NotNil(t, st.StopReason)
			assert.Equal(t, StopInterrupted, *st.StopReason)
			assert.Len(t, st.Units, 3, "one row per in-flight child, all of them finished")
			for i := range st.Units {
				assert.NotNil(t, st.Units[i].ExitCode, "the driver waited for every child it spawned")
			}
		})
	}
}

// §3.4's "no new unit is spawned" reaches inside the iteration as well: a unit
// that failed its first attempt still has a retry in its budget, and a signal
// that arrived while that attempt was running spends it on nothing.
//
// The stop is `interrupted` rather than `unit-failure` because the unit did not
// exhaust its attempts — §3.4 ranks unit-failure above interrupted only for the
// unit that actually ran out of them.
func TestRunDriver_SignalDuringAnAttemptSpendsNoRetry(t *testing.T) {
	guardSignal(t, os.Interrupt)
	root, spec, taskFile, records := seamProject(t,
		`{"spec":"s.md","tasks":[{"id":"alpha","title":"A","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]}`)
	recordRounds(t, spec, 0, 2, true)
	t.Setenv(fakerunner.EnvExits, "1")
	t.Setenv(fakerunner.EnvSleepMS, "1000")

	wf := driverWorkflow()
	wf.RunMaxUnitRetries = 1 // two attempts, so a retry is available to be skipped

	done := driveInBackground(t, root, spec, taskFile, wf)
	awaitFirstChild(t, records)
	signalSelf(t, os.Interrupt)
	res := awaitStop(t, done)

	assert.Equal(t, StopInterrupted, res.StopReason,
		"a unit with a retry left in its budget did not exhaust its attempts")
	assert.Equal(t, 1, claimedInvocations(t, records), "the retry was never spawned")
	require.Len(t, invocations(t, records), 1)
}
