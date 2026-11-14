package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// taskAuditProject is a git repository with spec.md, one task per id, the
// repository's own go-safety auditor, and task_audit_roles set to it.
func taskAuditProject(t *testing.T, ids ...string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n\nSome content.\n"), 0o600))
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)
	for _, id := range ids {
		_, stderr, code := runTP(t, dir, "add", `{"id":"`+id+`","title":"Task `+id+`","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"`+id+` acceptance met","source_sections":["s1"]}`)
		require.Equal(t, 0, code, "add %s: %s", id, stderr)
	}
	auditors := filepath.Join(dir, ".tp", "auditors")
	require.NoError(t, os.MkdirAll(auditors, 0o755))
	for _, role := range []string{"go-safety", "spec-coverage"} {
		data, err := os.ReadFile(filepath.Join("..", "..", ".tp", "auditors", role+".json"))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(auditors, role+".json"), data, 0o600))
	}
	_, stderr, code := runTP(t, dir, "set", "--workflow", "--project", `task_audit_roles=["go-safety"]`)
	require.Equal(t, 0, code, "set: %s", stderr)
	initGitRepo(t, dir)
	return dir
}

// commitFiles writes each path with a line naming it and commits them,
// returning the commit's sha.
func commitFiles(t *testing.T, dir string, paths ...string) string {
	t.Helper()
	for _, p := range paths {
		full := filepath.Join(dir, p)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte("package x\n\n// "+p+"\n"), 0o600))
	}
	git := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
		return strings.TrimSpace(string(out))
	}
	git(append([]string{"add", "--"}, paths...)...)
	git("commit", "-q", "-m", "change "+strings.Join(paths, " "))
	return git("rev-parse", "HEAD")
}

// closeWith closes id with sha as its commit.
func closeWith(t *testing.T, dir, id, sha string) {
	t.Helper()
	_, stderr, code := runTP(t, dir, "done", id, "--gate-passed", "--commit", sha, "--", id+" acceptance met")
	require.Equal(t, 0, code, "done %s: %s", id, stderr)
}

// emitTaskAudit runs tp audit spec.md --task-audit and returns its payload.
func emitTaskAudit(t *testing.T, dir string) map[string]any {
	t.Helper()
	out, stderr, code := runTP(t, dir, "audit", "spec.md", "--task-audit")
	require.Equal(t, 0, code, "task-audit emit: %s", stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res), out)
	return res
}

// answerTaskAudit writes one row per checklist item of every prompt, status
// from statusOf(file), to the prompt's output path.
func answerTaskAudit(t *testing.T, dir string, res map[string]any, statusOf func(item string) string) {
	t.Helper()
	for _, p := range res["prompts"].([]any) {
		prompt := p.(map[string]any)
		var lines []string
		for _, it := range prompt["checklist_items"].([]any) {
			item := it.(map[string]any)["item_id"].(string)
			status := statusOf(item)
			row := map[string]any{"role": prompt["role"], "item_id": item, "status": status, "location": "§1", "class": "x", "category": nil, "severity": nil, "notes": "n"}
			if status != "PASS" {
				row["category"], row["severity"] = "error-handling", "error"
			}
			b, err := json.Marshal(row)
			require.NoError(t, err)
			lines = append(lines, string(b))
		}
		path := prompt["output_path"].(string)
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600))
	}
}

func pass(string) string { return "PASS" }

// TestTaskAudit_NothingIsDueMidPhaseBelowTenFiles: a done task with three
// files and an open task left is not a full batch, so nothing is emitted.
func TestTaskAudit_NothingIsDueMidPhaseBelowTenFiles(t *testing.T) {
	t.Parallel()
	dir := taskAuditProject(t, "t1", "t2")
	closeWith(t, dir, "t1", commitFiles(t, dir, "a.go", "b.go", "c.go"))
	res := emitTaskAudit(t, dir)
	assert.Nil(t, res["task_audit"], "%v", res)
	assert.Equal(t, true, res["owed"])
	assert.Empty(t, res["prompts"])
}

// TestTaskAudit_EmitsTheLastBatchAtTheEndOfThePhase: with every task done,
// the partial batch is due: one prompt per owed role over the task's files,
// with the callee line, written under the task-audit inbox.
func TestTaskAudit_EmitsTheLastBatchAtTheEndOfThePhase(t *testing.T) {
	t.Parallel()
	dir := taskAuditProject(t, "t1", "t2")
	closeWith(t, dir, "t1", commitFiles(t, dir, "a.go", "b.go"))
	closeWith(t, dir, "t2", commitFiles(t, dir, "c.go"))
	res := emitTaskAudit(t, dir)
	ta := res["task_audit"].(map[string]any)
	assert.EqualValues(t, 1, ta["seq"])
	assert.Equal(t, []any{"a.go", "b.go", "c.go"}, ta["files"])
	assert.Equal(t, []any{"t1", "t2"}, ta["tasks"])
	prompts := res["prompts"].([]any)
	require.Len(t, prompts, 1)
	prompt := prompts[0].(map[string]any)
	assert.Equal(t, "go-safety", prompt["role"])
	assert.Contains(t, prompt["output_path"], "task-audit-inbox")
	assert.Contains(t, prompt["prompt"], "one hop")
	assert.EqualValues(t, 3, prompt["checklist_count"])

	again := emitTaskAudit(t, dir)
	assert.Equal(t, ta, again["task_audit"], "re-emitting the unrecorded batch returns it frozen")
}

// TestTaskAudit_RecordInboxRecordsTheBatchAndSettlesTheDebt: the role files
// record as task audit 1, the inbox empties, and nothing is owed after.
func TestTaskAudit_RecordInboxRecordsTheBatchAndSettlesTheDebt(t *testing.T) {
	t.Parallel()
	dir := taskAuditProject(t, "t1")
	closeWith(t, dir, "t1", commitFiles(t, dir, "a.go", "b.go"))
	res := emitTaskAudit(t, dir)
	answerTaskAudit(t, dir, res, func(item string) string {
		if strings.Contains(item, "-a-go") {
			return "FAIL"
		}
		return "PASS"
	})
	out, stderr, code := runTP(t, dir, "audit", "spec.md", "--task-audit", "--record-inbox")
	require.Equal(t, 0, code, "record: %s", stderr)
	var rec map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &rec), out)
	assert.EqualValues(t, 1, rec["seq"])
	assert.EqualValues(t, 1, rec["findings"])
	assert.Equal(t, false, rec["clean"])

	state := readReviewState(t, dir)
	audits := state["task_audits"].([]any)
	require.Len(t, audits, 1)
	assert.Equal(t, "task-audit-1.ndjson", audits[0].(map[string]any)["file"])
	_, err := os.Stat(filepath.Join(dir, ".tp-review", "spec", "task-audit-1.ndjson"))
	require.NoError(t, err)

	after := emitTaskAudit(t, dir)
	assert.Nil(t, after["task_audit"])
	assert.Equal(t, false, after["owed"])
}

// TestTaskAudit_RecordRefusesAMissingRoleFile: a batch whose role has not
// written its file does not record.
func TestTaskAudit_RecordRefusesAMissingRoleFile(t *testing.T) {
	t.Parallel()
	dir := taskAuditProject(t, "t1")
	closeWith(t, dir, "t1", commitFiles(t, dir, "a.go"))
	emitTaskAudit(t, dir)
	_, stderr, code := runTP(t, dir, "audit", "spec.md", "--task-audit", "--record-inbox")
	assert.Equal(t, 1, code, stderr)
	assert.Contains(t, stderr, "go-safety")
}

// TestTaskAudit_RecordRefusesAnItemOutsideTheBatch: a row on a file the batch
// did not hold is refused, so a role cannot grade what it was not asked.
func TestTaskAudit_RecordRefusesAnItemOutsideTheBatch(t *testing.T) {
	t.Parallel()
	dir := taskAuditProject(t, "t1")
	closeWith(t, dir, "t1", commitFiles(t, dir, "a.go"))
	res := emitTaskAudit(t, dir)
	answerTaskAudit(t, dir, res, pass)
	path := res["prompts"].([]any)[0].(map[string]any)["output_path"].(string)
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	_, err = fmt.Fprintln(f, `{"role":"go-safety","item_id":"file-go-safety-elsewhere-go.0000000000000000","status":"PASS","location":"§1","class":"x","category":null,"severity":null}`)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	_, stderr, code := runTP(t, dir, "audit", "spec.md", "--task-audit", "--record-inbox")
	assert.Equal(t, 1, code, stderr)
	assert.Contains(t, stderr, "file-go-safety-elsewhere-go")
}

// TestTaskAudit_ReleaseEmissionRefusedWhileATaskAuditIsInFlight: one audit
// emission of either kind is in flight per spec.
func TestTaskAudit_ReleaseEmissionRefusedWhileATaskAuditIsInFlight(t *testing.T) {
	t.Parallel()
	dir := taskAuditProject(t, "t1")
	closeWith(t, dir, "t1", commitFiles(t, dir, "a.go"))
	emitTaskAudit(t, dir)
	_, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "a.go")
	assert.Equal(t, 1, code, stderr)
	assert.Contains(t, stderr, "task audit 1")
}

// readReviewState reads spec.md's state.json.
func readReviewState(t *testing.T, dir string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".tp-review", "spec", "state.json"))
	require.NoError(t, err)
	var st map[string]any
	require.NoError(t, json.Unmarshal(data, &st))
	return st
}
