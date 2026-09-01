package cli

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

// filterReviewPrompts reduces an emission to the named role, or returns it
// unchanged when the flag is absent or names something this invocation does not
// emit. Extracted from runReview because inlining it pushed that function over
// the cognitive-complexity ratchet, which is the ratchet working: a filter with
// its own loop and branch is a second idea living in a function that already
// assembles a payload.
func filterReviewPrompts(prompts []reviewPrompt, name string) []reviewPrompt {
	if name == "" {
		return prompts
	}
	emitted := make([]string, 0, len(prompts))
	for i := range prompts {
		emitted = append(emitted, prompts[i].Role)
	}
	if idx := selectRoleIndex(emitted, name); idx >= 0 {
		return prompts[idx : idx+1]
	}
	return prompts
}

// filterAuditPrompts is filterReviewPrompts for the audit payload. The two are
// separate because tp review and tp audit carry different prompt structs over
// the same role names; selectRoleIndex is the shared decision.
func filterAuditPrompts(prompts []auditPrompt, name string) []auditPrompt {
	if name == "" {
		return prompts
	}
	emitted := make([]string, 0, len(prompts))
	for i := range prompts {
		emitted = append(emitted, prompts[i].Role)
	}
	if idx := selectRoleIndex(emitted, name); idx >= 0 {
		return prompts[idx : idx+1]
	}
	return prompts
}
