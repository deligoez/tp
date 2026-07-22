package engine

import (
	"fmt"
	"sort"

	"github.com/deligoez/tp/internal/model"
)

// Blocker classes (§4.6): an agent-clearable blocker the agent resolves and
// re-runs tp resume; an escalate blocker the driver stops on and hands to a
// human.
const (
	ClassAgentClearable = "agent-clearable"
	ClassEscalate       = "escalate"
)

// Blocker is one entry in tp resume's blockers array (§4.6): a stable code slug,
// a class, a human-gloss message (dropped under --compact), and a data object
// carrying the same facts machine-readably.
type Blocker struct {
	Code    string         `json:"code"`
	Class   string         `json:"class"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
}

// BlockerInputs carries the pre-computed facts BuildBlockers needs, so blocker
// derivation stays pure and testable independent of disk reads.
type BlockerInputs struct {
	Phase           string
	SpecPath        string
	Changes         []string
	TaskFile        *model.TaskFile
	ReviewRounds    int
	ReviewMaxRounds int
	ReviewConverged bool
	AuditRounds     int
	AuditMaxRounds  int
	AuditConverged  bool
	ReviewStale     bool
}

// BuildBlockers derives the blockers array in the fixed code order of §4.6:
// unexplained-changes, no-ready-task, review-budget-exhausted,
// audit-budget-exhausted, spec-stale. Multiple blockers may coexist. The result
// is never nil (an empty slice when the agent may run next_action directly).
func BuildBlockers(in *BlockerInputs) []Blocker {
	blockers := make([]Blocker, 0)

	// unexplained-changes — agent-clearable: uncommitted changes not on the keep-list.
	if len(in.Changes) > 0 {
		blockers = append(blockers, Blocker{
			Code:    "unexplained-changes",
			Class:   ClassAgentClearable,
			Message: fmt.Sprintf("%d uncommitted change(s) are not on the keep-list; commit them or record them with tp keep", len(in.Changes)),
			Data:    map[string]any{"count": len(in.Changes)},
		})
	}

	// no-ready-task — escalate: implement with open tasks but none ready (payload.task is null).
	if in.Phase == PhaseImplement {
		if _, _, hasTask := ImplementPreview(in.TaskFile); !hasTask && hasOpenTasks(in.TaskFile) {
			blockers = append(blockers, Blocker{
				Code:    "no-ready-task",
				Class:   ClassEscalate,
				Message: "no task is ready; open tasks are blocked by unfinished dependencies",
				Data:    map[string]any{"blocked_by": unmetDeps(in.TaskFile)},
			})
		}
	}

	// review-budget-exhausted — escalate: review capped without convergence (cap 0 never fires).
	if in.Phase == PhaseReview && in.ReviewMaxRounds != 0 && in.ReviewRounds >= in.ReviewMaxRounds && !in.ReviewConverged {
		blockers = append(blockers, Blocker{
			Code:    "review-budget-exhausted",
			Class:   ClassEscalate,
			Message: fmt.Sprintf("review reached its %d-round cap without converging; raising the cap is a user decision", in.ReviewMaxRounds),
			Data:    map[string]any{"cap": in.ReviewMaxRounds},
		})
	}

	// audit-budget-exhausted — escalate: audit capped without convergence (cap 0 never fires).
	if in.Phase == PhaseAudit && in.AuditMaxRounds != 0 && in.AuditRounds >= in.AuditMaxRounds && !in.AuditConverged {
		blockers = append(blockers, Blocker{
			Code:    "audit-budget-exhausted",
			Class:   ClassEscalate,
			Message: fmt.Sprintf("audit reached its %d-round cap without converging; raising the cap is a user decision", in.AuditMaxRounds),
			Data:    map[string]any{"cap": in.AuditMaxRounds},
		})
	}

	// spec-stale — escalate: the spec changed since the last review round while implementing/auditing.
	if in.ReviewStale && (in.Phase == PhaseImplement || in.Phase == PhaseAudit) {
		blockers = append(blockers, Blocker{
			Code:    "spec-stale",
			Class:   ClassEscalate,
			Message: "the spec changed after the last recorded review round; reconcile it before continuing",
			Data:    map[string]any{"spec": in.SpecPath},
		})
	}

	return blockers
}

// unmetDeps returns the sorted, de-duplicated dependency ids that are not done
// and block at least one open task — the dependencies holding up progress.
func unmetDeps(tf *model.TaskFile) []string {
	done := doneTaskIDs(tf)
	set := make(map[string]bool)
	for i := range tf.Tasks {
		if tf.Tasks[i].Status != model.StatusOpen {
			continue
		}
		for _, dep := range tf.Tasks[i].DependsOn {
			if !done[dep] {
				set[dep] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// hasOpenTasks reports whether any task in tf is open.
func hasOpenTasks(tf *model.TaskFile) bool {
	for i := range tf.Tasks {
		if tf.Tasks[i].Status == model.StatusOpen {
			return true
		}
	}
	return false
}
