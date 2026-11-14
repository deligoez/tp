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
// --task-audit --record-inbox (spec/backlog/per-task-code-audit.md).
func runTaskAuditMode(cmd *cobra.Command, args []string, taskAudit, recordInbox bool, roleFilter string) error {
	if !taskAudit {
		usageExit("--record-inbox on tp audit requires --task-audit", "record a release audit round with --record <file>")
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
	rec := nextTaskAudit(specPath, st)
	if rec == nil {
		return output.JSON(taskAuditResult{Spec: specPath, Owed: taskAuditsOwed(specPath, st), Prompts: []auditPrompt{}})
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
// first due batch frozen now. A batch with no file of its own records at
// once with no rows, since no role has anything to read.
func nextTaskAudit(specPath string, st *engine.ReviewState) *engine.TaskAuditRecord {
	for {
		seq := len(st.TaskAudits) + 1
		rec, err := engine.ReadTaskAuditEmission(specPath, seq)
		if err != nil {
			output.Error(ExitFile, err.Error(), "remove the file to emit the task audit again")
			os.Exit(ExitFile)
		}
		if rec != nil {
			return rec
		}
		rec = dueTaskAudit(specPath, st, seq)
		if rec == nil {
			return nil
		}
		rec.Head = engine.GitHead(filepath.Dir(specPath))
		rec.EmittedAt = time.Now().UTC().Format(time.RFC3339Nano)
		if err := engine.WriteTaskAuditEmission(specPath, rec); err != nil {
			output.Error(ExitFile, "cannot write task audit emission: "+err.Error())
			os.Exit(ExitFile)
		}
		if len(rec.Files) > 0 {
			return rec
		}
		recordTaskAuditRows(specPath, rec, nil)
		st = loadReviewStateOrExit(specPath)
	}
}

// dueTaskAudit is task audit seq if one is due: the first due batch of
// closed tasks, else, once the implement phase has ended, a re-grade of the
// files whose open task-audit rows a fix claim or a change made due.
func dueTaskAudit(specPath string, st *engine.ReviewState, seq int) *engine.TaskAuditRecord {
	ended := taskAuditPhaseEnded(specPath)
	if due := engine.DueTaskAudits(taskAuditCandidatesOf(specPath), st.TaskAudits, ended); len(due) > 0 {
		return &engine.TaskAuditRecord{Seq: seq, Members: due[0].Members, Files: due[0].Files(), Roles: due[0].Roles()}
	}
	if !ended {
		return nil
	}
	files, roles, fixPass := dueTaskAuditRegrade(specPath, st)
	if len(files) == 0 {
		return nil
	}
	return &engine.TaskAuditRecord{Seq: seq, Files: files[:min(len(files), engine.TaskAuditBatchFiles)], Roles: roles, Regrade: true, FixPass: fixPass}
}

// dueTaskAuditRegrade is the sorted files a re-grade asks again, the roles
// it asks, and whether a fix claim made any due (spec/backlog/per-task-code-
// audit.md §2.6). A file graded by a task audit is due for a role when that
// role's row on it is open in the carried set and either its latest grading
// claims a fix, or it was graded by a task audit at a commit the file has
// changed since. A row a fix pass graded after the latest audit round waits
// for the next round, so fix and re-grade cannot cycle past the round cap.
func dueTaskAuditRegrade(specPath string, st *engine.ReviewState) (files, roles []string, fixPass bool) {
	scan := newRegradeScan(specPath, st)
	recs := engine.TaskAuditsSinceEpoch(st)
	for i := range recs {
		for _, f := range recs[i].Files {
			for _, role := range recs[i].Roles {
				scan.visit(f, role)
			}
		}
	}
	slices.Sort(scan.files)
	slices.Sort(scan.roles)
	return scan.files, scan.roles, scan.fixPass
}

// regradeScan collects a re-grade's files and roles over the open carried
// rows a task audit graded.
type regradeScan struct {
	specPath     string
	st           *engine.ReviewState
	open         map[string]*engine.CarriedRow
	bySeq        map[int]*engine.TaskAuditRecord
	files, roles []string
	fixPass      bool
}

func newRegradeScan(specPath string, st *engine.ReviewState) *regradeScan {
	carried, _ := engine.AuditCarriedSet(specPath, st)
	s := &regradeScan{specPath: specPath, st: st,
		open: make(map[string]*engine.CarriedRow, len(carried)), bySeq: make(map[int]*engine.TaskAuditRecord, len(st.TaskAudits))}
	for i := range carried {
		if carried[i].Disposition != engine.CarryAccepted {
			s.open[engine.AuditRowRole(carried[i].Row)+"\x00"+rowItemID(carried[i].Row)] = &carried[i]
		}
	}
	for i := range st.TaskAudits {
		s.bySeq[st.TaskAudits[i].Seq] = &st.TaskAudits[i]
	}
	return s
}

// visit adds file and role when role's open row on file is due again.
func (s *regradeScan) visit(file, role string) {
	items := fileCheckItems([]engine.AuditFileEntry{{Path: file}}, role)
	row := s.open[role+"\x00"+items[0].ItemID]
	if row == nil {
		return
	}
	due, byFix := regradeDue(s.specPath, s.st, row, s.bySeq[row.TaskAudit], file)
	if !due {
		return
	}
	s.fixPass = s.fixPass || byFix
	if !slices.Contains(s.files, file) {
		s.files = append(s.files, file)
	}
	if !slices.Contains(s.roles, role) {
		s.roles = append(s.roles, role)
	}
}

// regradeDue reports whether an open row on file is asked again, and whether
// a fix claim is the reason. graded is the task audit holding the row's
// latest grading, nil when an audit round holds it.
func regradeDue(specPath string, st *engine.ReviewState, row *engine.CarriedRow, graded *engine.TaskAuditRecord, file string) (due, byFix bool) {
	if row.Disposition == engine.CarryResolved {
		waits := graded != nil && graded.FixPass && graded.AfterAuditRound == len(st.AuditRounds)
		return !waits, !waits
	}
	if graded != nil && engine.FileChangedSince(filepath.Dir(specPath), graded.Head, file) {
		return true, false
	}
	return false, false
}

// rowItemID is a row's item_id, empty when it has none.
func rowItemID(row map[string]any) string {
	id, _ := row["item_id"].(string)
	return id
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
	return engine.TaskAuditsOwed(taskAuditCandidatesOf(specPath), st.TaskAudits)
}

// taskAuditCandidatesOf is every done task of the spec's task file with the
// roles it still owes, its stamp meeting the roles the field lists now, and
// its auditable files.
func taskAuditCandidatesOf(specPath string) []engine.TaskAuditCandidate {
	wf, _ := engine.ResolveWorkflow(specPath, flagFile)
	tasks := auditTasksOf(specPath)
	out := make([]engine.TaskAuditCandidate, 0)
	for i := range tasks {
		t := &tasks[i]
		if t.Status != model.StatusDone || len(t.TaskAuditOwed) == 0 || t.ClosedAt == nil {
			continue
		}
		var roles []string
		for _, r := range t.TaskAuditOwed {
			if slices.Contains(wf.TaskAuditRoles, r) {
				roles = append(roles, r)
			}
		}
		out = append(out, engine.TaskAuditCandidate{
			ID: t.ID, ClosedAt: engine.TaskAuditClosedAt(t), Roles: roles, Files: taskAuditFiles(specPath, t),
		})
	}
	return out
}

// taskAuditFiles is the files a task's commits changed that a code role can
// audit: the release audit's type filter, no tp state, still on disk.
func taskAuditFiles(specPath string, t *model.Task) []string {
	var out []string
	for _, sha := range engine.TaskCommitSHAs(t) {
		for _, p := range execCommitFiles(filepath.Dir(specPath), sha) {
			if slices.Contains(out, p) || !isAuditableType(p) || isTPStatePath(p) || engine.IsBinaryPath(p) {
				continue
			}
			if _, err := os.Stat(p); err != nil {
				continue
			}
			out = append(out, p)
		}
	}
	return out
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
	prompts, _ := generateRoleAuditPrompts(roles, nil, &sel, specPath, "", claudeMDExcerptFor(specPath), nil, engine.PhaseTaskAudit, rec.Seq, 0, 0, 0)
	return appendClausesAudit(prompts)
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
	if rec, _ := engine.ReadTaskAuditEmission(specPath, seq); rec != nil {
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
		result["task_audits_owed"] = owed
	}
}

// loadReviewStateOrExit loads the spec's review state, an empty one when
// none exists, and exits 3 when it cannot be read.
func loadReviewStateOrExit(specPath string) *engine.ReviewState {
	st, err := engine.LoadReviewState(specPath)
	if err != nil {
		output.Error(ExitFile, err.Error())
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
	after := loadReviewStateOrExit(specPath)
	return output.JSON(map[string]any{
		"spec": specPath, "seq": entry.Seq, "file": entry.File, "findings": entry.Findings,
		"clean": entry.Clean, "owed": taskAuditsOwed(specPath, after),
	})
}

// taskAuditRoleRows reads every prompted role's file of the task audit and
// checks each row against the items the batch asked that role.
func taskAuditRoleRows(specPath string, rec *engine.TaskAuditRecord) []map[string]any {
	var missing, foreign []string
	var rows []map[string]any
	for _, p := range taskAuditPrompts(specPath, rec) {
		path := roleOutputPath(engine.PhaseTaskAudit, specPath, rec.Seq, p.Role)
		if dir := os.Getenv(engine.EnvRoundDir); dir != "" {
			path = engine.RoleFindingsPath(dir, p.Role)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			missing = append(missing, p.Role+" ("+path+")")
			continue
		}
		parsed, _, err := parseAuditRows(path, data)
		if err != nil {
			output.Error(ExitValidation, fmt.Sprintf("%s: %v", path, err))
			os.Exit(ExitValidation)
		}
		asked := make(map[string]bool)
		if p.ChecklistItems != nil {
			for _, it := range *p.ChecklistItems {
				asked[it.ItemID] = true
			}
		}
		for _, row := range parsed {
			id, _ := row["item_id"].(string)
			if !asked[id] || engine.AuditRowRole(row) != p.Role {
				foreign = append(foreign, p.Role+":"+id)
			}
		}
		rows = append(rows, parsed...)
	}
	if len(missing) > 0 {
		output.Error(ExitValidation, fmt.Sprintf("task audit %d is missing role files: %s", rec.Seq, strings.Join(missing, ", ")),
			"run each role's prompt, then record again")
		os.Exit(ExitValidation)
	}
	if len(foreign) > 0 {
		output.Error(ExitValidation, fmt.Sprintf("task audit %d holds rows on items it did not ask: %s", rec.Seq, strings.Join(foreign, ", ")),
			"a task-audit row grades one of its prompt's checklist items, under the prompt's role")
		os.Exit(ExitValidation)
	}
	return rows
}

// recordTaskAuditRows writes task audit rec.Seq's round file and appends its
// entry to the review state under the state lock, then clears its inbox.
func recordTaskAuditRows(specPath string, rec *engine.TaskAuditRecord, rows []map[string]any) engine.TaskAuditRecord {
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
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
		}
		b.Write(line)
		b.WriteByte('\n')
		if !engine.AuditRowIsPass(row) {
			findings++
		}
	}
	entry := *rec
	err := engine.WithReviewStateLock(specPath, func() error {
		st, err := engine.LoadReviewState(specPath)
		if err != nil {
			return err
		}
		if st == nil {
			st = &engine.ReviewState{Spec: specPath}
		}
		if len(st.TaskAudits)+1 != rec.Seq {
			return fmt.Errorf("task audit %d was recorded meanwhile", rec.Seq)
		}
		entry.File = engine.TaskAuditFileName(rec.Seq)
		entry.Findings = findings
		entry.Clean = engine.AuditRowsCleanLive(rows, policy)
		entry.AfterAuditRound = len(st.AuditRounds)
		entry.RecordedAt = time.Now().UTC().Format(time.RFC3339)
		if err := os.WriteFile(filepath.Join(engine.ReviewStateDir(specPath), entry.File), []byte(b.String()), 0o600); err != nil {
			return err
		}
		st.TaskAudits = append(st.TaskAudits, entry)
		return engine.SaveReviewState(specPath, st)
	})
	if err != nil {
		output.Error(ExitValidation, "cannot record task audit: "+err.Error())
		os.Exit(ExitValidation)
	}
	removeRoundInbox(specPath, engine.PhaseTaskAudit, rec.Seq)
	return entry
}
