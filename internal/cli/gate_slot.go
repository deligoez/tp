package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// gateTaker is which of a run's gate slots a gate takes
// (spec/backlog/gate-slots.md): a unit's worktree gate, the main-branch gate
// of the driver's own close, or none — a gate run by hand outside a run.
type gateTaker struct {
	// dir is the slot directory; "" takes no slot.
	dir  string
	main bool
	// runDir and seq name the unit attempt whose gate times the wait and
	// the gate add to; an empty runDir records none.
	runDir string
	seq    int
}

// unitGateTaker is a unit's worktree gate, which finds its run's slots from
// the environment the driver spawned it with.
func unitGateTaker() gateTaker {
	runDir := os.Getenv(engine.EnvRunDir)
	seq, err := strconv.Atoi(os.Getenv(engine.EnvUnitSeq))
	if runDir == "" || err != nil {
		return gateTaker{}
	}
	return gateTaker{dir: engine.GateSlotsDir(runDir), runDir: runDir, seq: seq}
}

// mainGateTaker is the main-branch gate of the driver's close, which alone
// sets engine.EnvGateMainSlots on its tp done child.
func mainGateTaker() gateTaker {
	return gateTaker{dir: os.Getenv(engine.EnvGateMainSlots), main: true}
}

// take waits for a slot when the run caps its gates and returns what to call
// once the gate has run: it frees the slot and adds the gate's time. With no
// cap or no slot directory it waits for nothing. The gate's own timeout starts
// after take returns, so the wait never counts against it.
func (g gateTaker) take(capN int) (release func()) {
	if g.dir == "" || capN <= 0 {
		return func() {}
	}
	start := time.Now()
	g.addTimes(start, 0, 0)
	slot, err := engine.AcquireGateSlot(context.Background(), g.dir, capN, g.main)
	waited := time.Since(start)
	g.addTimes(time.Time{}, waited, 0)
	if err != nil {
		output.Error(ExitFile, err.Error(), runtimeFailureHint)
		os.Exit(ExitFile)
	}
	if g.main {
		output.Notice(fmt.Sprintf("waited %s for a gate slot", waited.Round(time.Millisecond)))
	}
	gateStart := time.Now()
	return func() {
		slot.Release()
		g.addTimes(time.Time{}, 0, time.Since(gateStart))
	}
}

// addTimes records into the unit attempt's gate times. The record is the
// run's observability, so a failed write never fails the gate.
func (g gateTaker) addTimes(since time.Time, waited, gated time.Duration) {
	if g.runDir != "" {
		_ = engine.AddGateTimes(g.runDir, g.seq, since, waited, gated)
	}
}
