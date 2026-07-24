package engine

import (
	"fmt"
	"sort"

	"github.com/deligoez/tp/internal/model"
)

// NextAction is the resume oracle's next-action object (§4.2): the literal next
// tp command (nil for the decompose and release phases, which are agent work
// with no single tp command), a one-line human summary, and the phase-pinned
// payload carrying the immediate work.
type NextAction struct {
	Command      *string        `json:"command"`
	BriefCommand *string        `json:"brief_command"`
	Summary      string         `json:"summary"`
	Payload      map[string]any `json:"payload"`
}

// ReadyTasks returns the open tasks whose dependencies are all done, ordered as
// tp next claims them: most dependents first, then smallest estimate, then id.
// It mirrors tp next's selection so a resume preview names the very task tp next
// will return.
func ReadyTasks(tf *model.TaskFile) []*model.Task {
	done := doneTaskIDs(tf)
	dependents := make(map[string]int)
	for i := range tf.Tasks {
		for _, dep := range tf.Tasks[i].DependsOn {
			dependents[dep]++
		}
	}
	ready := make([]*model.Task, 0)
	for i := range tf.Tasks {
		t := &tf.Tasks[i]
		if t.Status == model.StatusOpen && depsSatisfied(t, done) {
			ready = append(ready, t)
		}
	}
	sort.Slice(ready, func(i, j int) bool {
		di, dj := dependents[ready[i].ID], dependents[ready[j].ID]
		if di != dj {
			return di > dj
		}
		if ready[i].EstimateMinutes != ready[j].EstimateMinutes {
			return ready[i].EstimateMinutes < ready[j].EstimateMinutes
		}
		return ready[i].ID < ready[j].ID
	})
	return ready
}

// doneTaskIDs is the set of done task ids in tf.
func doneTaskIDs(tf *model.TaskFile) map[string]bool {
	done := make(map[string]bool)
	for i := range tf.Tasks {
		if tf.Tasks[i].Status == model.StatusDone {
			done[tf.Tasks[i].ID] = true
		}
	}
	return done
}

// depsSatisfied reports whether every dependency of t is done.
func depsSatisfied(t *model.Task, done map[string]bool) bool {
	for _, dep := range t.DependsOn {
		if !done[dep] {
			return false
		}
	}
	return true
}

// ImplementPreview returns the preview for the implement phase's payload.task:
// the first WIP task in file order (wip=true) if any, else the highest-priority
// ready task (wip=false). hasTask is false when no task is ready and only open
// blocked tasks remain — the no-ready-task case (§4.4).
func ImplementPreview(tf *model.TaskFile) (id string, wip, hasTask bool) {
	for i := range tf.Tasks {
		if tf.Tasks[i].Status == model.StatusWIP {
			return tf.Tasks[i].ID, true, true
		}
	}
	ready := ReadyTasks(tf)
	if len(ready) == 0 {
		return "", false, false
	}
	return ready[0].ID, false, true
}

// roundPayload computes {round, unresolved_findings} for a review/audit
// sequence: round is len(rounds)+1, and unresolved is the count of the last
// round's findings whose resolved.status is absent or is a value other than
// "wontfix" — 0 for the first round, which has no previous round (§4.4).
func roundPayload(specPath string, rounds []ReviewRound) (round, unresolved int) {
	round = len(rounds) + 1
	if len(rounds) == 0 {
		return round, 0
	}
	rows, found := LoadRoundRows(specPath, &rounds[len(rounds)-1])
	if !found {
		return round, 0
	}
	for _, r := range rows {
		if findingResolvedStatus(r) != "wontfix" {
			unresolved++
		}
	}
	return round, unresolved
}

// findingResolvedStatus returns a finding row's resolved.status, or "" when the
// row has no resolved object or no status string.
func findingResolvedStatus(row map[string]any) string {
	resolved, ok := row["resolved"].(map[string]any)
	if !ok {
		return ""
	}
	s, _ := resolved["status"].(string)
	return s
}

// reviewRoundsOf and auditRoundsOf read a phase's rounds from a possibly-nil
// review state.
func reviewRoundsOf(st *ReviewState) []ReviewRound {
	if st == nil {
		return nil
	}
	return st.ReviewRounds
}

func auditRoundsOf(st *ReviewState) []ReviewRound {
	if st == nil {
		return nil
	}
	return st.AuditRounds
}

// BuildNextAction assembles the next_action for a phase (§4.4): review and audit
// carry {round, unresolved_findings} and the tp review/audit command; implement
// carries {task, wip} and tp next; decompose and release carry {} and a null
// command.
// BuildNextAction assembles the next_action for a phase (§4.4): review and audit
// carry {round, unresolved_findings} and the tp review/audit command; implement
// carries {task, wip} and tp next; decompose and release carry {} and a null
// command. §9.3: every phase that has a tp command also carries brief_command —
// the exact command that emits the brief for that phase's action (tp next --brief
// for implement, tp review <spec> --round N for review, tp audit <spec> for audit),
// so an orchestrator following next_action reaches a full brief without knowing
// the phase. Decompose and release carry a null brief_command.
func BuildNextAction(phase, specPath string, tf *model.TaskFile, st *ReviewState) NextAction {
	cmd := func(s string) *string { return &s }
	switch phase {
	case PhaseReview:
		rounds := reviewRoundsOf(st)
		// §10.2: an interrupted round (snapshot written, round file absent)
		// points at completing + recording that round rather than starting a
		// new one.
		if inFlight := InFlightRound(specPath, PhaseReview, len(rounds)); inFlight > 0 {
			return NextAction{
				Command:      cmd(fmt.Sprintf("tp review %s --record <findings-round-%d.ndjson>", specPath, inFlight)),
				BriefCommand: cmd(fmt.Sprintf("tp review %s --round %d", specPath, inFlight)),
				Summary:      fmt.Sprintf("record review round %d (its snapshot exists; the round was started but never recorded)", inFlight),
				Payload:      map[string]any{"action": "record-round", "round": inFlight},
			}
		}
		round, unresolved := roundPayload(specPath, rounds)
		return NextAction{
			Command:      cmd("tp review " + specPath),
			BriefCommand: cmd(fmt.Sprintf("tp review %s --round %d", specPath, round)),
			Summary:      fmt.Sprintf("run review round %d (%d unresolved from the previous round)", round, unresolved),
			Payload:      map[string]any{"round": round, "unresolved_findings": unresolved},
		}
	case PhaseAudit:
		rounds := auditRoundsOf(st)
		// §10.2: mirror — an interrupted audit round points at recording it.
		if inFlight := InFlightRound(specPath, PhaseAudit, len(rounds)); inFlight > 0 {
			return NextAction{
				Command:      cmd(fmt.Sprintf("tp audit %s --record <results-round-%d.ndjson>", specPath, inFlight)),
				BriefCommand: cmd("tp audit " + specPath),
				Summary:      fmt.Sprintf("record audit round %d (its snapshot exists; the round was started but never recorded)", inFlight),
				Payload:      map[string]any{"action": "record-round", "round": inFlight},
			}
		}
		round, unresolved := roundPayload(specPath, rounds)
		return NextAction{
			Command:      cmd("tp audit " + specPath),
			BriefCommand: cmd("tp audit " + specPath),
			Summary:      fmt.Sprintf("run audit round %d (%d unresolved from the previous round)", round, unresolved),
			Payload:      map[string]any{"round": round, "unresolved_findings": unresolved},
		}
	case PhaseImplement:
		id, wip, has := ImplementPreview(tf)
		var task any
		if has {
			task = map[string]any{"id": id}
		}
		return NextAction{
			Command:      cmd("tp next"),
			BriefCommand: cmd("tp next --brief"),
			Summary:      implementSummary(id, wip, has),
			Payload:      map[string]any{"task": task, "wip": wip},
		}
	case PhaseDecompose:
		return NextAction{Summary: "decompose the converged spec into tasks and tp import", Payload: map[string]any{}}
	case PhaseRelease:
		return NextAction{Summary: "audit converged; proceed to the human-approved release", Payload: map[string]any{}}
	default:
		return NextAction{Payload: map[string]any{}}
	}
}

// implementSummary glosses the implement next action.
func implementSummary(id string, wip, has bool) string {
	switch {
	case !has:
		return "no task is ready; open tasks remain blocked"
	case wip:
		return "resume the in-progress task " + id
	default:
		return "claim the next ready task " + id
	}
}
