package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A task-audit record unit under tp run leaves its round file and state.json
// committed on main (BUGS.md, 2026-11-25): no later close finds them as
// uncommitted changes off the keep-list. Anything else dirty stays out.
func TestDriver_ATaskAuditRecordLeavesNoRoundFileOnMain(t *testing.T) {
	root := t.TempDir()
	gitIn(t, root, "init", "-q", "-b", "main")
	gitIn(t, root, "config", "user.email", "t@example.com")
	gitIn(t, root, "config", "user.name", "t")
	spec := filepath.Join(root, "s.md")
	require.NoError(t, os.WriteFile(spec, []byte("# S\n"), 0o600))
	stateDir := ReviewStateDir(spec)
	require.NoError(t, os.MkdirAll(stateDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "state.json"), []byte("{}\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "other.txt"), []byte("a\n"), 0o600))
	gitIn(t, root, "add", ".")
	gitIn(t, root, "commit", "-q", "-m", "base")

	// What the record unit wrote: a new round file and the state entry.
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, TaskAuditFileName(1)), []byte(`{"verdict":"PASS"}`+"\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "state.json"), []byte(`{"task_audits":[{"seq":1}]}`+"\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "other.txt"), []byte("b\n"), 0o600))

	d := &driver{opts: &DriverOptions{Root: root, Spec: spec, CommitRoundFiles: func(paths []string, msg string) error {
		_, err := CommitRoundFiles(root, paths, msg)
		return err
	}}}
	a := &unitAttempt{unit: NextUnit{Kind: UnitTaskAuditRecord, ID: "1"}, target: UnitTarget{Spec: spec, ID: "1"}}
	require.True(t, a.succeeded(), "the fixture's record unit succeeded")
	require.NoError(t, d.commitRecords([]*unitAttempt{a}))

	assert.Equal(t, "M other.txt", gitIn(t, root, "status", "--porcelain", "-uall"), "only the unrelated change is left")
	assert.Contains(t, gitIn(t, root, "ls-files"), ".tp-review/s/"+TaskAuditFileName(1))
	assert.Contains(t, gitIn(t, root, "log", "-1", "--format=%s"), "task audit 1")
}
