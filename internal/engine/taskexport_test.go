package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// exportSpec is the spec text the export fixtures end on; exportSpecOld is
// the one their first round of each loop snapshotted, whose §2 differs.
const (
	exportSpec    = "# Spec\n\n## 1. One\n\nPara one.\n\n## 2. Two\n\nPara two.\n"
	exportSpecOld = "# Spec\n\n## 1. One\n\nPara one.\n\n## 2. Two\n\nPara two, as first written.\n"
)

// exportTasksJSON is a task file with one task per state: t1 done, t2 ready on
// it, t3 waiting on t2, t4 wip.
const exportTasksJSON = `{"spec":"s.md","workflow":{"review_clean_rounds":2,"review_max_rounds":4,"audit_clean_rounds":2,"audit_max_rounds":0},"tasks":[
{"id":"t1","title":"T1","status":"done","depends_on":[],"estimate_minutes":5,"acceptance":"a1","source_sections":["## 1. One"],"closed_reason":"- did it","commit_sha":"abc1234","commit_shas":["abc1234","def5678"]},
{"id":"t2","title":"T2","description":"the second","status":"open","depends_on":["t1"],"estimate_minutes":10,"acceptance":"a2","source_sections":["## 2. Two"]},
{"id":"t3","title":"T3","status":"open","depends_on":["t2"],"estimate_minutes":5,"acceptance":"a3","source_sections":["## 2. Two"]},
{"id":"t4","title":"T4","status":"wip","depends_on":[],"estimate_minutes":5,"acceptance":"a4","source_sections":["## 1. One"]}]}`

// writeStateFile writes name into the spec's state directory.
func writeStateFile(t *testing.T, specPath, name, content string) {
	t.Helper()
	dir := ReviewStateDir(specPath)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
}

// exportLoopsFixture records two review rounds (2 findings, then clean), one
// audit round with one finding, and two ground rounds (one FAIL, then clean),
// each loop's first round snapshotting exportSpecOld and its second the spec.
func exportLoopsFixture(t *testing.T, specPath string) {
	t.Helper()
	st, err := EnsureReviewState(specPath)
	require.NoError(t, err)
	st.ReviewRounds = []ReviewRound{
		{Round: 1, Findings: 2, Clean: false, RecordedAt: "2026-11-01T10:00:00Z", SpecHash: "sha256:old"},
		{Round: 2, Findings: 0, Clean: true, RecordedAt: "2026-11-02T10:00:00Z", SpecHash: "sha256:new"},
	}
	st.AuditRounds = []ReviewRound{{Round: 1, Findings: 1, Clean: false, RecordedAt: "2026-11-03T10:00:00Z"}}
	require.NoError(t, SaveReviewState(specPath, st))
	writeStateFile(t, specPath, "snapshot-round-1.md", exportSpecOld)
	writeStateFile(t, specPath, "snapshot-round-2.md", exportSpec)
	writeStateFile(t, specPath, "snapshot-audit-round-1.md", exportSpec)
	for round, verdict := range map[int]string{1: "FAIL", 2: "PASS"} {
		text := exportSpecOld
		if round == 2 {
			text = exportSpec
		}
		writeStateFile(t, specPath, fmt.Sprintf("snapshot-ground-round-%d.md", round), text)
		writeStateFile(t, specPath, fmt.Sprintf("floor-ground-round-%d.txt", round), "u1 §0 0123456789ab #1 24B\n")
		row := groundWireRow(t, groundClaimRow(), map[string]any{"unit_id": "u1", "verdict": verdict}, nil)
		writeStateFile(t, specPath, fmt.Sprintf("ground-round-%d.ndjson", round), string(row)+"\n")
	}
}

// TestTaskExport_CarriesTasksWithTheirState is §16's task half: every task
// with its status, dependencies, estimate, acceptance, source_sections,
// times, closing evidence and commits, and its state.
func TestTaskExport_CarriesTasksWithTheirState(t *testing.T) {
	dir, specPath, tfp := setupResumeProject(t, exportTasksJSON)
	require.NoError(t, os.WriteFile(specPath, []byte(exportSpec), 0o600))

	exp, err := TaskExportOf(dir, tfp)
	require.NoError(t, err)
	assert.Equal(t, 1, exp.Version)
	require.Len(t, exp.Tasks, 4)
	states := map[string]string{}
	for i := range exp.Tasks {
		states[exp.Tasks[i].ID] = exp.Tasks[i].State
	}
	assert.Equal(t, map[string]string{"t1": TaskStateDone, "t2": TaskStateReady, "t3": TaskStateWaitsOnTask, "t4": TaskStateWIP}, states)
	assert.Equal(t, []string{"t2"}, exp.Tasks[2].WaitsOn)
	assert.Empty(t, exp.Tasks[1].WaitsOn)

	t1 := exp.Tasks[0]
	assert.Equal(t, []string{"abc1234", "def5678"}, t1.CommitSHAs)
	require.NotNil(t, t1.ClosedReason)
	assert.Equal(t, "- did it", *t1.ClosedReason)
	t2 := exp.Tasks[1]
	assert.Equal(t, []string{"t1"}, t2.DependsOn)
	assert.Equal(t, 10, t2.EstimateMinutes)
	assert.Equal(t, "a2", t2.Acceptance)
	assert.Equal(t, []string{"## 2. Two"}, t2.SourceSections)
	assert.Equal(t, "the second", t2.Description)
}

