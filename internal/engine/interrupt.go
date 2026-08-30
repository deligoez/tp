package engine

import (
	"os"
	"os/signal"
	"syscall"
)

// interrupts is the driver's watch on the two signals §3.4 records as
// `interrupted`: SIGINT and SIGTERM.
//
// It is a checkpoint the loop reads, not a cancellation the runtime performs,
// because the driver has nothing to cancel. §3.4 gives a signal exactly the
// terms a cap gets — in-flight children finish, no new unit is spawned, and the
// run state is written before the driver exits — so the only thing a signal
// changes is whether the *next* unit is spawned. A handler that interrupted the
// wait would have to decide what to do with the children it walked away from,
// which is the design that sentence rules out.
//
// Registering the channel also takes the two signals off their default
// disposition, which is the point rather than a side effect: a run that died on
// the default would leave a stop_reason of null, and an operator who stopped an
// unattended run would be handed no record of why it ended.
type interrupts struct {
	ch chan os.Signal
	// seen is sticky. A checkpoint that reads the signal and then defers to a
	// higher-precedence reason must not be the only one that could ever have
	// seen it, so the arrival is remembered rather than consumed.
	seen bool
}

// watchInterrupts starts the watch. The channel is buffered so the runtime's
// delivery never blocks on a driver that is inside a child's wait — a dropped
// second signal costs nothing, since one is all the loop needs.
func watchInterrupts() *interrupts {
	w := &interrupts{ch: make(chan os.Signal, 1)}
	signal.Notify(w.ch, os.Interrupt, syscall.SIGTERM)
	return w
}

// stop ends the watch and restores both signals' default disposition, so a
// process that goes on doing something else after the run — a test binary, a
// future caller with a second run — is not left ignoring them.
func (w *interrupts) stop() {
	signal.Stop(w.ch)
}

// signalled reports whether the operator has signalled this run.
//
// It polls rather than blocks: every caller is a point in the loop where the
// answer decides whether to spawn the next unit, and a blocking read would
// stop a run nobody signalled. It is called only from the loop's own goroutine.
func (w *interrupts) signalled() bool {
	select {
	case <-w.ch:
		w.seen = true
	default:
	}
	return w.seen
}
