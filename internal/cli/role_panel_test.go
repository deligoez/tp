package cli_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// emitAuditPayload is emitPayload for the other command.
//
// It passes --affected-files because a clean tree makes `tp audit <spec>` exit
// 4 with "no changed files detected" -- it does not pass vacuously, it does not
// run at all, and a release tree is clean exactly when this gate fires. The
// spec itself is the affected file: every spec routes to spec-coverage, so the
// panel is non-empty whatever else the tree holds.
func emitAuditPayload(t *testing.T, spec string, args ...string) map[string]any {
	t.Helper()
	dir := filepath.Dir(spec)
	base := filepath.Base(spec)
	call := append([]string{"audit", base, "--affected-files", base, "--json"}, args...)
	stdout, stderr, code := runTPIn(t, dir, call...)
	require.Equal(t, 0, code, "audit emission failed: %s", stderr)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload), "emission must be JSON")
	return payload
}

// panelEmitter is one command's pair of emissions: the unrestricted payload and
// the same invocation narrowed to a single role.
//
// The two commands carry different prompt structs and different required flags,
// so property 7 is stated once here over a function pair rather than written
// twice with the audit half quietly weaker.
type panelEmitter struct {
	name string
	full func(t *testing.T, spec string) map[string]any
	one  func(t *testing.T, spec, role string) map[string]any
}

func panelEmitters() []panelEmitter {
	return []panelEmitter{
		{
			name: "review",
			full: func(t *testing.T, spec string) map[string]any { return emitPayload(t, spec) },
			one: func(t *testing.T, spec, role string) map[string]any {
				return emitPayload(t, spec, "--role", role)
			},
		},
		{
			name: "audit",
			full: func(t *testing.T, spec string) map[string]any { return emitAuditPayload(t, spec) },
			one: func(t *testing.T, spec, role string) map[string]any {
				return emitAuditPayload(t, spec, "--role", role)
			},
		},
	}
}

// TestPerRoleEmissionsConcatenateToTheWholePanel is v0.36.0 §6.2 property 7:
// the per-role emissions, concatenated in the UNRESTRICTED payload's order,
// equal that payload's prompts[] -- for both commands.
//
// This is the release's own gate (§7), and it is written as whole-panel
// equality rather than as a per-role spot check for a reason this repository
// paid for: every mechanism in v0.36.0 hides, regroups or narrows what a
// reviewer sees, which is the shape that cost v0.34.0 §7.1 eight rounds of
// suppressed findings. A check that samples one role cannot see a panel that
// silently lost one.
//
// Order comes from the payload, never from the sequence of invocations, which
// the caller controls -- a test that concatenated in call order would pass
// against an implementation that reordered the panel.
func TestPerRoleEmissionsConcatenateToTheWholePanel(t *testing.T) {
	for _, e := range panelEmitters() {
		t.Run(e.name, func(t *testing.T) {
			spec := relocatedSpec(t, "spec/0.36.0.md")

			full := e.full(t, spec)
			want, order := promptsOf(t, full)
			require.NotEmpty(t, order, "the unrestricted emission must produce at least one prompt")

			got := make([]string, 0, len(order))
			for _, role := range order {
				single := e.one(t, spec, role)
				bodies, singleOrder := promptsOf(t, single)
				require.Equal(t, []string{role}, singleOrder,
					"--role %s emits exactly that role", role)
				got = append(got, bodies[role])
			}

			wantOrdered := make([]string, 0, len(order))
			for _, role := range order {
				wantOrdered = append(wantOrdered, want[role])
			}
			assert.Equal(t, wantOrdered, got,
				"the per-role emissions concatenated in the payload's order equal the payload")
		})
	}
}

// TestRoleSelectionLeavesEveryOtherTopLevelKeyUnchanged is property 4's second
// half: only prompts[] and review_loop.instruction may differ.
//
// review_loop is exempted at its `instruction` member rather than as a whole
// key, because property 12 governs that member alone and leaving the rest of
// review_loop unconstrained is what an earlier draft of the spec did.
func TestRoleSelectionLeavesEveryOtherTopLevelKeyUnchanged(t *testing.T) {
	spec := relocatedSpec(t, "spec/0.36.0.md")

	full := emitPayload(t, spec)
	_, order := promptsOf(t, full)
	require.NotEmpty(t, order)

	single := emitPayload(t, spec, "--role", order[0])

	require.ElementsMatch(t, keysOf(full), keysOf(single), "the key set is unchanged")
	for k, v := range full {
		if k == "prompts" {
			continue
		}
		if k == "review_loop" {
			assertLoopEqualExceptInstruction(t, v, single[k])
			continue
		}
		assert.Equal(t, v, single[k], "top-level key %q is unchanged by --role", k)
	}
}

// keysOf lives in eject_advisory_test.go; this file reuses it.

// assertLoopEqualExceptInstruction compares review_loop member by member,
// skipping `instruction`. Comparing the whole object would fail on a difference
// property 12 requires; skipping the whole key would let the other members
// drift unwatched.
func assertLoopEqualExceptInstruction(t *testing.T, full, single any) {
	t.Helper()
	fullLoop, ok := full.(map[string]any)
	if !ok {
		assert.Equal(t, full, single, "review_loop is unchanged when it is not an object")
		return
	}
	singleLoop, ok := single.(map[string]any)
	require.True(t, ok, "review_loop keeps its shape under --role")

	require.ElementsMatch(t, keysOf(fullLoop), keysOf(singleLoop), "review_loop's members are unchanged")
	for k, v := range fullLoop {
		if k == "instruction" {
			continue
		}
		assert.Equal(t, v, singleLoop[k], "review_loop.%s is unchanged by --role", k)
	}
}
