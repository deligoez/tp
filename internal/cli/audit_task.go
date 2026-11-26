package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// taskAuditConflicts are the tp audit flags a task audit reads none of.
var taskAuditConflicts = []string{
	"affected-files", "affected-from-tasks", "base", "findings", "record", "status",
	"check", "harness-note", "merge", "resolve", "resolve-all", "force", "output",
}

// runTaskAuditMode routes tp audit <spec> --task-audit [--role r] and
// --task-audit --record-inbox (spec/backlog/per-task-code-audit.md at c15412d4).
func runTaskAuditMode(cmd *cobra.Command, args []string, taskAudit, recordInbox bool, roleFilter string) error {
	if !taskAudit {
		return runReleaseAuditRecordInbox(cmd, args)
	}
	for _, name := range taskAuditConflicts {
		if cmd.Flags().Changed(name) {
			usageExit("--task-audit cannot be combined with --"+name, "a task audit takes only the spec, --role and --record-inbox")
		}
	}
	if recordInbox && cmd.Flags().Changed("role") {
		usageExit("--role cannot be combined with --record-inbox", "--record-inbox records every role of the emitted task audit")
	}
	if len(args) != 1 {
		usageExit("spec path required", "")
	}
	specPath := engine.ResolveSpecArg(args[0])
	if _, err := os.Stat(specPath); err != nil {
		output.Error(ExitFile, fmt.Sprintf("spec not found: %s", specPath), specFileMissingHint)
		os.Exit(ExitFile)
	}
	enterTree(emissionTarget(specPath), "tp audit")
	if recordInbox {
		return runTaskAuditRecord(specPath)
	}
	return runTaskAudit(specPath, roleFilter, cmd.Flags().Changed("role"))
}

// usageExit exits 2 with msg and hint.
func usageExit(msg, hint string) {
	if hint == "" {
		output.Error(ExitUsage, msg)
	} else {
		output.Error(ExitUsage, msg, hint)
	}
	os.Exit(ExitUsage)
}

// taskAuditView is the task audit an emission names.
type taskAuditView struct {
	Seq     int      `json:"seq"`
	Tasks   []string `json:"tasks"`
	Files   []string `json:"files"`
	Roles   []string `json:"roles"`
	Head    string   `json:"head"`
	Regrade bool     `json:"regrade,omitempty"`
}

func viewOf(rec *engine.TaskAuditRecord) *taskAuditView {
	v := &taskAuditView{Seq: rec.Seq, Tasks: []string{}, Files: rec.Files, Roles: rec.Roles, Head: rec.Head, Regrade: rec.Regrade}
	for _, m := range rec.Members {
		if !slices.Contains(v.Tasks, m.ID) {
			v.Tasks = append(v.Tasks, m.ID)
		}
	}
	if v.Files == nil {
		v.Files = []string{}
	}
	return v
}

// taskAuditResult is tp audit --task-audit's payload.
type taskAuditResult struct {
	Spec      string         `json:"spec"`
	TaskAudit *taskAuditView `json:"task_audit"`
	Owed      bool           `json:"owed"`
	Prompts   []auditPrompt  `json:"prompts"`
}

// runTaskAudit emits the task audit due now: the unrecorded one already
// frozen, else the first due batch, frozen before any prompt is built.
// Nothing due prints a null task_audit and whether anything is still owed.
func runTaskAudit(specPath, roleFilter string, roleGiven bool) error {
	st := loadReviewStateOrExit(specPath)
	refuseWhileAuditRoundInFlight(specPath, st)
	rec := nextTaskAudit(specPath)
	if rec == nil {
		// The freeze may have changed the state; owed is read after it.
		return output.JSON(taskAuditResult{Spec: specPath, Owed: taskAuditsOwed(specPath, loadReviewStateOrExit(specPath)), Prompts: []auditPrompt{}})
	}
	prompts := taskAuditPrompts(specPath, rec)
	if roleGiven {
		kept := prompts[:0]
		for _, p := range prompts {
			if p.Role == roleFilter {
				kept = append(kept, p)
			}
		}
		if len(kept) == 0 {
			usageExit(fmt.Sprintf("--role %s is not a role of task audit %d", roleFilter, rec.Seq), "its roles are "+strings.Join(rec.Roles, ", "))
		}
		prompts = kept
	}
	return output.JSON(taskAuditResult{Spec: specPath, TaskAudit: viewOf(rec), Owed: true, Prompts: prompts})
}

// nextTaskAudit is the task audit to emit: the frozen unrecorded one, or the
// first due batch frozen now. A due batch always holds a file: a close with
// none left to read owes nothing, and a re-grade with none is not due.
func nextTaskAudit(specPath string) *engine.TaskAuditRecord {
	// Role units of one task audit run side by side and each emits it: the
	// read-or-freeze is one critical section, so they freeze it once.
	var rec *engine.TaskAuditRecord
	err := engine.WithReviewStateLock(specPath, func() error {
		var err error
		rec, err = freezeNextTaskAudit(specPath)
		return err
	})
	if err != nil {
		output.Error(ExitFile, "cannot freeze the next task audit: "+err.Error(), stateWriteHint)
		os.Exit(ExitFile)
	}
	return rec
}

// freezeNextTaskAudit is nextTaskAudit's body, run under the state lock.
func freezeNextTaskAudit(specPath string) (*engine.TaskAuditRecord, error) {
	st, err := engine.LoadReviewState(specPath)
	if err != nil {
		return nil, err
	}
	if st == nil {
		st = &engine.ReviewState{}
	}
	seq := len(st.TaskAudits) + 1
	rec, err := engine.ReadTaskAuditEmission(specPath, seq)
	if err != nil || rec != nil {
		return rec, err
	}
	rec = dueTaskAudit(specPath, st, seq)
	if rec == nil {
		return nil, nil
	}
	rec.Head = engine.GitHead(filepath.Dir(specPath))
	rec.EmittedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := engine.WriteTaskAuditEmission(specPath, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// dueTaskAudit is task audit seq if one is due: the engine's answer over the
// spec's candidates, the one the oracle schedules from.
func dueTaskAudit(specPath string, st *engine.ReviewState, seq int) *engine.TaskAuditRecord {
	return engine.DueTaskAudit(specPath, st, st.TaskAudits, seq, taskAuditPhaseEnded(specPath), taskAuditCandidatesOf(specPath))
}

// taskAuditPhaseEnded reports whether the implement phase is over, so a
// partial batch is due: every task of the spec's task file is done.
func taskAuditPhaseEnded(specPath string) bool {
	tasks := auditTasksOf(specPath)
	for i := range tasks {
		if tasks[i].Status != model.StatusDone {
			return false
		}
	}
	return true
}

// taskAuditsOwed reports whether a done task still owes a task audit.
func taskAuditsOwed(specPath string, st *engine.ReviewState) bool {
	return len(engine.TaskAuditOwedTasks(specPath, st)) > 0
}

// taskAuditCandidatesOf is every done task of the spec's task file with the
// roles it still owes, its stamp met with the roles task_audit_roles lists
// and the audit panel holds, and its auditable files.
func taskAuditCandidatesOf(specPath string) []engine.TaskAuditCandidate {
	return engine.TaskAuditCandidates(specPath, auditTasksOf(specPath), engine.TaskAuditRolesFor(specPath))
}

// taskAuditPrompts builds one prompt per role of the task audit that the
// spec's audit panel holds, over the batch's files as a named list.
func taskAuditPrompts(specPath string, rec *engine.TaskAuditRecord) []auditPrompt {
	panel := resolveRolePanel(specPath, engine.PhaseAuditors)
	roles := make([]model.Role, 0, len(rec.Roles))
	for i := range panel.roles {
		if r := &panel.roles[i]; r.ID != roleSpecCoverage && slices.Contains(rec.Roles, r.ID) {
			roles = append(roles, *r)
		}
	}
	entries := make([]engine.AuditFileEntry, 0, len(rec.Files))
	for _, f := range rec.Files {
		entries = append(entries, engine.AuditFileEntry{Path: f, Tasks: []string{}})
	}
	sel := engine.AuditFileSelection{CodeFiles: entries, CodeFilesTotal: len(entries), CodeFilesNamed: true}
	prompts, _, _ := generateRoleAuditPrompts(roles, nil, &sel, specPath, "", claudeMDExcerptFor(specPath),
		taskAuditPriorRows(specPath, entries), engine.PhaseTaskAudit, rec.Seq, 0, 0, 0)
	return appendClausesAudit(prompts)
}

// taskAuditPriorRows is, per role, the open carried rows on the batch's
// files, read from the spec's state rather than handed in: every reading of
// a file is shown the rows a PASS of it would clear, so no PASS clears a row
// its reader never saw.
func taskAuditPriorRows(specPath string, entries []engine.AuditFileEntry) map[string]*auditPriorRound {
	all := loadAuditPriorRound(specPath)
	out := make(map[string]*auditPriorRound, len(all))
	for role, p := range all {
		asked := make(map[string]bool, len(entries))
		for _, it := range fileCheckItems(entries, role) {
			asked[it.ItemID] = true
		}
		kept := &auditPriorRound{legacy: p.legacy}
		for i := range p.rows {
			if asked[p.rows[i].ItemID] {
				kept.rows = append(kept.rows, p.rows[i])
			}
		}
		if len(kept.rows) > 0 {
			out[role] = kept
		}
	}
	return out
}

// refuseWhileAuditRoundInFlight exits 1 while a release audit round is
// emitted and unrecorded: one audit emission of either kind is in flight per
// spec, so where a task audit sits among the audit rounds is never in doubt.
func refuseWhileAuditRoundInFlight(specPath string, st *engine.ReviewState) {
	if n := engine.InFlightRound(specPath, engine.PhaseAudit, len(st.AuditRounds)); n > 0 {
		output.Error(ExitValidation, fmt.Sprintf("audit round %d is emitted and unrecorded; a task audit waits for it", n),
			fmt.Sprintf("record audit round %d first: tp audit %s --record <file>", n, specPath))
		os.Exit(ExitValidation)
	}
}

// refuseWhileTaskAuditInFlight exits 1 while a task audit is emitted and
// unrecorded, the other half of the one-in-flight rule.
func refuseWhileTaskAuditInFlight(specPath string) {
	st, err := engine.LoadReviewState(specPath)
	if err != nil || st == nil {
		st = &engine.ReviewState{}
	}
	seq := len(st.TaskAudits) + 1
	rec, err := engine.ReadTaskAuditEmission(specPath, seq)
	if err != nil {
		// The task-audit side refuses this file too: neither emits past it.
		output.Error(ExitFile, err.Error(), "the frozen task audit cannot be read; restore it, or remove it once no role of it is running")
		os.Exit(ExitFile)
	}
	if rec != nil {
		output.Error(ExitValidation, fmt.Sprintf("task audit %d is emitted and unrecorded; an audit round waits for it", seq),
			fmt.Sprintf("run its roles and record it: tp audit %s --task-audit --record-inbox", specPath))
		os.Exit(ExitValidation)
	}
}

// addTaskAudits writes the task audits onto an audit status payload: how
// many are recorded, and the closed tasks still owing one, present only
// while any does.
func addTaskAudits(result map[string]any, specPath string, st *engine.ReviewState) {
	if st != nil && len(st.TaskAudits) > 0 {
		result["task_audits"] = len(st.TaskAudits)
	}
	if owed := engine.TaskAuditOwedTasks(specPath, st); len(owed) > 0 {
		if path, ok := engine.TaskAuditUnreadable(owed); ok {
			result["task_audits_unknown"] = path
		} else {
			result["task_audits_owed"] = owed
		}
	}
	// What no task audit reads is named, never silent: closes with no file
	// to audit, and configured roles this spec's panel cannot ask.
	if n := engine.TaskAuditNothingToRead(specPath, auditTasksOf(specPath)); n > 0 {
		result["task_audits_nothing_to_read"] = n
	}
	if outside := engine.TaskAuditRolesOutsidePanel(specPath); len(outside) > 0 {
		result["task_audit_roles_outside_panel"] = outside
	}
}

// loadReviewStateOrExit loads the spec's review state, an empty one when
// none exists, and exits 3 when it cannot be read.
func loadReviewStateOrExit(specPath string) *engine.ReviewState {
	st, err := engine.LoadReviewState(specPath)
	if err != nil {
		output.Error(ExitFile, err.Error(), stateWriteHint)
		os.Exit(ExitFile)
	}
	if st == nil {
		st = &engine.ReviewState{}
	}
	return st
}

// runTaskAuditRecord records the emitted task audit from its roles' files:
// every prompted role's file must be there, and every row must grade an item
// the batch asked.
func runTaskAuditRecord(specPath string) error {
	st := loadReviewStateOrExit(specPath)
	seq := len(st.TaskAudits) + 1
	rec, err := engine.ReadTaskAuditEmission(specPath, seq)
	if err != nil || rec == nil {
		output.Error(ExitFile, fmt.Sprintf("no task audit is emitted and unrecorded for %s", specPath),
			fmt.Sprintf("emit one first: tp audit %s --task-audit", specPath))
		os.Exit(ExitFile)
	}
	rows := taskAuditRoleRows(specPath, rec)
	entry := recordTaskAuditRows(specPath, rec, rows)
	read := make(map[string][]string)
	for _, role := range rec.Roles {
		read[role] = taskAuditRoleFiles(specPath, rec.Seq, role)
	}
	removeNamedInboxFiles(read)
	after := loadReviewStateOrExit(specPath)
	return output.JSON(map[string]any{
		"spec": specPath, "seq": entry.Seq, "file": entry.File, "findings": entry.Findings,
		"clean": entry.Clean, "owed": taskAuditsOwed(specPath, after),
	})
}

// taskAuditRoleRows reads every prompted role's files of the task audit, its
// whole file and any part files (engine.RoleInboxFiles), and checks the rows
// of each role against the items the batch asked it.
func taskAuditRoleRows(specPath string, rec *engine.TaskAuditRecord) []map[string]any {
	var missing, foreign, ungraded []string
	var rows []map[string]any
	asked := 0
	for _, p := range taskAuditPrompts(specPath, rec) {
		asked++
		paths := taskAuditRoleFiles(specPath, rec.Seq, p.Role)
		if len(paths) == 0 {
			missing = append(missing, p.Role+" ("+roleOutputPath(engine.PhaseTaskAudit, specPath, rec.Seq, p.Role)+")")
			continue
		}
		var parsed []map[string]any
		for _, path := range paths {
			parsed = append(parsed, readTaskAuditRoleFile(path)...)
		}
		parsed, _ = dedupAuditRows(parsed)
		f, u := checkTaskAuditRows(&p, parsed)
		foreign, ungraded = append(foreign, f...), append(ungraded, u...)
		rows = append(rows, parsed...)
	}
	missingHint := "run each role's prompt, then record again"
	if len(missing) > 0 && len(missing) == asked && os.Getenv(engine.EnvRoundDir) == "" {
		// No role's file is in the spec's folder: a round left flat in the
		// layout before v1.34.0 is moved, not run again (flatInboxHint).
		if flat := flatInboxHint(specPath, engine.PhaseTaskAudit, rec.Seq, "tp audit "+specPath+" --task-audit"); flat != "" {
			missingHint = flat
		}
	}
	for _, refusal := range []struct {
		list      []string
		msg, hint string
	}{
		{missing, "is missing role files", missingHint},
		{foreign, "holds rows on items it did not ask", "a task-audit row grades one of its prompt's checklist items, under the prompt's role"},
		{ungraded, "leaves items ungraded", "a task audit settles a file only when every role graded it: add a row for each item named, then record again"},
	} {
		if len(refusal.list) > 0 {
			output.Error(ExitValidation, fmt.Sprintf("task audit %d %s: %s", rec.Seq, refusal.msg, strings.Join(refusal.list, ", ")), refusal.hint)
			os.Exit(ExitValidation)
		}
	}
	return rows
}

// taskAuditRoleFiles is the files one role of task audit seq wrote: under tp
// run its file in the round directory, otherwise its whole file and part
// files in the inbox.
func taskAuditRoleFiles(specPath string, seq int, role string) []string {
	if dir := os.Getenv(engine.EnvRoundDir); dir != "" {
		if path := engine.RoleFindingsPath(dir, role); isRegularFile(path) {
			return []string{path}
		}
		return nil
	}
	return engine.RoleInboxFiles(namedInbox(), specPath, engine.PhaseTaskAudit, seq, role)
}

// readTaskAuditRoleFile parses one role file's rows, exiting 1 on a line that
// is not one JSON object.
func readTaskAuditRoleFile(path string) []map[string]any {
	data, err := os.ReadFile(path)
	if err == nil {
		var parsed []map[string]any
		if parsed, _, err = parseAuditRows(path, data); err == nil {
			return parsed
		}
	}
	output.Error(ExitValidation, fmt.Sprintf("%s: %v", path, err), "fix the line the message names in the role's file: every non-blank line is one JSON object the auditor wrote")
	os.Exit(ExitValidation)
	return nil
}

// checkTaskAuditRows is the rows of one role's file that grade no item its
// prompt asked, and the asked items no row grades: a file counts as read
// only when every role graded it.
func checkTaskAuditRows(p *auditPrompt, parsed []map[string]any) (foreign, ungraded []string) {
	asked := make(map[string]bool)
	if p.ChecklistItems != nil {
		for _, it := range *p.ChecklistItems {
			asked[it.ItemID] = false
		}
	}
	for _, row := range parsed {
		id, _ := row["item_id"].(string)
		if _, ok := asked[id]; !ok || engine.AuditRowRole(row) != p.Role {
			foreign = append(foreign, p.Role+":"+id)
			continue
		}
		asked[id] = true
	}
	for id, graded := range asked {
		if !graded {
			ungraded = append(ungraded, p.Role+":"+id)
		}
	}
	slices.Sort(ungraded)
	return foreign, ungraded
}

// recordTaskAuditRows writes task audit rec.Seq's round file and appends its
// entry to the review state under the state lock, then clears its inbox.
func recordTaskAuditRows(specPath string, rec *engine.TaskAuditRecord, rows []map[string]any) engine.TaskAuditRecord {
	var entry engine.TaskAuditRecord
	err := engine.WithReviewStateLock(specPath, func() error {
		var err error
		entry, err = writeTaskAuditRecord(specPath, rec, rows)
		return err
	})
	if err != nil {
		output.Error(ExitValidation, "cannot record task audit: "+err.Error(), stateWriteHint)
		os.Exit(ExitValidation)
	}
	removeRoundInbox(specPath, engine.PhaseTaskAudit, rec.Seq)
	return entry
}

// writeTaskAuditRecord writes task audit rec.Seq's round file and appends its
// entry to the review state; the caller holds the state lock.
func writeTaskAuditRecord(specPath string, rec *engine.TaskAuditRecord, rows []map[string]any) (engine.TaskAuditRecord, error) {
	wf, _ := engine.ResolveWorkflow(specPath, flagFile)
	policy := wf.AuditConvergeOn
	if !engine.ValidAuditConvergeOn(policy) {
		policy = engine.AuditConvergeOnAll
	}
	var b strings.Builder
	findings := 0
	for _, row := range rows {
		line, err := json.Marshal(row)
		if err != nil {
			return engine.TaskAuditRecord{}, err
		}
		b.Write(line)
		b.WriteByte('\n')
		if !engine.AuditRowIsPass(row) {
			findings++
		}
	}
	st, err := engine.LoadReviewState(specPath)
	if err != nil {
		return engine.TaskAuditRecord{}, err
	}
	if st == nil {
		st = &engine.ReviewState{Spec: specPath}
	}
	if len(st.TaskAudits)+1 != rec.Seq {
		return engine.TaskAuditRecord{}, fmt.Errorf("task audit %d was recorded meanwhile", rec.Seq)
	}
	entry := *rec
	entry.File = engine.TaskAuditFileName(rec.Seq)
	entry.Findings = findings
	entry.Clean = engine.AuditRowsCleanLive(rows, policy)
	entry.AfterAuditRound = len(st.AuditRounds)
	entry.RecordedAt = time.Now().UTC().Format(time.RFC3339)
	if err := os.WriteFile(filepath.Join(engine.ReviewStateDir(specPath), entry.File), []byte(b.String()), 0o600); err != nil {
		return engine.TaskAuditRecord{}, err
	}
	st.TaskAudits = append(st.TaskAudits, entry)
	return entry, engine.SaveReviewState(specPath, st)
}
