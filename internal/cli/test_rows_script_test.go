package cli

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTestRowsShellTestPasses puts scripts/check-test-rows-cite-sidecar-test.sh
// in the gate by running it.
//
// scripts/check-test-rows-cite-sidecar.py is a registered check, and since
// v1.20.1 it grades the spec under review rather than whichever spec the
// active pointer named. That made its rule about a spec with no Tests section
// reach every Decision-track note: a backlog note with no test heading is not
// applicable, while a release spec without one, or a note whose test heading
// the scan does not recognise, still fails.
func TestTestRowsShellTestPasses(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skipf("python3 is not on PATH, so the shell test cannot run: %v", err)
	}
	root := repoRoot(t)
	cmd := exec.Command(filepath.Join(root, "scripts", "check-test-rows-cite-sidecar-test.sh"))
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "scripts/check-test-rows-cite-sidecar-test.sh must exit 0:\n%s", out)
}
