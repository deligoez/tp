package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// roleModeFixture is a spec plus the one artefact a refusing mode needs to get
// far enough to be refused: an NDJSON file the positional modes accept.
func roleModeFixture(t *testing.T) (dir, spec, ndjson string) {
	t.Helper()
	spec = relocatedSpec(t, "spec/0.36.0.md")
	dir = filepath.Dir(spec)
	ndjson = filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(ndjson,
		[]byte(`{"severity":"low","location":"§1","class":"c","finding":"f"}`+"\n"), 0o600))
	return dir, filepath.Base(spec), filepath.Base(ndjson)
}

// TestRoleRefusedInNonEmittingModes pins v0.36.0 §4.2.2's refusal column.
//
// Each case is run with the refusing mode's own arguments supplied, so the
// command reaches its mode dispatch: a case that failed on a missing positional
// would report exit 2 for the wrong reason and pass while the rule was absent.
func TestRoleRefusedInNonEmittingModes(t *testing.T) {
	dir, spec, ndjson := roleModeFixture(t)

	cases := map[string][]string{
		"review/merge":       {"review", "--merge", ndjson, "--role", "architect"},
		"review/record":      {"review", spec, "--record", ndjson, "--role", "architect"},
		"review/status":      {"review", spec, "--status", "--role", "architect"},
		"review/report":      {"review", "--report", ndjson, "--role", "architect"},
		"review/resolve":     {"review", ndjson, "--resolve", "0", "fixed", "e", "--role", "architect"},
		"review/resolve-all": {"review", ndjson, "--resolve-all", "fixed", "e", "--role", "architect"},
		"audit/merge":        {"audit", "--merge", ndjson, "--role", "spec-coverage"},
		"audit/record":       {"audit", spec, "--record", ndjson, "--role", "spec-coverage"},
		"audit/status":       {"audit", spec, "--status", "--role", "spec-coverage"},
		"audit/resolve":      {"audit", ndjson, "--resolve", "0", "fixed", "e", "--role", "spec-coverage"},
		"audit/resolve-all":  {"audit", ndjson, "--resolve-all", "fixed", "e", "--role", "spec-coverage"},
	}

	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			_, stderr, code := runTPIn(t, dir, args...)
			assert.Equal(t, 2, code, "a mode that emits no prompt must refuse --role; stderr: %s", stderr)
			assert.Contains(t, stderr, "--role", "the refusal names the flag the caller passed")
		})
	}
}

// TestRoleAcceptedInEveryEmittingMode is the other half of the same table: a
// mode that emits prompts takes the flag. Asserting only the refusals would
// pass on an implementation that refused everywhere.
func TestRoleAcceptedInEveryEmittingMode(t *testing.T) {
	dir, spec, ndjson := roleModeFixture(t)
	baseline := filepath.Join(dir, "baseline.md")
	require.NoError(t, os.WriteFile(baseline, []byte("# tp v0.36.0 — The emitted round\n\n## 1. Overview\n\nold\n"), 0o600))

	cases := map[string][]string{
		"default": {"review", spec, "--role", "architect"},
		// No --round: the sandbox copies the spec's recorded state, so the round is
		// state-derived and a literal would conflict with it.
		"diff-from": {"review", spec, "--diff-from", "baseline.md", "--role", "architect"},
		"verify":    {"review", "--verify", spec, "--findings", ndjson, "--role", "verifier"},
	}

	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			_, stderr, code := runTPIn(t, dir, append(args, "--json")...)
			assert.Equal(t, 0, code, "a mode that emits prompts must accept --role; stderr: %s", stderr)
		})
	}
}
