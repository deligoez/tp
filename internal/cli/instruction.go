package cli

import "strings"

// Narrowing review_loop.instruction for a single-prompt payload (§4.2.3.1).
//
// The key is addressed to a caller holding the whole panel — it directs the
// reader to spawn a sub-agent per prompt, merge, record the round, order the
// regression prompt against the others, and run an uncounted delta pass between
// rounds. Under --role the payload holds one prompt and no regression prompt,
// so those sentences direct actions that payload cannot support.
//
// The rule the spec states is a property, not an edit: no sentence may direct
// an action the caller's own payload cannot support, and the result is a
// SENTENCE-SUBSET of the unrestricted key. Building it by subtraction is what
// §6.3 records as withdrawn — an enumerated forbidden list measured short by
// one. So this filters by what a single-prompt caller CAN act on, and a
// sentence nobody anticipated is dropped rather than kept.

// unitActionableSentence reports whether a sentence directs something a caller
// holding one prompt can do.
//
// Two survive today. "Read the spec at <path>" is the one thing this key tells
// a unit that its own prompt does not; the --no-state notice reports state
// rather than directing an action, and dropping a true statement about the
// round would be a second, unasked change.
//
// An earlier version of this comment said "narrowing to nothing would leave the
// unit worse off than narrowing at all". Audit round 2 falsified it in a
// reachable combination: under --spec-inline the "Read the spec at" sentence is
// never built, so the subset is empty. That is the RIGHT answer there — the
// spec is already in the payload, so there is no path to tell the unit about —
// and it is the right answer for an empty prompts[] too. The empty set is a
// sentence-subset, and inventing a sentence to avoid it would break the rule
// the narrowing exists to keep.
func unitActionableSentence(s string) bool {
	switch {
	case strings.HasPrefix(s, "Read the spec at "):
		return true
	case strings.HasPrefix(s, "Convergence is not being recorded"):
		return true
	default:
		return false
	}
}

// instructionForPayload returns the instruction a --role payload should carry.
//
// An EMPTY payload gets an empty key. Audit round 2 found the gap: making
// prompts: [] reachable in the five single-prompt modes left their instruction
// literals untouched, so a payload with nothing in it still read "Spawn a
// sub-agent with this prompt". §4.2.3.1 forbids exactly that — no sentence may
// direct an action the caller's own payload cannot support, and an empty
// payload supports none of them.
//
// The empty string rather than a new sentence, because the rule is
// sentence-SUBSET: the empty set is one, and any sentence written here would
// not be.
// It does NOT narrow. Narrowing is the default panel's business, and the
// caller applies it: the five single-prompt modes' keys already address a
// one-prompt payload ("Feed findings back into spec revision", "append the plan
// to the spec"), and running them through the subset filter emptied keys that
// were correct. Measured while making this change — the first version of this
// function narrowed unconditionally and blanked all five.
func instructionForPayload(instruction string, prompts int) string {
	if prompts == 0 {
		return ""
	}
	return instruction
}

// narrowInstructionForRole returns the sentence-subset of instruction that a
// single-prompt payload can act on.
//
// Splitting on period-plus-space rather than a bare period is deliberate: the
// key carries file paths and version numbers (`spec/0.36.0.md`,
// `<findings.ndjson>`) whose dots are never followed by a space, and splitting
// on those would cut a sentence in half and make the result no longer a subset.
//
// Rejoining with the same separator is what keeps the output a subset by
// construction rather than by inspection.
func narrowInstructionForRole(instruction string) string {
	parts := strings.Split(instruction, ". ")
	kept := make([]string, 0, len(parts))
	for i, part := range parts {
		sentence := strings.TrimSpace(part)
		// Only the last part carries the terminating period; trimming it
		// makes every sentence comparable, and it is put back on rejoin.
		if i == len(parts)-1 {
			sentence = strings.TrimSuffix(sentence, ".")
		}
		if sentence != "" && unitActionableSentence(sentence) {
			kept = append(kept, sentence)
		}
	}
	if len(kept) == 0 {
		return ""
	}
	return strings.Join(kept, ". ") + "."
}
