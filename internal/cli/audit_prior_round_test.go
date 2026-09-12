package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeRecordedAuditRound writes a state.json + audit-round-1.ndjson directly,
// giving full control over recorded_at and id_scheme — for legacy-round and
// changed-since tests the real --record (which stamps now + slug) cannot
// express. stateJSON names the round file audit-round-1.ndjson.
func writeRecordedAuditRound(t *testing.T, dir, stateJSON, roundContent string) {
	t.Helper()
	stateDir := filepath.Join(dir, ".tp-review", "spec")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "state.json"), []byte(stateJSON), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "audit-round-1.ndjson"), []byte(roundContent), 0o600))
}

// TestAuditPriorRound_Round1HasNone: a round-1 audit prompt (no recorded
// round) carries no prior-round section at all (§10.2).
func TestAuditPriorRound_Round1HasNone(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	dir, specPath := newAuditRepo(t)
	commitFile(t, dir, "code.go", "add code")

	assert.Contains(t, auditPriorChangedSince(t, dir, specPath, "2019-01-01T00:00:00Z"),
		`"evidence_file":"code.go","changed_since":true`,
		"a commit after recorded_at flips changed_since to true")
	assert.Contains(t, auditPriorChangedSince(t, dir, specPath, "2021-01-01T00:00:00Z"),
		`"evidence_file":"code.go","changed_since":false`,
		"no commit after recorded_at leaves changed_since false")
}

// TestAuditPriorRound_ChangedSinceIsMeasuredFromTheRecordCommit: once the round
// file is committed, changed_since is measured from that record commit's parent
// rather than from recorded_at, so a commit landing in the SAME SECOND as the
// record does not mark an untouched evidence file changed. The Prior Round flag
// and the acceptance carry read one helper, so they cannot disagree.
func TestAuditPriorRound_ChangedSinceIsMeasuredFromTheRecordCommit(t *testing.T) {
	t.Parallel()
	dir, specPath := newAuditRepo(t)
	commitFile(t, dir, "code.go", "add code")
	// Every test commit is dated 2020-01-01T00:00:00Z, so recording the round at
	// that instant puts code.go's commit inside the inclusive --since window.
	writePriorChangedSinceRound(t, dir, "2020-01-01T00:00:00Z")
	commitRoundFile(t, dir, carryRoundRel)

	assert.Contains(t, auditPriorPrompt(t, dir, specPath),
		`"evidence_file":"code.go","changed_since":false`,
		"a commit sharing the record's second is not a change to code.go")
}

func auditPriorChangedSince(t *testing.T, dir, specPath, recordedAt string) string {
	t.Helper()
	writePriorChangedSinceRound(t, dir, recordedAt)
	return auditPriorPrompt(t, dir, specPath)
}

// writePriorChangedSinceRound records one FAIL on code.go as audit round 1.
func writePriorChangedSinceRound(t *testing.T, dir, recordedAt string) {
	t.Helper()
	state := `{"spec":"spec.md","review_rounds":[],"audit_rounds":[` +
		`{"round":1,"findings":1,"clean":false,"recorded_at":"` + recordedAt + `",` +
		`"file":"audit-round-1.ndjson","spec_hash":"sha256:x","id_scheme":"slug"}]}`
	round := `{"item_id":"file-maintainability-conventions-code","status":"FAIL",` +
		`"role":"maintainability-conventions","evidence_file":"code.go"}` + "\n"
	writeRecordedAuditRound(t, dir, state, round)
}

// auditPriorPrompt returns the maintainability-conventions prompt of a fresh
// `tp audit` run over code.go.
func auditPriorPrompt(t *testing.T, dir, specPath string) string {
	t.Helper()
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
	t.Parallel()
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
	writeRecordedAuditRound(t, dir, state, round)

	stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "auth_helper.go")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	sec := auditPromptsByRole(t, stdout)["security"]["prompt"].(string)
	assert.Contains(t, sec, "## Prior Round")
	assert.Contains(t, sec, "positional", "legacy prior round states its ids are positional")
	assert.Contains(t, sec, "NOT comparable", "legacy prior round states its ids are not comparable")
}

// TestAuditPriorRound_EarlierDerivationRowsAreListedApart: a file_check row
// whose id was derived before the path digest names no item of this round, so
// it is listed after one line saying so, and a role matches it by
// evidence_file. Rows of the current derivation, and spec items, stay above
// that line, where their ids are this round's ids.
func TestAuditPriorRound_EarlierDerivationRowsAreListedApart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(routingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "auth_helper.go"), []byte("package main\n"), 0o600))
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)

	state := `{"spec":"spec.md","review_rounds":[],"audit_rounds":[` +
		`{"round":1,"findings":3,"clean":false,"recorded_at":"2024-01-01T00:00:00Z",` +
		`"file":"audit-round-1.ndjson","spec_hash":"sha256:x","id_scheme":"slug"}]}`
	round := `{"item_id":"file-security-auth-helper-go-apply-the-security-2","status":"FAIL","role":"security","evidence_file":"auth_helper.go"}` + "\n" +
		`{"item_id":"file-security-auth-helper-go-apply-the-security.0123456789abcdef","status":"FAIL","role":"security","evidence_file":"auth_helper.go"}` + "\n" +
		`{"item_id":"spec-steps-1","status":"PARTIAL","role":"security"}` + "\n"
	writeRecordedAuditRound(t, dir, state, round)

	stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "auth_helper.go")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	sec := auditPromptsByRole(t, stdout)["security"]["prompt"].(string)

	apart := strings.Index(sec, "earlier file_check id derivation")
	require.Positive(t, apart, "the earlier-derivation rows are introduced by one line")
	old := strings.Index(sec, "apply-the-security-2")
	current := strings.Index(sec, ".0123456789abcdef")
	specRow := strings.Index(sec, "spec-steps-1")
	assert.Greater(t, old, apart, "the earlier-derivation row is listed under that line")
	assert.Less(t, current, apart, "a current-derivation row stays with this round's ids")
	assert.Less(t, specRow, apart, "a spec item's row stays with this round's ids")
}

// TestAuditPriorRound_MissingRoundFileIsAnnounced: when state.json names a
// prior audit round whose NDJSON file is gone, the whole prior-round section
// disappears from every role prompt, and a round-2 prompt becomes
// indistinguishable from a round-1 one — so the role re-derives findings it was
// meant to re-check. The advisory therefore travels output.Notice rather than
// output.Info, which returns early in JSON mode.
//
// Driven through the CLI on purpose. An in-package caller leaves output's
// jsonMode at its zero value, where Info and Notice behave identically, so the
// assertion would hold just as well if the call reverted to Info — the exact
// regression it exists to catch. runTP never allocates a terminal, so this IS
// JSON mode, and both halves of the Notice contract are pinned: visible in JSON
// mode, silenced by --quiet.
func TestAuditPriorRound_MissingRoundFileIsAnnounced(t *testing.T) {
	t.Parallel()
	const want = "round 1 file audit-round-1.ndjson is missing; skipping its rows"

	setup := func(t *testing.T) string {
		t.Helper()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(routingSpec), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "auth_helper.go"), []byte("package main\n"), 0o600))
		_, _, code := runTP(t, dir, "init", "spec.md")
		require.Equal(t, 0, code)

		stateDir := filepath.Join(dir, ".tp-review", "spec")
		require.NoError(t, os.MkdirAll(stateDir, 0o755))
		state := `{"spec":"spec.md","review_rounds":[],"audit_rounds":[` +
			`{"round":1,"findings":1,"clean":false,"recorded_at":"2024-01-01T00:00:00Z",` +
			`"file":"audit-round-1.ndjson","spec_hash":"sha256:x","id_scheme":"slug"}]}`
		require.NoError(t, os.WriteFile(filepath.Join(stateDir, "state.json"), []byte(state), 0o600))
		// audit-round-1.ndjson is deliberately absent.
		return dir
	}

	t.Run("visible in JSON mode", func(t *testing.T) {
		dir := setup(t)
		stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "auth_helper.go")
		require.Equal(t, 0, code, "stderr: %s", stderr)
		assert.Contains(t, stderr, want,
			"the prior-round section was dropped without a word in the JSON mode every agent run uses")

		var payload map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &payload), "stdout stays parseable JSON")
		assert.NotContains(t, stdout, "is missing; skipping", "the advisory belongs on stderr, never in the payload")
		for role, p := range auditPromptsByRole(t, stdout) {
			assert.NotContains(t, p["prompt"].(string), "## Prior Round",
				"%s prompt carries no prior-round section once its rows are gone", role)
		}
	})

	t.Run("suppressed by --quiet", func(t *testing.T) {
		dir := setup(t)
		_, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "auth_helper.go", "--quiet")
		require.Equal(t, 0, code, "stderr: %s", stderr)
		assert.NotContains(t, stderr, want, "--quiet is the opt-out for the Notice channel")
	})
}

// auditPriorSecurityPrompt records a slug-scheme audit round 1 of the given
// rows directly and returns round 2's security prompt. Not a git repo, so a
// file-bearing row's changed_since is false.
func auditPriorSecurityPrompt(t *testing.T, round string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(routingSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "auth_helper.go"), []byte("package main\n"), 0o600))
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)

	state := `{"spec":"spec.md","review_rounds":[],"audit_rounds":[` +
		`{"round":1,"findings":1,"clean":false,"recorded_at":"2024-01-01T00:00:00Z",` +
		`"file":"audit-round-1.ndjson","spec_hash":"sha256:x","id_scheme":"slug"}]}`
	writeRecordedAuditRound(t, dir, state, round)

	stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "auth_helper.go")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	sec, ok := auditPromptsByRole(t, stdout)["security"]
	require.True(t, ok, "security prompt is emitted")
	return sec["prompt"].(string)
}

// TestAuditPriorRound_AcceptedRowCarriesItsDisposition: a prior row accepted
// wontfix with evidence carries that disposition and evidence into the next
// round's block and keeps its changed_since. Unless changed_since is true the
// auditor records it as before and tp carries the acceptance at --record; a
// FAIL recorded without that framing reads as a re-opened finding.
func TestAuditPriorRound_AcceptedRowCarriesItsDisposition(t *testing.T) {
	t.Parallel()
	sec := auditPriorSecurityPrompt(t,
		`{"item_id":"prior-accepted","status":"FAIL","role":"security","evidence_file":"auth_helper.go",`+
			`"resolved":{"status":"wontfix","evidence":"vendored code, out of scope","resolved_at":"2024-01-02T00:00:00Z"}}`+"\n")

	assert.Contains(t, sec, `{"role":"security","item_id":"prior-accepted","status":"FAIL","evidence_file":"auth_helper.go",`+
		`"changed_since":false,"disposition":"wontfix","disposition_evidence":"vendored code, out of scope"}`,
		"the accepted row carries its disposition and evidence beside changed_since")
	assert.NotContains(t, sec, "resolved_at", "the block carries the disposition, not the whole resolved record")
	assert.Contains(t, sec, "Unless changed_since is true: if it still stands, record it as before; tp carries the acceptance.",
		"an unchanged accepted row is recorded as before and carried by tp")
	assert.Contains(t, sec, "When changed_since is true, re-check it: the acceptance is not carried.",
		"a changed accepted row is re-checked, because tp does not carry it")
	assert.NotContains(t, sec, "verify the repair held", "no fixed row, so no fixed framing")
}

// TestAuditPriorRound_FixedRowAsksForTheRepairToBeVerified: a prior row
// dispositioned fixed carries "fixed" and its evidence, framed as a repair to
// verify — without the evidence the repair it claims cannot be checked.
func TestAuditPriorRound_FixedRowAsksForTheRepairToBeVerified(t *testing.T) {
	t.Parallel()
	sec := auditPriorSecurityPrompt(t,
		`{"item_id":"prior-fixed","status":"PARTIAL","role":"security",`+
			`"resolved":{"status":"fixed","evidence":"abc1234 escapes the token","resolved_at":"2024-01-02T00:00:00Z"}}`+"\n")

	assert.Contains(t, sec, `{"role":"security","item_id":"prior-fixed","status":"PARTIAL",`+
		`"disposition":"fixed","disposition_evidence":"abc1234 escapes the token"}`,
		"the fixed row carries its disposition and evidence")
	assert.Contains(t, sec, "verify the repair held", "a fixed row is framed as a repair to verify")
	assert.NotContains(t, sec, "tp carries the acceptance", "no accepted row, so no accepted framing")
}

// TestAuditPriorRound_UndisposedRowRendersAsBefore: a prior row with no
// disposition renders exactly as it did before dispositions were carried, and
// so does a wontfix whose evidence is blank — it clears nothing (§2), so it is
// still open and is re-checked like any undisposed row. With no disposition in
// the block, neither disposition framing is added.
func TestAuditPriorRound_UndisposedRowRendersAsBefore(t *testing.T) {
	t.Parallel()
	sec := auditPriorSecurityPrompt(t,
		`{"item_id":"prior-open","status":"FAIL","role":"security","evidence_file":"auth_helper.go"}`+"\n"+
			`{"item_id":"prior-blank","status":"FAIL","role":"security",`+
			`"resolved":{"status":"wontfix","evidence":"  ","resolved_at":"2024-01-02T00:00:00Z"}}`+"\n")

	assert.Contains(t, sec, "## Prior Round: context to re-check, not a verdict to repeat\n"+
		"These are your own non-PASS rows from the previous round. Re-check each item against the code and record your own status. Do NOT repeat the prior verdict without verifying.\n\n"+
		`{"role":"security","item_id":"prior-open","status":"FAIL","evidence_file":"auth_helper.go","changed_since":false}`+"\n"+
		`{"role":"security","item_id":"prior-blank","status":"FAIL"}`+"\n",
		"an undisposed block is byte-identical to the one emitted before dispositions were carried")
	assert.NotContains(t, sec, "disposition_evidence", "no row carries a disposition that counts")
	assert.NotContains(t, sec, "tp carries the acceptance", "no accepted row, so no accepted framing")
	assert.NotContains(t, sec, "verify the repair held", "no fixed row, so no fixed framing")
}
