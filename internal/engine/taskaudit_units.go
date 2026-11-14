package engine

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// UnitRoundPhase is the phase a unit's round directory is keyed by: the task
// audit's own for its two kinds, whose round is the task audit's seq, and the
// oracle's phase for every other kind.
func UnitRoundPhase(kind UnitKind, phase string) string {
	if kind == UnitTaskAuditRole || kind == UnitTaskAuditRecord {
		return PhaseTaskAudit
	}
	return phase
}

// implementTaskAuditUnits is the implement phase's task-audit step
// (spec/backlog/per-task-code-audit.md): the task audit in flight, else a full
// batch due now, runs before the next implement unit. Nil when neither is.
func implementTaskAuditUnits(root string, target UnitTarget, tf *model.TaskFile, st *ReviewState) (units []NextUnit, seq *int) {
	if units, seq := inFlightTaskAuditUnits(root, target, st); units != nil {
		return units, seq
	}
	return plannedTaskAuditUnits(target, NextTaskAuditPlan(target.Spec, st, oracleTaskAuditCandidates(target, tf), false))
}

// auditTaskAuditUnits is the audit phase's task-audit step, ahead of the
// release audit's own units: the task audit in flight, then the owed batches
// the end of the implement phase made due, then one fix unit for a task-audit
// row graded and not disposed, then a re-grade. A release audit round in
// flight goes first, since a task audit waits for it. Nil when none applies.
func auditTaskAuditUnits(root string, target UnitTarget, tf *model.TaskFile, st *ReviewState, recorded int) (units []NextUnit, seq *int) {
	if InFlightRound(target.Spec, PhaseAudit, recorded) > 0 {
		return nil, nil
	}
	if units, seq := inFlightTaskAuditUnits(root, target, st); units != nil {
		return units, seq
	}
	plan := NextTaskAuditPlan(target.Spec, st, oracleTaskAuditCandidates(target, tf), true)
	if plan != nil && !plan.Regrade {
		return plannedTaskAuditUnits(target, plan)
	}
	if fix, ok := taskAuditFixUnit(target, st); ok {
		collecting := recorded + 1
		return []NextUnit{fix}, &collecting
	}
	return plannedTaskAuditUnits(target, plan)
}

// oracleTaskAuditCandidates is the candidates over the task file the oracle
// was handed, with the roles task_audit_roles lists now.
func oracleTaskAuditCandidates(target UnitTarget, tf *model.TaskFile) []TaskAuditCandidate {
	if tf == nil {
		return nil
	}
	roles := EffectiveWorkflowForTaskFile(target.TaskFile).TaskAuditRoles
	if len(roles) == 0 {
		return nil
	}
	return TaskAuditCandidates(target.Spec, tf.Tasks, roles)
}

// inFlightTaskAuditUnits is the units the emitted, unrecorded task audit still
// owes: its roles whose file is not yet in its round directory, or, once each
// has one, its record unit. Nil when none is in flight, or its emission does
// not parse — the emission refuses that one too.
func inFlightTaskAuditUnits(root string, target UnitTarget, st *ReviewState) (units []NextUnit, seq *int) {
	n := 1
	if st != nil {
		n = len(st.TaskAudits) + 1
	}
	frozen, err := ReadTaskAuditEmission(target.Spec, n)
	if err != nil || frozen == nil {
		return nil, nil
	}
	dir := RoundDir(root, target.TaskFile, PhaseTaskAudit, n)
	units = make([]NextUnit, 0)
	for _, role := range taskAuditPanelRoles(target.Spec, frozen.Roles) {
		if !UnitTaskAuditRole.DurableWrite(UnitTarget{RoundDir: dir, ID: role}) {
			units = append(units, newNextUnit(UnitTaskAuditRole, role, target))
		}
	}
	if len(units) == 0 {
		units = append(units, newNextUnit(UnitTaskAuditRecord, strconv.Itoa(n), target))
	}
	return units, &n
}

// plannedTaskAuditUnits is one role unit per role of the task audit the
// emission would freeze now, the panel's roles it names; nil for no plan.
func plannedTaskAuditUnits(target UnitTarget, plan *TaskAuditRecord) (units []NextUnit, seq *int) {
	if plan == nil {
		return nil, nil
	}
	units = make([]NextUnit, 0, len(plan.Roles))
	for _, role := range taskAuditPanelRoles(target.Spec, plan.Roles) {
		units = append(units, newNextUnit(UnitTaskAuditRole, role, target))
	}
	if len(units) == 0 {
		return nil, nil
	}
	n := plan.Seq
	return units, &n
}

// taskAuditPanelRoles is the roles of the spec's resolved audit panel, in its
// order, that a task audit asks: the ones roles names, never spec-coverage —
// the roles whose prompts the emission builds.
func taskAuditPanelRoles(specPath string, roles []string) []string {
	panel, err := ResolveRolePanel(specPath, PhaseAuditors)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(roles))
	for i := range panel.Roles {
		if id := panel.Roles[i].ID; id != RoleSpecCoverage && slices.Contains(roles, id) {
			out = append(out, id)
		}
	}
	return out
}

// taskAuditFixUnit is the audit-fix unit for the first carried row whose
// latest grading is a task audit and whose findings under its key are not
// disposed in that task audit's round file — the exact negation of the
// task-audit half of the kind's durable write.
func taskAuditFixUnit(target UnitTarget, st *ReviewState) (NextUnit, bool) {
	rows, _ := AuditCarriedSet(target.Spec, st)
	for i := range rows {
		if rows[i].TaskAudit == 0 {
			continue
		}
		key := auditRowKey(rows[i].Row)
		if !rowIsDisposed(rows[i].File, key) {
			return newNextUnit(UnitAuditFix, key, target), true
		}
	}
	return NextUnit{}, false
}

// TaskAuditFixFile is the task-audit round file holding the latest grading of
// the carried row under key, a `role:item_id`, or "" when an audit round
// holds it, nothing carries the key, or the spec's state cannot be read.
func TaskAuditFixFile(specPath, key string) string {
	if specPath == "" {
		return ""
	}
	st, err := LoadReviewState(specPath)
	if err != nil || st == nil || len(st.TaskAudits) == 0 {
		return ""
	}
	rows, _ := AuditCarriedSet(specPath, st)
	for i := range rows {
		if rows[i].TaskAudit > 0 && auditRowKey(rows[i].Row) == key {
			return rows[i].File
		}
	}
	return ""
}

// auditFixDisposed is the audit-fix kind's predicate: every finding under the
// unit's key is disposed where the oracle emits a fix unit for it — the round's
// merged file, and the task-audit round file when a task audit holds the key's
// latest grading. With no such task audit it is the merged file alone, as it
// always was.
func auditFixDisposed(t UnitTarget) bool {
	merged := MergedFindingsPath(t.RoundDir)
	file := TaskAuditFixFile(t.Spec, t.ID)
	if file == "" {
		return rowIsDisposed(merged, t.ID)
	}
	return rowIsDisposed(file, t.ID) && !hasUndisposedFinding(merged, t.ID)
}

// hasUndisposedFinding reports whether the file holds a finding under key that
// carries no disposition; an unreadable file holds none.
func hasUndisposedFinding(path, key string) bool {
	role, itemID, found := strings.Cut(key, ":")
	if !found {
		return false
	}
	rows, ok := readNDJSONRows(path)
	if !ok {
		return false
	}
	for _, row := range rows {
		if AuditRowRole(row) == role && rowString(row, "item_id") == itemID && !AuditRowIsPass(row) && !rowDisposed(row) {
			return true
		}
	}
	return false
}

// resumeBriefCommand is a unit's brief as tp resume prints it, the spec named
// relative to the repository root. An audit-fix unit whose row a task audit
// graded names that task audit's round file the same way, found through the
// spec's real path, so the brief does not depend on the working directory.
func resumeBriefCommand(start, specPath, shown string, u *NextUnit) string {
	if u.Kind == UnitAuditFix {
		if file := TaskAuditFixFile(specPath, u.ID); file != "" {
			return auditFixBrief(shown, repoRelative(start, file), u.ID)
		}
	}
	return u.Kind.BriefCommand(UnitTarget{Spec: shown, ID: u.ID})
}

// auditFixBrief is the brief of an audit-fix unit whose row a task audit
// graded: the audit's status, then the round file the fix is claimed in.
func auditFixBrief(spec, file, key string) string {
	return fmt.Sprintf("tp audit %s --status && echo 'fix %s, graded in %s, then claim it: tp audit %s --resolve %s fixed \"<evidence>\"'",
		spec, key, file, file, key)
}
