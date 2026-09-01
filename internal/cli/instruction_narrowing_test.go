package cli_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// orchestratorDirectives is the fixture property 12 says this task owns: the
// sentences of review_loop.instruction that direct an action a single-prompt
// payload cannot support.
//
// It is a reading, not a computation. §6.3 records the enumerated *forbidden*
// list as a withdrawn construction — it measured short by one — so the LOAD-
// BEARING check on the narrowed key is the subset assertion, which cannot
// measure short: a sentence nobody thought of is dropped by default rather than
// kept by default.
//
// This list is used two ways, and an earlier version of this comment claimed
// only the first. Positively, it asserts what the UNRESTRICTED key still says,
// which is how "unchanged without --role" is checked with no cross-version
// baseline to compare against. Negatively, it spot-checks the narrowed key here
// and in instruction_modes_test.go — a redundant belt beside the subset braces,
// and sound only because it is redundant. Nothing rests on this list being
// complete.
var orchestratorDirectives = []string{
	"spawn a sub-agent via the Agent tool",
	"tp review --merge",
	"--record <findings.ndjson>",
	"--status --check exits 0",
}

// instructionOf returns review_loop.instruction from an emission.
func instructionOf(t *testing.T, payload map[string]any) string {
	t.Helper()
	loop, ok := payload["review_loop"].(map[string]any)
	require.True(t, ok, "the payload carries review_loop")
	s, ok := loop["instruction"].(string)
	require.True(t, ok, "review_loop.instruction is a string")
	return s
}

// sentencesOf splits an instruction the way property 12's "sentence-subset"
// reads it: on a period followed by a space, keeping the final sentence.
//
// The separator is period-plus-space rather than a bare period because the key
// carries file paths and version numbers — `spec/0.36.0.md`,
// `<findings.ndjson>` — whose dots are never followed by a space.
func sentencesOf(s string) []string {
	out := make([]string, 0, 8)
	for _, part := range strings.Split(s, ". ") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, strings.TrimSuffix(part, "."))
		}
	}
	return out
}

// TestUnrestrictedInstructionStillCarriesEveryOrchestratorDirective is property
// 12's second half: without --role the key is unchanged.
//
// It is asserted positively — the orchestrator directives are still there —
// because a cross-version baseline is not available to compare against (§6.3:
// a go test binary compiles one tree, so v0.35.2's emitter is not callable).
func TestUnrestrictedInstructionStillCarriesEveryOrchestratorDirective(t *testing.T) {
	spec := relocatedSpec(t, "spec/0.36.0.md")
	full := instructionOf(t, emitPayload(t, spec))

	for _, d := range orchestratorDirectives {
		assert.Contains(t, full, d,
			"the unrestricted instruction is unchanged and still directs the orchestrator to %q", d)
	}
}

// TestRoleInstructionIsASentenceSubsetDirectingNothingUnsupported is property
// 12's first half.
//
// Subset is asserted structurally rather than by naming the surviving
// sentences: every sentence of the narrowed form must appear verbatim among the
// unrestricted form's sentences. That is what makes "sentence-subset" a
// property instead of a list — a reworded sentence fails it even if it says
// something true.
func TestRoleInstructionIsASentenceSubsetDirectingNothingUnsupported(t *testing.T) {
	spec := relocatedSpec(t, "spec/0.36.0.md")

	full := emitPayload(t, spec)
	_, order := promptsOf(t, full)
	require.NotEmpty(t, order)

	fullText := instructionOf(t, full)
	narrowText := instructionOf(t, emitPayload(t, spec, "--role", order[0]))

	require.NotEqual(t, fullText, narrowText,
		"--role narrows the key; an unchanged key is the defect this property exists for")

	fullSentences := sentencesOf(fullText)
	for _, s := range sentencesOf(narrowText) {
		assert.Contains(t, fullSentences, s,
			"every sentence under --role appears verbatim in the unrestricted key")
	}

	for _, d := range orchestratorDirectives {
		assert.NotContains(t, narrowText, d,
			"the narrowed key directs no action a single-prompt payload cannot support: %q", d)
	}
}

// TestRoleInstructionKeepsWhatOnePromptCanAct is the other direction: on this
// invocation, narrowing that emptied the key would satisfy every assertion
// above while telling the unit nothing.
//
// "On this invocation" is doing work, and an earlier version of this comment
// left it out — it framed an emptied key as the defect the test exists to
// catch, which two later measurements falsified. Under --spec-inline the key is
// legitimately empty (the spec is in the payload, so there is no path to name),
// and for an empty prompts[] it is empty by rule (§4.2.3.1: a payload with no
// prompt supports no directive). What this test pins is narrower: with one
// prompt and an out-of-line spec, the sentence naming that spec survives.
func TestRoleInstructionKeepsWhatOnePromptCanAct(t *testing.T) {
	spec := relocatedSpec(t, "spec/0.36.0.md")

	full := emitPayload(t, spec)
	_, order := promptsOf(t, full)
	require.NotEmpty(t, order)

	narrowText := instructionOf(t, emitPayload(t, spec, "--role", order[0]))
	assert.NotEmpty(t, narrowText, "the narrowed key is not emptied")
	assert.Contains(t, narrowText, "Read the spec at",
		"a unit is still told where the spec is")
}
