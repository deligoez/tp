package cli_test

import (
	"encoding/json"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/fakerunner"
)

// seedFoundRequests writes requests F1..Fn into the project's found store,
// each a request that needs a choice, in the given state.
func seedFoundRequests(t *testing.T, dir string, states ...string) {
	t.Helper()
	err := engine.UpdateFoundRequests(dir, filepath.Join(dir, "spec.tasks.json"), func(reqs []engine.FoundRequest) ([]engine.FoundRequest, error) {
		for i, state := range states {
			n := len(reqs) + 1
			reqs = append(reqs, engine.FoundRequest{
				ID: "F" + strconv.Itoa(n), Seq: n, Location: "x.go:" + strconv.Itoa(i+1),
				Reproduction: "go test ./x", Question: "which reading?", Alternatives: []string{"a", "b"},
				State: state,
			})
		}
		return reqs, nil
	})
	require.NoError(t, err)
}

// spec/1.31.0.md §5: with undecided requests and no implement unit ready,
// the run spawns a decision unit carrying at most 5 of them, then another
// for the rest; once none is undecided and still no task is ready, it stops
// with no-ready-task (§4).
func TestRunCommand_UndecidedRequestsSpawnDecisionUnitsOfAtMostFive(t *testing.T) {
	t.Parallel()
	dir := runExitProject(t, exitBlockedTask)
	undecided := engine.FoundStateUndecided
	seedFoundRequests(t, dir, undecided, undecided, undecided, undecided, undecided, undecided)
	env, records := seam(t, fakerunner.EnvDurable+"=1")

	stdout, stderr, code := runTPEnv(t, dir, env, "run")
	require.Equal(t, 4, code, stderr)

	recs, err := fakerunner.Records(records)
	require.NoError(t, err)
	require.Len(t, recs, 2, "one decision unit of five, then one of the sixth: %s", stderr)
	for _, rec := range recs {
		assert.Equal(t, string(engine.UnitDecision), rec.Env[engine.EnvUnitKind])
	}
	assert.Equal(t, "F1,F2,F3,F4,F5", recs[0].Env[engine.EnvUnitID])
	assert.Equal(t, "F6", recs[1].Env[engine.EnvUnitID])
	assert.Equal(t, string(engine.StopNoUnits), stopReasonOf(t, stdout))
	assertNoReadyTask(t, dir)
}

// assertNoReadyTask checks that what stopped the run is the no-ready-task
// blocker, as tp resume reports it.
func assertNoReadyTask(t *testing.T, dir string) {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "resume")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, `"code": "no-ready-task"`)
}

// spec/1.31.0.md §4: with only waiting tasks and no undecided request — an
// applied one does not count — the run stops with no-ready-task and spawns
// no decision unit.
func TestRunCommand_OnlyWaitingTasksAndNoUndecidedRequestStopWithNoReadyTask(t *testing.T) {
	t.Parallel()
	dir := runExitProject(t, exitBlockedTask)
	seedFoundRequests(t, dir, engine.FoundStateApplied)
	env, records := seam(t, fakerunner.EnvDurable+"=1")

	stdout, stderr, code := runTPEnv(t, dir, env, "run")
	require.Equal(t, 4, code, stderr)
	assert.Equal(t, string(engine.StopNoUnits), stopReasonOf(t, stdout))
	assertNoReadyTask(t, dir)

	recs, err := fakerunner.Records(records)
	require.NoError(t, err)
	assert.Empty(t, recs, "no undecided request, so no decision unit")
}

// tp found decide is the decision unit's brief: the named requests still
// undecided, the ones already answered, the three answers, and that a rule-9
// decision is never the unit's; an unknown request exits 2.
func TestFoundDecide_BriefsTheNamedUndecidedRequests(t *testing.T) {
	t.Parallel()
	dir := runExitProject(t, exitBlockedTask)
	seedFoundRequests(t, dir, engine.FoundStateUndecided, engine.FoundStateUndecided, engine.FoundStateApplied)

	stdout, stderr, code := runTP(t, dir, "found", "decide", "F1,F3")
	require.Equal(t, 0, code, stderr)
	var brief struct {
		Identity string                `json:"identity"`
		Requests []engine.FoundRequest `json:"requests"`
		Answered []string              `json:"answered"`
		Answers  map[string]string     `json:"answers"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &brief), stdout)
	require.Len(t, brief.Requests, 1)
	assert.Equal(t, "F1", brief.Requests[0].ID)
	assert.Equal(t, "which reading?", brief.Requests[0].Question)
	assert.Equal(t, []string{"F3"}, brief.Answered)
	assert.Contains(t, brief.Identity, "never makes a rule-9 decision")
	assert.Contains(t, brief.Identity, "When in doubt, it answers human")
	assert.ElementsMatch(t, []string{"task", "conforms", "human"}, slices.Collect(maps.Keys(brief.Answers)))

	_, _, code = runTP(t, dir, "found", "decide", "F9")
	assert.Equal(t, 2, code, "an unknown request is refused")
}
