package cli_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// legalRoleMode is one mode §4.2.2 calls legal for --role, with the role name
// its own emission produces.
type legalRoleMode struct {
	args []string
	role string
}

// TestInstructionIsASubsetInEveryModeRoleIsLegalIn is property 12 quantified
// over §4.2.2's legal set rather than over the default mode alone.
//
// The default mode is the only one whose key is addressed to a caller holding
// the whole panel -- it directs a merge, a record, a repeat, and an ordering
// against a regression prompt, none of which a single-prompt payload has the
// inputs for. The single-prompt modes were measured and their keys already
// speak to one prompt: "Feed findings back into spec revision", "append the
// plan to the spec", "run a full review round" are all things the holder of one
// prompt's output can do. So the property holds there by the key being
// unchanged, and that is asserted rather than assumed.
//
// Quantifying over the modes is what makes the property a property. A test
// scoped to the default mode would pass while a future edit taught a
// perspective emitter to talk about "the three role prompts".
func TestInstructionIsASubsetInEveryModeRoleIsLegalIn(t *testing.T) {
	spec := relocatedSpec(t, "spec/0.36.0.md")
	dir := filepath.Dir(spec)
	base := filepath.Base(spec)
	findings := writeOneFinding(t, dir)

	modes := map[string]legalRoleMode{
		"default":       {nil, ""},
		"regression":    {[]string{"--perspective", "regression"}, ""},
		"code-audit":    {[]string{"--perspective", "code-audit", "--affected-files", base}, ""},
		"documentation": {[]string{"--perspective", "documentation", "--docs-path", "."}, ""},
		"testing":       {[]string{"--perspective", "testing", "--test-path", "."}, ""},
		"verify":        {[]string{"--verify", "--findings", findings}, ""},
	}

	for name, m := range modes {
		t.Run(name, func(t *testing.T) {
			full := emitPayload(t, spec, m.args...)
			_, order := promptsOf(t, full)
			require.NotEmpty(t, order, "%s must emit a prompt for --role to select", name)

			// The role is read from the mode's own emission, never written
			// down: --perspective testing emits `test-planner`, which is in no
			// corpus, and a hard-coded name would rot silently.
			narrow := emitPayload(t, spec, append(append([]string{}, m.args...), "--role", order[0])...)

			fullText := instructionOf(t, full)
			narrowText := instructionOf(t, narrow)

			fullSentences := sentencesOf(fullText)
			for _, s := range sentencesOf(narrowText) {
				assert.Contains(t, fullSentences, s,
					"%s: every sentence under --role appears verbatim in the unrestricted key", name)
			}

			for _, d := range orchestratorDirectives {
				assert.NotContains(t, narrowText, d,
					"%s: the narrowed key carries no orchestrator directive (%q)", name, d)
			}

			if name == "default" {
				assert.NotEqual(t, fullText, narrowText,
					"the default key is the panel-addressed one, so --role must shorten it")
				return
			}
			// Every other mode already emits a one-prompt key, so the subset is
			// the whole. Asserting equality here is what would catch a future
			// edit that started narrowing them without a rule saying to.
			assert.Equal(t, fullText, narrowText,
				"%s already addresses a one-prompt payload, so --role leaves its key alone", name)
		})
	}
}

// TestAnEmptyPayloadDirectsNothing is the experiment the test above could not
// run, and audit round 2 said so in as many words: it builds the narrowed
// payload with `order[0]` — the name the mode emits anyway — so it never
// constructs an empty prompts[], and property 12 was only ever asked the
// question it passes.
//
// Making prompts: [] reachable in the five single-prompt modes is what created
// the case. Before that repair those modes ignored --role, so an empty payload
// could not happen and their "Spawn a sub-agent with this prompt" was true.
// Afterwards it was a directive over nothing.
//
// The foreign role is what constructs the case: a name tp recognises that this
// mode cannot emit. Asking for the mode's own name proves nothing here.
func TestAnEmptyPayloadDirectsNothing(t *testing.T) {
	dir, cases := legalModeCases(t)

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			stdout, stderr, code := runTPIn(t, dir,
				append(c.args, "--role", c.foreignRole, "--json")...)
			require.Equal(t, 0, code, "%s: a recognised name is not an error; stderr: %s", name, stderr)

			var payload struct {
				Prompts    []json.RawMessage `json:"prompts"`
				ReviewLoop *struct {
					Instruction string `json:"instruction"`
				} `json:"review_loop"`
			}
			require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
			require.Empty(t, payload.Prompts,
				"%s: the case this test exists for is an EMPTY payload", name)

			if payload.ReviewLoop == nil {
				return // tp audit emits no review_loop; nothing to direct.
			}
			assert.Empty(t, payload.ReviewLoop.Instruction,
				"%s: a payload with no prompt can support no directive, so the key is empty", name)
		})
	}
}

// TestCompactKeepsTheReasonWhenRoleEmptiesThePayload pins §8.4's own criterion
// applied to §4.2.1's exit-0-empty case: an empty payload flips skipped_roles
// from explanatory to decision-critical, so --compact keeps it there and only
// there.
//
// The fixture is a ROUND-1 spec and the role is `regression`, and both halves
// matter. Audit round 4 found the first version of this test asserting only
// that the skipped_roles key was PRESENT, on a fixture whose value was `[]` —
// it used a cross-phase name, which §4.2.1 says is in no entry at all. A
// mutation that kept the empty box and threw away every reason passed it, and
// that is precisely the regression the exception exists to prevent. At round 1
// `regression` is skipped `no-baseline`, so the array carries a reason and the
// assertion has something to be about.
//
// That was the fifth property this cycle lost to a guard that never built its
// own case; it is asserted here on the reason, not on the box.
func TestCompactKeepsTheReasonWhenRoleEmptiesThePayload(t *testing.T) {
	// No .tp-review alongside it: round 1 is the only round that skips a role
	// in this repository, and a skipped role is what carries a reason.
	spec := relocatedSpecAtRoundOne(t, "spec/0.36.0.md")
	dir := filepath.Dir(spec)
	base := filepath.Base(spec)

	read := func(args ...string) (reasons map[string]string, present bool, prompts int) {
		t.Helper()
		stdout, stderr, code := runTPIn(t, dir, args...)
		require.Equal(t, 0, code, "stderr: %s", stderr)
		var payload struct {
			Prompts []json.RawMessage `json:"prompts"`
			Skipped *[]struct {
				Role   string `json:"role"`
				Reason string `json:"reason"`
			} `json:"skipped_roles"`
		}
		require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
		reasons = map[string]string{}
		if payload.Skipped != nil {
			for _, sr := range *payload.Skipped {
				reasons[sr.Role] = sr.Reason
			}
		}
		return reasons, payload.Skipped != nil, len(payload.Prompts)
	}

	// The fixture must actually skip `regression` with a reason, or the
	// assertion below is about an empty array again.
	full, _, _ := read("review", base, "--json")
	require.Equal(t, engine.SkipNoBaseline, full["regression"],
		"round 1 skips regression with a reason; without one this test measures a box")

	emptyReasons, emptyPresent, emptyPrompts := read("review", base, "--compact", "--role", "regression", "--json")
	require.Zero(t, emptyPrompts, "the case this test exists for is an empty payload")
	require.True(t, emptyPresent, "--compact keeps skipped_roles when --role emptied the payload")
	assert.Equal(t, engine.SkipNoBaseline, emptyReasons["regression"],
		"and keeps the REASON, not an empty box: the reason is the payload here, not commentary on it")

	_, fullPresent, fullPrompts := read("review", base, "--compact", "--role", "implementer", "--json")
	require.Equal(t, 1, fullPrompts)
	assert.False(t, fullPresent,
		"--compact still omits skipped_roles when there is a prompt to explain: §8.4 is not repealed")

	_, plainPresent, plainPrompts := read("review", base, "--compact", "--json")
	require.Positive(t, plainPrompts)
	assert.False(t, plainPresent, "--compact without --role is untouched")
}
