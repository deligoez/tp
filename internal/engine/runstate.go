package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// RunState is §3.5's run state file — .tp/run-<base>.json, named per task file
// because the run lock is per task file and two runs over different cycles in
// one repository are permitted.
//
// It is observability, not truth: no decision, the driver's or a hook's, reads
// it back. A driver that dies leaves it behind and the next tp run re-derives
// everything it needs from the cycle state, which is why the accrued totals
// restart at zero on a new run — the caps bound a run, not a cycle.
type RunState struct {
	RunID      string       `json:"run_id"`
	StartedAt  string       `json:"started_at"`
	Phase      string       `json:"phase"`
	StopReason *string      `json:"stop_reason"`
	Totals     RunTotals    `json:"totals"`
	Units      []RunUnitRow `json:"units"`
}

// RunTotals are the run's accrued figures — what §3.4's caps bound. Units
// counts attempts rather than distinct units, so a retried unit is counted
// again and run_max_units, this number and --status's units-done never
// disagree. SpendUSD sums only the rows a runner metered; a runner declaring
// no spend_key contributes nothing and leaves cap-budget inert.
type RunTotals struct {
	Units            int     `json:"units"`
	WallClockSeconds int     `json:"wall_clock_seconds"`
	SpendUSD         float64 `json:"spend_usd"`
}

// RunUnitRow is one unit attempt. The three result fields are pointers because
// the row is written twice: appended null before the child is spawned, then
// updated in place when it exits. Null therefore means "still running" (or, in
// a file left by a dead driver, "never finished"), which a zero exit code and a
// zero duration could not express. SpendUSD stays null for a runner that meters
// nothing, so an unmetered unit is never reported as having cost zero.
type RunUnitRow struct {
	Seq             int      `json:"seq"`
	Kind            UnitKind `json:"kind"`
	ID              string   `json:"id"`
	Attempt         int      `json:"attempt"`
	ExitCode        *int     `json:"exit_code"`
	DurationSeconds *float64 `json:"duration_seconds"`
	SpendUSD        *float64 `json:"spend_usd"`
	LogPath         string   `json:"log_path"`
}

// RunStatePath returns a cycle's run state file, .tp/run-<base>.json under the
// repository root — absolute, for the reason TP_RUN_DIR is (§3.1.1): the
// readers of this path do not all share a working directory.
func RunStatePath(root, taskFile string) string {
	return absoluteUnder(root, filepath.Join(".tp", "run-"+RunBase(taskFile)+".json"))
}

// ReadRunState reads a cycle's run state file. The error is os.ErrNotExist when
// no run state exists for the task file, which is the condition `tp run
// --status` reports as exit 3 rather than as a failure.
//
// It is the reporting path's reader and nothing else: the driver never reads
// its own run state back, since two sources of truth for one fact is the drift
// §3.5 exists to avoid.
func ReadRunState(root, taskFile string) (*RunState, error) {
	data, err := os.ReadFile(RunStatePath(root, taskFile))
	if err != nil {
		return nil, err
	}
	var st RunState
	if unmarshalErr := json.Unmarshal(data, &st); unmarshalErr != nil {
		return nil, fmt.Errorf("parse run state: %w", unmarshalErr)
	}
	if st.Units == nil {
		st.Units = make([]RunUnitRow, 0)
	}
	return &st, nil
}

// RunRecorder owns one run's state file. Every mutation writes the whole file
// atomically (temp + rename), because concurrent role siblings finish in the
// same iteration and a reader — `tp run --status`, an operator — takes no lock.
//
// The mutex is what serializes those siblings: they are goroutines of the one
// driver process, and the run lock (§3.2) already keeps a second driver off the
// same task file, so a file lock here would guard nothing a mutex does not.
type RunRecorder struct {
	path  string
	start time.Time
	mu    sync.Mutex
	state RunState
}

// NewRunRecorder starts a run's state file, replacing any file a previous run
// left at the same path: the totals restart at zero because they bound this run
// (§3.5), and a resumed cycle inherits none of the dead run's accrual. The
// initial file is written before the first unit, so a run is visible as soon as
// it starts.
func NewRunRecorder(root, taskFile, runID, phase string) (*RunRecorder, error) {
	started := time.Now().UTC()
	r := &RunRecorder{
		path:  RunStatePath(root, taskFile),
		start: started,
		state: RunState{
			RunID:     runID,
			StartedAt: started.Format(time.RFC3339Nano),
			Phase:     phase,
			Units:     make([]RunUnitRow, 0),
		},
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return nil, fmt.Errorf("create run state dir: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.writeLocked(); err != nil {
		return nil, err
	}
	return r, nil
}

// StartUnit appends a unit attempt's row and writes the file, which the driver
// does before spawning the child. The three result fields are cleared here
// rather than trusted from the caller: an appended row reporting an exit code
// its child has not produced yet is exactly the completion-token fragility the
// design removes. The row is copied, so the caller's own value is untouched.
func (r *RunRecorder) StartUnit(row *RunUnitRow) error {
	started := *row
	started.ExitCode = nil
	started.DurationSeconds = nil
	started.SpendUSD = nil

	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.Units = append(r.state.Units, started)
	return r.writeLocked()
}

// FinishUnit updates the row appended for seq once its child has exited, and
// reports an error for a seq no row carries — a driver finishing an attempt it
// never started is a bug worth surfacing rather than a row worth inventing.
//
// spend is nil for a runner that declares no spend_key; the row and the run
// totals then both leave that attempt out.
func (r *RunRecorder) FinishUnit(seq, exitCode int, duration time.Duration, spend *float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	row := r.rowLocked(seq)
	if row == nil {
		return fmt.Errorf("run state: no unit row with seq %d", seq)
	}
	code := exitCode
	row.ExitCode = &code
	seconds := duration.Seconds()
	row.DurationSeconds = &seconds
	if spend != nil {
		amount := *spend
		row.SpendUSD = &amount
	}
	return r.writeLocked()
}

// SetPhase records the phase the driver is now working in, so a run that
// crosses from implement into audit reports where it stopped rather than where
// it began.
func (r *RunRecorder) SetPhase(phase string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.Phase = phase
	return r.writeLocked()
}

// Stop records the run's stop reason (§3.4) and writes the final state. Its
// presence is what separates a stopped run from one whose driver died: a
// crashed run's file simply never gains one.
func (r *RunRecorder) Stop(reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.StopReason = &reason
	return r.writeLocked()
}

// Snapshot returns the state as last written, with the totals accrued to now.
// It is how the driver reads its own accrual for a cap check without reading
// the file back. The copied rows share the caller-invisible result pointers,
// which is safe because a row's pointers are replaced on update and never
// written through.
func (r *RunRecorder) Snapshot() RunState {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.accrueLocked()
	out := r.state
	out.Units = make([]RunUnitRow, len(r.state.Units))
	copy(out.Units, r.state.Units)
	return out
}

// rowLocked returns the row carrying seq, or nil. Seq is unique within a run —
// every attempt takes a fresh one (§3.1) — so the first match is the only one.
func (r *RunRecorder) rowLocked(seq int) *RunUnitRow {
	for i := range r.state.Units {
		if r.state.Units[i].Seq == seq {
			return &r.state.Units[i]
		}
	}
	return nil
}

// accrueLocked recomputes the totals from the rows and the run's own clock, so
// the accrual can never drift from the rows it summarizes — the failure a
// separately incremented counter invites when a row is updated twice.
func (r *RunRecorder) accrueLocked() {
	spend := 0.0
	for i := range r.state.Units {
		if r.state.Units[i].SpendUSD != nil {
			spend += *r.state.Units[i].SpendUSD
		}
	}
	r.state.Totals.Units = len(r.state.Units)
	r.state.Totals.WallClockSeconds = int(time.Since(r.start).Seconds())
	r.state.Totals.SpendUSD = spend
}

// writeLocked accrues the totals and writes the file atomically. The caller
// holds r.mu.
func (r *RunRecorder) writeLocked() error {
	r.accrueLocked()
	data, err := json.MarshalIndent(&r.state, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(r.path, append(data, '\n'))
}

// writeFileAtomic writes data to a unique sibling temp file and renames it over
// path, so a reader taking no lock sees either the previous file or the new one
// and never a half-written one. The temp name is unique rather than a fixed
// path+".tmp": two writers sharing one temp path race to rename it and the
// loser's rename fails after the winner has moved it away.
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
