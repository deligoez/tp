package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The project layer is outranked by a task file's own workflow block, so a
// write to .tp/config.json can be accepted, reported as updated, and have no
// effect. That is what a project hit when it added a step to its gate: the
// write answered {"updated":{...}} while the gate that ran stayed the one
// `tp init --quality-gate` had authored.
//
// tp warns rather than refuses, because the write is correct wherever no
// override exists — the ordinary case, and the one this repository is in. What
// was wrong was reporting success without saying the value is shadowed.

func TestSetProjectWorkflow_WarnsWhenTheWriteIsShadowed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("# Spec\n## 1. A\ncontent\n"), 0o600))

	_, stderr, code := runTP(t, dir, "init", "spec.md", "--quality-gate", "go test ./...")
	require.Equal(t, 0, code, "init: %s", stderr)

	_, stderr, code = runTP(t, dir, "set", "--workflow", "--project", "quality_gate=make check")
	require.Equal(t, 0, code, "the write itself is legitimate and must still succeed")

	assert.Contains(t, stderr, "shadowed",
		"a write that cannot take effect must say so")
	assert.Contains(t, stderr, "quality_gate",
		"the warning must name the field")
	assert.Contains(t, stderr, "override",
		"the warning must name the layer that wins")

	// And the warning is true: the resolved value is still the override's.
	stdout, _, code := runTP(t, dir, "config", "--resolved")
	require.Equal(t, 0, code)
	assert.Contains(t, stdout, "go test ./...",
		"the shadowed value is what actually applies")
}

func TestSetProjectWorkflow_SilentWhenTheWriteResolves(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("# Spec\n## 1. A\ncontent\n"), 0o600))

	_, stderr, code := runTP(t, dir, "init", "spec.md", "--quality-gate", "go test ./...")
	require.Equal(t, 0, code, "init: %s", stderr)

	// review_max_rounds is not in the task file's workflow block, so the
	// project write is the layer that wins and there is nothing to warn about.
	_, stderr, code = runTP(t, dir, "set", "--workflow", "--project", "review_max_rounds=9")
	require.Equal(t, 0, code)
	assert.NotContains(t, stderr, "shadowed",
		"warning on a write that does take effect would be noise on every unshadowed field")
}
