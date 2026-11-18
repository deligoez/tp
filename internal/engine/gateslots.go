package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

// Gate slots bound how many quality gates one run holds at once
// (spec/backlog/gate-slots.md): run_max_parallel_gates slot files under the
// run directory, each held by an flock for as long as its gate runs, so a
// holder that dies frees its slot at once — the kernel drops the lock with the
// process.
const (
	gateSlotsDirName = "gate-slots"
	gateMainLockName = "main.lock"
)

// GateSlotsDir is where a run's slot files live: under its run directory, so
// two runs never share a slot and a child finds them from TP_RUN_DIR.
func GateSlotsDir(runDir string) string {
	return filepath.Join(runDir, gateSlotsDirName)
}

// gateSlotPoll is how often a waiting gate tries the slots again.
const gateSlotPoll = 50 * time.Millisecond

// GateSlot is one held slot; Release frees it. A slot holding no lock is a
// gate under no cap, and releasing it does nothing.
type GateSlot struct {
	lock *flock.Flock
}

// Release frees the slot.
func (s *GateSlot) Release() {
	if s != nil && s.lock != nil {
		_ = s.lock.Unlock()
	}
}

// AcquireGateSlot waits until one of capN slots under dir is free and takes
// it; a capN of 0 or less is no cap, and the slot returned holds nothing. It
// gives up with ctx's error when ctx ends first.
//
// The main-branch gate goes first: it holds main.lock exclusively while it
// waits, and a unit gate tries the slots only under a shared hold of that
// lock, so no unit takes a slot while the main gate waits for one. Both locks
// are flocks on files opened close-on-exec, so the gate's own child never
// inherits them and a holder that dies drops them with its last descriptor.
func AcquireGateSlot(ctx context.Context, dir string, capN int, main bool) (*GateSlot, error) {
	if capN <= 0 {
		return &GateSlot{}, nil
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	mainLock := flock.New(filepath.Join(dir, gateMainLockName))
	gate := mainLock
	if main {
		ok, err := mainLock.TryLockContext(ctx, gateSlotPoll)
		if !ok {
			return nil, lockWaitError(ctx, err)
		}
		defer func() { _ = mainLock.Unlock() }()
		gate = nil
	}
	ticker := time.NewTicker(gateSlotPoll)
	defer ticker.Stop()
	for {
		slot, err := tryGateSlots(dir, capN, gate)
		if slot != nil || err != nil {
			return slot, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

// tryGateSlots takes the first free slot without waiting. A non-nil gate is a
// unit's view of main.lock: while the main gate holds it, the unit takes
// nothing.
func tryGateSlots(dir string, capN int, gate *flock.Flock) (*GateSlot, error) {
	if gate != nil {
		ok, err := gate.TryRLock()
		if err != nil || !ok {
			return nil, err
		}
		defer func() { _ = gate.Unlock() }()
	}
	for i := range capN {
		lock := flock.New(filepath.Join(dir, fmt.Sprintf("slot-%d.lock", i)))
		ok, err := lock.TryLock()
		if err != nil {
			return nil, err
		}
		if ok {
			return &GateSlot{lock: lock}, nil
		}
	}
	return nil, nil
}

// EnvGateMainSlots names the slot directory the main-branch close takes the
// main gate's slot from. The driver sets it on its own tp done child only, so
// a close run by hand outside a run takes no slot.
const EnvGateMainSlots = "TP_GATE_MAIN_SLOTS"

// GateTimes is one gate taker's time waiting for a slot and time in the gate:
// a unit attempt's, summed over every close it asked for, which the driver
// copies onto the attempt's row when the unit exits; or a main-branch close's,
// which the driver adds to the run's totals. WaitingSince is set while a wait
// is in progress, so a taker killed mid-wait still says since when it waited.
type GateTimes struct {
	WaitingSince  string  `json:"waiting_since,omitempty"`
	WaitedSeconds float64 `json:"waited_seconds"`
	GateSeconds   float64 `json:"gate_seconds"`
}

// GateTimesPath is a unit attempt's gate time file under its run directory,
// keyed by its seq so a retry starts from zero.
func GateTimesPath(runDir string, seq int) string {
	return filepath.Join(runDir, fmt.Sprintf("gate-%d.json", seq))
}

// MainGateTimesPath is the gate time file of the main-branch close of task
// id, which the driver reads once the close has exited.
func MainGateTimesPath(runDir, id string) string {
	return filepath.Join(runDir, "gate-main-"+id+".json")
}

// ReadGateTimes reads a gate time file; false when none was written.
func ReadGateTimes(path string) (GateTimes, bool) {
	data, err := os.ReadFile(path) //nolint:gosec // a path under the run dir, built by GateTimesPath or MainGateTimesPath
	if err != nil {
		return GateTimes{}, false
	}
	var gt GateTimes
	if json.Unmarshal(data, &gt) != nil {
		return GateTimes{}, false
	}
	return gt, true
}

// AddGateTimes adds waited and gated to the gate times at path and sets
// WaitingSince to since, or clears it for a zero since. The write is atomic.
func AddGateTimes(path string, since time.Time, waited, gated time.Duration) error {
	gt, _ := ReadGateTimes(path)
	gt.WaitingSince = ""
	if !since.IsZero() {
		gt.WaitingSince = since.UTC().Format(time.RFC3339Nano)
	}
	gt.WaitedSeconds += waited.Seconds()
	gt.GateSeconds += gated.Seconds()
	data, err := json.MarshalIndent(&gt, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return writeFileAtomic(path, append(data, '\n'))
}

// lockWaitError is the error of a lock wait that ended without the lock.
func lockWaitError(ctx context.Context, err error) error {
	if err != nil {
		return err
	}
	return ctx.Err()
}
