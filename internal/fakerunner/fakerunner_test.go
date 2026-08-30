package fakerunner_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/fakerunner"
)

// build compiles the fake runner and returns its path together with a fresh
// record directory, which is the whole setup a test needs.
func build(t *testing.T) (bin, dir string) {
	t.Helper()
	dir = t.TempDir()
	bin, err := fakerunner.Build(dir)
	require.NoError(t, err)
	return bin, dir
}

// spawn runs the fake runner once with extra environment and returns its exit
// code, so a test can assert on the scripted code as well as on the record.
func spawn(t *testing.T, bin, dir string, env []string, args ...string) int {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), append([]string{fakerunner.EnvDir + "=" + dir}, env...)...)
	err := cmd.Run()
	var exit *exec.ExitError
	if err != nil {
		require.ErrorAs(t, err, &exit, "the fake runner failed for a reason other than its scripted exit")
		return exit.ExitCode()
	}
	return 0
}

// Every invocation is recorded with its argv, its environment and the two
// timestamps that make concurrency observable from outside the driver.
func TestFakeRunner_RecordsEachInvocation(t *testing.T) {
	bin, dir := build(t)

	spawn(t, bin, dir, []string{"TP_UNIT_ID=first"}, "implement", "first", "", "0")
	spawn(t, bin, dir, []string{"TP_UNIT_ID=second"}, "review-role", "second", "", "1.5")

	records, err := fakerunner.Records(dir)
	require.NoError(t, err)
	require.Len(t, records, 2, "one record per invocation, in the order they claimed their slots")

	assert.Equal(t, 0, records[0].Seq)
	assert.Equal(t, []string{"implement", "first", "", "0"}, records[0].Argv)
	assert.Equal(t, "first", records[0].Env["TP_UNIT_ID"], "the child's environment is recorded whole")

	assert.Equal(t, 1, records[1].Seq)
	assert.Equal(t, []string{"review-role", "second", "", "1.5"}, records[1].Argv)
	assert.Equal(t, "second", records[1].Env["TP_UNIT_ID"])

	for _, rec := range records {
		assert.False(t, rec.SpawnedAt.IsZero(), "a spawn time is recorded")
		assert.False(t, rec.ExitedAt.IsZero(), "an exit time is recorded")
		assert.False(t, rec.ExitedAt.Before(rec.SpawnedAt), "a child cannot exit before it spawned")
	}
}

// Scripted exit codes are consumed in invocation order, and the list runs out
// into success, so a test scripts only the failures it cares about.
func TestFakeRunner_ScriptedExitCodes(t *testing.T) {
	bin, dir := build(t)
	scripted := []string{fakerunner.EnvExits + "=2,1"}

	assert.Equal(t, 2, spawn(t, bin, dir, scripted, "implement", "a", "", "0"))
	assert.Equal(t, 1, spawn(t, bin, dir, scripted, "implement", "a", "", "0"))
	assert.Equal(t, 0, spawn(t, bin, dir, scripted, "implement", "a", "", "0"),
		"an invocation past the end of the list succeeds")

	records, err := fakerunner.Records(dir)
	require.NoError(t, err)
	require.Len(t, records, 3)
	assert.Equal(t, []int{2, 1, 0}, []int{records[0].ExitCode, records[1].ExitCode, records[2].ExitCode},
		"the record carries the code the child actually exited with")
}

// The fake writes §3.2.1's final log line carrying the seam's spend_key, which
// is what makes the spend and budget-cap paths testable without an agent.
func TestFakeRunner_WritesFinalSpendLine(t *testing.T) {
	bin, dir := build(t)
	logPath := filepath.Join(t.TempDir(), "logs", "1-implement-a.jsonl")

	spawn(t, bin, dir, []string{fakerunner.EnvSpend + "=1.25"}, "implement", "a", logPath, "0")

	data, err := os.ReadFile(logPath)
	require.NoError(t, err, "the fake creates its log path's parents rather than needing the driver to")
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")

	var final map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[len(lines)-1]), &final))
	assert.InDelta(t, 1.25, final["total_cost_usd"], 1e-9)
}

// Two children spawned at once produce overlapping recorded windows. This is
// the assertion §10.1 test 5 rests on, so it is measured here rather than
// assumed: without it, "role units ran concurrently" cannot be told apart from
// "they ran quickly one after the other".
func TestFakeRunner_ConcurrentInvocationsOverlap(t *testing.T) {
	bin, dir := build(t)
	slow := []string{fakerunner.EnvSleepMS + "=200"}

	var wg sync.WaitGroup
	for _, id := range []string{"role-a", "role-b"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			spawn(t, bin, dir, slow, "review-role", id, "", "0")
		}()
	}
	wg.Wait()

	records, err := fakerunner.Records(dir)
	require.NoError(t, err)
	require.Len(t, records, 2)
	a, b := records[0], records[1]
	assert.True(t, a.SpawnedAt.Before(b.ExitedAt) && b.SpawnedAt.Before(a.ExitedAt),
		"the two recorded windows overlap: %v-%v and %v-%v", a.SpawnedAt, a.ExitedAt, b.SpawnedAt, b.ExitedAt)
}

// The two halves fit: the runner TP_RUNNER_SEAM resolves to spawns this binary,
// and what it records is exactly the argv the seam's template expanded to.
func TestSeamPinsTheFakeRunner(t *testing.T) {
	bin, dir := build(t)
	t.Setenv(engine.EnvRunnerSeam, bin)

	v := engine.TemplateValues{
		Prompt:       "run the brief",
		Kind:         engine.UnitAuditRole,
		ID:           "go-safety",
		LogPath:      filepath.Join(t.TempDir(), "7-audit-role-go-safety.jsonl"),
		MaxBudgetUSD: 3,
	}
	runner, err := engine.ResolveUnitRunner(engine.DefaultRunner(), v)
	require.NoError(t, err)
	require.Equal(t, bin, runner.Cmd)

	assert.Equal(t, 0, spawn(t, bin, dir, nil, runner.Args...))

	records, err := fakerunner.Records(dir)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, []string{"audit-role", "go-safety", v.LogPath, "3"}, records[0].Argv,
		"the seam's positional list reaches the child as the unit's own identity")
}
