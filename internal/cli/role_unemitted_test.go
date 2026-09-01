package cli_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unemittedRecognised returns a name §4.2.1 recognises that this invocation does
// not emit, together with the reason the payload records for it.
//
// It is read from the payload rather than written down: which roles are skipped
// depends on the round, the corpus and the spec, so a hard-coded name would pin
// a fixture rather than the rule.
func unemittedRecognised(t *testing.T, payload map[string]any) (name, reason string) {
	t.Helper()
	rows, _ := payload["skipped_roles"].([]any)
	for _, r := range rows {
		row, ok := r.(map[string]any)
		if !ok {
			continue
		}
		id, _ := row["role"].(string)
		why, _ := row["reason"].(string)
		if id != "" {
			return id, why
		}
	}
	return "", ""
}

// TestRecognisedUnemittedRoleExitsZeroWithEmptyPrompts is §6 property 5's
// exit-0 half: a name tp recognises that the round does not emit is not an
// error.
//
// The case is real rather than hypothetical -- `engine.roleUnits` and emission
// are computed by different filters, so a unit is spawned for a role whose
// prompt is skipped, and under an exit-2 rule that unit's own first command
// would fail before it did any work.
func TestRecognisedUnemittedRoleExitsZeroWithEmptyPrompts(t *testing.T) {
	// Round 1: the only round that skips a role in this repository, so the
	// case the property describes is actually present.
	spec := relocatedSpecAtRoundOne(t, "spec/0.36.0.md")
	dir := filepath.Dir(spec)

	name, reason := unemittedRecognised(t, emitPayload(t, spec))
	require.NotEmpty(t, name, "round 1 skips regression with no-baseline; the case must be present")

	stdout, stderr, code := runTPIn(t, dir, "review", filepath.Base(spec), "--role", name)
	require.Equal(t, 0, code, "a recognised unemitted role is not an error; stderr: %s", stderr)

	var out struct {
		Prompts []json.RawMessage `json:"prompts"`
		Skipped []struct {
			Role   string `json:"role"`
			Reason string `json:"reason"`
		} `json:"skipped_roles"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Empty(t, out.Prompts, "the role emits no prompt")
	assert.NotNil(t, out.Prompts, "prompts stays an array, never null")

	// The role's own entry is present, carrying the reason tp already computes.
	var got string
	for _, s := range out.Skipped {
		if s.Role == name {
			got = s.Reason
		}
	}
	assert.Equal(t, reason, got, "the payload echoes %q's own skipped_roles reason", name)
}

// TestRoleInTheOtherPhasesCorpusIsRecognised pins the half of §4.2.1's
// definition that "emitted plus skipped_roles" cannot reach: recognition spans
// the corpus for *either* phase.
//
// tp review never emits or skips an auditor id, so under the narrower rule an
// auditor's own `tp review <spec> --role <id>` would exit 2 -- a usage error for
// a name the repository's own corpus defines.
func TestRoleInTheOtherPhasesCorpusIsRecognised(t *testing.T) {
	spec := relocatedSpec(t, "spec/0.36.0.md")
	dir := filepath.Dir(spec)

	// spec-coverage is an auditor in every corpus, user or embedded, and is
	// never a reviewer.
	stdout, stderr, code := runTPIn(t, dir, "review", filepath.Base(spec), "--role", "spec-coverage")
	require.Equal(t, 0, code,
		"an auditor id is recognised by tp review even though review never emits it; stderr: %s", stderr)

	var out struct {
		Prompts []json.RawMessage `json:"prompts"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Empty(t, out.Prompts)

	// And symmetrically: a reviewer id is recognised by tp audit.
	_, stderr2, code2 := runTPIn(t, dir, "audit", filepath.Base(spec),
		"--affected-files", filepath.Base(spec), "--role", "implementer")
	assert.Equal(t, 0, code2,
		"a reviewer id is recognised by tp audit; stderr: %s", stderr2)
}

// TestEmptyRoleIsRefusedAsUnknown pins §4.2.1's last bullet: `--role ""` is an
// unknown role, not an absent flag, so "flag given empty" and "flag absent" are
// never the same command.
func TestEmptyRoleIsRefusedAsUnknown(t *testing.T) {
	spec := relocatedSpec(t, "spec/0.36.0.md")
	dir := filepath.Dir(spec)

	_, _, code := runTPIn(t, dir, "review", filepath.Base(spec), "--role", "")
	assert.Equal(t, 2, code, `--role "" is refused as an unknown role`)
}
