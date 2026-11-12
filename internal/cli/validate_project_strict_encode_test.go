package cli_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateProjectStrict_AFailedEncodeIsReported: under --strict with a
// deviation, a payload tp cannot write to stdout is reported on stderr, as
// on the non-strict path, rather than exiting 1 with neither payload nor
// message. Stdout is a file opened read-only, so every write to it fails.
func TestValidateProjectStrict_AFailedEncodeIsReported(t *testing.T) {
	t.Parallel()
	dir := writeProjectConfigDir(t, `{"review_max_rounds":3}`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "s.tasks.json"),
		[]byte(`{"spec":"s.md","tasks":[],"workflow":{"review_max_rounds":5}}`), 0o600))
	stdout, stderr, code := runTP(t, dir, "validate", "--project", "--strict")
	require.Equal(t, 1, code, "the fixture deviates: %s", stderr)
	require.Contains(t, stdout, "review_max_rounds", "the fixture deviates")

	readOnly := filepath.Join(t.TempDir(), "stdout")
	require.NoError(t, os.WriteFile(readOnly, nil, 0o600))
	out, err := os.Open(readOnly)
	require.NoError(t, err)
	defer func() { _ = out.Close() }()
	cmd := exec.Command(binaryPath, "validate", "--project", "--strict")
	cmd.Dir = dir
	cmd.Env = dashboardEnv(false)
	cmd.Stdout = out
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	var exitErr *exec.ExitError
	require.True(t, errors.As(cmd.Run(), &exitErr), "the run fails; stderr: %s", errBuf.String())
	assert.NotZero(t, exitErr.ExitCode())
	assert.Contains(t, errBuf.String(), "bad file descriptor", "the encode error is reported")
}
