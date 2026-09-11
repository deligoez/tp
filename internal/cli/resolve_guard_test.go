package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resolveFixture writes one findings file and returns its directory and name.
func resolveFixture(t *testing.T, name string, lines ...string) (dir, file string) {
	t.Helper()
	dir = t.TempDir()
	body := ""
	for _, l := range lines {
		body += l + "\n"
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600))
	return dir, name
}

func fileBytes(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	require.NoError(t, err)
	return string(b)
}

const (
	reviewHigh   = `{"severity":"high","finding":"h","location":"§1","evidence":"read"}`
	reviewMedium = `{"severity":"medium","finding":"m","location":"§1","evidence":"read"}`
	auditFail    = `{"role":"go-safety","item_id":"a","status":"FAIL","severity":"error"}`
)

// TestResolve_AnAcceptanceNeedsEvidence: a wontfix or duplicate with blank
// evidence would be recorded as a disposition and clear nothing — the silent
// success this release removes — so it is refused and the file is untouched,
// in both phases and in both the single and the bulk form.
func TestResolve_AnAcceptanceNeedsEvidence(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		line string
		args func(file string) []string
	}{
		"review resolve":     {reviewMedium, func(f string) []string { return []string{"review", f, "--resolve", "0", "wontfix"} }},
		"review resolve-all": {reviewMedium, func(f string) []string { return []string{"review", f, "--resolve-all", "duplicate", "  "} }},
		"audit resolve":      {auditFail, func(f string) []string { return []string{"audit", f, "--resolve", "0", "wontfix", ""} }},
		"audit resolve-all":  {auditFail, func(f string) []string { return []string{"audit", f, "--resolve-all", "duplicate"} }},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir, file := resolveFixture(t, "findings.ndjson", tc.line)
			before := fileBytes(t, dir, file)
			_, stderr, code := runTPFence(t, dir, false, tc.args(file)...)
			assert.Equal(t, 2, code, "stderr: %s", stderr)
			assert.Contains(t, stderr, "evidence")
			assert.Equal(t, before, fileBytes(t, dir, file), "a refused disposition writes nothing")
		})
	}

	// fixed needs no evidence: it says the text or code changed.
	dir, file := resolveFixture(t, "findings.ndjson", reviewMedium)
	_, stderr, code := runTPFence(t, dir, false, "review", file, "--resolve", "0", "fixed")
	assert.Equal(t, 0, code, "stderr: %s", stderr)
}

// TestResolve_UnattendedCannotAcceptABlockingReviewFinding: accepting a
// critical or high review finding without a spec change is the operator's
// decision; under TP_UNATTENDED it exits 2 and names tp escalate. A medium one,
// and a fixed disposition of any severity, stay a unit's to write.
func TestResolve_UnattendedCannotAcceptABlockingReviewFinding(t *testing.T) {
	t.Parallel()
	dir, file := resolveFixture(t, "findings.ndjson", reviewHigh, reviewMedium)
	before := fileBytes(t, dir, file)

	_, stderr, code := runTPFence(t, dir, true, "review", file, "--resolve", "0", "wontfix", "out of scope")
	assert.Equal(t, 2, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "tp escalate")
	_, stderr, code = runTPFence(t, dir, true, "review", file, "--resolve-all", "wontfix", "out of scope")
	assert.Equal(t, 2, code, "the bulk form reaches the high row too: %s", stderr)
	assert.Equal(t, before, fileBytes(t, dir, file))

	_, stderr, code = runTPFence(t, dir, true, "review", file, "--resolve", "1", "wontfix", "cosmetic")
	assert.Equal(t, 0, code, "a medium finding is a unit's to accept: %s", stderr)
	_, stderr, code = runTPFence(t, dir, true, "review", file, "--resolve", "0", "fixed", "spec edited")
	assert.Equal(t, 0, code, "fixed is not an acceptance: %s", stderr)
}

// TestResolve_UnattendedCannotAcceptAnAuditFinding: on the audit side every
// acceptance is the operator's, since it closes a finding with no code change;
// fixed is the audit-fix unit's own write and stays open to it.
func TestResolve_UnattendedCannotAcceptAnAuditFinding(t *testing.T) {
	t.Parallel()
	dir, file := resolveFixture(t, "results.ndjson", auditFail)
	before := fileBytes(t, dir, file)

	_, stderr, code := runTPFence(t, dir, true, "audit", file, "--resolve", "go-safety:a", "wontfix", "not reachable")
	assert.Equal(t, 2, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "tp escalate")
	assert.Equal(t, before, fileBytes(t, dir, file))

	_, stderr, code = runTPFence(t, dir, true, "audit", file, "--resolve", "go-safety:a", "fixed", "code changed")
	assert.Equal(t, 0, code, "stderr: %s", stderr)
}
