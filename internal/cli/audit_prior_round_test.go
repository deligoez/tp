package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeRecordedAuditRound writes a state.json + round NDJSON directly, giving
// full control over recorded_at and id_scheme — for legacy-round and
// changed-since tests the real --record (which stamps now + slug) cannot
// express.
func writeRecordedAuditRound(t *testing.T, dir, stateJSON, roundFile, roundContent string) {
	t.Helper()
	stateDir := filepath.Join(dir, ".tp-review", "spec")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "state.json"), []byte(stateJSON), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, roundFile), []byte(roundContent), 0o600))
}

// TestAuditPriorRound_Round1HasNone: a round-1 audit prompt (no recorded
// round) carries no prior-round section at all (§10.2).
func TestAuditPriorRound_Round1HasNone(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(routingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "auth_helper.go"), []byte("package main\n"), 0o600))
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)

	stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "auth_helper.go")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	for role, p := range auditPromptsByRole(t, stdout) {
		assert.NotContains(t, p["prompt"].(string), "## Prior Round",
			"round-1 %s prompt has no prior-round section", role)
	}
}

// TestAuditPriorRound_Round2CarriesRoleScopedNonPass: a round-2+ audit prompt
// carries a prior-round section listing ONLY that role's own prior non-PASS
// rows. PASS rows are excluded; a row with no file path omits changed_since;
// the section frames prior findings as context to re-check, not a verdict to
// repeat (§10.2). The redundant role field is retained per row for readability.
func TestAuditPriorRound_Round2CarriesRoleScopedNonPass(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(routingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "auth_helper.go"), []byte("package main\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plain.go"), []byte("package main\n"), 0o600))
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)

	// Round 1: rows across roles and statuses. Not a git repo, so the
	// changed-since flag is false for every file-bearing row.
	round1 := `{"item_id":"prior-sec-fail","status":"FAIL","role":"security","evidence_file":"auth_helper.go"}` + "\n" +
		`{"item_id":"prior-sec-pass","status":"PASS","role":"security","evidence_file":"auth_helper.go"}` + "\n" +
		`{"item_id":"prior-spec-partial","status":"PARTIAL","role":"spec-coverage","evidence_file":"spec.md"}` + "\n" +
		`{"item_id":"prior-nofile","status":"FAIL","role":"security"}` + "\n"
	auditRecord(t, dir, round1)

	stdout, stderr, code := runTP(t, dir, "audit", "spec.md",
		"--affected-files", "auth_helper.go", "--affected-files", "plain.go")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	byRole := auditPromptsByRole(t, stdout)

	sec := byRole["security"]["prompt"].(string)
	assert.Contains(t, sec, "## Prior Round", "round-2 security prompt carries the prior-round section")
	assert.Contains(t, sec, "context to re-check, not a verdict to repeat",
		"section frames prior findings as context, not a verdict")
	// FAIL row with a file path: role, item id, status, evidence_file, changed_since=false.
	assert.Contains(t, sec, `{"role":"security","item_id":"prior-sec-fail","status":"FAIL","evidence_file":"auth_helper.go","changed_since":false}`)
	// FAIL row with no file path: changed_since is omitted (no evidence_file).
	assert.Contains(t, sec, `{"role":"security","item_id":"prior-nofile","status":"FAIL"}`)
	// PASS row is excluded from the prior-round section.
	assert.NotContains(t, sec, "prior-sec-pass", "PASS rows are not carried as prior context")
	// Another role's rows do not appear in the security section.
	assert.NotContains(t, sec, "prior-spec-partial", "the section is scoped to this role only")

	spec := byRole["spec-coverage"]["prompt"].(string)
	assert.Contains(t, spec, "## Prior Round")
	assert.Contains(t, spec, `{"role":"spec-coverage","item_id":"prior-spec-partial","status":"PARTIAL","evidence_file":"spec.md","changed_since":false}`)
	assert.NotContains(t, spec, "prior-sec-fail", "spec-coverage prompt does not carry security's rows")

	// A role that was all-PASS in round 1 (maintainability) carries no section.
	if maint, ok := byRole["maintainability-conventions"]; ok {
		assert.NotContains(t, maint["prompt"].(string), "## Prior Round",
			"a role with no prior non-PASS rows carries no prior-round section")
	}
}

// TestAuditPriorRound_ChangedSinceFlag: the changed-since flag is true when a
// commit touching the row's evidence_file landed after the prior round's
// recorded_at, and false otherwise (§10.2). Test commits are dated
// 2020-01-01, so a 2019 recorded_at sees them (true) and a 2021 one does not.
func TestAuditPriorRound_ChangedSinceFlag(t *testing.T) {
	dir, specPath := newAuditRepo(t)
	commitFile(t, dir, "code.go", "add code")

	assert.Contains(t, auditPriorChangedSince(t, dir, specPath, "2019-01-01T00:00:00Z"),
		`"evidence_file":"code.go","changed_since":true`,
		"a commit after recorded_at flips changed_since to true")
	assert.Contains(t, auditPriorChangedSince(t, dir, specPath, "2021-01-01T00:00:00Z"),
		`"evidence_file":"code.go","changed_since":false`,
		"no commit after recorded_at leaves changed_since false")
}

func auditPriorChangedSince(t *testing.T, dir, specPath, recordedAt string) string {
	t.Helper()
	state := `{"spec":"spec.md","review_rounds":[],"audit_rounds":[` +
		`{"round":1,"findings":1,"clean":false,"recorded_at":"` + recordedAt + `",` +
		`"file":"audit-round-1.ndjson","spec_hash":"sha256:x","id_scheme":"slug"}]}`
	round := `{"item_id":"file-maintainability-conventions-code","status":"FAIL",` +
		`"role":"maintainability-conventions","evidence_file":"code.go"}` + "\n"
	writeRecordedAuditRound(t, dir, state, "audit-round-1.ndjson", round)
	stdout, stderr, code := runTP(t, dir, "audit", specPath, "--affected-files", "code.go")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	byRole := auditPromptsByRole(t, stdout)
	p, ok := byRole["maintainability-conventions"]
	require.True(t, ok, "maintainability-conventions prompt is emitted")
	return p["prompt"].(string)
}

// TestAuditPriorRound_LegacyRoundDisclaimer: a prior-round section built from
// a marker-less (legacy) round states its ids are positional and not
// comparable to this round's stable ids (§10.2, §10.9).
func TestAuditPriorRound_LegacyRoundDisclaimer(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(routingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "auth_helper.go"), []byte("package main\n"), 0o600))
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)

	// A marker-less (legacy) round 1: positional ids, no id_scheme field.
	state := `{"spec":"spec.md","review_rounds":[],"audit_rounds":[` +
		`{"round":1,"findings":1,"clean":false,"recorded_at":"2024-01-01T00:00:00Z",` +
		`"file":"audit-round-1.ndjson","spec_hash":"sha256:legacy"}]}`
	round := `{"item_id":"file-security-2","status":"FAIL","role":"security","evidence_file":"auth_helper.go"}` + "\n"
	writeRecordedAuditRound(t, dir, state, "audit-round-1.ndjson", round)

	stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "auth_helper.go")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	sec := auditPromptsByRole(t, stdout)["security"]["prompt"].(string)
	assert.Contains(t, sec, "## Prior Round")
	assert.Contains(t, sec, "positional", "legacy prior round states its ids are positional")
	assert.Contains(t, sec, "NOT comparable", "legacy prior round states its ids are not comparable")
}
