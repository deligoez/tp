package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// writeFinderlessQueue writes a queue beside dir's task file holding one open
// entry no finder waits on and one decided entry: the case spec/1.31.0.md §6
// says changes neither a loop's verdict nor resume's phase.
func writeFinderlessQueue(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, engine.WriteFoundQueue(filepath.Join(dir, "spec.tasks.json"), []engine.FoundQueueEntry{
		{State: engine.FoundQueueOpen, Location: "x.go:1", Question: "Which way?", Alternatives: []string{"a", "b"}},
		{State: engine.FoundQueueDecided, Location: "y.go:2", Answer: "conforms", Finders: []string{"t1"}},
	}))
}

// statusCheck runs `tp <phase> spec.md --status --check` and returns its
// payload and exit code.
func statusCheck(t *testing.T, dir, phase string) (out map[string]any, code int) {
	t.Helper()
	stdout, stderr, code := runTPFence(t, dir, false, phase, "spec.md", "--status", "--check")
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout=%s stderr=%s", stdout, stderr)
	return out, code
}

// withoutFoundOpen is a payload with the queue's own count taken out, so
// the rest compares field for field.
func withoutFoundOpen(out map[string]any) map[string]any {
	rest := make(map[string]any, len(out))
	for k, v := range out {
		if k != "found_open" {
			rest[k] = v
		}
	}
	return rest
}

// §6: with an open entry that holds no finder, `--status --check` exits and
// reports as it would without it, and the converged payload carries the open
// entry count. The mutant that holds convergence open while any entry is open
// turns converged false and the exit code to 1.
func TestFoundQueue_AFinderlessEntryLeavesTheLoopVerdictAndIsCounted(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{"review", "audit"} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()
			dir := setupConvergeOnProject(t)
			if phase == "review" {
				_, stderr, code := recordRound(t, dir, "")
				require.Equal(t, 0, code, stderr)
			} else {
				for range 2 {
					_, stderr, code := auditRecord(t, dir, auditAllPassRound)
					require.Equal(t, 0, code, stderr)
				}
			}
			before, beforeCode := statusCheck(t, dir, phase)
			require.Equal(t, true, before["converged"], "the fixture converges, or the count is not on a converged payload")
			assert.Equal(t, float64(0), before["found_open"], "an empty queue is counted as zero")

			writeFinderlessQueue(t, dir)

			after, afterCode := statusCheck(t, dir, phase)
			assert.Equal(t, beforeCode, afterCode, "--check exits as it would without the entry")
			assert.Equal(t, withoutFoundOpen(before), withoutFoundOpen(after), "the payload reads as it would without the entry")
			assert.Equal(t, float64(1), after["found_open"], "the converged payload carries the open entry count, not the decided one")
		})
	}
}

// The --record sinks carry the same count as --status on a converged round.
func TestFoundQueue_TheConvergedRecordPayloadCarriesTheOpenCount(t *testing.T) {
	t.Parallel()
	dir := setupConvergeOnProject(t)
	writeFinderlessQueue(t, dir)
	out, stderr, code := recordRound(t, dir, "")
	require.Equal(t, 0, code, stderr)
	require.Equal(t, true, out["converged"])
	assert.Equal(t, float64(1), out["found_open"])

	audited := setupConvergeOnProject(t)
	writeFinderlessQueue(t, audited)
	for range 2 {
		out, stderr, code = auditRecord(t, audited, auditAllPassRound)
		require.Equal(t, 0, code, stderr)
	}
	require.Equal(t, true, out["converged"])
	assert.Equal(t, float64(1), out["found_open"])
}

// writeMistypedQueue writes a queue beside dir's task file that the task-file
// reader accepts (it decodes only id, state and finders) and the full queue
// reader rejects: an open entry whose question is a number.
func writeMistypedQueue(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, os.WriteFile(engine.FoundQueuePath(filepath.Join(dir, "spec.tasks.json")),
		[]byte(`{"entries":[{"id":"Q1","state":"open","location":"x.go:1","question":5}]}`), 0o600))
}

// A queue file that cannot be parsed fails a converged --status payload, as it
// fails tp found list, instead of reporting an empty queue as found_open: 0.
func TestFoundQueue_AnUnparsableQueueFailsTheConvergedStatus(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{"review", "audit"} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()
			dir := setupConvergeOnProject(t)
			if phase == "review" {
				_, stderr, code := recordRound(t, dir, "")
				require.Equal(t, 0, code, stderr)
			} else {
				for range 2 {
					_, stderr, code := auditRecord(t, dir, auditAllPassRound)
					require.Equal(t, 0, code, stderr)
				}
			}
			before, _ := statusCheck(t, dir, phase)
			require.Equal(t, true, before["converged"], "the fixture converges, or the count is not on the payload")
			writeMistypedQueue(t, dir)

			stdout, stderr, code := runTPFence(t, dir, false, phase, "spec.md", "--status")
			assert.Equal(t, 3, code, "exit 3 (file error), as tp found list exits; stdout=%s stderr=%s", stdout, stderr)
			assert.Contains(t, stderr, "cannot read found queue")
			assert.NotContains(t, stdout, `"found_open"`, "no count is reported for a queue that was not read")
		})
	}
}

// On --record the round is already stored, so an unparsable queue is a
// notice and the count is left out; the record still succeeds.
func TestFoundQueue_AnUnparsableQueueIsANoticeOnTheConvergedRecord(t *testing.T) {
	t.Parallel()
	dir := setupConvergeOnProject(t)
	writeMistypedQueue(t, dir)
	out, stderr, code := recordRound(t, dir, "")
	require.Equal(t, 0, code, stderr)
	require.Equal(t, true, out["converged"])
	assert.NotContains(t, out, "found_open")
	assert.Contains(t, stderr, "cannot read found queue")

	audited := setupConvergeOnProject(t)
	writeMistypedQueue(t, audited)
	for range 2 {
		out, stderr, code = auditRecord(t, audited, auditAllPassRound)
		require.Equal(t, 0, code, stderr)
	}
	require.Equal(t, true, out["converged"])
	assert.NotContains(t, out, "found_open")
	assert.Contains(t, stderr, "cannot read found queue")
}

// resumeQueueFixture is a task file with a finder, an unrelated open task and
// a done task, each task named by some queue entry.
func resumeQueueFixture(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n"), 0o600))
	writeTaskFile(t, dir, `{"version":1,"spec":"spec.md","tasks":[
		{"id":"finder","title":"F","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["# S"]},
		{"id":"other","title":"O","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["# S"]},
		{"id":"gone","title":"G","status":"done","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["# S"],"closed_at":"2026-11-25T00:00:00Z","closed_reason":"- done in a fixture"}
	]}`)
	return dir
}

// §6: tp resume prints the open entry count and how many tasks wait on them.
// Two open entries name the finder (counted once), a third names the done
// task (which waits on nothing), and a decided one names the other task.
func TestResume_PrintsTheOpenEntriesAndTheTasksTheyHold(t *testing.T) {
	t.Parallel()
	dir := resumeQueueFixture(t)
	assert.NotContains(t, resumeResult(t, dir), "found_queue", "no open entry, no queue line")

	require.NoError(t, engine.WriteFoundQueue(filepath.Join(dir, "spec.tasks.json"), []engine.FoundQueueEntry{
		{State: engine.FoundQueueOpen, Location: "a.go:1", Question: "A?", Finders: []string{"finder"}},
		{State: engine.FoundQueueOpen, Location: "b.go:1", Question: "B?", Finders: []string{"finder"}},
		{State: engine.FoundQueueOpen, Location: "c.go:1", Question: "C?", Finders: []string{"gone"}},
		{State: engine.FoundQueueDecided, Location: "d.go:1", Answer: "conforms", Finders: []string{"other"}},
	}))

	assert.Equal(t, map[string]any{"open": float64(3), "holds": float64(1)}, resumeResult(t, dir)["found_queue"])
}

// §6: an open entry that holds no finder leaves resume's phase, units, next
// action and blockers as they would be without it.
func TestResume_AFinderlessEntryLeavesThePhase(t *testing.T) {
	t.Parallel()
	dir := resumeQueueFixture(t)
	before := resumeResult(t, dir)
	writeFinderlessQueue(t, dir)
	after := resumeResult(t, dir)

	for _, key := range []string{"phase", "next_units", "next_action", "blockers", "round"} {
		assert.Equal(t, before[key], after[key], key)
	}
	assert.Equal(t, map[string]any{"open": float64(1), "holds": float64(0)}, after["found_queue"])
}
