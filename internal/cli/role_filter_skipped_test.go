package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// roleFilterReason is the wire value of the skipped_roles reason the --role
// filter records for every role it narrowed away. It is spelled here rather
// than taken from the engine constant because the string is the contract.
const roleFilterReason = "role-filter"

// narrowedEmission is the part of an emission payload these tests read.
type narrowedEmission struct {
	roles   []string
	skipped map[string]string
	present bool
}

// readNarrowedEmission runs one emission in dir and returns its prompt roles,
// its skipped_roles by role, and whether the skipped_roles key was present.
func readNarrowedEmission(t *testing.T, dir string, args ...string) narrowedEmission {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, args...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var payload struct {
		Prompts []struct {
			Role string `json:"role"`
		} `json:"prompts"`
		Skipped *[]struct {
			Role   string `json:"role"`
			Reason string `json:"reason"`
		} `json:"skipped_roles"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	out := narrowedEmission{skipped: map[string]string{}, present: payload.Skipped != nil}
	for _, p := range payload.Prompts {
		out.roles = append(out.roles, p.Role)
	}
	if payload.Skipped != nil {
		for _, s := range *payload.Skipped {
			out.skipped[s.Role] = s.Reason
		}
	}
	return out
}

// roundTwoWithFixedFinding builds a review whose round 2 emits the built-in
// regression role: round 1 is recorded with one finding, which is then
// resolved fixed, so the round has something to guard.
func roundTwoWithFixedFinding(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\ncontent\n"), 0o600))

	_, stderr, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code, "round 1 emission: %s", stderr)
	_, stderr, code = recordRound(t, dir,
		`{"evidence":"read the cited section","severity":"high","category":"consistency","location":"L1","finding":"f1","suggestion":"s"}`+"\n")
	require.Equal(t, 0, code, "round 1 record: %s", stderr)

	rows := filepath.Join(".tp-review", "spec", "review-round-1.ndjson")
	require.FileExists(t, filepath.Join(dir, rows))
	_, stderr, code = runTP(t, dir, "review", "--resolve", rows, "0", "fixed", "fixed in L1")
	require.Equal(t, 0, code, "resolve: %s", stderr)
	return dir
}

// TestReviewRoleFilterNamesEveryNarrowedRole: `tp review --role implementer`
// on a round that emits implementer, tester, architect and regression keeps
// only implementer, and names the three it dropped in skipped_roles with
// reason role-filter — the regression prompt included, which is the one a
// caller holding one role's payload has no other way to learn existed.
func TestReviewRoleFilterNamesEveryNarrowedRole(t *testing.T) {
	t.Parallel()
	dir := roundTwoWithFixedFinding(t)

	// The fixture must really emit the four-role panel, regression included,
	// or the narrowed assertion below is about a smaller set.
	full := readNarrowedEmission(t, dir, "review", "spec.md")
	require.Equal(t, []string{"implementer", "tester", "architect", "regression"}, full.roles)

	narrowed := readNarrowedEmission(t, dir, "review", "spec.md", "--role", "implementer")
	require.Equal(t, []string{"implementer"}, narrowed.roles)
	assert.Equal(t, map[string]string{
		"tester":     roleFilterReason,
		"architect":  roleFilterReason,
		"regression": roleFilterReason,
	}, narrowed.skipped, "every role --role narrowed away is named, regression included")
}

// TestReviewRoleFilterSkipsSurviveCompact: the same narrowing under --compact
// keeps the skipped_roles key and its reasons. A payload that --role cut down
// from four prompts to one is not the whole panel, and the entries saying so
// are the only place a compact caller learns it.
func TestReviewRoleFilterSkipsSurviveCompact(t *testing.T) {
	t.Parallel()
	dir := roundTwoWithFixedFinding(t)

	narrowed := readNarrowedEmission(t, dir, "review", "spec.md", "--role", "implementer", "--compact")
	require.Equal(t, []string{"implementer"}, narrowed.roles)
	require.True(t, narrowed.present, "--compact keeps a non-empty skipped_roles under --role")
	assert.Equal(t, map[string]string{
		"tester":     roleFilterReason,
		"architect":  roleFilterReason,
		"regression": roleFilterReason,
	}, narrowed.skipped)
}

// TestReviewWithoutRoleFilterSkipsUnchanged is the control: with no --role
// nothing is narrowed, so the same round reports no skip at all, and
// --compact still omits the key (§8.4).
func TestReviewWithoutRoleFilterSkipsUnchanged(t *testing.T) {
	t.Parallel()
	dir := roundTwoWithFixedFinding(t)

	full := readNarrowedEmission(t, dir, "review", "spec.md")
	require.Len(t, full.roles, 4)
	assert.True(t, full.present, "skipped_roles is present without --compact")
	assert.Empty(t, full.skipped, "nothing narrowed, nothing skipped")

	compact := readNarrowedEmission(t, dir, "review", "spec.md", "--compact")
	require.Len(t, compact.roles, 4)
	assert.False(t, compact.present, "--compact without --role still omits skipped_roles")
}

// TestAuditRoleFilterNamesEveryNarrowedRole: tp audit filters through the same
// rule, so `tp audit --role spec-coverage` names every other emitted auditor
// with reason role-filter, with and without --compact. Without --role the
// fixture skips nothing, which is what makes every entry below the filter's.
func TestAuditRoleFilterNamesEveryNarrowedRole(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(routingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plain.go"), []byte("package main\n"), 0o600))
	args := []string{"audit", "spec.md", "--affected-files", "plain.go"}

	full := readNarrowedEmission(t, dir, args...)
	require.Contains(t, full.roles, "spec-coverage")
	require.Greater(t, len(full.roles), 1, "the fixture must emit more than the role --role keeps")
	require.Empty(t, full.skipped, "the fixture skips nothing without --role")

	want := map[string]string{}
	for _, role := range full.roles {
		if role != "spec-coverage" {
			want[role] = roleFilterReason
		}
	}

	for _, extra := range [][]string{nil, {"--compact"}} {
		narrowed := readNarrowedEmission(t, dir, append(append(append([]string{}, args...), "--role", "spec-coverage"), extra...)...)
		require.Equal(t, []string{"spec-coverage"}, narrowed.roles, "%v", extra)
		require.True(t, narrowed.present, "%v: skipped_roles present", extra)
		assert.Equal(t, want, narrowed.skipped, "%v: every auditor --role narrowed away is named", extra)
	}

	compact := readNarrowedEmission(t, dir, append(append([]string{}, args...), "--compact")...)
	assert.False(t, compact.present, "--compact without --role still omits skipped_roles")
}

// assertSkippedRolesNarrowed is property 4's skipped_roles clause as the
// role-filter reason amends it: the --role payload carries the unrestricted
// payload's skipped_roles unchanged, followed by one role-filter entry for
// every other emitted role, in the payload's order.
func assertSkippedRolesNarrowed(t *testing.T, full, single any, order []string, kept string) {
	t.Helper()
	fullRows, ok := full.([]any)
	require.True(t, ok, "the unrestricted payload carries a skipped_roles array")
	want := append([]any{}, fullRows...)
	for _, role := range order {
		if role != kept {
			want = append(want, map[string]any{"role": role, "reason": roleFilterReason})
		}
	}
	assert.Equal(t, want, single, "--role keeps the round's skips and names every role it narrowed away")
}
