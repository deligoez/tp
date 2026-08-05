package output_test

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/output"
)

// captureStderr runs fn with os.Stderr redirected to a pipe and returns what was
// written.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	fn()
	require.NoError(t, w.Close())
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	require.NoError(t, r.Close())
	return string(data)
}

// Notice is the advisory channel for project-owned data a command just wrote:
// it survives JSON mode (which Info and Success do not) and it goes to stderr,
// so a piped stdout payload stays parseable.
func TestNotice_NotSuppressedByJSONMode(t *testing.T) {
	t.Cleanup(func() { output.Configure(false, false, false) })
	output.Configure(true, false, true)
	require.True(t, output.IsJSON(), "JSON mode must be on for this case")

	got := captureStderr(t, func() { output.Notice("advisory line") })
	assert.Contains(t, got, "advisory line")

	// The contrast that motivates the primitive: Info is swallowed here.
	info := captureStderr(t, func() { output.Info("info line") })
	assert.Empty(t, info)
}
