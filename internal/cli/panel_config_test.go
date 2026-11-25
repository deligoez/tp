package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readConfigKeys reads dir's .tp/config.json as its top-level keys.
func readConfigKeys(t *testing.T, dir string) map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".tp", "config.json"))
	require.NoError(t, err)
	var cfg map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &cfg))
	return cfg
}

// TestSetProject_KeepsThePanelBlock: tp set --workflow --project rewrites
// .tp/config.json with the new field and the plugin panel's block intact.
func TestSetProject_KeepsThePanelBlock(t *testing.T) {
	t.Parallel()
	dir := writePlaceholderConfig(t, `{"workflow":{},"panel":{"language":"tr"}}`)
	_, stderr, code := runTP(t, dir, "set", "--workflow", "--project", "review_max_rounds=4")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	cfg := readConfigKeys(t, dir)
	assert.JSONEq(t, `{"review_max_rounds":4}`, string(cfg["workflow"]), "the set ran")
	assert.JSONEq(t, `{"language":"tr"}`, string(cfg["panel"]), "the panel block survives the write")
}
