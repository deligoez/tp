package engine

import (
	"context"
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

// lockWaitError is the error of a lock wait that ended without the lock.
func lockWaitError(ctx context.Context, err error) error {
	if err != nil {
		return err
	}
	return ctx.Err()
}
