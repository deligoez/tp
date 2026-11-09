package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readSpecTaskFile returns spec.tasks.json's bytes in dir.
func readSpecTaskFile(t *testing.T, dir string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	return data
}

// TestEpicOperatorWrites_UnattendedExitTwo is spec/1.5.0.md §10 row 14 for the
// refusals: under TP_UNATTENDED every write of a done record, a met record or
// a binding exits 2, the removals included, names its own escalation decision
// and leaves the graph and the task file byte-identical. tp epic import keeps
// other (corrections C3). A fence missing on a removal fails it.
func TestEpicOperatorWrites_UnattendedExitTwo(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		args     []string
		decision string
	}{
		"done":       {[]string{"epic", "done", "a", "--evidence", "shipped"}, "epic-done"},
		"done undo":  {[]string{"epic", "done", "d", "--undo"}, "epic-done"},
		"gate met":   {[]string{"epic", "gate", "A", "--met", "--evidence", "agreed"}, "gate-met"},
		"set epic":   {[]string{"set", "--epic", "a"}, "epic-binding"},
		"set unbind": {[]string{"set", "--epic", ""}, "epic-binding"},
		"init epic":  {[]string{"init", "other.md", "--epic", "a"}, "epic-binding"},
		"import":     {[]string{"epic", "import", "graph.json", "--force"}, "other"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := bindingDir(t, "b")
			require.NoError(t, os.WriteFile(filepath.Join(dir, "other.md"), []byte("# Other\n"), 0o600))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "graph.json"), []byte(bindingChain), 0o600))
			graph, taskFile := readEpicGraphFile(t, dir), readSpecTaskFile(t, dir)

			_, stderr, code := runTPFence(t, dir, true, tc.args...)
			require.Equal(t, 2, code, "stderr: %s", stderr)
			assert.Contains(t, stderr, "tp escalate --decision "+tc.decision)
			assert.Equal(t, graph, readEpicGraphFile(t, dir), "a refused write leaves the graph alone")
			assert.Equal(t, taskFile, readSpecTaskFile(t, dir), "a refused write leaves the task file alone")
			assert.NoFileExists(t, filepath.Join(dir, "other.tasks.json"), "a refused init creates nothing")
		})
	}
}
