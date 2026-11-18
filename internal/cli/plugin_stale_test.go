package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pluginRootWith writes a plugin manifest and returns the plugin root.
func pluginRootWith(t *testing.T, manifest string) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".claude-plugin"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".claude-plugin", "plugin.json"), []byte(manifest), 0o600))
	return root
}

// TestPluginStaleNotice_NamesAnOlderPluginAndTheRemedy: a session that loaded
// an older plugin works from skill text that predates the binary it calls.
func TestPluginStaleNotice_NamesAnOlderPluginAndTheRemedy(t *testing.T) {
	root := pluginRootWith(t, `{"name":"tp","version":"1.16.1"}`)
	got := pluginStaleNotice("v1.23.0", root)
	assert.Contains(t, got, "1.16.1")
	assert.Contains(t, got, "v1.23.0")
	assert.Contains(t, got, "claude plugin update tp@tp")
	assert.Contains(t, got, "new session")
}

// TestPluginStaleNotice_SilentUnlessProvablyStale: the notice is a
// decision-critical line only when it is certain; every other case says
// nothing, and nothing here can fail tp resume.
func TestPluginStaleNotice_SilentUnlessProvablyStale(t *testing.T) {
	current := pluginRootWith(t, `{"name":"tp","version":"1.23.0"}`)
	newer := pluginRootWith(t, `{"name":"tp","version":"1.24.0"}`)
	other := pluginRootWith(t, `{"name":"another","version":"0.1.0"}`)
	broken := pluginRootWith(t, `{not json`)
	old := pluginRootWith(t, `{"name":"tp","version":"1.16.1"}`)
	cases := map[string]struct{ binary, root string }{
		"same version":     {"v1.23.0", current},
		"plugin newer":     {"v1.23.0", newer},
		"another plugin":   {"v1.23.0", other},
		"unreadable":       {"v1.23.0", broken},
		"no plugin root":   {"v1.23.0", ""},
		"missing manifest": {"v1.23.0", t.TempDir()},
		"dev build":        {"dev", old},
		"pseudo-version":   {"v1.23.1-0.20261118093420-104822c4904b", old},
		"dirty build":      {"v1.23.0+dirty", old},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Empty(t, pluginStaleNotice(c.binary, c.root))
		})
	}
}
