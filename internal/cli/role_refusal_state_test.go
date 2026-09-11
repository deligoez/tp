package cli_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// roleRefusalSpec is small on purpose: the refusal is decided on the role
// name, so the spec only has to be one both commands will emit against.
const roleRefusalSpec = "# Refusal\n\n## 1. Models\n\nA Task has a title and a status.\n\n## 2. API\n\nPOST /tasks creates a task.\n"

// roleRefusalArgs is the refused invocation for one command. Audit needs a
// file to audit, or it exits 4 on an empty diff before it reaches the role.
func roleRefusalArgs(cmd string) []string {
	args := []string{cmd, "spec.md", "--role", "bogus"}
	if cmd == "audit" {
		args = append(args, "--affected-files", "spec.md")
	}
	return args
}

// stateTreeBytes returns every file under dir's .tp-review/ with its bytes, or
// nil when the directory does not exist. Absence and emptiness are different
// answers, so the caller can tell "nothing was created" from "a directory was
// created and left empty".
func stateTreeBytes(t *testing.T, dir string) map[string]string {
	t.Helper()
	root := filepath.Join(dir, ".tp-review")
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil
	}
	tree := make(map[string]string)
	require.NoError(t, filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() {
			tree[rel+"/"] = ""
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		tree[rel] = string(data)
		return nil
	}))
	return tree
}

// TestUnknownRoleWritesNoStateOnAFreshSpec pins the refusal's write contract:
// an unknown --role exits 2 and leaves no state directory behind. Before the
// fix both commands wrote their round snapshot — and review its state.json —
// before the role filter ran, so a typo in --role opened a round no prompt ever
// came from.
func TestUnknownRoleWritesNoStateOnAFreshSpec(t *testing.T) {
	t.Parallel()
	for _, cmd := range []string{"review", "audit"} {
		t.Run(cmd, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(roleRefusalSpec), 0o600))

			_, stderr, code := runTPIn(t, dir, roleRefusalArgs(cmd)...)

			assert.Equal(t, 2, code, "an unknown role is a usage error; stderr: %s", stderr)
			assert.Contains(t, stderr, "unknown role: bogus")
			assert.Nil(t, stateTreeBytes(t, dir),
				"a refused --role must not create the state directory")
		})
	}
}

// TestUnknownRoleLeavesExistingStateByteIdentical covers the case absence
// cannot: a spec that already has state. Both phases emit round 1 first, then
// the spec is edited, so a refusal that still wrote its snapshot would replace
// snapshot-*-round-1.md with different bytes — re-emitting over an unchanged
// spec would rewrite identical bytes and prove nothing.
func TestUnknownRoleLeavesExistingStateByteIdentical(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte(roleRefusalSpec), 0o600))

	for _, cmd := range []string{"review", "audit"} {
		args := []string{cmd, "spec.md"}
		if cmd == "audit" {
			args = append(args, "--affected-files", "spec.md")
		}
		_, stderr, code := runTPIn(t, dir, args...)
		require.Equal(t, 0, code, "the seeding %s emission must succeed; stderr: %s", cmd, stderr)
	}
	require.NoError(t, os.WriteFile(specPath, []byte(roleRefusalSpec+"\n## 3. Later\n\nAn edit after round 1.\n"), 0o600))

	before := stateTreeBytes(t, dir)
	require.NotEmpty(t, before, "the seeding emissions must have written state for the comparison to mean anything")

	for _, cmd := range []string{"review", "audit"} {
		_, stderr, code := runTPIn(t, dir, roleRefusalArgs(cmd)...)
		assert.Equal(t, 2, code, "%s: an unknown role is a usage error; stderr: %s", cmd, stderr)
		assert.Equal(t, before, stateTreeBytes(t, dir),
			"%s: a refused --role must leave the state directory byte-identical", cmd)
	}
}
