package engine

import (
	"path/filepath"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// NextUnit is one entry in tp resume's next_units array (§4.1): a unit's kind,
// the durable subject that identifies it within that kind (§3.1.1), and the
// command that emits its brief.
//
// Concurrency is deliberately not a field. §3.3's table fixes it per kind, and
// UnitKind.Concurrent() reads that table, so repeating it per entry would give
// it a second place to drift — the oracle instead guarantees the property the
// driver actually needs: a non-concurrent kind is never returned alongside
// another unit.
type NextUnit struct {
	Kind         UnitKind `json:"kind"`
	ID           string   `json:"id"`
	BriefCommand string   `json:"brief_command"`
}

// BuildNextUnits derives the units the driver should execute now and the round
// they belong to (§4.1).
//
// The round is the one being collected — the next unrecorded round of a
// round-based phase — and nil outside review and audit, which is what makes the
// driver leave TP_ROUND and TP_ROUND_DIR unset rather than empty (§3.1.1).
// It is a property of the phase, not of the returned slice, so a round-based
// phase still reports its round when it returns no unit.
//
// next_units is empty in exactly three cases (§4.1): the release phase, which is
// the releasable condition itself; a phase whose work is blocked; and a phase
// awaiting an operator decision. The latter two are precisely the escalate
// blocker class (§4.6) — raising a round cap, unblocking a dependency chain and
// reconciling a stale spec are all user decisions, so deriving emptiness from
// the blockers keeps the oracle from ever handing the driver a unit that cannot
// make progress.
//
// The returned slice is never nil: an empty next_units must serialize as [].
func BuildNextUnits(taskFilePath, specPath, phase string, tf *model.TaskFile, st *ReviewState, blockers []Blocker) (units []NextUnit, round *int) {
	units = make([]NextUnit, 0)
	target := UnitTarget{TaskFile: taskFilePath, Spec: specPath}

	switch phase {
	case PhaseReview:
		round = collectingRound(len(reviewRoundsOf(st)))
	case PhaseAudit:
		round = collectingRound(len(auditRoundsOf(st)))
	}

	if phase == PhaseRelease || hasEscalateBlocker(blockers) {
		return units, round
	}

	switch phase {
	case PhaseImplement:
		// The subject is the task tp next would claim — the in-progress task
		// when one exists, so a crashed unit is resumed rather than skipped.
		// No ready task is the no-ready-task blocker's condition and returns
		// nothing.
		id, _, has := ImplementPreview(tf)
		if has {
			units = append(units, newNextUnit(UnitImplement, id, target))
		}
	case PhaseDecompose:
		units = append(units, newNextUnit(UnitDecompose, SpecBaseName(specPath), target))
	case PhaseReview:
		units = append(units, roleUnits(UnitReviewRole, specPath, PhaseReviewers, target)...)
	case PhaseAudit:
		units = append(units, roleUnits(UnitAuditRole, specPath, PhaseAuditors, target)...)
	}
	return soloIfNotConcurrent(units), round
}

// collectingRound returns the round a phase is currently collecting given its
// count of recorded rounds: the next one. A round whose prompts were emitted but
// never recorded is that same number, so an interrupted round is resumed rather
// than skipped past.
func collectingRound(recorded int) *int {
	r := recorded + 1
	return &r
}

// hasEscalateBlocker reports whether any blocker is one only a human can clear.
func hasEscalateBlocker(blockers []Blocker) bool {
	for i := range blockers {
		if blockers[i].Class == ClassEscalate {
			return true
		}
	}
	return false
}

// SpecBaseName is the spec's base name without its extension — the durable
// subject of the two kinds whose work is the spec as a whole, decompose and
// review-resolve (§3.1.1).
func SpecBaseName(specPath string) string {
	base := filepath.Base(specPath)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// roleUnits returns one unit per active role of a corpus phase, in corpus order.
//
// The panel is resolved the way tp review and tp audit resolve it — the domain-
// filtered corpus, then the spec's frontmatter overrides, then its enabled:
// false drop — so the oracle names exactly the roles the phase would emit, which
// is what §3.3 means by the role count being "bounded by the number of active
// roles for the phase, which the corpus fixes". The warnings those resolvers
// return are advisory and belong to the emitting command, not to a read-only
// oracle; a malformed role file yields no unit at all, because a panel that
// cannot be resolved is a panel the driver must not guess at.
func roleUnits(kind UnitKind, specPath, corpusPhase string, target UnitTarget) []NextUnit {
	fm := ParseFrontmatter(specPath)
	roles, _, err := ResolveActiveCorpus(filepath.Dir(specPath), fm.Domain, corpusPhase)
	if err != nil {
		return nil
	}
	roles, _, disabled := ResolveOverrideFocus(roles, fm, corpusPhase)
	roles = DropDisabledRoles(roles, disabled)

	units := make([]NextUnit, 0, len(roles))
	for i := range roles {
		units = append(units, newNextUnit(kind, roles[i].ID, target))
	}
	return units
}

// newNextUnit builds one entry, taking the brief from the kind's own mapping so
// no command string is spelled twice.
func newNextUnit(kind UnitKind, id string, target UnitTarget) NextUnit {
	return NextUnit{Kind: kind, ID: id, BriefCommand: kind.BriefCommand(target)}
}

// soloIfNotConcurrent enforces §4.1's rule that a non-concurrent kind is never
// returned alongside another unit. It is a chokepoint rather than a property of
// each branch above: the invariant protects the shared files every non-role kind
// writes, and one place that can be read and tested is worth more than five
// branches that each happen to be right today.
func soloIfNotConcurrent(units []NextUnit) []NextUnit {
	for i := range units {
		if !units[i].Kind.Concurrent() {
			return units[:1]
		}
	}
	return units
}

// renderNextAction makes next_action a rendering of next_units[0] rather than a
// second opinion about what to do next (§4.1).
//
// The payload names the unit the driver will run — its kind and its id — so the
// human-facing object and the machine surface can be checked against each other
// instead of drifting apart. command, brief_command and summary keep their
// phase-level forms, which already address that same unit: they are the human
// half of the contract, and the round qualifier they carry for a reader lives on
// the machine surface as `round`.
func renderNextAction(na NextAction, units []NextUnit) NextAction {
	if len(units) == 0 {
		return na
	}
	payload := make(map[string]any, len(na.Payload)+1)
	for k, v := range na.Payload {
		payload[k] = v
	}
	payload["unit"] = map[string]any{"kind": string(units[0].Kind), "id": units[0].ID}
	na.Payload = payload
	return na
}
