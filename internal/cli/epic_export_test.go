package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// twoEpicsPerMilestone is §9 row 13's fixture: tracks [[A, B], [L1]] with two
// epics in each milestone, listed out of track order in the file.
const twoEpicsPerMilestone = `{
  "version": 1,
  "milestones": [["A", "B"], ["L1"]],
  "epics": [
    {"id": "l1", "title": "Ledger one", "milestone": "L1"},
    {"id": "b1", "title": "Billing one", "milestone": "B", "tags": ["ui", "api"], "depends_on": [{"id": "a1", "why": "bills a | b"}, {"id": "a2", "why": "reads totals"}]},
    {"id": "a1", "title": "Accounts one", "milestone": "A", "tags": ["ui"]},
    {"id": "l2", "title": "Ledger two", "milestone": "L1", "depends_on": [{"id": "l1", "why": "extends the ledger"}]},
    {"id": "b2", "title": "Billing two", "milestone": "B", "tags": ["api"]},
    {"id": "a2", "title": "Accounts two", "milestone": "A", "depends_on": [{"id": "a1", "why": "needs accounts"}]}
  ]
}
`

// TestEpicExport_OneTablePerMilestoneInTrackOrder is §9 row 13: a mutant that
// grouped tables by tag would print a ui and an api table instead.
func TestEpicExport_OneTablePerMilestoneInTrackOrder(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, twoEpicsPerMilestone)
	path := filepath.Join(dir, ".tp", "epics.json")
	before, err := os.ReadFile(path)
	require.NoError(t, err)

	stdout, stderr, code := runTP(t, dir, "epic", "export", "--md")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	want := `## A

| id | title | tags | depends on |
|---|---|---|---|
| a1 | Accounts one | ui |  |
| a2 | Accounts two |  | a1: needs accounts |

## B

| id | title | tags | depends on |
|---|---|---|---|
| b1 | Billing one | ui, api | a1: bills a \| b; a2: reads totals |
| b2 | Billing two | api |  |

## L1

| id | title | tags | depends on |
|---|---|---|---|
| l1 | Ledger one |  |  |
| l2 | Ledger two |  | l1: extends the ledger |
`
	assert.Equal(t, want, stdout)

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, before, after, "tp epic export writes no file")
	entries, err := os.ReadDir(filepath.Join(dir, ".tp"))
	require.NoError(t, err)
	assert.Len(t, entries, 1, "tp epic export creates nothing under .tp/")
}

// TestEpicExport_WithoutMdPrintsJSON: spec/1.5.0.md §7 makes JSON the
// format a bare export prints (lifting spec/1.3.0.md §8 Non-goal 3), so it
// exits 0 with a JSON document instead of the tables.
func TestEpicExport_WithoutMdPrintsJSON(t *testing.T) {
	t.Parallel()
	stdout, stderr, code := runTP(t, epicGraphDir(t, twoEpicsPerMilestone), "epic", "export")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.True(t, json.Valid([]byte(stdout)), "stdout: %s", stdout)
}
