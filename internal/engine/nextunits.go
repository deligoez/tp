package engine

import (
	"maps"
	"path/filepath"
	"strconv"
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
func BuildNextUnits(root, taskFilePath, specPath, phase string, tf *model.TaskFile, st *ReviewState, blockers []Blocker) (units []NextUnit, round *int) {
	units = make([]NextUnit, 0)
	target := UnitTarget{TaskFile: taskFilePath, Spec: specPath}

	recorded, roundBased := recordedRounds(phase, st)
	if roundBased {
		round = collectingRound(recorded)
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
	case PhaseReview, PhaseAudit:
		units, round = roundBasedUnits(root, phase, recorded, target)
	}
	return soloIfNotConcurrent(units), round
}

// recordedRounds returns a phase's count of recorded rounds and whether it is
// round-based at all. The count is the pivot for everything below: round N
// recorded means round N's findings may still need disposing, and round N+1 is
// the one being collected.
func recordedRounds(phase string, st *ReviewState) (int, bool) {
	switch phase {
	case PhaseReview:
		return len(reviewRoundsOf(st)), true
	case PhaseAudit:
		return len(auditRoundsOf(st)), true
	}
	return 0, false
}

// roundBasedUnits derives a round-based phase's units and the round they belong
// to (§4.1). Two rounds are in play and durable state alone decides which owns
// the phase.
//
// A recorded round whose merged findings still hold work owns it: its
// review-resolve or audit-fix unit comes before another round is collected, and
// the reported round is that recorded one rather than the next. Otherwise the
// phase is collecting round recorded+1, and its units are the role panel minus
// every role whose findings file already satisfies §3.3's predicate — so a
// resumed round never re-runs a role that finished, and does re-run a role that
// left a malformed file rather than skipping it.
//
// The panel and the roles still pending are kept apart because they answer
// different questions: an empty panel is a corpus that could not be resolved or
// one wholly deactivated, while a non-empty panel with nothing pending is a
// collected round — the two cases §4.1 separates with its non-empty guard. Only
// the second gets the round's record unit; the first keeps returning nothing, so
// its emptiness stays a no-units stop a human sees.
func roundBasedUnits(root, phase string, recorded int, target UnitTarget) (units []NextUnit, round *int) {
	roleKind, corpusPhase := UnitReviewRole, PhaseReviewers
	if phase == PhaseAudit {
		roleKind, corpusPhase = UnitAuditRole, PhaseAuditors
	}

	if recorded > 0 {
		dir := RoundDir(root, target.TaskFile, phase, recorded)
		if unit, ok := dispositionUnit(phase, dir, target); ok {
			return []NextUnit{unit}, &recorded
		}
	}

	collecting := recorded + 1
	dir := RoundDir(root, target.TaskFile, phase, collecting)
	panel := roleUnits(roleKind, target.Spec, corpusPhase, target)
	pending := make([]NextUnit, 0, len(panel))
	for _, unit := range panel {
		if roleKind.DurableWrite(UnitTarget{RoundDir: dir, ID: unit.ID}) {
			continue
		}
		pending = append(pending, unit)
	}
	if len(panel) > 0 && len(pending) == 0 {
		if unit, ok := recordUnit(phase, collecting, target); ok {
			return []NextUnit{unit}, &collecting
		}
	}
	return pending, &collecting
}

// recordUnit returns the single review-record or audit-record unit a collected
// round still owes, and false when that round already has its recorded entry
// (§4.1). Its id is the round number, which is the record kinds' durable
// subject (§3.1.1) and what makes `(kind, id)` identify the unit.
//
// The suppressing condition is the round file's own presence rather than the
// negation of the kind's §3.3 durable write, and the two differ in exactly one
// direction. That predicate is the merged file AND the round file, so it is
// also false for a round already recorded whose merged file has since gone;
// emitting there would re-record a recorded round and inflate the very
// convergence count §3.4 says no stop reason may touch. The round file's
// absence implies the durable write's absence, so testing it alone is the
// conjunction of both — and the caller's non-empty panel guard is what keeps
// the emission off a round holding zero role files.
//
// It is deliberately not derived from the review state's round count: the
// writers put round files first (WithReviewStateLock), so a crash between the
// two leaves a round recorded that the count has not yet seen.
func recordUnit(phase string, round int, target UnitTarget) (NextUnit, bool) {
	if fileExists(roundFilePath(target.Spec, phase, round)) {
		return NextUnit{}, false
	}
	kind := UnitReviewRecord
	if phase == PhaseAudit {
		kind = UnitAuditRecord
	}
	return newNextUnit(kind, strconv.Itoa(round), target), true
}

// dispositionUnit returns the one resolve or fix unit a recorded round still
// owes, and false when it owes none (§4.1).
//
// The emission condition is the exact negation of the kind's own §3.3 durable
// write, read from the same file, so the oracle can never hand the driver a unit
// that is already complete. A merged file that is absent or holds an unparseable
// line yields no unit either: a round whose findings cannot be read is not a
// round with a finding to dispose, and inventing one would put the driver in a
// loop over a file nothing can fix.
//
// review-resolve is one unit for the whole spec and audit-fix is one unit per
// row, which is why only the audit side needs a row selector — the `role:item_id`
// key rowIsDisposed parses back.
func dispositionUnit(phase, roundDir string, target UnitTarget) (NextUnit, bool) {
	rows, ok := readNDJSONRows(MergedFindingsPath(roundDir))
	if !ok {
		return NextUnit{}, false
	}
	for _, row := range rows {
		if phase == PhaseAudit {
			if AuditRowIsPass(row) || rowDisposed(row) {
				continue
			}
			return newNextUnit(UnitAuditFix, auditRowKey(row), target), true
		}
		if !rowDisposed(row) {
			return newNextUnit(UnitReviewResolve, SpecBaseName(target.Spec), target), true
		}
	}
	return NextUnit{}, false
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
	// The id goes onto the target as well as the entry: UnitTarget carries an
	// ID field and BriefCommand reads it, so the role kinds can name their own
	// role (v0.36.0 §4.2.3). Passing it only alongside — as a draft of the spec
	// assumed BriefCommand was forced to do — is what left the flag unreachable.
	target.ID = id
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
	maps.Copy(payload, na.Payload)
	payload["unit"] = map[string]any{"kind": string(units[0].Kind), "id": units[0].ID}
	na.Payload = payload
	return na
}
