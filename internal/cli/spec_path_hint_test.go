package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSpecPathHintIsOneString: every command that can be handed a spec path
// answers a bad one with the SAME hint. Three sites used to hand-roll their own
// wording next to the shared const, which is how the rule drifted in the first
// place — the const is only worth having if nothing paraphrases it.
func TestSpecPathHintIsOneString(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	missing := filepath.Join(dir, "nope.md")

	findings := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(findings, []byte("{}\n"), 0o600))
	results := filepath.Join(dir, "results.ndjson")
	require.NoError(t, os.WriteFile(results, []byte(""), 0o600))

	cases := map[string][]string{
		"review":          {"review", missing},
		"review --verify": {"review", "--verify", missing, "--findings", findings},
		"review --record": {"review", missing, "--record", results},
		"review --status": {"review", missing, "--status"},
		"audit":           {"audit", missing},
		"audit --record":  {"audit", missing, "--record", results},
		"audit --status":  {"audit", missing, "--status"},
	}

	for name, args := range cases {
		_, stderr, code := runTP(t, dir, args...)
		require.Equal(t, 3, code, "%s: a missing spec is a file error", name)

		var payload map[string]any
		require.NoError(t, json.Unmarshal([]byte(stderr), &payload), "%s: %s", name, stderr)
		hint, _ := payload["hint"].(string)
		assert.Equal(t,
			"check the spec path — this command takes the spec markdown file, not the task file",
			hint, "%s must emit the shared spec-path hint verbatim", name)
	}
}
