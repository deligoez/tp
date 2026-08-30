package engine

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resolve is the happy path in one line: the shape must resolve for the kind,
// and the caller asserts on what it resolved to.
func resolve(t *testing.T, raw string, kind UnitKind) RunnerSpec {
	t.Helper()
	spec, err := ResolveRunner(json.RawMessage(raw), kind)
	require.NoError(t, err, "runner %s resolves for %s", raw, kind)
	return spec
}

// A bare string is §3.2's first shape: a built-in template name. Absence — a nil
// value, an empty one, a literal null — is the same as the default template,
// which is what the workflow layer stores when no layer sets the field.
func TestResolveRunner_TemplateName(t *testing.T) {
	for _, raw := range []string{`"opencode"`, `  "opencode"  `} {
		spec := resolve(t, raw, UnitImplement)
		assert.Equal(t, "opencode", spec.Template, "a string is a built-in template name")
		assert.Nil(t, spec.Runner, "a template name resolves to no runner object")
	}

	for _, raw := range []string{``, `null`, string(DefaultRunner())} {
		spec := resolve(t, raw, UnitAuditRole)
		assert.Equal(t, RunnerDefault, spec.Template, "an absent runner is the built-in default template")
		assert.Nil(t, spec.Runner)
	}
}

// The second shape: an object carrying cmd, with the four §3.2 fields.
func TestResolveRunner_RunnerObject(t *testing.T) {
	spec := resolve(t, `{
		"cmd": "my-agent",
		"args": ["run", "{prompt}"],
		"env": {"MY_AGENT_MODEL": "small"},
		"spend_key": "usage.cost_usd"
	}`, UnitReviewRole)

	assert.Empty(t, spec.Template, "a runner object is not a template name")
	require.NotNil(t, spec.Runner)
	assert.Equal(t, "my-agent", spec.Runner.Cmd)
	assert.Equal(t, []string{"run", "{prompt}"}, spec.Runner.Args)
	assert.Equal(t, map[string]string{"MY_AGENT_MODEL": "small"}, spec.Runner.Env)
	assert.Equal(t, "usage.cost_usd", spec.Runner.SpendKey)
}

// The third shape: a map from unit kind to either of the other two, with
// default covering every kind it does not list. This is what lets an operator
// judge with a different model from the one that produced the work.
func TestResolveRunner_PerKindMap(t *testing.T) {
	const raw = `{
		"audit-role": "opencode",
		"implement": {"cmd": "my-agent", "args": ["go"]},
		"default": "claude"
	}`

	assert.Equal(t, "opencode", resolve(t, raw, UnitAuditRole).Template,
		"a listed kind dispatches to its own entry")

	implement := resolve(t, raw, UnitImplement)
	require.NotNil(t, implement.Runner, "a map value may itself be a runner object")
	assert.Equal(t, "my-agent", implement.Runner.Cmd)

	for _, kind := range []UnitKind{UnitReviewRole, UnitReviewRecord, UnitReviewResolve, UnitDecompose, UnitAuditRecord, UnitAuditFix} {
		assert.Equal(t, "claude", resolve(t, raw, kind).Template,
			"%s is unlisted, so it dispatches to default", kind)
	}
}

// The informative cases are the ambiguous ones: the two object shapes are told
// apart by their keys alone, so cmd's presence decides even when the object
// also carries a key that belongs to the other shape.
func TestResolveRunner_ObjectsAreToldApartByCmd(t *testing.T) {
	bare := resolve(t, `{"cmd": "my-agent"}`, UnitImplement)
	require.NotNil(t, bare.Runner, "an object carrying cmd is a runner")
	assert.Equal(t, "my-agent", bare.Runner.Cmd)

	// Carries cmd AND default: cmd decides, so this is a runner used for every
	// kind, not a map whose default is a runner.
	for _, kind := range []UnitKind{UnitImplement, UnitAuditRole} {
		spec := resolve(t, `{"cmd": "my-agent", "default": "claude"}`, kind)
		require.NotNil(t, spec.Runner, "cmd makes it a runner however else it is keyed")
		assert.Equal(t, "my-agent", spec.Runner.Cmd)
		assert.Empty(t, spec.Template, "the default key is not read as a template here")
	}

	// No cmd: anything else is a map, so this one dispatches every kind to its
	// default rather than being a runner with no executable.
	for _, kind := range []UnitKind{UnitImplement, UnitAuditRole} {
		spec := resolve(t, `{"default": "claude"}`, kind)
		assert.Equal(t, "claude", spec.Template, "an object without cmd is a per-kind map")
		assert.Nil(t, spec.Runner)
	}
}

// Test 56's two named usage errors, plus the shapes that are neither of the
// three. Each is a *RunnerShapeError, which the CLI maps to exit 2.
func TestResolveRunner_UsageErrors(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		message string
	}{
		{"map without default", `{"audit-role": "opencode"}`, "default"},
		{"empty object is a map without default", `{}`, "default"},
		{"runner object without cmd, in a map value", `{"default": {"args": ["go"]}}`, "cmd"},
		{"a map does not nest", `{"default": {"default": "claude"}}`, "cmd"},
		// The message, not just the word cmd: an object carrying the key is
		// read as a runner, so it must fail as one — under a discriminator that
		// stopped reading cmd these two would still name cmd, as an unknown map
		// key, and a laxer assertion could not tell the two apart.
		{"cmd present but null", `{"cmd": null}`, "needs a cmd to spawn"},
		{"cmd present but empty", `{"cmd": "  "}`, "needs a cmd to spawn"},
		{"unknown map key", `{"audit_role": "opencode", "default": "claude"}`, "unit kind"},
		{"map value of the wrong type", `{"audit-role": 42, "default": "claude"}`, "template name"},
		{"empty template name", `""`, "empty"},
		{"number", `42`, "template name"},
		{"array", `["claude"]`, "template name"},
		{"boolean", `true`, "template name"},
		{"malformed JSON", `{"cmd": `, "template name"},
		{"wrong-typed args", `{"cmd": "my-agent", "args": "go"}`, "runner object"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ResolveRunner(json.RawMessage(tc.raw), UnitImplement)

			var shapeErr *RunnerShapeError
			require.ErrorAs(t, err, &shapeErr, "%s is a usage error", tc.raw)
			assert.Contains(t, shapeErr.Error(), tc.message)
			assert.NotEmpty(t, shapeErr.Hint(), "a usage error carries an actionable hint")
		})
	}
}

// An invalid entry is rejected whatever kind is being resolved: an operator
// learns about the typo in the audit branch at run start, not three hours in
// when the first audit unit is spawned.
func TestResolveRunner_MapIsCheckedWholeNotJustTheSelectedKind(t *testing.T) {
	const raw = `{"audit-role": {"args": ["go"]}, "default": "claude"}`

	var shapeErr *RunnerShapeError
	require.ErrorAs(t, resolveErr(t, raw, UnitImplement), &shapeErr,
		"the unselected audit-role entry is still checked")
	assert.Contains(t, shapeErr.Error(), "audit-role", "the error names the entry at fault")
}

// The missing default is a property of the map, not of the kind that happens
// to be spawning: the case the shipped test could not see is a default-less map
// whose CURRENT kind is listed, which resolved with no error at all and left
// the run to discover the hole at the first unlisted kind — the very lateness
// whole-map validation exists to prevent. Both listed kinds are asserted, so
// the check cannot pass by being right about one of them.
func TestResolveRunner_MapWithoutDefaultFailsForAListedKindToo(t *testing.T) {
	const raw = `{"implement":"claude","review-role":"opencode"}`

	for _, kind := range []UnitKind{UnitImplement, UnitReviewRole} {
		var shapeErr *RunnerShapeError
		require.ErrorAs(t, resolveErr(t, raw, kind), &shapeErr,
			"%s is listed, but the map still has no default", kind)
		assert.Contains(t, shapeErr.Error(), runnerDefaultKey,
			"the error names the missing key")
		assert.Equal(t, runnerField, shapeErr.Field,
			"the map as a whole is at fault, not the entry for the current kind")
		assert.NotContains(t, shapeErr.Error(), string(kind),
			"a whole-map verdict does not depend on which kind asked")
	}
}

// resolveErr is the error half of resolve, for the cases that must fail.
func resolveErr(t *testing.T, raw string, kind UnitKind) error {
	t.Helper()
	_, err := ResolveRunner(json.RawMessage(raw), kind)
	return err
}
