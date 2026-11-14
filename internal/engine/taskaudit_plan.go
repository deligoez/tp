package engine

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// TaskAuditCandidates is every done task of tasks that carries a
// task_audit_owed stamp, with the roles it still owes (its stamp met with
// roles, the roles task_audit_roles lists now) and its auditable files. The
// files are computed only for a task that owes a role: no reader of a
// candidate looks at the files of one that owes none.
func TaskAuditCandidates(specPath string, tasks []model.Task, roles []string) []TaskAuditCandidate {
	out := make([]TaskAuditCandidate, 0)
	root := ""
	for i := range tasks {
		t := &tasks[i]
		if t.Status != model.StatusDone || len(t.TaskAuditOwed) == 0 || t.ClosedAt == nil {
			continue
		}
		var owed []string
		for _, r := range t.TaskAuditOwed {
			if slices.Contains(roles, r) {
				owed = append(owed, r)
			}
		}
		c := TaskAuditCandidate{ID: t.ID, ClosedAt: TaskAuditClosedAt(t), Roles: owed}
		if len(owed) > 0 {
			if root == "" {
				root = taskAuditRoot(specPath)
			}
			c.Files = taskAuditFiles(specPath, root, t)
		}
		out = append(out, c)
	}
	return out
}

// TaskAuditRolesFor is the roles a task audit of specPath can ask: the
// spec-adjacent task file's resolved task_audit_roles that its audit panel
// holds, never spec-coverage. A role the panel lacks is reported by
// TaskAuditRolesOutsidePanel, never owed, since no prompt could ask it.
func TaskAuditRolesFor(specPath string) []string {
	in, _ := splitTaskAuditRoles(specPath)
	return in
}

// TaskAuditRolesOutsidePanel is the task_audit_roles the spec's audit panel
// does not hold: configured, and asked by no task audit of this spec.
func TaskAuditRolesOutsidePanel(specPath string) []string {
	_, out := splitTaskAuditRoles(specPath)
	return out
}

func splitTaskAuditRoles(specPath string) (in, out []string) {
	taskPath := strings.TrimSuffix(specPath, filepath.Ext(specPath)) + ".tasks.json"
	roles := EffectiveWorkflowForTaskFile(taskPath).TaskAuditRoles
	if len(roles) == 0 {
		return nil, nil
	}
	panel, err := ResolveRolePanel(specPath, PhaseAuditors)
	for _, r := range roles {
		if r == RoleSpecCoverage {
			continue
		}
		held := err == nil && slices.ContainsFunc(panel.Roles, func(p model.Role) bool { return p.ID == r })
		if held {
			in = append(in, r)
		} else {
			out = append(out, r)
		}
	}
	return in, out
}

// TaskAuditNothingToRead counts the stamped closes that owe a role yet had no
// auditable file in the tree to read: an evidence-only close, a close with no
// recorded commit, or one whose files are gone. They owe nothing and are
// counted so the reader can tell them from closes that were audited.
func TaskAuditNothingToRead(specPath string, tasks []model.Task) int {
	n := 0
	for _, c := range TaskAuditCandidates(specPath, tasks, TaskAuditRolesFor(specPath)) {
		if len(c.Roles) > 0 && len(c.Files) == 0 {
			n++
		}
	}
	return n
}

// taskAuditRoot is the directory a task commit's paths are relative to: the
// repository holding the spec, or the spec's directory outside one.
func taskAuditRoot(specPath string) string {
	dir := filepath.Dir(specPath)
	if root := gitToplevel(dir); root != "" {
		return root
	}
	return dir
}

// taskAuditFiles is the files a task's commits changed that a code role can
// audit: the release audit's type filter, no tp state, still on disk under
// root.
func taskAuditFiles(specPath, root string, t *model.Task) []string {
	var changed []string
	if len(t.CommitFiles) > 0 && t.CommitFilesTotal == 0 {
		// tp done recorded them, whole: no git to run.
		changed = t.CommitFiles
	} else {
		for _, sha := range TaskCommitSHAs(t) {
			changed = append(changed, commitChangedFiles(filepath.Dir(specPath), sha, t.ID)...)
		}
	}
	var out []string
	for _, p := range changed {
		if slices.Contains(out, p) || !IsAuditableType(p) || IsTPStatePath(p) || IsBinaryPath(p) {
			continue
		}
		// A file gone from the tree (deleted or renamed away) owes nothing.
		if _, err := os.Stat(filepath.Join(root, p)); err != nil {
			continue
		}
		out = append(out, p)
	}
	return out
}

// DueTaskAudit is task audit seq if one is due given the task audits already
// recorded: the first due batch of closed tasks, else, once the implement
// phase has ended, a re-grade of the files whose open task-audit rows a fix
// claim or a change made due. The re-grade reads st, the state on disk.
func DueTaskAudit(specPath string, st *ReviewState, recorded []TaskAuditRecord, seq int, ended bool, cands []TaskAuditCandidate) *TaskAuditRecord {
	cands = dropGradedFiles(specPath, st, subtractReadFiles(cands, recorded))
	if due := DueTaskAudits(cands, recorded, ended); len(due) > 0 {
		return &TaskAuditRecord{Seq: seq, Members: due[0].Members, Files: due[0].Files(), Roles: due[0].Roles()}
	}
	if !ended {
		return nil
	}
	files, roles, fixPass := TaskAuditRegrade(specPath, st)
	if len(files) == 0 {
		return nil
	}
	return &TaskAuditRecord{Seq: seq, Files: files[:min(len(files), TaskAuditBatchFiles)], Roles: roles, Regrade: true, FixPass: fixPass}
}

// dropGradedFiles drops from each candidate the files every role it owes
// already has a recorded row on, read by a task audit at a commit the file
// has not changed since: those rows stand, so reading the file again buys
// only a second opinion that could clear a real row
// (spec/1.22.0-measurements.md). A candidate keeps its membership with no
// file left, so presence is unaffected, and ten is counted after the drop.
// The answer is deterministic only because one audit emission is in flight
// per spec: batch n+1 is packed after batch n is recorded, never beside it.
// An audit round records no commit, so only task audits cover a file.
func dropGradedFiles(specPath string, st *ReviewState, cands []TaskAuditCandidate) []TaskAuditCandidate {
	cover := taskAuditCoverage(specPath, st)
	if len(cover) == 0 {
		return cands
	}
	dir := filepath.Dir(specPath)
	out := make([]TaskAuditCandidate, 0, len(cands))
	for _, c := range cands {
		if len(c.Roles) > 0 {
			kept := make([]string, 0, len(c.Files))
			for _, f := range c.Files {
				if !coveredForEveryRole(cover, dir, c.Roles, f) {
					kept = append(kept, f)
				}
			}
			c.Files = kept
		}
		out = append(out, c)
	}
	return out
}

// taskAuditCoverage maps each (role, item) a task audit since the newest
// epoch recorded a row on to the commits those task audits read.
func taskAuditCoverage(specPath string, st *ReviewState) map[string][]string {
	cover := make(map[string][]string)
	recs := TaskAuditsSinceEpoch(st)
	for i := range recs {
		if recs[i].Head == "" || recs[i].File == "" {
			continue
		}
		rows, ok := LoadRoundRows(specPath, &ReviewRound{File: recs[i].File})
		if !ok {
			continue
		}
		for _, row := range rows {
			key := AuditRowRole(row) + "\x00" + rowString(row, "item_id")
			cover[key] = append(cover[key], recs[i].Head)
		}
	}
	return cover
}

// coveredForEveryRole reports whether every role has a recorded row on file
// read at a commit the file has not changed since.
func coveredForEveryRole(cover map[string][]string, dir string, roles []string, file string) bool {
	for _, role := range roles {
		if !slices.ContainsFunc(cover[role+"\x00"+FileCheckItemID(role, file)], func(head string) bool {
			return !FileChangedSince(dir, head, file)
		}) {
			return false
		}
	}
	return true
}

// NextTaskAuditPlan is the task audit an emission would freeze now when none
// is in flight, nil when nothing is due. It writes nothing: the oracle and the
// emission compute the same answer from the same state.
func NextTaskAuditPlan(specPath string, st *ReviewState, cands []TaskAuditCandidate, ended bool) *TaskAuditRecord {
	var recorded []TaskAuditRecord
	if st != nil {
		recorded = st.TaskAudits
	}
	return DueTaskAudit(specPath, st, recorded, len(recorded)+1, ended, cands)
}

// TaskAuditRegrade is the sorted files a re-grade asks again, the roles it
// asks, and whether a fix claim made any due (spec/backlog/per-task-code-
// audit.md §2.6). A file graded by a task audit is due for a role when that
// role's row on it is open in the carried set and either its latest grading
// claims a fix, or it was graded by a task audit at a commit the file has
// changed since. A row a fix pass graded after the latest audit round waits
// for the next round, so fix and re-grade cannot cycle past the round cap.
func TaskAuditRegrade(specPath string, st *ReviewState) (files, roles []string, fixPass bool) {
	if st == nil {
		return nil, nil, false
	}
	scan := newRegradeScan(specPath, st)
	recs := TaskAuditsSinceEpoch(st)
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
	st           *ReviewState
	open         map[string]*CarriedRow
	bySeq        map[int]*TaskAuditRecord
	files, roles []string
	fixPass      bool
}

func newRegradeScan(specPath string, st *ReviewState) *regradeScan {
	carried, _ := AuditCarriedSet(specPath, st)
	s := &regradeScan{specPath: specPath, st: st,
		open: make(map[string]*CarriedRow, len(carried)), bySeq: make(map[int]*TaskAuditRecord, len(st.TaskAudits))}
	for i := range carried {
		if carried[i].Disposition != CarryAccepted {
			s.open[AuditRowRole(carried[i].Row)+"\x00"+rowString(carried[i].Row, "item_id")] = &carried[i]
		}
	}
	for i := range st.TaskAudits {
		s.bySeq[st.TaskAudits[i].Seq] = &st.TaskAudits[i]
	}
	return s
}

// visit adds file and role when role's open row on file is due again.
func (s *regradeScan) visit(file, role string) {
	row := s.open[role+"\x00"+FileCheckItemID(role, file)]
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
func regradeDue(specPath string, st *ReviewState, row *CarriedRow, graded *TaskAuditRecord, file string) (due, byFix bool) {
	if row.Disposition == CarryResolved {
		waits := graded != nil && graded.FixPass && graded.AfterAuditRound == len(st.AuditRounds)
		return !waits, !waits
	}
	if graded != nil && FileChangedSince(filepath.Dir(specPath), graded.Head, file) {
		return true, false
	}
	return false, false
}
