package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicDoneRecord returns the done record .tp/epics.json in dir holds for id,
// nil when it holds none.
func epicDoneRecord(t *testing.T, dir, id string) *struct {
	At       string `json:"at"`
	Evidence string `json:"evidence"`
} {
	t.Helper()
	var g struct {
		Epics []struct {
			ID   string `json:"id"`
			Done *struct {
				At       string `json:"at"`
				Evidence string `json:"evidence"`
			} `json:"done"`
		} `json:"epics"`
	}
	require.NoError(t, json.Unmarshal(readEpicGraphFile(t, dir), &g))
	for _, e := range g.Epics {
		if e.ID == id {
			return e.Done
		}
	}
	t.Fatalf("epic %q not in the graph", id)
	return nil
}

// TestEpicDone_RecordsATasklessEpicRefusesABoundOneAndUndoes is §10 row 4: on
// the chain with b bound, done c records c done; the same on b exits 1 and
// writes nothing, since b's status is derived from its file; --undo removes
// c's record. A done refused on the wrong epic turns it red.
func TestEpicDone_RecordsATasklessEpicRefusesABoundOneAndUndoes(t *testing.T) {
	t.Parallel()
	dir := boundEpicDir(t, bindingChain)

	_, stderr, code := runTP(t, dir, "epic", "done", "c", "--evidence", "https://go.example/decision")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	done := epicDoneRecord(t, dir, "c")
	require.NotNil(t, done, "done c records c")
	assert.Equal(t, "https://go.example/decision", done.Evidence)
	assert.NotEmpty(t, done.At)

	before := readEpicGraphFile(t, dir)
	_, stderr, code = runTP(t, dir, "epic", "done", "b", "--evidence", "shipped")
	assert.Equal(t, 1, code, "a bound epic's done is refused; stderr: %s", stderr)
	assert.Contains(t, stderr, "b.tasks.json")
	assert.Equal(t, before, readEpicGraphFile(t, dir), "a refused done writes nothing")

	_, stderr, code = runTP(t, dir, "epic", "done", "c", "--undo")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Nil(t, epicDoneRecord(t, dir, "c"), "--undo removes c's record")
}

// TestEpicDone_ConditionNeedsItsAcceptance is §10 row 5: a condition epic with
// an empty acceptance is refused, writing nothing, and once its acceptance is
// written the same done is accepted.
func TestEpicDone_ConditionNeedsItsAcceptance(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{"version": 1, "milestones": [["A"]], "epics": [
    {"id": "go", "title": "Go", "milestone": "A", "root": true, "condition": true}
  ]}`)
	before := readEpicGraphFile(t, dir)

	_, stderr, code := runTP(t, dir, "epic", "done", "go", "--evidence", "board said yes")
	assert.Equal(t, 1, code, "a condition with an empty acceptance is refused; stderr: %s", stderr)
	assert.Contains(t, stderr, "acceptance")
	assert.Equal(t, before, readEpicGraphFile(t, dir), "a refused done writes nothing")

	_, stderr, code = runTP(t, dir, "epic", "set", "go", "acceptance=the board approves the budget")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	_, stderr, code = runTP(t, dir, "epic", "done", "go", "--evidence", "board said yes")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.NotNil(t, epicDoneRecord(t, dir, "go"), "with its acceptance written the condition is recorded done")
}
