package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const missingCmd = "definitely-not-a-cmd-xyz"

func TestUnresolvedGateHead(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "run.sh"), []byte("#!/bin/sh\n"), 0o600))
	require.NoError(t, os.Chmod(filepath.Join(dir, "run.sh"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plain.sh"), []byte("#!/bin/sh\n"), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o755))

	for _, tc := range []struct {
		gate, want string
	}{
		{"", ""},
		{"   ", ""},
		{"true", ""},
		{"exit 3", ""},
		{"cd sub && make", ""},
		{"FOO=1 BAR=2 true", ""},
		{"(true)", ""},
		{"$GATE", ""},
		{"./run.sh && true", ""},
		{"true 2>&1 | " + missingCmd, ""}, // only the first segment is resolved
		{missingCmd + " && true", missingCmd},
		{"FOO=1 " + missingCmd, missingCmd},
		{`"` + missingCmd + `" --flag`, missingCmd},
		{"./plain.sh", "./plain.sh"},        // exists, not executable: exit 126 at close
		{"./absent.sh", "./absent.sh"},      // does not exist: exit 127 at close
		{"./sub", "./sub"},                  // a directory is not a command
		{missingCmd + "; true", missingCmd}, // ; separates segments too
	} {
		assert.Equal(t, tc.want, UnresolvedGateHead(tc.gate, dir), "gate %q", tc.gate)
	}
}

func TestGateMissingCommand(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	assert.Equal(t, missingCmd, GateMissingCommand("true && "+missingCmd, dir, nil),
		"a later segment's head is named when the first resolves")
	assert.Equal(t, "go", GateMissingCommand("true", dir, []string{"ok", "./check.sh: line 3: go: command not found"}),
		"with every head resolving, the command sh named in the output is used")
	assert.Equal(t, "./gate.sh", GateMissingCommand("true", dir, []string{"sh: 1: ./gate.sh: Permission denied"}))
	assert.Empty(t, GateMissingCommand("true", dir, []string{"exit status 127"}),
		"nothing to name is reported as nothing, not guessed")
}
