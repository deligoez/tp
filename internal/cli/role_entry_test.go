package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// entriesOf returns each prompts[] element as its canonical JSON encoding,
// keyed by role.
//
// The whole ELEMENT, not its `prompt` member. Property 4 says the selected
// entry is byte-identical to that role's entry in the unrestricted invocation,
// and an entry carries more than its body: output_path, category,
// checklist_items and affected_files all travel with it. A comparison of bodies
// alone would pass an implementation that dropped output_path under --role,
// which is the field every other property in this release keys off.
func entriesOf(t *testing.T, payload map[string]any) map[string]string {
	t.Helper()
	raw, ok := payload["prompts"].([]any)
	require.True(t, ok, "payload must carry a prompts array")

	out := make(map[string]string, len(raw))
	for _, entry := range raw {
		p, isMap := entry.(map[string]any)
		require.True(t, isMap, "every prompts[] entry is an object")
		role, _ := p["role"].(string)
		encoded, err := json.Marshal(p)
		require.NoError(t, err)
		out[role] = string(encoded)
	}
	return out
}

// TestSelectedEntryIsByteIdenticalWholeEntry is v0.36.0 §6.2 property 4 at the
// granularity the property is written at, for both commands.
//
// The sibling tests in role_selection_test.go compare the prompt BODY; this
// compares the entry. They are not the same assertion: the body is one member
// of an object whose other members are what the driver and the emission
// properties read.
func TestSelectedEntryIsByteIdenticalWholeEntry(t *testing.T) {
	for _, e := range panelEmitters() {
		t.Run(e.name, func(t *testing.T) {
			spec := relocatedSpec(t, "spec/0.36.0.md")

			want := entriesOf(t, e.full(t, spec))
			require.NotEmpty(t, want, "the unrestricted emission must produce at least one entry")

			for role, entry := range want {
				got := entriesOf(t, e.one(t, spec, role))
				require.Len(t, got, 1, "--role %s reduces prompts[] to one entry", role)
				assert.Equal(t, entry, got[role],
					"%s --role %s emits that role's whole entry unchanged", e.name, role)
			}
		})
	}
}

// TestAuditRoleLeavesEveryOtherTopLevelKeyUnchanged is the audit half of
// property 4's second sentence. The review half lives in role_panel_test.go;
// the two payloads are assembled by different functions, so one standing in for
// the other would leave half the contract unmeasured.
func TestAuditRoleLeavesEveryOtherTopLevelKeyUnchanged(t *testing.T) {
	spec := relocatedSpec(t, "spec/0.36.0.md")

	full := emitAuditPayload(t, spec)
	_, order := promptsOf(t, full)
	require.NotEmpty(t, order)

	single := emitAuditPayload(t, spec, "--role", order[0])

	require.ElementsMatch(t, keysOf(full), keysOf(single), "the key set is unchanged")
	for k, v := range full {
		if k == "prompts" {
			continue
		}
		// tp audit emits no review_loop, so property 12's exception has no
		// subject here: every other key is compared whole.
		assert.Equal(t, v, single[k], "top-level key %q is unchanged by --role", k)
	}
}
