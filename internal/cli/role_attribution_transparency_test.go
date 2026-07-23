package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeReviewerRole writes a reviewer role file under .tp/reviewers/.
func writeReviewerRole(t *testing.T, dir, name, content string) {
	t.Helper()
	revDir := filepath.Join(dir, ".tp", "reviewers")
	require.NoError(t, os.MkdirAll(revDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(revDir, name), []byte(content), 0o600))
}

// skippedRolesFrom parses the skipped_roles array from an emission result.
func skippedRolesFrom(t *testing.T, stdout string) []map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	raw, ok := out["skipped_roles"].([]any)
	if !ok {
		return nil
	}
	res := make([]map[string]any, 0, len(raw))
	for _, r := range raw {
		res = append(res, r.(map[string]any))
	}
	return res
}

// TestReviewSkippedRoles_RegressionNoBaseline: at round 1 the built-in
// regression role has no snapshot-round-0.md baseline and is named in
// skipped_roles; once round 2 emits it (spec changed), skipped_roles is empty.
func TestReviewSkippedRoles_RegressionNoBaseline(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\noriginal\n"), 0o600))

	// Round 1: regression skipped with no-baseline.
	stdout, _, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code)
	skipped := skippedRolesFrom(t, stdout)
	require.Len(t, skipped, 1)
	assert.Equal(t, "regression", skipped[0]["role"])
	assert.Equal(t, "no-baseline", skipped[0]["reason"])

	// Record a clean round 1, change the spec, and re-emit (round 2): the
	// regression role is now emitted, so skipped_roles is empty.
	_, _, code = recordRound(t, dir, "")
	require.Equal(t, 0, code)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\nchanged\n"), 0o600))
	stdout, _, code = runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, []any{}, out["skipped_roles"], "regression emitted at round 2 → no skips")
	roles := []string{}
	for _, p := range out["prompts"].([]any) {
		roles = append(roles, p.(map[string]any)["role"].(string))
	}
	assert.Contains(t, roles, "regression")
}

// TestReviewSkippedRoles_DomainMismatch: a user reviewer corpus with a
// prose-only role under a software spec is named with reason domain-mismatch.
func TestReviewSkippedRoles_DomainMismatch(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\ncontent\n"), 0o600))
	writeReviewerRole(t, dir, "sw-role.json", `{"id":"sw-role","title":"SW","instructions":"x","focus":["q"]}`)
	writeReviewerRole(t, dir, "prose-role.json", `{"id":"prose-role","title":"Prose","instructions":"x","focus":["q"],"domains":["prose"]}`)

	stdout, _, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code)
	skipped := skippedRolesFrom(t, stdout)

	byReason := map[string]string{}
	for _, s := range skipped {
		byReason[s["role"].(string)] = s["reason"].(string)
	}
	assert.Equal(t, "domain-mismatch", byReason["prose-role"])
	assert.Equal(t, "no-baseline", byReason["regression"], "regression still skipped at round 1")
	_, hasSW := byReason["sw-role"]
	assert.False(t, hasSW, "sw-role applies to software and is emitted, not skipped")
}

// TestReviewSkippedRoles_CompactOmits: --compact strips the explanatory
// skipped_roles field from review prompt emission (§8.4).
func TestReviewSkippedRoles_CompactOmits(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\ncontent\n"), 0o600))

	stdout, _, code := runTP(t, dir, "review", "spec.md", "--compact")
	require.Equal(t, 0, code)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	_, hasField := out["skipped_roles"]
	assert.False(t, hasField, "skipped_roles omitted under --compact")
}

// TestReviewMerge_AttributionExcludes: attribution_excludes surfaces the
// regression exclusion only when it causes merged_count to exceed the
// overlap-report finding count, and is omitted under --compact (overlap_report
// survives --compact).
func TestReviewMerge_AttributionExcludes(t *testing.T) {
	dir := t.TempDir()

	t.Run("present when a regression-only finding drops from overlap", func(t *testing.T) {
		f1 := writeFindingsFile(t, dir, "attr-reg.ndjson", []string{
			`{"severity":"high","role":"implementer","class":"A","location":"§1","finding":"impl unique"}`,
			`{"severity":"high","role":"regression","class":"D","location":"§4","finding":"regression only"}`,
		})
		stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", "--json", f1)
		require.Equal(t, 0, code, "merge: %s", stderr)
		var sum map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &sum))
		assert.Equal(t, float64(2), sum["merged_count"])
		assert.Equal(t, []any{"regression"}, sum["attribution_excludes"])
	})

	t.Run("absent when counts already match", func(t *testing.T) {
		f1 := writeFindingsFile(t, dir, "attr-noreg.ndjson", []string{
			`{"severity":"high","role":"implementer","class":"A","location":"§1","finding":"impl shared"}`,
			`{"severity":"high","role":"tester","class":"A","location":"§1 same","finding":"tester shared"}`,
		})
		stdout, _, code := runTPMerge(t, dir, "review", "--merge", "--json", f1)
		require.Equal(t, 0, code)
		var sum map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &sum))
		_, hasExcludes := sum["attribution_excludes"]
		assert.False(t, hasExcludes, "no regression-only cluster → field omitted")
	})

	t.Run("omitted under --compact while overlap_report stays", func(t *testing.T) {
		f1 := writeFindingsFile(t, dir, "attr-compact.ndjson", []string{
			`{"severity":"high","role":"regression","class":"D","location":"§4","finding":"regression only"}`,
		})
		stdout, _, code := runTPMerge(t, dir, "review", "--merge", "--json", "--compact", f1)
		require.Equal(t, 0, code)
		var sum map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &sum))
		_, hasExcludes := sum["attribution_excludes"]
		assert.False(t, hasExcludes, "attribution_excludes omitted under --compact")
		_, hasOverlap := sum["overlap_report"]
		assert.True(t, hasOverlap, "review overlap_report survives --compact")
	})
}

// TestReviewStatus_AttributionExcludes: --status reports attribution_excludes
// from the latest recorded round when a regression-only finding is present, and
// omits it under --compact (overlap_report survives).
func TestReviewStatus_AttributionExcludes(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\ncontent\n"), 0o600))
	merged := writeFindingsFile(t, dir, "merged.ndjson", []string{
		`{"severity":"high","role":"implementer","class":"A","location":"§1","finding":"impl unique"}`,
		`{"severity":"high","role":"regression","class":"D","location":"§4","finding":"regression only"}`,
	})
	_, _, code := runTP(t, dir, "review", "spec.md", "--record", merged)
	require.Equal(t, 0, code)

	stdout, _, code := runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, []any{"regression"}, out["attribution_excludes"])

	// --compact drops attribution_excludes but keeps overlap_report.
	stdout, _, code = runTP(t, dir, "review", "spec.md", "--status", "--compact")
	require.Equal(t, 0, code)
	var compactOut map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &compactOut))
	_, hasExcludes := compactOut["attribution_excludes"]
	assert.False(t, hasExcludes)
	_, hasOverlap := compactOut["overlap_report"]
	assert.True(t, hasOverlap)
}

// TestAuditSkippedRoles_NoChecklistItems: an auditor whose routed checklist is
// empty is named with reason no-checklist-items; --compact omits skipped_roles.
func TestAuditSkippedRoles_NoChecklistItems(t *testing.T) {
	dir := t.TempDir()
	// Minimal spec with no numbered lists or tables: spec-coverage's checklist
	// is empty, so it is skipped.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. Models\n### 1.1 Task\nCreate a Task model.\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "code.go")
	require.Equal(t, 0, code, "audit emission: %s", stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	skipped, ok := out["skipped_roles"].([]any)
	require.True(t, ok, "skipped_roles present")
	require.NotEmpty(t, skipped, "at least spec-coverage is skipped (empty checklist)")
	seen := map[string]string{}
	for _, s := range skipped {
		m := s.(map[string]any)
		seen[m["role"].(string)] = m["reason"].(string)
	}
	assert.Equal(t, "no-checklist-items", seen["spec-coverage"])
	for _, reason := range seen {
		assert.Equal(t, "no-checklist-items", reason, "no user corpus → only no-checklist-items applies")
	}

	// --compact omits skipped_roles.
	stdout, _, code = runTP(t, dir, "audit", "spec.md", "--affected-files", "code.go", "--compact")
	require.Equal(t, 0, code)
	var compactOut map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &compactOut))
	_, hasField := compactOut["skipped_roles"]
	assert.False(t, hasField, "skipped_roles omitted under --compact")
}

// TestAuditMerge_OverlapReport: the audit overlap report clusters non-PASS rows
// by (item_id, category) and credits contributing roles; PASS rows are excluded.
// Omitted under --compact.
func TestAuditMerge_OverlapReport(t *testing.T) {
	dir := t.TempDir()
	rows := []string{
		`{"item_id":"i1","status":"FAIL","role":"spec-coverage","category":"cat-a"}`,
		`{"item_id":"i1","status":"FAIL","role":"security","category":"cat-a"}`,
		`{"item_id":"i2","status":"FAIL","role":"spec-coverage","category":"cat-b"}`,
		`{"item_id":"i3","status":"PASS","role":"spec-coverage","category":"cat-c"}`,
	}
	a := writeFindingsFile(t, dir, "audit.ndjson", rows)

	stdout, stderr, code := runTPMerge(t, dir, "audit", "--merge", "--json", a)
	require.Equal(t, 0, code, "audit merge: %s", stderr)
	var sum map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &sum))
	assert.Equal(t, float64(3), sum["findings"], "three non-PASS rows")
	report, ok := sum["overlap_report"].([]any)
	require.True(t, ok, "overlap_report present")
	byRole := map[string]map[string]any{}
	for _, r := range report {
		m := r.(map[string]any)
		byRole[m["role"].(string)] = m
	}
	// cat-a cluster shared by spec-coverage + security; cat-b unique to spec-coverage.
	assert.Equal(t, float64(1), byRole["spec-coverage"]["unique"])
	assert.Equal(t, float64(1), byRole["spec-coverage"]["shared"])
	assert.Equal(t, false, byRole["spec-coverage"]["trim_candidate"])
	assert.Equal(t, float64(0), byRole["security"]["unique"])
	assert.Equal(t, float64(1), byRole["security"]["shared"])
	assert.Equal(t, true, byRole["security"]["trim_candidate"])

	// --compact omits the audit overlap_report.
	stdout, _, code = runTPMerge(t, dir, "audit", "--merge", "--json", "--compact", a)
	require.Equal(t, 0, code)
	var compactSum map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &compactSum))
	_, hasOverlap := compactSum["overlap_report"]
	assert.False(t, hasOverlap, "audit overlap_report omitted under --compact")
}

// TestAuditStatus_OverlapReport: --status reports the audit overlap report from
// the latest recorded round over non-PASS rows; omitted under --compact.
func TestAuditStatus_OverlapReport(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\ncontent\n"), 0o600))
	rows := []string{
		`{"item_id":"i1","status":"FAIL","role":"spec-coverage","category":"cat-a"}`,
		`{"item_id":"i1","status":"FAIL","role":"security","category":"cat-a"}`,
	}
	a := writeFindingsFile(t, dir, "audit.ndjson", rows)
	_, _, code := runTP(t, dir, "audit", "spec.md", "--record", a)
	require.Equal(t, 0, code)

	stdout, _, code := runTP(t, dir, "audit", "spec.md", "--status")
	require.Equal(t, 0, code)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	report, ok := out["overlap_report"].([]any)
	require.True(t, ok, "overlap_report present in audit --status")
	require.Len(t, report, 2, "spec-coverage and security both contributed")

	// --compact omits the audit overlap_report.
	stdout, _, code = runTP(t, dir, "audit", "spec.md", "--status", "--compact")
	require.Equal(t, 0, code)
	var compactOut map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &compactOut))
	_, hasOverlap := compactOut["overlap_report"]
	assert.False(t, hasOverlap, "audit overlap_report omitted under --compact")
}

// TestReviewSkippedRoles_NoSpecChange: under explicit --diff-from, a reviewer
// role whose focus is scoped (via §N.M references) entirely to unchanged sections
// is skipped with reason no-spec-change; a role scoped to a changed section, and
// a role with generic (non-§-scoped) focus, are both emitted (§9.1). Generic focus
// always emits so the feature never hollows out a normal corpus.
func TestReviewSkippedRoles_NoSpecChange(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "base.md"), []byte("# Spec\n## 1. A\nold\n## 2. B\nold\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\nold\n## 2. B\nnew\n## 3. C\nnew\n"), 0o600))
	writeReviewerRole(t, dir, "scoped-unchanged.json", `{"id":"scoped-unchanged","title":"U","instructions":"x","focus":["Review §1 for issues"]}`)
	writeReviewerRole(t, dir, "scoped-changed.json", `{"id":"scoped-changed","title":"C","instructions":"x","focus":["Review §2 for issues"]}`)
	writeReviewerRole(t, dir, "generic.json", `{"id":"generic","title":"G","instructions":"x","focus":["Check error handling"]}`)

	// Diff vs base.md: §1 unchanged, §2 modified, §3 added → changed = {2, 3}.
	stdout, _, code := runTP(t, dir, "review", "spec.md", "--no-state", "--diff-from", "base.md")
	require.Equal(t, 0, code)

	skipped := skippedRolesFrom(t, stdout)
	byReason := map[string]string{}
	for _, s := range skipped {
		byReason[s["role"].(string)] = s["reason"].(string)
	}
	assert.Equal(t, "no-spec-change", byReason["scoped-unchanged"], "§1 unchanged → skipped")
	assert.NotContains(t, byReason, "scoped-changed", "§2 changed → emitted, not skipped")
	assert.NotContains(t, byReason, "generic", "generic focus → emitted, not skipped")

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	emitted := map[string]bool{}
	for _, p := range out["prompts"].([]any) {
		emitted[p.(map[string]any)["role"].(string)] = true
	}
	assert.True(t, emitted["scoped-changed"], "role scoped to a changed section is emitted")
	assert.True(t, emitted["generic"], "generic-focus role is emitted")
	assert.False(t, emitted["scoped-unchanged"], "role scoped to unchanged sections is not emitted")
}
