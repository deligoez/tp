package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicGatePayload is the JSON tp epic gate prints (spec/1.5.0.md §7).
type epicGatePayload struct {
	Milestone string `json:"milestone"`
	Green     bool   `json:"green"`
	Members   []struct {
		ID     string `json:"id"`
		State  string `json:"state"`
		Status string `json:"status"`
	} `json:"members"`
	Deferrable []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"deferrable"`
	Threshold        string          `json:"threshold"`
	Met              json.RawMessage `json:"met"`
	ThresholdChanged bool            `json:"threshold_changed"`
}

// runEpicGate runs tp epic gate m in dir and decodes the payload it printed.
func runEpicGate(t *testing.T, dir, m string) (out epicGatePayload, code int) {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "epic", "gate", m)
	if code == 0 || code == 1 {
		require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s stderr: %s", stdout, stderr)
	}
	return out, code
}

// TestEpicGate_ReportsMembersGreenOrRedAndDeferrableApart: A's default gate
// holds a (done) and b (not done), never the condition, and lists the
// deferrable epic apart; one red member makes the gate red and exits 1.
func TestEpicGate_ReportsMembersGreenOrRedAndDeferrableApart(t *testing.T) {
	dir := epicGraphDir(t, `{"version": 1, "milestones": [["A", "B"]],
  "gates": {"A": {"members": [], "deferrable": ["later"]}},
  "epics": [
    {"id": "a", "title": "A", "milestone": "A", "root": true, "done": {"at": "2026-01-05T09:00:00Z", "evidence": "shipped"}},
    {"id": "b", "title": "B", "milestone": "A", "root": true},
    {"id": "go", "title": "Go", "milestone": "A", "root": true, "condition": true},
    {"id": "later", "title": "Later", "milestone": "A", "root": true}
  ]}`)

	out, code := runEpicGate(t, dir, "A")

	assert.Equal(t, 1, code, "a red gate exits 1")
	assert.Equal(t, "A", out.Milestone)
	assert.False(t, out.Green)
	require.Len(t, out.Members, 2)
	assert.Equal(t, "a", out.Members[0].ID)
	assert.Equal(t, "green", out.Members[0].State)
	assert.Equal(t, "done", out.Members[0].Status)
	assert.Equal(t, "b", out.Members[1].ID)
	assert.Equal(t, "red", out.Members[1].State)
	assert.Equal(t, "planned", out.Members[1].Status)
	require.Len(t, out.Deferrable, 1)
	assert.Equal(t, "later", out.Deferrable[0].ID)
	assert.Equal(t, "planned", out.Deferrable[0].Status)
}

