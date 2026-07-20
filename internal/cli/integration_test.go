package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAudit_FullShape(t *testing.T) {
	dir := t.TempDir()
	spec := "# Spec\n## Table\n| Col | Desc |\n|-----|------|\n| a | first |\n## Steps\n1. do the thing\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "auth_helper.go"), []byte("package main\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plain.go"), []byte("package main\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "audit", "spec.md",
		"--affected-files", "auth_helper.go", "--affected-files", "plain.go")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	prompts := out["prompts"].([]any)
	require.Len(t, prompts, 3, "3 role-tagged prompts when all roles populated")

	roles := make([]string, 0, 3)
	for _, p := range prompts {
		m := p.(map[string]any)
		roles = append(roles, m["role"].(string))
		// Required per-prompt fields (§3, §5, §17)
		assert.NotEmpty(t, m["prompt"], "prompt body present")
		assert.NotNil(t, m["checklist_items"], "checklist_items present")
		assert.NotNil(t, m["affected_files"], "affected_files present")
		for _, item := range m["checklist_items"].([]any) {
			im := item.(map[string]any)
			assert.NotEmpty(t, im["item_id"])
			assert.NotEmpty(t, im["type"])
			assert.NotEmpty(t, im["expected_evidence"], "expected_evidence never empty")
		}
		for _, af := range m["affected_files"].([]any) {
			am := af.(map[string]any)
			assert.NotEmpty(t, am["path"])
			assert.NotNil(t, am["tasks"])
			assert.NotEmpty(t, am["diff_summary"])
		}
	}
	assert.Equal(t, []string{"spec-coverage", "security", "maintainability-conventions"}, roles)
}

func TestAudit_FileFilterCap(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## S\ntext\n"), 0o600))

	files := make([]string, 0, 50)
	for i := 0; i < 50; i++ {
		name := fmt.Sprintf("auth_%02d.go", i) // all security-matching by path
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("package main\n"), 0o600))
		files = append(files, "--affected-files", name)
	}
	args := append([]string{"audit", "spec.md"}, files...)
	stdout, stderr, code := runTP(t, dir, args...)
	require.Equal(t, 0, code, "stderr: %s", stderr)

	byRole := auditPromptsByRole(t, stdout)
	assert.LessOrEqual(t, len(byRole["security"]["affected_files"].([]any)), 20, "affected_files capped at 20")
	assert.LessOrEqual(t, len(byRole["maintainability-conventions"]["affected_files"].([]any)), 10, "maintainability capped at 10")
}

func TestAudit_NoLegacyCategoryField(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## Table\n| C |\n|---|\n| x |\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0o600))

	stdout, _, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "a.go")
	require.Equal(t, 0, code)
	assert.NotContains(t, stdout, `"category"`, "audit JSON has no category key at any level")
}

func TestSelfLoop_ReviewToImport(t *testing.T) {
	dir := t.TempDir()
	spec := "# Feature\nOverview of the feature.\n## 1. Goal\ndo something useful\n## 2. Detail\nmore detail here\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))

	// lint
	_, _, code := runTP(t, dir, "lint", "spec.md")
	require.Equal(t, 0, code)

	// init
	_, _, code = runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)

	// review round 1 (generates prompts, snapshot)
	_, _, code = runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code)

	// record a dirty round
	dirtyFile := filepath.Join(dir, "r1.ndjson")
	require.NoError(t, os.WriteFile(dirtyFile,
		[]byte(`{"severity":"high","category":"consistency","location":"L1","finding":"issue","suggestion":"fix"}`+"\n"), 0o600))
	_, _, code = runTP(t, dir, "review", "spec.md", "--record", dirtyFile)
	require.Equal(t, 0, code)

	// bare-array import fails: unconverged
	importFile := filepath.Join(dir, "tasks.json")
	taskJSON := `[{"id":"t1","title":"T","estimate_minutes":5,"acceptance":"done","source_sections":["1. Goal"],"depends_on":[]}]`
	require.NoError(t, os.WriteFile(importFile, []byte(taskJSON), 0o600))
	_, stderr, code := runTP(t, dir, "import", importFile, "--spec", "spec.md")
	assert.Equal(t, 1, code, "unconverged import blocked")
	assert.Contains(t, stderr, "review not converged")

	// record two clean rounds → converged
	empty := filepath.Join(dir, "clean.ndjson")
	require.NoError(t, os.WriteFile(empty, []byte(""), 0o600))
	for i := 0; i < 2; i++ {
		_, _, code = runTP(t, dir, "review", "spec.md", "--record", empty)
		require.Equal(t, 0, code)
	}

	// import succeeds
	_, stderr, code = runTP(t, dir, "import", importFile, "--spec", "spec.md")
	require.Equal(t, 0, code, "converged import succeeds: %s", stderr)

	// edit spec → import fails stale
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec+"\n## 3. New\nadded section\n"), 0o600))
	_, stderr, code = runTP(t, dir, "import", importFile, "--spec", "spec.md")
	assert.Equal(t, 1, code, "stale spec blocks import")
	assert.Contains(t, stderr, "spec changed since round")
}

func TestGateLoop_EndToEnd(t *testing.T) {
	// Failing gate blocks tp done
	t.Run("failing gate blocks done", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
		_, _, code := runTP(t, dir, "init", "spec.md", "--quality-gate", "exit 1")
		require.Equal(t, 0, code)
		addTask(t, dir, `{"id":"t1","title":"T","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)

		_, _, code = runTP(t, dir, "done", "t1", "task complete and fully verified")
		assert.Equal(t, 4, code, "failing gate blocks done")
		assert.Equal(t, "open", showTask(t, dir, "t1")["status"])
	})

	// Passing gate stamps gate_passed_at
	t.Run("passing gate stamps gate_passed_at", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
		_, _, code := runTP(t, dir, "init", "spec.md", "--quality-gate", "true")
		require.Equal(t, 0, code)
		addTask(t, dir, `{"id":"t1","title":"T","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)

		_, stderr, code := runTP(t, dir, "done", "t1", "task complete and fully verified")
		require.Equal(t, 0, code, "stderr: %s", stderr)
		assert.NotNil(t, showTask(t, dir, "t1")["gate_passed_at"])
	})

	// --skip-gate records the reason
	t.Run("skip-gate records reason", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
		_, _, code := runTP(t, dir, "init", "spec.md", "--quality-gate", "exit 1")
		require.Equal(t, 0, code)
		addTask(t, dir, `{"id":"t1","title":"T","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)

		_, _, code = runTP(t, dir, "done", "t1", "task complete and fully verified", "--skip-gate", "ci offline")
		require.Equal(t, 0, code)
		task := showTask(t, dir, "t1")
		assert.Equal(t, "ci offline", task["gate_skipped_reason"])
		assert.Nil(t, task["gate_passed_at"])
	})
}
