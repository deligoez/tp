package cli

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
