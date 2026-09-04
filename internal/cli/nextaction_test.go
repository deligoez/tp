package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// writeBareSpec writes a spec.md with no task file; review --record/--status
// resolve the default workflow (review_clean_rounds=2) without one.
func writeBareSpec(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))
}

// classedMediumRow is a surviving non-blocking finding carrying a class: it
// dirties the record round but the live severity-aware predicate keeps the round
// clean, so it feeds the mechanize signal without blocking convergence.
func classedMediumRow(class, loc string) string {
	return `{"severity":"medium","category":"style","location":"` + loc + `","finding":"f","suggestion":"s","class":"` + class + `"}`
}

func naStr(t *testing.T, m map[string]any) string {
	t.Helper()
	na, ok := m["next_action"].(string)
	require.True(t, ok, "next_action must be present and a string; got %v", m["next_action"])
	return na
}

// --- Review branches ---

// TestNextAction_ReviewConverged: branch 1 — a clean converged round names the
// decompose-then-tp-import forward step.
func TestNextAction_ReviewConverged(t *testing.T) {
	t.Parallel()
	dir := setupConvergeOnProject(t) // one clean round converges
	out, stderr, code := recordRound(t, dir, "")
	require.Equal(t, 0, code, "record failed: %s", stderr)
	require.Equal(t, true, out["converged"])
	assert.Contains(t, naStr(t, out), "tp import spec.tasks.json")

	stdout, _, code := runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code)
	assert.Contains(t, naStr(t, parseStatusJSON(t, stdout)), "tp import spec.tasks.json")
}

// TestNextAction_ReviewConvergedWithNonBlockingOpen: precedence totality — a
// converged round with surviving non-blocking findings names the forward step,
// never a disposal directive (§8.2 branch 1 wins over branch 2).
func TestNextAction_ReviewConvergedWithNonBlockingOpen(t *testing.T) {
	t.Parallel()
	dir := setupConvergeOnProject(t) // one clean round converges
	out, stderr, code := recordRound(t, dir, classedMediumRow("style", "L1")+"\n")
	require.Equal(t, 0, code, "record failed: %s", stderr)
	require.Equal(t, true, out["converged"])
	require.Equal(t, float64(1), out["nonblocking_open"], "the surviving medium is open")
	na := naStr(t, out)
	assert.Contains(t, na, "tp import", "converged names the forward step despite open non-blocking findings")
	assert.NotContains(t, na, "revise the spec")
	assert.NotContains(t, na, "--resolve")
}

// TestNextAction_ReviewBlocking: branch 2 — a surviving blocking (high) finding
// names the revise-and-re-review directive and never --resolve/--verify.
func TestNextAction_ReviewBlocking(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeBareSpec(t, dir) // default review_clean_rounds=2
	out, stderr, code := recordRound(t, dir,
		`{"severity":"high","category":"completeness","location":"L1","finding":"missing","suggestion":"add"}`+"\n")
	require.Equal(t, 0, code, "record failed: %s", stderr)
	require.Equal(t, false, out["clean"], "a surviving high blocks")
	na := naStr(t, out)
	assert.Contains(t, na, "revise the spec")
	assert.NotContains(t, na, "--resolve")
	assert.NotContains(t, na, "--resolve-all")
	assert.NotContains(t, na, "--verify")

	stdout, _, code := runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code)
	assert.Contains(t, naStr(t, parseStatusJSON(t, stdout)), "revise the spec")
}

// TestNextAction_ReviewMechanize: branch 3 — a clean-but-not-converged round with
// a mechanizable recurring class names the compound register-a-check directive.
func TestNextAction_ReviewMechanize(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeBareSpec(t, dir) // default review_clean_rounds=2
	// Five medium findings of the same class in one round hit the >=5-in-one-round
	// mechanize threshold; medium keeps the round live-clean, so nothing blocks.
	rows := strings.Join([]string{
		classedMediumRow("naming", "L1"), classedMediumRow("naming", "L2"), classedMediumRow("naming", "L3"),
		classedMediumRow("naming", "L4"), classedMediumRow("naming", "L5"),
	}, "\n") + "\n"
	out, stderr, code := recordRound(t, dir, rows)
	require.Equal(t, 0, code, "record failed: %s", stderr)
	require.Equal(t, true, out["clean"], "medium survivors keep the round clean")
	require.Equal(t, false, out["converged"], "one clean round of two required is not converged")
	na := naStr(t, out)
	assert.Contains(t, na, "tp set --workflow checks", "branch 3 names the check-registration command")
	assert.Contains(t, na, "naming")
	assert.Contains(t, na, "tp review spec.md --record", "branch 3 is compound: then run the next round")
	assert.Contains(t, na, engine.MechanizePhaseQualifier,
		"the phase qualifier reaches the --record payload, not just the emitter")

	stdout, _, code := runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code)
	assert.Contains(t, naStr(t, parseStatusJSON(t, stdout)), "tp set --workflow checks", "branch 3 reachable on --status")
}

// TestNextAction_ReviewOverSpecificationExcluded: a recurring over-specification
// class is un-mechanizable, so branch 3 does not fire — the state falls through
// to branch 4's plain next-round command.
func TestNextAction_ReviewOverSpecificationExcluded(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeBareSpec(t, dir) // default review_clean_rounds=2
	rows := strings.Join([]string{
		classedMediumRow("over-specification", "L1"), classedMediumRow("over-specification", "L2"),
		classedMediumRow("over-specification", "L3"), classedMediumRow("over-specification", "L4"),
		classedMediumRow("over-specification", "L5"),
	}, "\n") + "\n"
	out, stderr, code := recordRound(t, dir, rows)
	require.Equal(t, 0, code, "record failed: %s", stderr)
	require.Equal(t, true, out["clean"])
	require.Equal(t, false, out["converged"])
	na := naStr(t, out)
	assert.NotContains(t, na, "tp set --workflow checks", "over-specification does not trigger branch 3")
	assert.Contains(t, na, "run the next review round", "falls through to branch 4")
}

// TestNextAction_ReviewCleanNotConverged: branch 4 — a clean round short of the
// required consecutive count names the plain next-round command.
func TestNextAction_ReviewCleanNotConverged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeBareSpec(t, dir) // default review_clean_rounds=2
	out, stderr, code := recordRound(t, dir, "")
	require.Equal(t, 0, code, "record failed: %s", stderr)
	require.Equal(t, true, out["clean"])
	require.Equal(t, false, out["converged"])
	na := naStr(t, out)
	assert.Contains(t, na, "run the next review round")
	assert.Contains(t, na, "tp review spec.md --record <file>")
	assert.NotContains(t, na, "tp set --workflow")
}

// TestNextAction_ReviewGatesNoExitCode: next_action is present but changes no exit
// code — a non-converged --status --check still exits 1, and the field is there.
func TestNextAction_ReviewGatesNoExitCode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeBareSpec(t, dir)
	_, stderr, code := recordRound(t, dir,
		`{"severity":"high","category":"completeness","location":"L1","finding":"missing","suggestion":"add"}`+"\n")
	require.Equal(t, 0, code, "record failed: %s", stderr)

	// Plain --status: exit 0, next_action present.
	stdout, _, code := runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code)
	assert.NotEmpty(t, naStr(t, parseStatusJSON(t, stdout)))

	// --status --check on a non-converged state: exit 1, driven by convergence,
	// with next_action still present (advisory, gates nothing).
	stdout, _, code = runTP(t, dir, "review", "spec.md", "--status", "--check")
	assert.Equal(t, 1, code, "--check exit is governed by convergence, not next_action")
	assert.NotEmpty(t, naStr(t, parseStatusJSON(t, stdout)), "next_action present even when --check exits 1")
}

// --- Audit branches ---

// setupAuditProject inits spec.md and sets audit_clean_rounds to n.
func setupAuditProject(t *testing.T, n string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)
	_, _, code = runTP(t, dir, "set", "--workflow", "audit_clean_rounds="+n)
	require.Equal(t, 0, code)
	return dir
}

// TestNextAction_AuditConverged: converged audit names the terminal release marker.
func TestNextAction_AuditConverged(t *testing.T) {
	t.Parallel()
	dir := setupAuditProject(t, "1")
	out, stderr, code := auditRecord(t, dir, `{"id":"x","role":"r","status":"PASS"}`+"\n")
	require.Equal(t, 0, code, "record failed: %s", stderr)
	require.Equal(t, true, out["converged"])
	na := naStr(t, out)
	assert.Contains(t, na, "proceed to release")
	assert.NotContains(t, na, "tp audit", "the terminal marker names no further tp command")

	stdout, _, code := runTP(t, dir, "audit", "spec.md", "--status")
	require.Equal(t, 0, code)
	assert.Contains(t, naStr(t, parseStatusJSON(t, stdout)), "proceed to release")
}

// TestNextAction_AuditCleanNotConverged: a clean audit round short of the required
// count names the next-round command.
func TestNextAction_AuditCleanNotConverged(t *testing.T) {
	t.Parallel()
	dir := setupAuditProject(t, "2")
	out, stderr, code := auditRecord(t, dir, `{"id":"x","role":"r","status":"PASS"}`+"\n")
	require.Equal(t, 0, code, "record failed: %s", stderr)
	require.Equal(t, true, out["clean"])
	require.Equal(t, false, out["converged"])
	na := naStr(t, out)
	assert.Contains(t, na, "run the next audit round")
	assert.Contains(t, na, "tp audit spec.md --record <file>")
}

// TestNextAction_AuditNonConverged: an audit round with open non-PASS rows names
// the fix-and-re-audit directive, and next_action gates no exit code.
func TestNextAction_AuditNonConverged(t *testing.T) {
	t.Parallel()
	dir := setupAuditProject(t, "2")
	out, stderr, code := auditRecord(t, dir, `{"id":"x","role":"r","status":"FAIL"}`+"\n")
	require.Equal(t, 0, code, "record failed: %s", stderr)
	require.Equal(t, false, out["clean"])
	na := naStr(t, out)
	assert.Contains(t, na, "address the findings")
	assert.Contains(t, na, "tp audit spec.md --record <file>")

	stdout, _, code := runTP(t, dir, "audit", "spec.md", "--status")
	require.Equal(t, 0, code)
	assert.Contains(t, naStr(t, parseStatusJSON(t, stdout)), "address the findings")

	// --status --check exits 1 on a non-converged audit; next_action still present.
	stdout, _, code = runTP(t, dir, "audit", "spec.md", "--status", "--check")
	assert.Equal(t, 1, code)
	assert.NotEmpty(t, naStr(t, parseStatusJSON(t, stdout)))
}
