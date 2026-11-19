package engine

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/fakerunner"
	"github.com/deligoez/tp/internal/model"
)

// rollingTasks builds a task file under run_max_parallel cap whose tasks have
// disjoint touches, each entry "<id>" or "<id>:solo".
func rollingTasks(capacity int, ids ...string) string {
	rows := make([]string, 0, len(ids))
	for _, entry := range ids {
		id, solo, _ := strings.Cut(entry, ":")
		row := `{"id":"` + id + `","title":"T","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"],"touches":["` + id + `.txt"]`
		if solo == "solo" {
			row += `,"solo":true`
		}
		rows = append(rows, row+"}")
	}
	return `{"spec":"s.md","workflow":{"run_max_parallel":` + strconv.Itoa(capacity) + `},"tasks":[` + strings.Join(rows, ",") + `]}`
}

// driveRolling runs the loop with the batch test's claim, reopen and release
// functions and the given workflow, returning whatever it ended with.
func driveRolling(t *testing.T, root, spec, taskFile string, wf *model.Workflow, mods ...func(*DriverOptions)) (DriverResult, error) {
	t.Helper()
	opts := rollingOptions(root, spec, taskFile, wf)
	for _, mod := range mods {
		mod(opts)
	}
	type outcome struct {
		res DriverResult
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		res, err := RunDriver(opts)
		done <- outcome{res, err}
	}()
	select {
	case o := <-done:
		return o.res, o.err
	case <-time.After(90 * time.Second):
		t.Fatal("the driver loop did not terminate")
		return DriverResult{}, nil
	}
}

// byUnit groups the recorded invocations by TP_UNIT_ID, in spawn order.
func byUnit(recs []fakerunner.Invocation) map[string][]fakerunner.Invocation {
	out := map[string][]fakerunner.Invocation{}
	for _, r := range recs {
		out[r.Env[EnvUnitID]] = append(out[r.Env[EnvUnitID]], r)
	}
	return out
}

// TestRolling_AShortUnitsSlotIsRefilledWhileALongSiblingRuns: under cap 2, the
// slot a short unit frees is refilled from the oracle as soon as its close is
// integrated, so the third task starts while the long one is still running
// (spec/backlog/rolling-implement.md §2). A barrier starts it only after.
func TestRolling_AShortUnitsSlotIsRefilledWhileALongSiblingRuns(t *testing.T) {
	root, spec, taskFile, records := batchProject(t, rollingTasks(2, "alpha", "beta", "gamma"))
	t.Setenv(fakerunner.EnvDurable, "1")
	t.Setenv(fakerunner.EnvSleepByID, "alpha=2000,beta=100,gamma=100")

	res, err := driveRolling(t, root, spec, taskFile, driverWorkflow())
	require.NoError(t, err)
	assert.Equal(t, StopConverged, res.StopReason)
	units := byUnit(invocations(t, records))
	require.Len(t, units["alpha"], 1)
	require.Len(t, units["gamma"], 1)
	assert.True(t, units["gamma"][0].SpawnedAt.Before(units["alpha"][0].ExitedAt),
		"gamma takes beta's slot while alpha still runs")
	for _, id := range []string{"alpha", "beta", "gamma"} {
		assert.Equal(t, model.StatusDone, taskByID(t, taskFile, id).Status)
	}
	assert.NotContains(t, gitIn(t, root, "worktree", "list"), "tp-worktrees")
}

// TestRolling_ARetryRunsInItsOwnSlotWhileASiblingRuns: a failed attempt is
// retried in the same worktree, in its own slot, while the other slot's unit
// keeps running.
func TestRolling_ARetryRunsInItsOwnSlotWhileASiblingRuns(t *testing.T) {
	root, spec, taskFile, records := batchProject(t, rollingTasks(2, "alpha", "beta"))
	t.Setenv(fakerunner.EnvDurable, "1")
	t.Setenv(fakerunner.EnvFailFirst, "alpha")
	t.Setenv(fakerunner.EnvSleepByID, "alpha=100,beta=2000")
	wf := driverWorkflow()
	wf.RunMaxUnitRetries = 1

	res, err := driveRolling(t, root, spec, taskFile, wf)
	require.NoError(t, err)
	assert.Equal(t, StopConverged, res.StopReason)
	units := byUnit(invocations(t, records))
	require.Len(t, units["alpha"], 2, "alpha is attempted twice")
	require.Len(t, units["beta"], 1)
	assert.Equal(t, units["alpha"][0].Dir, units["alpha"][1].Dir, "the retry runs in the same worktree")
	assert.True(t, strings.HasPrefix(CanonicalPath(units["alpha"][1].Dir), CanonicalPath(WorktreeBase(root))))
	assert.True(t, overlapped(&units["alpha"][1], &units["beta"][0]), "the retry runs beside beta, not after it")
}

// TestRolling_RunMaxUnitsIsCheckedBeforeEverySpawn: the unit cap is checked
// before each spawn, so a pool of three under run_max_units 2 starts two.
func TestRolling_RunMaxUnitsIsCheckedBeforeEverySpawn(t *testing.T) {
	root, spec, taskFile, records := batchProject(t, rollingTasks(3, "alpha", "beta", "gamma"))
	t.Setenv(fakerunner.EnvDurable, "1")
	wf := driverWorkflow()
	wf.RunMaxUnits = 2

	res, err := driveRolling(t, root, spec, taskFile, wf)
	require.NoError(t, err)
	assert.Equal(t, StopCapUnits, res.StopReason)
	// Records come in the order the children claimed their slots, not the
	// order they were spawned in, so two siblings may land either way round.
	assert.ElementsMatch(t, []string{"alpha", "beta"}, spawnedIDs(invocations(t, records)), "run_max_units is never overshot")
	assert.Equal(t, model.StatusOpen, taskByID(t, taskFile, "gamma").Status, "the third task is never claimed")
}

// TestRolling_NothingStartsWhileAUnitIsIntegrated: the serial rule. From the
// cherry-pick through the close to a discard's reset --keep, no worktree is
// added and no child's process starts, though the sibling's slot is free the
// whole time. Alpha exits while gamma, beta's refill, is being started, so
// alpha's integration is due right then; gamma's first close fails its gate,
// so one window holds the reset.
func TestRolling_NothingStartsWhileAUnitIsIntegrated(t *testing.T) {
	root, spec, taskFile, _ := batchProject(t, rollingTasks(2, "alpha", "beta", "gamma", "zeta"))
	t.Setenv(fakerunner.EnvDurable, "1")
	t.Setenv(fakerunner.EnvSleepByID, "alpha=700,beta=100,gamma=100,zeta=100")

	var mu sync.Mutex
	var violations []string
	inside, windows := "", 0
	poolHook = func(point, id string) {
		mu.Lock()
		defer mu.Unlock()
		switch point {
		case "integrating":
			inside = id
			windows++
		case "integrated":
			inside = ""
		case "added", "spawned":
			if inside != "" {
				violations = append(violations, point+" "+id+" during the integration of "+inside)
			}
		}
	}
	t.Cleanup(disarm)
	refused := false
	res, err := driveRolling(t, root, spec, taskFile, driverWorkflow(), func(o *DriverOptions) {
		inner := o.CloseTask
		o.CloseTask = func(id, reason string, commits []string, logPath string) error {
			time.Sleep(400 * time.Millisecond)
			mu.Lock()
			first := id == "gamma" && !refused
			refused = refused || first
			mu.Unlock()
			if first {
				return errors.New("quality gate failed")
			}
			return inner(id, reason, commits, logPath)
		}
	})
	require.NoError(t, err)
	assert.Equal(t, StopConverged, res.StopReason)
	assert.True(t, refused, "gamma's first close was discarded")
	assert.True(t, taskByID(t, taskFile, "gamma").Solo, "the discard reopened gamma solo")
	assert.GreaterOrEqual(t, windows, 3, "alpha and beta integrate and gamma's pick is discarded, all in the pool")
	assert.Empty(t, violations)
}

// TestRolling_ASoloLeadDrainsThePool: when the next task to run is solo, the
// pool is not refilled; the running units finish, the solo task runs alone in
// the main tree, and only then does the task behind it start.
func TestRolling_ASoloLeadDrainsThePool(t *testing.T) {
	root, spec, taskFile, records := batchProject(t, rollingTasks(2, "alpha", "beta", "gamma:solo", "zeta"))
	t.Setenv(fakerunner.EnvDurable, "1")
	t.Setenv(fakerunner.EnvSleepByID, "alpha=1500,beta=100,gamma=200,zeta=100")

	res, err := driveRolling(t, root, spec, taskFile, driverWorkflow())
	require.NoError(t, err)
	assert.Equal(t, StopConverged, res.StopReason)
	recs := invocations(t, records)
	units := byUnit(recs)
	require.Len(t, units["gamma"], 1)
	require.Len(t, units["zeta"], 1)
	gamma := units["gamma"][0]
	for i := range recs {
		if recs[i].Env[EnvUnitID] != "gamma" {
			assert.False(t, overlapped(&gamma, &recs[i]), "the solo task runs alone, not beside %s", recs[i].Env[EnvUnitID])
		}
	}
	assert.Equal(t, CanonicalPath(root), CanonicalPath(gamma.Dir), "alone means in the main tree")
	assert.True(t, units["zeta"][0].SpawnedAt.After(gamma.ExitedAt), "zeta waits behind the solo task: spawn order %v", spawnedIDs(recs))
}

// TestRolling_AnEscalationDrainsThePool: an escalation stops new spawns and
// withholds every retry; the running sibling finishes and is integrated, one
// that fails is not attempted again, and the run stops on the escalation.
func TestRolling_AnEscalationDrainsThePool(t *testing.T) {
	root, spec, taskFile, records := batchProject(t, rollingTasks(3, "alpha", "beta", "gamma", "zeta"))
	t.Setenv(fakerunner.EnvDurable, "1")
	t.Setenv(fakerunner.EnvEscalateIDs, "alpha")
	t.Setenv(fakerunner.EnvFailFirst, "gamma")
	t.Setenv(fakerunner.EnvSleepByID, "alpha=100,beta=1500,gamma=600,zeta=100")
	wf := driverWorkflow()
	wf.RunMaxUnitRetries = 1

	res, err := driveRolling(t, root, spec, taskFile, wf)
	require.NoError(t, err)
	assert.Equal(t, StopEscalation, res.StopReason)
	assert.NotEmpty(t, res.EscalationPath)
	units := byUnit(invocations(t, records))
	assert.Len(t, units["gamma"], 1, "a failed sibling is not retried after the escalation: %v", spawnedIDs(invocations(t, records)))
	assert.Empty(t, units["zeta"], "nothing is spawned after the escalation")
	assert.Equal(t, model.StatusDone, taskByID(t, taskFile, "beta").Status, "the running sibling is integrated")
}

// TestRolling_ACrashedRunsOpenRowsResumeAlone: open rows a crashed run left
// in the run state are abandoned when tp run starts. Their tasks are wip
// resumes, each run alone in the main tree, never members of the new pool.
func TestRolling_ACrashedRunsOpenRowsResumeAlone(t *testing.T) {
	root, spec, taskFile, records := batchProject(t, rollingTasks(2, "alpha", "beta", "gamma"))
	require.NoError(t, editTasks(taskFile, []string{"alpha", "beta"}, func(task *model.Task) { task.Status = model.StatusWIP }))
	crashed := `{"run_id":"old","started_at":"2026-11-18T00:00:00Z","phase":"implement","stop_reason":null,` +
		`"totals":{"units":2,"wall_clock_seconds":1,"spend_usd":0},"units":[` +
		`{"seq":1,"kind":"implement","id":"alpha","attempt":1,"exit_code":null,"duration_seconds":null,"spend_usd":null,"log_path":"x","worktree":"/gone/alpha"},` +
		`{"seq":2,"kind":"implement","id":"beta","attempt":1,"exit_code":null,"duration_seconds":null,"spend_usd":null,"log_path":"y","worktree":"/gone/beta"}]}`
	require.NoError(t, os.MkdirAll(filepath.Dir(RunStatePath(root, taskFile)), 0o750))
	require.NoError(t, os.WriteFile(RunStatePath(root, taskFile), []byte(crashed), 0o600))
	t.Setenv(fakerunner.EnvDurable, "1")
	t.Setenv(fakerunner.EnvSleepMS, "100")

	res, err := driveRolling(t, root, spec, taskFile, driverWorkflow())
	require.NoError(t, err)
	assert.Equal(t, StopConverged, res.StopReason)
	recs := invocations(t, records)
	require.Equal(t, []string{"alpha", "beta", "gamma"}, spawnedIDs(recs), "the wip tasks resume first")
	for _, i := range []int{0, 1} {
		assert.Equal(t, CanonicalPath(root), CanonicalPath(recs[i].Dir), "%s resumes in the main tree", spawnedIDs(recs)[i])
		for j := range recs {
			if j != i {
				assert.False(t, overlapped(&recs[i], &recs[j]), "a wip resume runs alone")
			}
		}
	}
}

// rollingOptions are the loop's options with the batch test's close, claim,
// reopen and release functions.
func rollingOptions(root, spec, taskFile string, wf *model.Workflow) *DriverOptions {
	closer := &fakeCloser{closed: map[string][]string{}}
	return &DriverOptions{
		Root: root, TaskFile: taskFile, Spec: spec, Workflow: *wf, CloseTask: closer.close(taskFile),
		ClaimTasks: func(ids []string) error {
			return editTasks(taskFile, ids, func(task *model.Task) {
				if task.Status == model.StatusOpen {
					task.Status = model.StatusWIP
				}
			})
		},
		ReopenSolo: func(id string) error {
			return editTasks(taskFile, []string{id}, func(task *model.Task) {
				task.Status, task.Solo = model.StatusOpen, true
			})
		},
		ReleaseTasks: func(ids []string) error {
			return editTasks(taskFile, ids, func(task *model.Task) {
				if task.Status == model.StatusWIP {
					task.Status = model.StatusOpen
				}
			})
		},
	}
}
