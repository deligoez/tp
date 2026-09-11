package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// Role filtering for `tp review --role` and `tp audit --role` (v0.36.0 §4.2).
//
// The flag reduces an emission to one role's prompt. The match set is what the
// same invocation would emit with the flag removed — not the corpus, and not
// the active role set for the phase. Those differ: the built-in `regression`
// role is emitted and belongs to no corpus, and `--perspective testing` emits
// `test-planner`, which is in neither. Deriving the set from the emission is
// the only definition that holds in every mode the flag is legal in.

// selectRoleIndex returns the position of name in roles, or -1 when the
// emission does not carry it.
//
// The caller slices its own prompt type rather than this function doing it,
// because tp review and tp audit carry different prompt structs over the same
// role names. Keeping the decision here and the slicing there is what lets one
// rule serve both commands without coupling their payloads.
func selectRoleIndex(roles []string, name string) int {
	for i, role := range roles {
		if role == name {
			return i
		}
	}
	return -1
}

// roleQuery is everything §4.2.1 needs to classify a name: what the caller
// typed, whether they typed it at all, and where tp looks the name up.
//
// `given` is separate from a non-empty `name` because `--role ""` is refused as
// an unknown role rather than treated as absent — "flag given empty" and "flag
// absent" are never the same command.
type roleQuery struct {
	name    string
	given   bool
	specDir string
	domain  string
	// siblings holds the roles a tp run driver runs as this unit's sibling
	// units, and is nil by hand (withSiblingUnits).
	siblings map[string]bool
}

// withSiblingUnits records the panel's active roles as the ones a driver
// covers with sibling units, when a driver runs this invocation (TP_RUN_ID
// set). By hand it returns q unchanged.
//
// The driver partitions the panel across role units on purpose -- one unit
// per active role, resolved by the same engine.ResolveRolePanel -- so naming
// the siblings under role-filter in every unit's brief is per-unit noise. What
// a driven unit cannot learn from any sibling is a prompt NO unit covers, which
// today is review's built-in regression role: it is emitted and belongs to no
// corpus. filterByRole lists only those.
func (q roleQuery) withSiblingUnits(panel *rolePanel) roleQuery {
	if os.Getenv(engine.EnvRunID) == "" {
		return q
	}
	q.siblings = make(map[string]bool, len(panel.roles))
	for i := range panel.roles {
		q.siblings[panel.roles[i].ID] = true
	}
	return q
}

// roleQueryFor builds the query from a spec path, so every emitting mode
// classifies a name the same way.
//
// It exists because five of tp review's seven emitting modes used to bypass the
// filter entirely: --verify and the four --perspective values return before the
// default path reaches it, so `--role` was accepted and silently discarded
// there. §4.2.2 calls those modes legal and §6.2 property 6 says the flag is
// accepted in every mode that emits prompts -- accepted-and-ignored is not
// accepted.
//
// All seven call sites use it, and that is worth stating because the first
// version of this comment claimed it while two of them still built the struct
// by hand, taking `domain` from panel.fm.Domain instead. Those were equal
// today, so it was latent drift rather than a defect -- but a comment that
// counts its own callers has to be able to count.
func roleQueryFor(specPath, name string, given bool) roleQuery {
	return roleQuery{
		name:    name,
		given:   given,
		specDir: filepath.Dir(specPath),
		domain:  engine.ParseFrontmatter(specPath).Domain,
	}
}

// roleOutcome is how §4.2.1 classifies the name a caller passed.
type roleOutcome int

const (
	// roleEmitted: the name is in the set this invocation would emit.
	roleEmitted roleOutcome = iota
	// roleSkipped: tp recognises the name, and this round does not emit it.
	// Exit 0 with an empty prompts[] — a unit's own brief carries this name,
	// and the unit set and the emitted set are computed by different filters,
	// so they legitimately diverge.
	roleSkipped
	// roleUnknown: tp recognises the name nowhere. Exit 2.
	roleUnknown
)

// classifyRole places a name in one of §4.2.1's three classes.
//
// The emitted set comes first because it is the only one that produces a
// prompt. Everything after it is recognition, which spans both phases and both
// corpora — a name this command never emits or skips can still be one the
// repository defines for the other phase.
func classifyRole(q roleQuery, emitted []string, skipped []engine.SkippedRole) (int, roleOutcome) {
	if idx := selectRoleIndex(emitted, q.name); idx >= 0 {
		return idx, roleEmitted
	}
	for i := range skipped {
		if skipped[i].Role == q.name {
			return -1, roleSkipped
		}
	}
	if engine.RoleIsRecognised(q.specDir, q.domain, q.name) {
		return -1, roleSkipped
	}
	return -1, roleUnknown
}

// unknownRoleHint names what the invocation would have emitted.
//
// The names, not a bare "unknown role": the emission the caller asked for is
// the one that refused, so without them they have to run the command again
// without the flag to find out what to type.
func unknownRoleHint(emitted []string, skipped []engine.SkippedRole) string {
	var b strings.Builder
	b.WriteString("this invocation emits: ")
	if len(emitted) == 0 {
		b.WriteString("(nothing)")
	} else {
		b.WriteString(strings.Join(emitted, ", "))
	}
	if len(skipped) > 0 {
		names := make([]string, 0, len(skipped))
		for i := range skipped {
			names = append(names, skipped[i].Role+" ("+skipped[i].Reason+")")
		}
		b.WriteString("; skipped this round: " + strings.Join(names, ", "))
	}
	return b.String()
}

// applyRoleFilter is the shared body of both commands' filters: it classifies
// the name and either returns the selected index, signals an empty emission, or
// exits 2. Each command slices its own prompt type from the index.
func applyRoleFilter(q roleQuery, emitted []string, skipped []engine.SkippedRole) (idx int, emit bool) {
	if !q.given {
		return -1, true
	}
	i, outcome := classifyRole(q, emitted, skipped)
	switch outcome {
	case roleEmitted:
		return i, true
	case roleSkipped:
		return -1, false
	default:
		output.Error(ExitUsage, "unknown role: "+q.name, unknownRoleHint(emitted, skipped))
		os.Exit(ExitUsage)
		return -1, false
	}
}

// filterReviewPrompts applies §4.2's rule to a review payload. It returns the
// kept prompts and skipped extended by every emitted role the filter narrowed
// away (filterByRole).
func filterReviewPrompts(prompts []reviewPrompt, q roleQuery, skipped []engine.SkippedRole) ([]reviewPrompt, []engine.SkippedRole) {
	return filterByRole(prompts, func(p *reviewPrompt) string { return p.Role }, q, skipped)
}

// filterAuditPrompts is filterReviewPrompts for the audit payload; the two
// commands carry different prompt structs over the same role names.
func filterAuditPrompts(prompts []auditPrompt, q roleQuery, skipped []engine.SkippedRole) ([]auditPrompt, []engine.SkippedRole) {
	return filterByRole(prompts, func(p *auditPrompt) string { return p.Role }, q, skipped)
}

// filterByRole is the body both filters share: classify the name against the
// emission, slice the prompt it selects, and name every other emitted prompt
// in skipped_roles with reason role-filter.
//
// The narrowed-away roles are appended AFTER classification, so classifyRole
// and unknownRoleHint see the skip list the round produced, never the filter's
// own entries. Without them a one-role payload reported `skipped_roles: []`
// for a round that emitted four prompts, regression among them, and a caller
// holding it could not tell it had been handed a slice of the panel. A role a
// sibling unit covers is left out when a driver runs this one (q.siblings).
func filterByRole[P any](prompts []P, roleOf func(*P) string, q roleQuery, skipped []engine.SkippedRole) ([]P, []engine.SkippedRole) {
	emitted := make([]string, 0, len(prompts))
	for i := range prompts {
		emitted = append(emitted, roleOf(&prompts[i]))
	}
	idx, emit := applyRoleFilter(q, emitted, skipped)
	if idx < 0 && emit {
		return prompts, skipped
	}
	dropped := make([]string, 0, len(emitted))
	for i, role := range emitted {
		if i != idx && !q.siblings[role] {
			dropped = append(dropped, role)
		}
	}
	skipped = append(skipped, engine.RoleFilterSkippedRoles(dropped)...)
	if idx < 0 {
		return []P{}, skipped
	}
	return prompts[idx : idx+1], skipped
}

// skippedRolesSurviveCompact answers §8.4's question for one emission: does
// this payload carry its skipped_roles?
//
// §8.4 omits the field under --compact because it is explanatory, and for a
// payload with prompts in it that is right. An empty --role payload flips which
// of the two it is. Audit round 3 measured `--compact --role
// <recognised-but-skipped>` returning {"prompts": [], "skipped_roles": null}
// with an empty instruction and an empty stderr -- zero bytes saying why --
// while the same call with a TYPO was diagnosed in full, so the correct
// invocation was the only one left in the dark. When --role empties the
// payload the reason stops being commentary on the payload and becomes the
// payload, which is §8.4's own criterion for surviving --compact.
//
// The same holds for a payload --role narrowed without emptying it: once the
// filter drops prompts, a non-empty skipped_roles is what says the payload is
// a slice of the round rather than all of it -- the role-filter entries, and
// the round's own skips beside them. Without --role nothing is narrowed and
// §8.4 applies unchanged.
//
// It lives here rather than inline in each command because both ask it, and
// because the condition is the kind that reads as a typo when it is spelled
// twice.
func skippedRolesSurviveCompact(roleGiven bool, prompts, skipped int) bool {
	return !IsCompact() || (roleGiven && (prompts == 0 || skipped > 0))
}
