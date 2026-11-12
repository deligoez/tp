package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEpicExport_AMalformedProjectConfigExitsThreeWithoutTheFallbackWarning
// is spec/1.8.0.md §2: tp epic export over a .tp/config.json that is not
// JSON exits 3 naming the config, and does not first warn that it uses the
// default placeholder pattern, which it never gets to use.
func TestEpicExport_AMalformedProjectConfigExitsThreeWithoutTheFallbackWarning(t *testing.T) {
	t.Parallel()
	dir := boundEpicDir(t, epicChainWithGate)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "config.json"), []byte("{"), 0o600))

	_, stderr, code := runTP(t, dir, "epic", "export")
	require.Equal(t, 3, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "config.json")
	assert.NotContains(t, stderr, "using the default placeholder pattern")
}
