package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSuiteStateWrapperCatchesEpicStateWrites runs a copy of
// scripts/check-suite-state.sh in a temp tree with a stub `go` that writes the
// epic state a stray test could touch, and requires the wrapper to fail. Each
// case writes exactly one watched path, so dropping either from WATCHED turns
// that case green-to-red on its own.
func TestSuiteStateWrapperCatchesEpicStateWrites(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "scripts", "check-suite-state.sh"))
	require.NoError(t, err)

	cases := map[string]string{
		"epic graph":        ".tp/epics.json",
		"epic review state": ".tp/epic-review/0123abcd/round-1.ndjson",
	}
	for name, rel := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(root, "scripts"), 0o750))
			script := filepath.Join(root, "scripts", "check-suite-state.sh")
			require.NoError(t, os.WriteFile(script, body, 0o755)) //nolint:gosec // a copy the test executes itself

			bin := t.TempDir()
			target := filepath.Join(root, rel)
			stub := "#!/bin/sh\nmkdir -p '" + filepath.Dir(target) + "'\nprintf 'x' > '" + target + "'\nexit 0\n"
			require.NoError(t, os.WriteFile(filepath.Join(bin, "go"), []byte(stub), 0o755)) //nolint:gosec // a stub the test executes itself

			cmd := exec.Command(script, "./...")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			out, runErr := cmd.CombinedOutput()
			require.Error(t, runErr, "a write to %s must fail the wrapper: %s", rel, out)
			assert.Contains(t, string(out), "mutated live review state")
		})
	}
}
