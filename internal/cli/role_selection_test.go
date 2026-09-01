package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// emitPayload runs one emission in the spec's own directory and returns the
// decoded payload. The working directory matters: the role corpus and the
// review state are both resolved from the spec's path.
func emitPayload(t *testing.T, spec string, args ...string) map[string]any {
	t.Helper()
	dir := filepath.Dir(spec)
	stdout, stderr, code := runTPIn(t, dir, append([]string{"review", filepath.Base(spec), "--json"}, args...)...)
	require.Equal(t, 0, code, "emission failed: %s", stderr)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload), "emission must be JSON")
	return payload
}

// promptsOf returns the payload's prompts as role-keyed bodies plus the order
// they were emitted in, which §6.2 property 7 compares against.
func promptsOf(t *testing.T, payload map[string]any) (bodies map[string]string, order []string) {
	t.Helper()
	raw, ok := payload["prompts"].([]any)
	require.True(t, ok, "payload must carry a prompts array")

	bodies = map[string]string{}
	order = make([]string, 0, len(raw))
	for _, entry := range raw {
		p, isMap := entry.(map[string]any)
		require.True(t, isMap, "every prompts[] entry is an object")
		role, _ := p["role"].(string)
		body, _ := p["prompt"].(string)
		bodies[role] = body
		order = append(order, role)
	}
	return bodies, order
}

// TestRoleSelectionIsByteIdenticalToTheUnrestrictedEntry pins v0.36.0 §6.2
// property 4: --role <name> emits exactly that role's entry, unchanged.
//
// The role under test is taken from the unrestricted emission rather than
// written here. A hard-coded name would pass while the panel silently changed
// under it, and this repository has already paid for one hand-written list that
// omitted the largest entry in the payload.
func TestRoleSelectionIsByteIdenticalToTheUnrestrictedEntry(t *testing.T) {
	spec := relocatedSpec(t, "spec/0.36.0.md")

	full := emitPayload(t, spec)
	bodies, order := promptsOf(t, full)
	require.NotEmpty(t, order, "the unrestricted emission must produce at least one prompt")

	for _, role := range order {
		t.Run(role, func(t *testing.T) {
			one := emitPayload(t, spec, "--role", role)
			selected, selectedOrder := promptsOf(t, one)

			require.Len(t, selectedOrder, 1, "--role %s must reduce prompts[] to one entry", role)
			assert.Equal(t, role, selectedOrder[0], "the surviving entry must be the role that was asked for")
			assert.Equal(t, bodies[role], selected[role],
				"--role %s must emit that role's prompt unchanged, byte for byte", role)
		})
	}
}

// TestAuditRoleSelectionIsByteIdentical is property 4's other half. It is a
// separate test rather than a subtest because tp audit needs its affected
// files named: on a clean tree it exits 4 with "no changed files detected", so
// a shared harness would have to branch on the command anyway.
func TestAuditRoleSelectionIsByteIdentical(t *testing.T) {
	spec := relocatedSpec(t, "spec/0.36.0.md")
	dir := filepath.Dir(spec)

	// A file to audit, inside the sandbox: --affected-files is what keeps the
	// emission independent of whatever the repository's working tree holds.
	subject := filepath.Join(dir, "subject.go")
	require.NoError(t, os.WriteFile(subject, []byte("package subject\n\nfunc F() int { return 1 }\n"), 0o600))

	emit := func(extra ...string) (map[string]string, []string) {
		t.Helper()
		args := append([]string{"audit", filepath.Base(spec), "--affected-files", "subject.go", "--json"}, extra...)
		stdout, stderr, code := runTPIn(t, dir, args...)
		require.Equal(t, 0, code, "audit emission failed: %s", stderr)
		var payload map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
		return promptsOf(t, payload)
	}

	bodies, order := emit()
	require.NotEmpty(t, order, "the unrestricted audit emission must produce at least one prompt")

	for _, role := range order {
		t.Run(role, func(t *testing.T) {
			selected, selectedOrder := emit("--role", role)
			require.Len(t, selectedOrder, 1, "tp audit --role %s must reduce prompts[] to one entry", role)
			assert.Equal(t, role, selectedOrder[0])
			assert.Equal(t, bodies[role], selected[role],
				"tp audit --role %s must emit that role's prompt unchanged, byte for byte", role)
		})
	}
}
