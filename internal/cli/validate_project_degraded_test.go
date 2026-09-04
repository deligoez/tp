package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The two ways `tp validate --project` can be prevented from reading the policy
// it validates against, and the one property both owe: a scan that could not be
// performed must not be reported in the JSON as a scan that came back clean.
//
// The file already holds that rule for the two inputs it was written for — an
// unreadable subtree and a malformed task file both land in `skipped`. These
// are the same rule applied to the config itself, which had no carrier: one
// path reported `project_config: false` (indistinguishable from a project that
// simply has no config) and the other reported `project_config: true` with an
// empty deviation list (indistinguishable from a clean one). Both exited 0, so
// --strict would pass a gate over a policy it never read.
//
// stderr is deliberately not the assertion. It already carried both cases, and
// --quiet erases it; the agent-facing payload is what a driver branches on.
func TestValidateProject_UnreadableConfigIsNotReportedAsClean(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root can read a 0000 directory, so the stat cannot be made to fail")
	}
	dir := writeProjectConfigDir(t, `{"review_max_rounds":9}`)

	// A task file that genuinely deviates: the point is that this deviation
	// disappears from the report, not merely that the config is unreadable.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "z.tasks.json"),
		[]byte(`{"spec":"s.md","tasks":[],"workflow":{"review_max_rounds":3}}`), 0o600))

	tp := filepath.Join(dir, ".tp")
	require.NoError(t, os.Chmod(tp, 0o000))
	t.Cleanup(func() { _ = os.Chmod(tp, 0o755) })

	stdout, stderr, code := runTP(t, dir, "validate", "--project")
	require.Equal(t, 0, code, "an unreadable config is reported, not fatal: %s", stderr)

	var payload struct {
		Skipped []string `json:"skipped"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	require.NotEmpty(t, payload.Skipped,
		"a config that could not be stat'd must not read as a project without one")
	assert.Contains(t, payload.Skipped[0], "config.json",
		"the unreadable config is named in skipped")
}

// The config is present and readable but does not parse. Nothing downstream can
// know what the project's policy is, so every deviation silently becomes a
// non-deviation and the payload says the project is clean.
func TestValidateProject_MalformedConfigIsNotReportedAsClean(t *testing.T) {
	t.Parallel()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "config.json"),
		[]byte(`{"workflow":{"review_max_rounds":9`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "z.tasks.json"),
		[]byte(`{"spec":"s.md","tasks":[],"workflow":{"review_max_rounds":3}}`), 0o600))

	stdout, stderr, code := runTP(t, dir, "validate", "--project")
	require.Equal(t, 0, code, "a malformed config is reported, not fatal: %s", stderr)

	var payload struct {
		Skipped []string `json:"skipped"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	require.NotEmpty(t, payload.Skipped,
		"a config that could not be parsed must not read as a clean project")
	assert.Contains(t, payload.Skipped[0], "config.json",
		"the unparseable config is named in skipped")
}
