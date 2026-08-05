package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ejectAdvisory is the §3.4 line, verbatim.
const ejectAdvisory = "note: these roles are starting points; rewrite their focus for your project's stack and conventions."

// §7 item 16: the advisory reaches stderr with stdout piped — the JSON-mode case
// output.Info would swallow — and the stdout payload keys stay exactly
// {ejected, domain}.
func TestInitEjectRoles_AdvisoryOnStderrWithPipedStdout(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))

	stdout, stderr, code := runTP(t, dir, "init", "--eject-roles")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, ejectAdvisory)
	assert.NotContains(t, stdout, "starting points", "the advisory never touches stdout")

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.ElementsMatch(t, []string{"ejected", "domain"}, keysOf(out),
		"nothing is added to the eject JSON payload")
}

// keysOf returns the top-level keys of a decoded JSON object.
func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// The wording names no language, so the same line is emitted for prose.
func TestInitEjectRoles_AdvisoryForProseDomain(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))

	_, stderr, code := runTP(t, dir, "init", "--eject-roles", "--domain", "prose")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, ejectAdvisory)
}
