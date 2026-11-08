package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEpicWriters_LockContentionTimeoutExitsFour holds each tp epic writer's
// lock from this process past a 1s lock_timeout_seconds at the project layer.
// Every one must fail the way every other tp writer does (§12.2): exit 4
// (STATE) with LockTimeoutError's message and hint, not the exit 3 a generic
// write failure draws. Each row is one lock site; dropping its LockTimeoutError
// branch fails that row.
func TestEpicWriters_LockContentionTimeoutExitsFour(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, dir string) (target string, args []string)
	}{
		{"add", func(t *testing.T, dir string) (string, []string) {
			t.Helper()
			return filepath.Join(dir, ".tp", "epics.json"),
				[]string{"epic", "add", `{"id": "z", "title": "Z", "milestone": "A"}`}
		}},
		{"import", func(t *testing.T, dir string) (string, []string) {
			t.Helper()
			payload := filepath.Join(dir, "graph.json")
			require.NoError(t, os.WriteFile(payload, []byte(epicChain), 0o600))
			return filepath.Join(dir, ".tp", "epics.json"), []string{"epic", "import", payload, "--force"}
		}},
		{"review --record", func(t *testing.T, dir string) (string, []string) {
			t.Helper()
			out := runEpicReview(t, dir, "--epic", "b")
			path := epicRecordFile(t, dir, epicHighRow)
			return filepath.Join(dir, out.StateDir), []string{"epic", "review", "--epic", "b", "--record", path}
		}},
		{"review --resolve", func(t *testing.T, dir string) (string, []string) {
			t.Helper()
			return recordedEpicRound(t, dir), []string{"epic", "review", "--epic", "b", "--resolve", "0", "fixed"}
		}},
		{"review --resolve-all", func(t *testing.T, dir string) (string, []string) {
			t.Helper()
			return recordedEpicRound(t, dir), []string{"epic", "review", "--epic", "b", "--resolve-all", "fixed"}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Resolve the macOS /var -> /private/var symlink so this process and
			// the tp subprocess agree on the absolute target (and the lock hash).
			dir, err := filepath.EvalSymlinks(epicGraphDir(t, epicChain))
			require.NoError(t, err)
			writeEpicProjectConfig(t, dir, `{"lock_timeout_seconds": 1}`)
			target, args := tc.setup(t, dir)

			release := holdTaskFileLock(t, target)
			defer release()

			_, stderr, code := runTP(t, dir, args...)
			assert.Equal(t, 4, code, "a lock held past lock_timeout_seconds exits 4 (STATE): %s", stderr)
			assertLockTimeoutErrorObject(t, stderr)
		})
	}
}
