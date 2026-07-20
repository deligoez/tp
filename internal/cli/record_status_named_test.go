package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewRecord_CleanAndDirty(t *testing.T) {
	setup := func(t *testing.T) string {
		t.Helper()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
		return dir
	}

	t.Run("empty file records a clean round", func(t *testing.T) {
		dir := setup(t)
		out, _, code := recordRound(t, dir, "")
		require.Equal(t, 0, code)
		assert.Equal(t, true, out["clean"])
		assert.Equal(t, float64(0), out["findings"])
	})

	t.Run("whitespace-only lines skipped", func(t *testing.T) {
		dir := setup(t)
		out, _, code := recordRound(t, dir, "\n   \n\t\n")
		require.Equal(t, 0, code)
		assert.Equal(t, true, out["clean"])
		assert.Equal(t, float64(0), out["findings"])
	})

	t.Run("parse error exits 1 with line number", func(t *testing.T) {
		dir := setup(t)
		_, stderr, code := recordRound(t, dir, "\n{bad json\n")
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr, "line 2")
	})

	t.Run("all pre-resolved wontfix rows record a clean round", func(t *testing.T) {
		dir := setup(t)
		out, _, code := recordRound(t, dir,
			`{"finding":"a","resolved":{"status":"wontfix","evidence":"verifier: not real"}}`+"\n"+
				`{"finding":"b","resolved":{"status":"wontfix","evidence":"verifier: duplicate concern"}}`+"\n")
		require.Equal(t, 0, code)
		assert.Equal(t, true, out["clean"])
		assert.Equal(t, float64(2), out["findings"])
	})

	t.Run("pre-resolved fixed row exits 1", func(t *testing.T) {
		dir := setup(t)
		_, stderr, code := recordRound(t, dir, `{"finding":"a","resolved":{"status":"fixed","evidence":"e"}}`+"\n")
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr, "line 1")
	})

	t.Run("wontfix with empty evidence exits 1", func(t *testing.T) {
		dir := setup(t)
		_, stderr, code := recordRound(t, dir, `{"finding":"a","resolved":{"status":"wontfix","evidence":""}}`+"\n")
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr, "evidence")
	})

	t.Run("pre-resolved duplicate row dirties the round", func(t *testing.T) {
		dir := setup(t)
		out, _, code := recordRound(t, dir, `{"finding":"a","resolved":{"status":"duplicate","evidence":"dup"}}`+"\n")
		require.Equal(t, 0, code)
		assert.Equal(t, false, out["clean"])
	})
}

func TestReviewStatus_Check(t *testing.T) {
	setup := func(t *testing.T, checksJSON string, cleanRounds int) string {
		t.Helper()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
		_, _, code := runTP(t, dir, "init", "spec.md")
		require.Equal(t, 0, code)
		if checksJSON != "" {
			_, _, code = runTP(t, dir, "set", "--workflow", "checks="+checksJSON)
			require.Equal(t, 0, code)
		}
		for i := 0; i < cleanRounds; i++ {
			_, _, rc := recordRound(t, dir, "")
			require.Equal(t, 0, rc)
		}
		return dir
	}

	t.Run("converged fresh and passing checks exit 0", func(t *testing.T) {
		dir := setup(t, `[{"class":"ok-check","cmd":"true"}]`, 2)
		_, _, code := runTP(t, dir, "review", "spec.md", "--status", "--check")
		assert.Equal(t, 0, code)
	})

	t.Run("unconverged fails", func(t *testing.T) {
		dir := setup(t, `[{"class":"ok-check","cmd":"true"}]`, 1)
		_, _, code := runTP(t, dir, "review", "spec.md", "--status", "--check")
		assert.Equal(t, 1, code)
	})

	t.Run("stale fails", func(t *testing.T) {
		dir := setup(t, `[{"class":"ok-check","cmd":"true"}]`, 2)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec edited\n"), 0o600))
		_, _, code := runTP(t, dir, "review", "spec.md", "--status", "--check")
		assert.Equal(t, 1, code)
	})

	t.Run("failing check fails", func(t *testing.T) {
		dir := setup(t, `[{"class":"bad-check","cmd":"exit 1"}]`, 2)
		_, _, code := runTP(t, dir, "review", "spec.md", "--status", "--check")
		assert.Equal(t, 1, code)
	})

	t.Run("plain status lists checks without running them", func(t *testing.T) {
		dir := setup(t, `[{"class":"never-run","cmd":"echo ran >> check_ran.txt"}]`, 0)
		stdout, _, code := runTP(t, dir, "review", "spec.md", "--status")
		require.Equal(t, 0, code)
		var out map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &out))
		checks := out["mechanical_checks"].([]any)
		require.Len(t, checks, 1)
		assert.Equal(t, "never-run", checks[0].(map[string]any)["class"])
		_, err := os.Stat(filepath.Join(dir, "check_ran.txt"))
		assert.True(t, os.IsNotExist(err), "plain --status must not execute checks")
	})
}

func TestAuditStatus_Check(t *testing.T) {
	setup := func(t *testing.T) string {
		t.Helper()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
		return dir
	}

	t.Run("converged via audit_clean_rounds exits 0", func(t *testing.T) {
		dir := setup(t)
		for i := 0; i < 2; i++ {
			_, _, code := auditRecord(t, dir, "")
			require.Equal(t, 0, code)
		}
		stdout, _, code := runTP(t, dir, "audit", "spec.md", "--status", "--check")
		assert.Equal(t, 0, code)
		var out map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &out))
		_, hasMech := out["mechanical_checks"]
		assert.False(t, hasMech, "audit status shape has no mechanical_checks")
	})

	t.Run("dirty fails", func(t *testing.T) {
		dir := setup(t)
		_, _, code := auditRecord(t, dir, `{"id":"x","status":"FAIL"}`+"\n")
		require.Equal(t, 0, code)
		_, _, code = runTP(t, dir, "audit", "spec.md", "--status", "--check")
		assert.Equal(t, 1, code)
	})

	t.Run("stale fails", func(t *testing.T) {
		dir := setup(t)
		for i := 0; i < 2; i++ {
			_, _, code := auditRecord(t, dir, "")
			require.Equal(t, 0, code)
		}
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec edited\n"), 0o600))
		_, _, code := runTP(t, dir, "audit", "spec.md", "--status", "--check")
		assert.Equal(t, 1, code)
	})
}
