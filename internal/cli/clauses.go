package cli

import "strings"

// The two clauses v0.36.0 §2 and §3 append to every emitted role prompt.
//
// They are constants rather than composed strings because §6.2 property 1
// asserts the emitted body's suffix byte for byte: a version assembled from
// fragments would let a reworded fragment pass while the prompt changed.
//
// Both are single lines with no embedded newline. The spec gives them as fenced
// blocks for exactly that reason — a hard-wrapped blockquote left an
// implementer three transcription choices (join with a space, join with \n,
// keep or strip the "> " prefixes), each producing a different artefact and
// each passing a loose reading of "verbatim".
const (
	// isolationClause is §2.2: the working-tree constraint the interactive path
	// has no fence for, carved out so a unit can still run tp itself.
	isolationClause = "Do not edit any file in the working tree. Read anything, and run tp itself freely — its own state writes are expected. Write no other file except the output file this prompt names. If proving a defect would require changing code, report it with its evidence instead of making the edit."

	// incrementalClause is §3.2: write each row as it is decided, because a
	// unit that buffers and dies loses the whole round.
	incrementalClause = "Write each row to the output file as you decide it, not once at the end. A run that dies with its rows unwritten loses the whole round; a partially written file is still usable."
)

// clauseSuffix returns what §2.3 appends to an emitted role prompt: a blank
// line, §2.2's clause, a blank line, §3.2's clause, and no trailing newline.
//
// 468 bytes — 2 + 287 + 2 + 177. The net change to a body is one less, because
// §2.3 removes the body's trailing newline before appending; §1.1's table
// carries both figures and two drafts of the spec confused them in opposite
// directions, which is why the suffix is built here and the arithmetic is not
// repeated at the call site.
func clauseSuffix() string {
	return "\n\n" + isolationClause + "\n\n" + incrementalClause
}

// appendClausesReview puts §2.3's suffix on every review prompt that names an
// output file, and on no other.
//
// The predicate is the prompt's own output_path rather than its role: the same
// role carries a non-empty output_path in one mode and an empty one in another,
// so a rule written against role names would classify one prompt two ways.
//
// The trailing newline goes first. §1.1 calls that byte not optional — without
// the strip the body gains a blank line and the net change is 468 rather than
// the 467 the table derives.
func appendClausesReview(prompts []reviewPrompt) []reviewPrompt {
	suffix := clauseSuffix()
	for i := range prompts {
		if prompts[i].OutputPath == "" {
			continue
		}
		prompts[i].Prompt = strings.TrimSuffix(prompts[i].Prompt, "\n") + suffix
	}
	return prompts
}

// appendClausesAudit is appendClausesReview for the audit payload; the two
// commands carry different prompt structs over the same two fields.
func appendClausesAudit(prompts []auditPrompt) []auditPrompt {
	suffix := clauseSuffix()
	for i := range prompts {
		if prompts[i].OutputPath == "" {
			continue
		}
		prompts[i].Prompt = strings.TrimSuffix(prompts[i].Prompt, "\n") + suffix
	}
	return prompts
}
