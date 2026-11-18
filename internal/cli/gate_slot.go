package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
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
	// times is the gate time file the wait and the gate add to; "" records
	// none.
	times string
	// runDir and seq name the unit attempt this gate belongs to; an empty
	// runDir is a gate outside a run.
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
	return gateTaker{dir: engine.GateSlotsDir(runDir), times: engine.GateTimesPath(runDir, seq), runDir: runDir, seq: seq}
}

// mainGateTaker is the main-branch gate of the driver's close, which alone
// sets engine.EnvGateMainSlots on its tp done child.
func mainGateTaker() gateTaker {
	return gateTaker{dir: os.Getenv(engine.EnvGateMainSlots), times: os.Getenv(engine.EnvGateMainTimes), main: true}
}

// take waits for a slot when the run caps its gates and returns what to call
// once the gate has run: it frees the slot and adds the gate's time. With no
// cap or no slot directory it waits for nothing. The gate's own timeout starts
// after take returns, so the wait never counts against it.
//
// SIGINT or SIGTERM during the wait ends it: tp done exits before any gate
// runs, holding no slot and asking for no close. The signals are caught for
// the wait alone, so a signal during the gate acts as it always has.
func (g gateTaker) take(capN int) (release func()) {
	if g.dir == "" || capN <= 0 {
		return func() {}
	}
	start := time.Now()
	g.addTimes(start, 0, 0)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	slot, err := engine.AcquireGateSlot(ctx, g.dir, capN, g.main)
	interrupted := ctx.Err() != nil
	stop()
	waited := time.Since(start)
	g.addTimes(time.Time{}, waited, 0)
	if err != nil && interrupted {
		output.Error(ExitState, "interrupted while waiting for a gate slot: no gate ran and no close was asked for",
			"ask for the close again when the run resumes")
		os.Exit(ExitState)
	}
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

// addTimes records into the taker's gate time file. The record is the run's
// observability, so a failed write never fails the gate.
func (g gateTaker) addTimes(since time.Time, waited, gated time.Duration) {
	if g.times != "" {
		_ = engine.AddGateTimes(g.times, since, waited, gated)
	}
}
