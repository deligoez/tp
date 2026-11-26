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
		_, stderr, code := runTP(t, dir, "add", `{"id":"`+id+`","title":"Task `+id+`","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"`+id+` acceptance met","source_sections":["# Spec"]}`)
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
		if _, err := os.Stat(full); err == nil {
			continue // an edit the caller made stands
		}
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

// TestTaskAudit_ARoundInTheFlatLayoutNamesTheFolderToMoveItTo: a task audit
// whose role files sit flat in <dir>/task-audit-inbox/, the layout before
// v1.34.0, is refused as before, and the hint names the spec folder to move
// them into rather than asking for every role to run again, as review and
// audit do.
func TestTaskAudit_ARoundInTheFlatLayoutNamesTheFolderToMoveItTo(t *testing.T) {
	t.Parallel()
	dir := taskAuditProject(t, "t1")
	closeWith(t, dir, "t1", commitFiles(t, dir, "a.go"))
	out, stderr, code := runTP(t, dir, "audit", "spec.md", "--task-audit", "--inbox", "outside")
	require.Equal(t, 0, code, "task-audit emit: %s", stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res), out)
	prompts := res["prompts"].([]any)
	require.NotEmpty(t, prompts, out)
	folder := filepath.Dir(prompts[0].(map[string]any)["output_path"].(string))
	for _, pm := range prompts {
		p := pm.(map[string]any)["output_path"].(string)
		require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(folder), filepath.Base(p)), nil, 0o600))
	}

	_, stderr, code = runTP(t, dir, "audit", "spec.md", "--task-audit", "--record-inbox", "--inbox", "outside")
	assert.Equal(t, 1, code, stderr)
	var env map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &env), stderr)
	hint := env["hint"].(string)
	assert.Contains(t, hint, "move them into "+folder, "the hint names the folder the flat files belong in")
	assert.NotContains(t, hint, "run each role's prompt", "the roles ran; their files only sit in the old layout")
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

// TestTaskAudit_StatusNamesTheOwedTasksAndTheStep: while a closed task owes a
// task audit the audit is not converged, --check exits 1, and next_action
// names the command that clears it.
func TestTaskAudit_StatusNamesTheOwedTasksAndTheStep(t *testing.T) {
	t.Parallel()
	dir := taskAuditProject(t, "t1")
	closeWith(t, dir, "t1", commitFiles(t, dir, "a.go"))
	out, stderr, code := runTP(t, dir, "audit", "spec.md", "--status")
	require.Equal(t, 0, code, stderr)
	var st map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &st), out)
	assert.Equal(t, []any{"t1"}, st["task_audits_owed"])
	assert.Equal(t, false, st["converged"])
	assert.Contains(t, st["next_action"], "--task-audit")
	_, _, code = runTP(t, dir, "audit", "spec.md", "--status", "--check")
	assert.Equal(t, 1, code)
}

// failedTaskAudit closes t1 over a.go and records task audit 1 with a.go
// graded FAIL, returning the project and the failing row's item id.
func failedTaskAudit(t *testing.T) (dir, item string) {
	t.Helper()
	dir = taskAuditProject(t, "t1")
	closeWith(t, dir, "t1", commitFiles(t, dir, "a.go"))
	res := emitTaskAudit(t, dir)
	answerTaskAudit(t, dir, res, func(id string) string { item = id; return "FAIL" })
	_, stderr, code := runTP(t, dir, "audit", "spec.md", "--task-audit", "--record-inbox")
	require.Equal(t, 0, code, "record: %s", stderr)
	return dir, item
}

// recordTaskAuditWith answers the emitted task audit with status for every
// item and records it.
func recordTaskAuditWith(t *testing.T, dir string, res map[string]any, status string) {
	t.Helper()
	answerTaskAudit(t, dir, res, func(string) string { return status })
	_, stderr, code := runTP(t, dir, "audit", "spec.md", "--task-audit", "--record-inbox")
	require.Equal(t, 0, code, "record: %s", stderr)
}

// claimFixed marks the task-audit row fixed in its round file.
func claimFixed(t *testing.T, dir string, seq int, item string) {
	t.Helper()
	file := filepath.Join(".tp-review", "spec", fmt.Sprintf("task-audit-%d.ndjson", seq))
	_, stderr, code := runTP(t, dir, "audit", file, "--resolve", "go-safety:"+item, "fixed", "changed the exit into a returned error")
	require.Equal(t, 0, code, "resolve: %s", stderr)
}

// TestTaskAuditRegrade_NothingIsAskedAgainWithoutAFixOrAChange: an open row
// on an unchanged file with no fix claim is not re-graded; it holds the audit.
func TestTaskAuditRegrade_NothingIsAskedAgainWithoutAFixOrAChange(t *testing.T) {
	t.Parallel()
	dir, _ := failedTaskAudit(t)
	commitFiles(t, dir, "unrelated.go")
	res := emitTaskAudit(t, dir)
	assert.Nil(t, res["task_audit"], "an unrelated commit re-grades nothing")
}

// TestTaskAuditRegrade_AFixClaimIsAskedOnce: a fix claim makes one re-grade
// due, marked as a fix pass; a PASS there clears the carried row.
func TestTaskAuditRegrade_AFixClaimIsAskedOnce(t *testing.T) {
	t.Parallel()
	dir, item := failedTaskAudit(t)
	claimFixed(t, dir, 1, item)
	res := emitTaskAudit(t, dir)
	ta := res["task_audit"].(map[string]any)
	assert.Equal(t, true, ta["regrade"])
	assert.Equal(t, []any{"a.go"}, ta["files"])
	recordTaskAuditWith(t, dir, res, "PASS")

	audits := readReviewState(t, dir)["task_audits"].([]any)
	require.Len(t, audits, 2)
	assert.Equal(t, true, audits[1].(map[string]any)["fix_pass"])
	assert.Nil(t, emitTaskAudit(t, dir)["task_audit"])
	out, _, _ := runTP(t, dir, "audit", "spec.md", "--status")
	assert.NotContains(t, out, "carried_blocking", "the PASS cleared the row")
}

// TestTaskAuditRegrade_ASecondFixClaimWaitsForAnAuditRound: a row a fix pass
// re-graded FAIL is not asked again before the next audit round, so a fix
// and a re-grade cannot cycle outside the round cap.
func TestTaskAuditRegrade_ASecondFixClaimWaitsForAnAuditRound(t *testing.T) {
	t.Parallel()
	dir, item := failedTaskAudit(t)
	claimFixed(t, dir, 1, item)
	recordTaskAuditWith(t, dir, emitTaskAudit(t, dir), "FAIL")
	claimFixed(t, dir, 2, item)
	assert.Nil(t, emitTaskAudit(t, dir)["task_audit"])
}

// TestTaskAuditRegrade_AChangeToTheFileIsAskedAgain: a commit that changes
// the graded file makes a re-grade due without a fix claim, and it is no fix
// pass.
func TestTaskAuditRegrade_AChangeToTheFileIsAskedAgain(t *testing.T) {
	t.Parallel()
	dir, _ := failedTaskAudit(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package x\n\n// changed\n"), 0o600))
	commitFiles(t, dir, "a.go")
	res := emitTaskAudit(t, dir)
	ta := res["task_audit"].(map[string]any)
	assert.Equal(t, true, ta["regrade"])
	recordTaskAuditWith(t, dir, res, "FAIL")
	audits := readReviewState(t, dir)["task_audits"].([]any)
	assert.Nil(t, audits[1].(map[string]any)["fix_pass"])
}

// TestTaskAudit_AnEpochCutsTheTaskAuditsBeforeIt: tp review --rewritten
// places its epoch after the latest task audit too.
func TestTaskAudit_AnEpochCutsTheTaskAuditsBeforeIt(t *testing.T) {
	t.Parallel()
	dir := taskAuditProject(t, "t1")
	closeWith(t, dir, "t1", commitFiles(t, dir, "a.go"))
	recordTaskAuditWith(t, dir, emitTaskAudit(t, dir), "PASS")
	// An epoch needs a round that read the spec's text; a task audit read code.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "r1.ndjson"), []byte(`{"role":"spec-coverage","item_id":"x","status":"PASS","location":"§1","class":"x","category":null,"severity":null}`+"\n"), 0o600))
	_, stderr, code := runTP(t, dir, "audit", "spec.md", "--record", "r1.ndjson")
	require.Equal(t, 0, code, "record: %s", stderr)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n\nRewritten content.\n"), 0o600))
	_, stderr, code = runTP(t, dir, "review", "spec.md", "--rewritten", "the spec was rewritten")
	require.Equal(t, 0, code, "rewritten: %s", stderr)
	epochs := readReviewState(t, dir)["epochs"].([]any)
	assert.EqualValues(t, 1, epochs[len(epochs)-1].(map[string]any)["after_task_audit"])
}

// firstPrompt is the first prompt of an emission payload.
func firstPrompt(t *testing.T, res map[string]any) string {
	t.Helper()
	prompts, _ := res["prompts"].([]any)
	require.NotEmpty(t, prompts, "%v", res)
	return prompts[0].(map[string]any)["prompt"].(string)
}

// TestTaskAuditPrior_AReadingOfAFileIsShownItsOpenRow: a file changed after
// its task audit graded it FAIL is asked again, and that prompt lists the
// open row with its reason, so a PASS that clears it was shown it.
func TestTaskAuditPrior_AReadingOfAFileIsShownItsOpenRow(t *testing.T) {
	t.Parallel()
	dir, item := failedTaskAudit(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package x\n\n// changed for another reason\n"), 0o600))
	commitFiles(t, dir, "a.go")
	prompt := firstPrompt(t, emitTaskAudit(t, dir))
	_, section, found := strings.Cut(prompt, "## Prior Round")
	require.True(t, found, "the prompt carries a Prior Round section")
	assert.Contains(t, section, item)
	assert.Contains(t, prompt, `"notes":"n"`, "the row's reason travels with it")
}

// TestTaskAuditPrior_AReleaseRoundIsShownAnOpenTaskAuditRow: the release
// round's Prior Round lists an open task-audit row on a file in its pool,
// before any release round was recorded.
func TestTaskAuditPrior_AReleaseRoundIsShownAnOpenTaskAuditRow(t *testing.T) {
	t.Parallel()
	dir, item := failedTaskAudit(t)
	out, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "a.go", "--role", "go-safety")
	require.Equal(t, 0, code, stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res), out)
	prompt := firstPrompt(t, res)
	_, section, found := strings.Cut(prompt, "## Prior Round")
	require.True(t, found, "the prompt carries a Prior Round section")
	assert.Contains(t, section, item)
}

// dedupeProject has t1 over nine files with a.go among them and t2 over b.go,
// ten files that fill task audit 1, and an open t3.
func dedupeProject(t *testing.T) string {
	t.Helper()
	dir := taskAuditProject(t, "t1", "t2", "t3")
	nine := []string{"a.go"}
	for i := range 8 {
		nine = append(nine, fmt.Sprintf("n%d.go", i))
	}
	closeWith(t, dir, "t1", commitFiles(t, dir, nine...))
	closeWith(t, dir, "t2", commitFiles(t, dir, "b.go"))
	return dir
}

// changeAndClose writes a new body to a.go, commits it with c.go and closes
// t3 over that commit.
func changeAndClose(t *testing.T, dir, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package x\n\n// "+body+"\n"), 0o600))
	closeWith(t, dir, "t3", commitFiles(t, dir, "a.go", "c.go"))
}

// TestTaskAuditDedupe_AnUnchangedGradedFileIsNotReadAgain: t3 changed a.go
// before task audit 1 read it, so a.go is unchanged since that grading and
// drops from task audit 2, which counts its ten after the drop.
func TestTaskAuditDedupe_AnUnchangedGradedFileIsNotReadAgain(t *testing.T) {
	t.Parallel()
	dir := dedupeProject(t)
	changeAndClose(t, dir, "t3 changed it")
	first := emitTaskAudit(t, dir)
	require.Equal(t, []any{"t1", "t2"}, first["task_audit"].(map[string]any)["tasks"])
	recordTaskAuditWith(t, dir, first, "PASS")
	second := emitTaskAudit(t, dir)["task_audit"].(map[string]any)
	assert.Equal(t, []any{"t3"}, second["tasks"], "t3 is still named, so presence is unaffected")
	assert.Equal(t, []any{"c.go"}, second["files"])
}

// TestTaskAuditDedupe_AChangedGradedFileIsReadAgain: t3 changed a.go after
// task audit 1 read it, so task audit 2 reads it again.
func TestTaskAuditDedupe_AChangedGradedFileIsReadAgain(t *testing.T) {
	t.Parallel()
	dir := dedupeProject(t)
	recordTaskAuditWith(t, dir, emitTaskAudit(t, dir), "PASS")
	changeAndClose(t, dir, "t3 changed it after")
	second := emitTaskAudit(t, dir)["task_audit"].(map[string]any)
	assert.Equal(t, []any{"a.go", "c.go"}, second["files"])
}

// TestTaskAudit_RecordRefusesAnUngradedItem: a role that skipped a file does
// not settle it.
func TestTaskAudit_RecordRefusesAnUngradedItem(t *testing.T) {
	t.Parallel()
	dir := taskAuditProject(t, "t1")
	closeWith(t, dir, "t1", commitFiles(t, dir, "a.go", "b.go"))
	res := emitTaskAudit(t, dir)
	answerTaskAudit(t, dir, res, pass)
	path := res["prompts"].([]any)[0].(map[string]any)["output_path"].(string)
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	first, _, _ := strings.Cut(string(data), "\n")
	require.NoError(t, os.WriteFile(path, []byte(first+"\n"), 0o600))
	_, stderr, code := runTP(t, dir, "audit", "spec.md", "--task-audit", "--record-inbox")
	assert.Equal(t, 1, code, stderr)
	assert.Contains(t, stderr, "ungraded")
}

// TestTaskAudit_StatusCountsClosesWithNothingToRead: a stamped close with no
// auditable file owes nothing and is counted, not silent.
func TestTaskAudit_StatusCountsClosesWithNothingToRead(t *testing.T) {
	t.Parallel()
	dir := taskAuditProject(t, "t1")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.md"), []byte("# notes\n"), 0o600))
	closeWith(t, dir, "t1", commitFiles(t, dir, "notes.md"))
	out, stderr, code := runTP(t, dir, "audit", "spec.md", "--status")
	require.Equal(t, 0, code, stderr)
	var st map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &st), out)
	assert.EqualValues(t, 1, st["task_audits_nothing_to_read"])
	assert.Nil(t, st["task_audits_owed"])
}

// commitAll commits every change in dir, tp state included, so tp resume
// reports no unexplained change.
func commitAll(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "--allow-empty", "-m", "state"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
}

// assertNoStall is the no-stall property: while a closed task owes a task
// audit, the oracle hands tp run a unit that moves it, or names a blocker,
// never neither.
func assertNoStall(t *testing.T, dir, fixture string) {
	t.Helper()
	commitAll(t, dir)
	out, stderr, code := runTP(t, dir, "audit", "spec.md", "--status")
	require.Equal(t, 0, code, "%s status: %s", fixture, stderr)
	var st map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &st), out)
	owed, _ := st["task_audits_owed"].([]any)
	if len(owed) == 0 {
		return
	}
	out, stderr, code = runTP(t, dir, "resume")
	require.Equal(t, 0, code, "%s resume: %s", fixture, stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res), out)
	blockers, _ := res["blockers"].([]any)
	for _, u := range nextUnitsOf(t, res) {
		if kind := u["kind"].(string); strings.HasPrefix(kind, "task-audit") || kind == "audit-fix" {
			return
		}
	}
	assert.NotEmpty(t, blockers, "%s: %v owed, no task-audit unit and no blocker: %v", fixture, owed, res["next_units"])
}

// TestTaskAudit_NoStall: in every state the review found a stall in, a debt
// either has a unit to move it or a named blocker.
func TestTaskAudit_NoStall(t *testing.T) {
	t.Parallel()
	t.Run("zero-file close", func(t *testing.T) {
		t.Parallel()
		dir := taskAuditProject(t, "t1")
		require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.md"), []byte("# n\n"), 0o600))
		closeWith(t, dir, "t1", commitFiles(t, dir, "notes.md"))
		assertNoStall(t, dir, "zero-file close")
	})
	t.Run("fully deduped close", func(t *testing.T) {
		t.Parallel()
		dir := dedupeProject(t)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package x\n\n// t3\n"), 0o600))
		closeWith(t, dir, "t3", commitFiles(t, dir, "a.go"))
		recordTaskAuditWith(t, dir, emitTaskAudit(t, dir), "PASS")
		assertNoStall(t, dir, "fully deduped close")
	})
	t.Run("sliced task with a deleted slice", func(t *testing.T) {
		t.Parallel()
		dir := taskAuditProject(t, "t1")
		var twelve []string
		for i := range 12 {
			twelve = append(twelve, fmt.Sprintf("f%02d.go", i))
		}
		closeWith(t, dir, "t1", commitFiles(t, dir, twelve...))
		recordTaskAuditWith(t, dir, emitTaskAudit(t, dir), "PASS")
		for _, f := range twelve[10:] {
			require.NoError(t, os.Remove(filepath.Join(dir, f)))
		}
		assertNoStall(t, dir, "sliced task with a deleted slice")
	})
	t.Run("role outside the panel", func(t *testing.T) {
		t.Parallel()
		dir := taskAuditProject(t, "t1")
		_, stderr, code := runTP(t, dir, "set", "--workflow", "--project", `task_audit_roles=["ghost-role"]`)
		require.Equal(t, 0, code, stderr)
		closeWith(t, dir, "t1", commitFiles(t, dir, "a.go"))
		assertNoStall(t, dir, "role outside the panel")
	})
	t.Run("reopen between emission and record", func(t *testing.T) {
		t.Parallel()
		dir := taskAuditProject(t, "t1")
		closeWith(t, dir, "t1", commitFiles(t, dir, "a.go"))
		res := emitTaskAudit(t, dir)
		_, stderr, code := runTP(t, dir, "reopen", "t1")
		require.Equal(t, 0, code, stderr)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package x\n\n// reworked\n"), 0o600))
		closeWith(t, dir, "t1", commitFiles(t, dir, "a.go"))
		recordTaskAuditWith(t, dir, res, "PASS")
		assertNoStall(t, dir, "reopen between emission and record")
	})
	t.Run("unparseable emission", func(t *testing.T) {
		t.Parallel()
		dir := taskAuditProject(t, "t1")
		closeWith(t, dir, "t1", commitFiles(t, dir, "a.go"))
		emitTaskAudit(t, dir)
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp-review", "spec", "task-audit-inbox", "batch-1.json"), []byte("{"), 0o600))
		assertNoStall(t, dir, "unparseable emission")
	})
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

// TestTaskAudit_RecordInboxMergesARolesPartFiles (audit-round-ergonomics.md
// §2.3): a role whose checklist was answered in two parts and no whole file
// records as if one file held every row.
func TestTaskAudit_RecordInboxMergesARolesPartFiles(t *testing.T) {
	t.Parallel()
	dir := taskAuditProject(t, "t1")
	closeWith(t, dir, "t1", commitFiles(t, dir, "a.go", "b.go"))
	res := emitTaskAudit(t, dir)
	prompt := res["prompts"].([]any)[0].(map[string]any)
	whole := prompt["output_path"].(string)
	if !filepath.IsAbs(whole) {
		whole = filepath.Join(dir, whole)
	}
	require.NoError(t, os.MkdirAll(filepath.Dir(whole), 0o755))
	for i, it := range prompt["checklist_items"].([]any) {
		row, err := json.Marshal(map[string]any{"role": prompt["role"], "item_id": it.(map[string]any)["item_id"], "status": "PASS", "category": nil, "severity": nil, "notes": "n"})
		require.NoError(t, err)
		part := strings.TrimSuffix(whole, ".ndjson") + fmt.Sprintf(".p%d.ndjson", i)
		require.NoError(t, os.WriteFile(part, append(row, '\n'), 0o600))
	}

	out, stderr, code := runTP(t, dir, "audit", "spec.md", "--task-audit", "--record-inbox")
	require.Equal(t, 0, code, "record: %s", stderr)
	var rec map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &rec), out)
	assert.Equal(t, true, rec["clean"], "every item graded across the parts")
}
