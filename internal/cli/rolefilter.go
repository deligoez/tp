package cli

import (
	"os"
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
// prompt; skipped_roles is what separates a divergence the driver creates from
// a name the caller mistyped.
func classifyRole(name string, emitted []string, skipped []engine.SkippedRole) (int, roleOutcome) {
	if idx := selectRoleIndex(emitted, name); idx >= 0 {
		return idx, roleEmitted
	}
	for i := range skipped {
		if skipped[i].Role == name {
			return -1, roleSkipped
		}
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
func applyRoleFilter(name string, emitted []string, skipped []engine.SkippedRole) (idx int, emit bool) {
	if name == "" {
		return -1, true
	}
	i, outcome := classifyRole(name, emitted, skipped)
	switch outcome {
	case roleEmitted:
		return i, true
	case roleSkipped:
		return -1, false
	default:
		output.Error(ExitUsage, "unknown role: "+name, unknownRoleHint(emitted, skipped))
		os.Exit(ExitUsage)
		return -1, false
	}
}

// filterReviewPrompts applies §4.2's rule to a review payload.
func filterReviewPrompts(prompts []reviewPrompt, name string, skipped []engine.SkippedRole) []reviewPrompt {
	emitted := make([]string, 0, len(prompts))
	for i := range prompts {
		emitted = append(emitted, prompts[i].Role)
	}
	idx, emit := applyRoleFilter(name, emitted, skipped)
	switch {
	case idx >= 0:
		return prompts[idx : idx+1]
	case !emit:
		return []reviewPrompt{}
	default:
		return prompts
	}
}

// filterAuditPrompts is filterReviewPrompts for the audit payload; the two
// commands carry different prompt structs over the same role names.
func filterAuditPrompts(prompts []auditPrompt, name string, skipped []engine.SkippedRole) []auditPrompt {
	emitted := make([]string, 0, len(prompts))
	for i := range prompts {
		emitted = append(emitted, prompts[i].Role)
	}
	idx, emit := applyRoleFilter(name, emitted, skipped)
	switch {
	case idx >= 0:
		return prompts[idx : idx+1]
	case !emit:
		return []auditPrompt{}
	default:
		return prompts
	}
}
