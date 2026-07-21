package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGate_MultiIDAtomicOnMidGateStateChange: when a target task's state
// changes during the gate run, a multi-ID `tp done` closes NO task and exits
// with the state error (§6.2 item 6).
func TestGate_MultiIDAtomicOnMidGateStateChange(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))

	// The gate flips task "a" from open to wip while it runs — a concurrent
	// writer during the gate window. The gate itself exits 0.
	gate := `python3 -c "import json; p='spec.tasks.json'; d=json.load(open(p)); [t.update({'status':'wip','started_at':'2026-01-01T00:00:00Z'}) for t in d['tasks'] if t['id']=='a']; json.dump(d, open(p,'w'))"`
	_, _, code := runTP(t, dir, "init", "spec.md", "--quality-gate", gate)
	require.Equal(t, 0, code)
	addTask(t, dir, `{"id":"a","title":"A","depends_on":[],"estimate_minutes":5,"acceptance":"A complete","source_sections":["s1"]}`)
	addTask(t, dir, `{"id":"b","title":"B","depends_on":[],"estimate_minutes":5,"acceptance":"B complete","source_sections":["s1"]}`)

	_, stderr, code := runTP(t, dir, "done", "a", "b", "both complete and verified")
	assert.Equal(t, 4, code, "mid-gate state change aborts multi-ID atomically")
	assert.Contains(t, stderr, "changed state during the gate run")

	// No task closed: b stays open, a is whatever the gate left it (wip), neither done.
	assert.NotEqual(t, "done", showTask(t, dir, "a")["status"])
	assert.NotEqual(t, "done", showTask(t, dir, "b")["status"])
}
