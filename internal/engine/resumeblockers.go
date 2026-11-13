package engine

import (
	"fmt"
	"path/filepath"
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

	// ReviewInFlightRound is the review round emitted and not recorded, 0
	// when none; ReviewInFlightOverOldText is whether its snapshot differs
	// from the spec, which refuses a new emission until it is recorded or
	// discarded. Both shape the spec-stale blocker's way out.
	ReviewInFlightRound       int
	ReviewInFlightOverOldText bool

	// ReviewBlockingFixedAtCap and AuditBlockingFixedAtCap count the latest
	// round's blocking findings marked fixed at the cap (LoopDone): the state
	// only the operator can end, named in the budget blocker's message.
	ReviewBlockingFixedAtCap int
	AuditBlockingFixedAtCap  int

	// ReviewCarriedBlocking is the review loop's carried blocking rows
	// (LoopDone.CarriedBlocking), and ReviewLatestRound the latest recorded
	// review round: a carried row from an earlier round is one no further
	// round can clear.
	ReviewCarriedBlocking []CarriedRow
	ReviewLatestRound     int

	// EpicBlock is the task file's epic block (EpicBlockFor), nil for an
	// unbound file or outside the implement phase.
	EpicBlock *EpicBlock
}

// BuildBlockers derives the blockers array in the fixed code order of §4.6:
// unexplained-changes, no-ready-task, epic-blocked, review-budget-exhausted,
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
		if _, _, hasTask := ImplementPreview(in.TaskFile); !hasTask && HasOpenTasks(in.TaskFile) {
			blockers = append(blockers, Blocker{
				Code:    "no-ready-task",
				Class:   ClassEscalate,
				Message: "no task is ready; open tasks are blocked by unfinished dependencies",
				Data:    map[string]any{"blocked_by": unmetDeps(in.TaskFile)},
			})
		}
	}

	// epic-blocked — escalate: implement on a file whose epic is blocked,
	// with no wip task to resume (spec/1.5.0.md §6).
	if b := epicBlockedBlocker(in); b != nil {
		blockers = append(blockers, *b)
	}

	// review-budget-exhausted — escalate: the cap is reached and a finding of the
	// latest round still carries no disposition (cap 0 never fires). ReviewConverged
	// is the loop verdict, so a review the cap ended does not fire it.
	if in.Phase == PhaseReview && in.ReviewMaxRounds != 0 && in.ReviewRounds >= in.ReviewMaxRounds && !in.ReviewConverged {
		blockers = append(blockers, Blocker{
			Code:    "review-budget-exhausted",
			Class:   ClassEscalate,
			Message: budgetBlockerMessage("review", in.ReviewMaxRounds, in.ReviewBlockingFixedAtCap),
			Data:    map[string]any{"cap": in.ReviewMaxRounds, "blocking_fixed_at_cap": in.ReviewBlockingFixedAtCap},
		})
	}

	// review-carried-blocking — escalate: a blocking row of a round before the
	// latest stands unresolved in the carried set, so its roles are told not
	// to re-report it and no further round can clear it.
	if b := carriedBlockingBlocker(in); b != nil {
		blockers = append(blockers, *b)
	}

	// audit-budget-exhausted — escalate: the same at the audit cap (cap 0 never fires).
	if in.Phase == PhaseAudit && in.AuditMaxRounds != 0 && in.AuditRounds >= in.AuditMaxRounds && !in.AuditConverged {
		blockers = append(blockers, Blocker{
			Code:    "audit-budget-exhausted",
			Class:   ClassEscalate,
			Message: budgetBlockerMessage("audit", in.AuditMaxRounds, in.AuditBlockingFixedAtCap),
			Data:    map[string]any{"cap": in.AuditMaxRounds, "blocking_fixed_at_cap": in.AuditBlockingFixedAtCap},
		})
	}

	// spec-stale — escalate: the spec changed since the last review round while
	// implementing/auditing.
	if in.ReviewStale && (in.Phase == PhaseImplement || in.Phase == PhaseAudit) {
		blockers = append(blockers, specStaleBlocker(in))
	}

	return blockers
}

// specStaleBlocker names the way out as data (spec/backlog/reconcile.md §2.2):
// commands, in order, is what clears the blocker, with <file> left literal as
// in the review loop's own next_action. With no round in flight that is a
// review round over the new text, emitted then recorded. With one in flight
// a new emission is refused while its snapshot differs from the spec, so the
// first command is that round's record; over the old text a round over the new
// one follows, and the round's discard is the operator's decision, named under
// operator and never as a command to run. An emission the review cap refuses
// is never a command either: with no round in flight it moves under operator
// as raise-review-cap, and after a record that reaches the cap it is dropped.
func specStaleBlocker(in *BlockerInputs) Blocker {
	emit := "tp review " + in.SpecPath
	record := emit + " --record <file>"
	b := Blocker{
		Code:  "spec-stale",
		Class: ClassEscalate,
		Message: fmt.Sprintf("the spec changed after the last recorded review round; run a review round over the new text: %s, then %s",
			emit, record),
		Data: map[string]any{"spec": in.SpecPath, "commands": []string{emit, record}},
	}
	round := in.ReviewInFlightRound
	if round == 0 {
		if in.reviewCapReached(in.ReviewRounds) {
			atReviewCap(in, &b, emit, record)
		}
		return b
	}
	b.Data["in_flight_round"] = round
	if !in.ReviewInFlightOverOldText {
		b.Message = fmt.Sprintf("the spec changed after the last recorded review round, and review round %d is in flight over the current text; record it: %s",
			round, record)
		b.Data["commands"] = []string{record}
		return b
	}
	b.Message = fmt.Sprintf("the spec changed after the last recorded review round, and review round %d is in flight over the old text, so a new emission is refused; record it first: %s, then run a round over the new text: %s, then %s. Discarding it instead is the operator's decision (%s): %s --force",
		round, record, emit, record, EscalateDiscardEmission, emit)
	b.Data["commands"] = []string{record, emit, record}
	if in.reviewCapReached(in.ReviewRounds + 1) {
		b.Message = fmt.Sprintf("the spec changed after the last recorded review round, and review round %d is in flight over the old text, so a new emission is refused; record it: %s. Its record reaches review's %d-round cap, so no round over the new text follows it; discarding it instead is the operator's decision (%s): %s --force",
			round, record, in.ReviewMaxRounds, EscalateDiscardEmission, emit)
		b.Data["commands"] = []string{record}
	}
	b.Data["operator"] = map[string]any{
		"decision": EscalateDiscardEmission,
		"commands": []string{emit + " --force", record},
	}
	return b
}

// reviewCapReached reports whether recorded review rounds reach the review
// cap, which refuses the next emission (exit 4) while the spec is stale: a
// stale spec is never converged. A cap of 0 is uncapped.
func (in *BlockerInputs) reviewCapReached(recorded int) bool {
	return in.ReviewMaxRounds > 0 && recorded >= in.ReviewMaxRounds
}

// atReviewCap rewords a spec-stale blocker with no round in flight once the
// review budget is spent: the emission exits 4, so it is no command for the
// agent, and raising the cap is the operator's decision (SKILL.md, "Gate,
// Budget & Escalation Policy").
func atReviewCap(in *BlockerInputs, b *Blocker, emit, record string) {
	b.Message = fmt.Sprintf("the spec changed after the last recorded review round, and review reached its %d-round cap, so a round over the new text is refused; raising the cap is the operator's decision (%s): then %s, then %s",
		in.ReviewMaxRounds, EscalateRaiseReviewCap, emit, record)
	b.Data["commands"] = []string{}
	b.Data["operator"] = map[string]any{
		"decision": EscalateRaiseReviewCap,
		"commands": []string{emit, record},
	}
}

// epicBlockedBlocker is the epic-blocked blocker, or nil: it stands in the
// implement phase while the file's epic is blocked and no task is wip. A wip
// task stays resumable, so its implement unit is offered as before; with none,
// no task can start, and only finishing the dependencies clears it. When
// every dependency is a condition no agent can, so the message and data
// name the operator's decision and record of each.
func epicBlockedBlocker(in *BlockerInputs) *Blocker {
	blk := in.EpicBlock
	if in.Phase != PhaseImplement || blk == nil || !blk.Blocks() {
		return nil
	}
	if _, wip, _ := ImplementPreview(in.TaskFile); wip {
		return nil
	}
	b := &Blocker{
		Code:    "epic-blocked",
		Class:   ClassEscalate,
		Message: fmt.Sprintf("no task can start: epic %s is blocked by %s; finish those epics first", blk.Epic, blk.Describe()),
		Data:    map[string]any{"epic": blk.Epic, "blocked_by_epics": blk.BlockedBy},
	}
	if len(blk.BlockedByAnyOf) > 0 {
		b.Data["blocked_by_any_of"] = blk.BlockedByAnyOf
	}
	if op := blk.OperatorAction(); op != nil {
		b.Message = fmt.Sprintf("no task can start: epic %s is blocked by the condition(s) %s, the operator's decision (%s): %s",
			blk.Epic, blk.Describe(), op.Decision, op.Steps())
		b.Data["operator"] = op
	}
	return b
}

// carriedBlockingBlocker is the review-carried-blocking blocker, or nil. It
// stands in the review phase while the carried set holds an unresolved
// blocking row recorded before the latest round: the latest round's own rows
// are the review-resolve unit's, and an earlier one no round will re-raise.
// Its message names the first row's command and the epoch; its data counts
// them all and lists the first CarriedRowsShown.
func carriedBlockingBlocker(in *BlockerInputs) *Blocker {
	if in.Phase != PhaseReview || in.ReviewConverged {
		return nil
	}
	earlier := make([]CarriedRow, 0, len(in.ReviewCarriedBlocking))
	for i := range in.ReviewCarriedBlocking {
		if row := in.ReviewCarriedBlocking[i]; row.Round < in.ReviewLatestRound {
			// Named through the spec as tp resume shows it, so the command
			// runs from where the payload's other commands do.
			row.File = filepath.Join(ReviewStateDir(in.SpecPath), filepath.Base(row.File))
			earlier = append(earlier, row)
		}
	}
	if len(earlier) == 0 {
		return nil
	}
	return &Blocker{
		Code:    "review-carried-blocking",
		Class:   ClassEscalate,
		Message: CarriedBlockingClause(earlier, in.SpecPath),
		Data: map[string]any{
			"count": len(earlier),
			"rows":  CarriedRowsJSON(earlier),
		},
	}
}

// budgetBlockerMessage words a *-budget-exhausted blocker: what is left at the
// cap, and — when blocking findings were marked fixed there — that only the
// operator can end it.
func budgetBlockerMessage(phase string, capRounds, blockingFixed int) string {
	if blockingFixed > 0 {
		return fmt.Sprintf("%s reached its %d-round cap; %s", phase, capRounds, blockingFixedAtCap(blockingFixed))
	}
	return fmt.Sprintf("%s reached its %d-round cap; disposition the remaining findings of the latest round (accepting a blocking one without a change is the operator's decision)", phase, capRounds)
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

// HasOpenTasks reports whether any task in tf is open: the only kind the epic
// block stops from starting, and what tells no-ready-task from all done.
func HasOpenTasks(tf *model.TaskFile) bool {
	for i := range tf.Tasks {
		if tf.Tasks[i].Status == model.StatusOpen {
			return true
		}
	}
	return false
}
