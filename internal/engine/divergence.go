package engine

import (
	"fmt"
	"sort"
)

// DivergenceHint is §2.6's constant, emitted verbatim as the `hint` field of
// every divergence object. It names the operator rather than issuing a bare
// imperative because the reader is usually an agent and accepting open findings
// is a user-approved decision; the action it asks for — surface the divergence
// and stop — is one that reader can take.
//
// The constant lives here rather than at the emitting call site so the code,
// skills/tp/REFERENCE.md and the guard tests all quote one string.
const DivergenceHint = "spec-coverage is the only role that measures spec conformance; " +
	"the remaining findings are outside it. Whether they gate this release is the operator's " +
	"decision, not the agent's — surface it rather than deciding either way; audit convergence " +
	"still counts every non-PASS row."

// Divergence is §2.4's divergence object: the report that spec conformance has
// been clean long enough to converge while findings from the other lenses are
// still open, and that whether those gate the release is the operator's call.
//
// None of the five fields carries omitempty and none uses absence to mean zero:
// an emitted object always carries all five, OpenRoles as an emitted [] when
// every open row is unattributed and UnattributedOpen as 0 when none are. The
// object itself is read by presence and the fields positionally, which is why
// ComputeAuditDivergence returns nil — an omitted key — rather than a zero
// object when the conditions do not hold.
type Divergence struct {
	OtherRolesOpen   int      `json:"other_roles_open"`
	OpenRoles        []string `json:"open_roles"`
	UnattributedOpen int      `json:"unattributed_open"`
	Message          string   `json:"message"`
	Hint             string   `json:"hint"`
}

// DivergenceInputs carries §2.4's five conditions' inputs under names that read
// as the conditions themselves at the call site. Every value is one the audit
// output already computes, so the object is never derived from a second source
// that could disagree with the payload it is emitted beside. It is passed by
// pointer and is neither retained nor mutated.
type DivergenceInputs struct {
	// Rounds is the recorded audit rounds, oldest first, as
	// ReviewState.AuditRounds stores them. Only the latest round's stored
	// RolesHash is read, for condition 5; an empty slice fails that condition
	// as it has already failed condition 1.
	Rounds []ReviewRound

	// LatestRows is the latest recorded round's rows exactly as
	// ComputeAuditRoleStreaks handed them back — nil when that round
	// contributes none. Condition 2 and all three counted fields are computed
	// from it, so the roleless rows the streaks cannot express are still
	// visible here, and the round's file is not read a second time.
	LatestRows []map[string]any

	// SpecCoverageCleanRounds is SpecCoverageCleanRounds' result over the same
	// walk: condition 1's left-hand side, nil when the latest round measured no
	// conformance at all.
	SpecCoverageCleanRounds *int

	// RequiredCleanRounds is the effective audit_clean_rounds, condition 1's
	// right-hand side. It is the resolved value, never a literal 2, and it is
	// never used as a message count.
	RequiredCleanRounds int

	// Stale is condition 3: the same value the output reports as `stale`,
	// engine.StateStale over these rounds and the current spec hash. It is
	// false by construction on --record.
	Stale bool

	// Converged is condition 4: the same value the output reports as
	// `converged`. It is what keeps the hint from being printed beside an
	// already-open gate, including at a resolved audit_clean_rounds of 0 where
	// engine.Converged reduces to "not stale".
	Converged bool

	// CurrentRolesHash is condition 5's right-hand side: the auditor corpus
	// hash computed now on --status — the one that output already computes for
	// roles_stale — and, on --record, the hash stamped on the round being
	// recorded, read back rather than recomputed. The equality therefore holds
	// by construction on --record, so condition 5 constrains --status alone.
	// An unreadable corpus hashes to "", which no non-empty stored hash equals.
	CurrentRolesHash string
}

// ComputeAuditDivergence builds §2.4's divergence object, or returns nil when
// any of the five conditions fails. nil is the omitted key: the object is never
// emitted as JSON null.
//
// The conditions are:
//
//  1. SpecCoverageCleanRounds is non-nil and at least RequiredCleanRounds.
//  2. The latest recorded round holds at least one non-PASS row not attributed
//     to spec-coverage, including any row carrying no role.
//  3. The spec is not stale.
//  4. The sequence is not converged.
//  5. The latest recorded round's stored roles_hash equals CurrentRolesHash.
//
// Condition 4 is load-bearing, not redundant. Under audit_converge_on: all,
// condition 2's rows also make the latest round unclean, so the two conditions
// coincide and only one of them appears to do any work; under blocking they
// come apart — a round holding open rows that are all advisory is stamped clean
// and can converge the sequence, and condition 4 is then the only thing
// withholding the object (§6.5). It is also the object's own premise:
// divergence hands the driver a decision about whether open findings should
// gate a release, and once the gate is open there is no such decision. The same
// condition is what makes the signal immune to a resolved audit_clean_rounds of
// 0, where condition 1 is satisfied by a streak of 0 and engine.Converged
// reduces to "not stale" — a state in which the object would otherwise be
// emitted beside converged: true and a hint that misdescribes an open gate.
//
// What the suppression costs is §6.5's, and §7 rows 21 and 22 pin it: an open
// row carrying no role may be spec-coverage's, and under blocking the
// unattributed_open caveat is withheld along with the object.
//
// The evaluation order below is not observable — nothing here reads a file,
// emits an advisory or otherwise has an effect — so the four scalar conditions
// are decided first and the row walk runs only when they all hold.
func ComputeAuditDivergence(in *DivergenceInputs) *Divergence {
	// Condition 1.
	if in.SpecCoverageCleanRounds == nil || *in.SpecCoverageCleanRounds < in.RequiredCleanRounds {
		return nil
	}
	// Conditions 3 and 4.
	if in.Stale || in.Converged {
		return nil
	}
	// Condition 5. An empty Rounds has already failed condition 1, so the
	// guard here only keeps the index safe.
	if len(in.Rounds) == 0 || in.Rounds[len(in.Rounds)-1].RolesHash != in.CurrentRolesHash {
		return nil
	}

	// Condition 2, and with it all three counted fields. A row carrying no role
	// is counted in OtherRolesOpen because it cannot be shown to belong to
	// another role — but neither can it be shown to belong to spec-coverage, so
	// UnattributedOpen records it and the message carries it inline.
	openRoles := make([]string, 0, len(in.LatestRows))
	seen := make(map[string]struct{}, len(in.LatestRows))
	otherRolesOpen, unattributedOpen := 0, 0
	for _, row := range in.LatestRows {
		if AuditRowIsPass(row) {
			continue
		}
		role := AuditRowRole(row)
		if role == RoleSpecCoverage {
			continue
		}
		otherRolesOpen++
		if role == "" {
			unattributedOpen++
			continue
		}
		if _, dup := seen[role]; dup {
			continue
		}
		seen[role] = struct{}{}
		openRoles = append(openRoles, role)
	}
	if otherRolesOpen == 0 {
		return nil
	}
	// The same ascending byte order §2.2 sorts role_streaks by, so an id whose
	// leading byte is uppercase keeps its position in both.
	sort.Strings(openRoles)

	return &Divergence{
		OtherRolesOpen:   otherRolesOpen,
		OpenRoles:        openRoles,
		UnattributedOpen: unattributedOpen,
		Message: divergenceMessage(
			*in.SpecCoverageCleanRounds, otherRolesOpen, unattributedOpen),
		Hint: DivergenceHint,
	}
}

// divergenceMessage renders §2.4's three message forms. The round count is
// always the spec-coverage streak and never the threshold it was compared
// against, and the finding count is always other_roles_open; only those two
// nouns inflect.
//
// The third form exists because the first clause of the other two positively
// asserts the findings are open *from other roles*, which is false exactly when
// no role holds any — the state in which open_roles is [].
func divergenceMessage(cleanRounds, otherRolesOpen, unattributedOpen int) string {
	rounds := countedNoun(cleanRounds, "round")
	findings := countedNoun(otherRolesOpen, "finding")
	switch unattributedOpen {
	case 0:
		return fmt.Sprintf("spec-coverage clean %d %s; %d %s open from other roles",
			cleanRounds, rounds, otherRolesOpen, findings)
	case otherRolesOpen:
		return fmt.Sprintf(
			"spec-coverage clean %d %s; %d %s open, none attributed to a role (possibly spec-coverage's)",
			cleanRounds, rounds, otherRolesOpen, findings)
	default:
		return fmt.Sprintf(
			"spec-coverage clean %d %s; %d %s open from other roles (including %d with no role, which may be spec-coverage's)",
			cleanRounds, rounds, otherRolesOpen, findings, unattributedOpen)
	}
}

// countedNoun agrees a regular noun with the number written in front of it.
func countedNoun(n int, singular string) string {
	if n == 1 {
		return singular
	}
	return singular + "s"
}
