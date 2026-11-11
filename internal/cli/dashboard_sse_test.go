package cli_test

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dashboardTick bounds the server's derivation tick from above, so a test
// waiting two of them has seen two ticks.
const dashboardTick = 2 * time.Second

// sseEvent is one Server-Sent Event: its name and its data line.
type sseEvent struct {
	name string
	data string
}

// openEvents connects to srv's event stream and delivers each event on the
// returned channel until the test ends.
func openEvents(t *testing.T, srv *dashboardServer) <-chan sseEvent {
	t.Helper()
	resp, err := http.Get(strings.TrimSuffix(srv.url, "/") + "/api/events") //nolint:gosec,noctx // the test's own loopback child
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream")
	t.Cleanup(func() { _ = resp.Body.Close() })
	events := make(chan sseEvent, 64)
	go func() {
		defer close(events)
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 1<<20), 1<<24)
		var ev sseEvent
		for sc.Scan() {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				ev.name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				ev.data = strings.TrimPrefix(line, "data: ")
			case line == "" && ev.name != "":
				events <- ev
				ev = sseEvent{}
			}
		}
	}()
	return events
}

// collectEvents gathers every event that arrives within d.
func collectEvents(events <-chan sseEvent, d time.Duration) []sseEvent {
	var got []sseEvent
	deadline := time.After(d)
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return got
			}
			got = append(got, ev)
		case <-deadline:
			return got
		}
	}
}

// eventNames lists the names of events in order.
func eventNames(events []sseEvent) []string {
	names := make([]string, 0, len(events))
	for _, ev := range events {
		names = append(names, ev.name)
	}
	return names
}

// dashboardRepo is a git repository holding spec.md and its task file with
// one open task, t1, and, when graph is set, an epic graph none of whose
// epics the task file binds.
func dashboardRepo(t *testing.T, graph bool) string {
	t.Helper()
	dir := t.TempDir()
	if graph {
		require.NoError(t, os.Mkdir(filepath.Join(dir, ".tp"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "epics.json"), []byte(epicChainWithGate), 0o600))
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n\n## 1. X\n\nx.\n"), 0o600))
	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, dir, "add", `{"id":"t1","title":"X","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"x done","source_sections":["## 1. X"]}`)
	require.Equal(t, 0, code, stderr)
	initGitRepo(t, dir)
	return dir
}

// listDone is the done count of the export-list event's only row.
func listDone(t *testing.T, ev sseEvent) int {
	t.Helper()
	var list struct {
		Files []struct {
			Tasks map[string]int `json:"tasks"`
		} `json:"files"`
	}
	require.NoError(t, json.Unmarshal([]byte(ev.data), &list), ev.data)
	require.Len(t, list.Files, 1, ev.data)
	return list.Files[0].Tasks["done"]
}

// TestDashboard_PushesOnlyChangesAndATpDoneReachesTheStream is §18 row 3: on
// a graph with one unbound task file, and on a task file with no graph, the
// stream sends the first state, nothing over the next two ticks, and the
// task list once tp done closes the task; a client connecting afterwards gets
// the current state at once.
func TestDashboard_PushesOnlyChangesAndATpDoneReachesTheStream(t *testing.T) {
	t.Parallel()
	for name, graph := range map[string]bool{"graph": true, "no graph": false} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := dashboardRepo(t, graph)
			srv := startDashboard(t, dir, freePort(t))
			events := openEvents(t, srv)

			want := []string{"export-list", "summary"}
			if graph {
				want = []string{"epic-export", "export-list", "summary"}
			}
			first := collectEvents(events, dashboardTick)
			require.ElementsMatch(t, want, eventNames(first), "the first state")
			for _, ev := range first {
				assert.NotContains(t, ev.data, `"error"`, ev.name)
				if ev.name == "export-list" {
					assert.Equal(t, 0, listDone(t, ev))
				}
			}
			assert.Empty(t, eventNames(collectEvents(events, 2*dashboardTick+dashboardTick/2)), "an unchanged repository pushes nothing")

			_, stderr, code := runTP(t, dir, "done", "t1", "x is done")
			require.Equal(t, 0, code, stderr)
			pushed := collectEvents(events, 2*dashboardTick+dashboardTick/2)
			var list *sseEvent
			for i := range pushed {
				if pushed[i].name == "export-list" {
					list = &pushed[i]
				}
			}
			require.NotNil(t, list, "tp done reaches the stream; got %v", eventNames(pushed))
			assert.Equal(t, 1, listDone(t, *list))

			again := collectEvents(openEvents(t, srv), dashboardTick/2)
			require.ElementsMatch(t, want, eventNames(again), "a reconnecting client gets the current state")
		})
	}
}

// TestDashboard_PushesAGraphAddedWhileRunningAndADerivationsError is §2's
// live graph and failure: a graph written after the server started is pushed
// with no restart, and a malformed one is pushed as tp's error and hint.
func TestDashboard_PushesAGraphAddedWhileRunningAndADerivationsError(t *testing.T) {
	t.Parallel()
	dir := dashboardRepo(t, false)
	srv := startDashboard(t, dir, freePort(t))
	events := openEvents(t, srv)
	assert.NotContains(t, eventNames(collectEvents(events, dashboardTick)), "epic-export")

	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tp"), 0o755))
	graph := filepath.Join(dir, ".tp", "epics.json")
	require.NoError(t, os.WriteFile(graph, []byte(epicChainWithGate), 0o600))
	added := waitForEvent(t, events, "epic-export")
	assert.Contains(t, added.data, `"version"`)

	require.NoError(t, os.WriteFile(graph, []byte("{not json"), 0o600))
	failed := waitForEvent(t, events, "epic-export")
	var e struct {
		Error string `json:"error"`
		Code  int    `json:"code"`
		Hint  string `json:"hint"`
	}
	require.NoError(t, json.Unmarshal([]byte(failed.data), &e), failed.data)
	assert.NotEmpty(t, e.Error)
	assert.Equal(t, 3, e.Code)
	assert.NotEmpty(t, e.Hint)
}

// waitForEvent returns the first event named name within three ticks.
func waitForEvent(t *testing.T, events <-chan sseEvent, name string) sseEvent {
	t.Helper()
	deadline := time.After(3 * dashboardTick)
	for {
		select {
		case ev, ok := <-events:
			require.True(t, ok, "the stream closed before %s", name)
			if ev.name == name {
				return ev
			}
		case <-deadline:
			t.Fatalf("no %s event within three ticks", name)
		}
	}
}
