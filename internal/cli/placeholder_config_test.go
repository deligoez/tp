package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// writePlaceholderConfig seeds a project whose .tp/config.json is body.
func writePlaceholderConfig(t *testing.T, body string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "config.json"), []byte(body), 0o600))
	return dir
}

// TestValidateProject_RefusesAPlaceholderPatternThatDoesNotCompile: tp
// validate --project exits 1 naming epic.threshold_placeholder and the
// compile error; a pattern that compiles passes.
func TestValidateProject_RefusesAPlaceholderPatternThatDoesNotCompile(t *testing.T) {
	t.Parallel()
	bad := writePlaceholderConfig(t, `{"workflow":{},"epic":{"threshold_placeholder":"(["}}`)
	_, stderr, code := runTP(t, bad, "validate", "--project")
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "epic.threshold_placeholder")
	assert.Contains(t, stderr, "missing closing")

	good := writePlaceholderConfig(t, `{"workflow":{},"epic":{"threshold_placeholder":"^draft:"}}`)
	_, stderr, code = runTP(t, good, "validate", "--project")
	assert.Equal(t, 0, code, "stderr: %s", stderr)
}

// configEpic decodes tp config's epic block.
func configEpic(t *testing.T, dir string, args ...string) map[string]json.RawMessage {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, append([]string{"config"}, args...)...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var payload struct {
		Epic map[string]json.RawMessage `json:"epic"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload), stdout)
	return payload.Epic
}

// TestConfig_ShowsTheThresholdPlaceholder: tp config prints the effective
// epic.threshold_placeholder, the default when the key is absent, and
// --resolved names the layer it came from.
func TestConfig_ShowsTheThresholdPlaceholder(t *testing.T) {
	t.Parallel()
	set := writePlaceholderConfig(t, `{"workflow":{},"epic":{"threshold_placeholder":"^draft:"}}`)
	assert.JSONEq(t, `"^draft:"`, string(configEpic(t, set)["threshold_placeholder"]))
	assert.JSONEq(t, `{"value":"^draft:","source":"project"}`, string(configEpic(t, set, "--resolved")["threshold_placeholder"]))

	unset := writePlaceholderConfig(t, `{"workflow":{}}`)
	def, err := json.Marshal(engine.DefaultThresholdPlaceholder)
	require.NoError(t, err)
	assert.JSONEq(t, string(def), string(configEpic(t, unset)["threshold_placeholder"]))
	assert.JSONEq(t, `{"value":`+string(def)+`,"source":"default"}`, string(configEpic(t, unset, "--resolved")["threshold_placeholder"]))
}

// TestConfig_ReportsTheDefaultForAPlaceholderThatDoesNotCompile: a project
// pattern that does not compile is not the effective one, so tp config
// prints the default the epic commands use, --resolved names the default
// layer, and stderr says the project's pattern does not compile.
func TestConfig_ReportsTheDefaultForAPlaceholderThatDoesNotCompile(t *testing.T) {
	t.Parallel()
	dir := writePlaceholderConfig(t, `{"workflow":{},"epic":{"threshold_placeholder":"(unclosed"}}`)
	def, err := json.Marshal(engine.DefaultThresholdPlaceholder)
	require.NoError(t, err)
	for _, args := range [][]string{nil, {"--resolved"}} {
		stdout, stderr, code := runTP(t, dir, append([]string{"config"}, args...)...)
		require.Equal(t, 0, code, "stderr: %s", stderr)
		var payload struct {
			Epic map[string]json.RawMessage `json:"epic"`
		}
		require.NoError(t, json.Unmarshal([]byte(stdout), &payload), stdout)
		want := string(def)
		if args != nil {
			want = `{"value":` + string(def) + `,"source":"default"}`
		}
		assert.JSONEq(t, want, string(payload.Epic["threshold_placeholder"]), "%v", args)
		assert.Contains(t, stderr, "epic.threshold_placeholder", "%v", args)
		assert.Contains(t, stderr, "does not compile", "%v", args)
	}
}

// TestConfigExtract_KeepsTheThresholdPlaceholder: tp config --extract --force
// rewrites .tp/config.json with the hoisted workflow and the epic block
// intact.
func TestConfigExtract_KeepsTheThresholdPlaceholder(t *testing.T) {
	t.Parallel()
	dir := writePlaceholderConfig(t, `{"workflow":{},"epic":{"threshold_placeholder":"^draft:"}}`)
	for _, name := range []string{"a", "b"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name+".tasks.json"),
			[]byte(`{"spec":"`+name+`.md","tasks":[],"workflow":{"review_max_rounds":5}}`), 0o600))
	}
	_, stderr, code := runTP(t, dir, "config", "--extract", "--force")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	data, err := os.ReadFile(filepath.Join(dir, ".tp", "config.json"))
	require.NoError(t, err)
	var cfg map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &cfg))
	assert.JSONEq(t, `{"review_max_rounds":5}`, string(cfg["workflow"]), "the extract ran")
	assert.JSONEq(t, `{"threshold_placeholder":"^draft:"}`, string(cfg["epic"]))
}
