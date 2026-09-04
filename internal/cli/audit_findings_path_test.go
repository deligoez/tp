package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuditMissingFindingsFileFailsLoudly guards tp's convergence guarantee.
// readFindings answers os.IsNotExist with nil, and runAudit used to add no
// up-front stat, so a typo'd --findings path produced exit 0 with empty stderr
// and a checklist that silently carried ZERO finding entries: the round
// verified none of the review findings and stayed recordable as clean. A
// missing path must abort with tp's file exit code, naming the path — the way
// tp review rejects the same typo. Replaces TestAuditFindingsAbsentIsOptional,
// which asserted the silent-empty behavior.
func TestAuditMissingFindingsFileFailsLoudly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n## Table\n| Col |\n|-----|\n| a |\n"), 0o600))

	aPath := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(aPath, []byte("package main\n"), 0o600))

	missing := filepath.Join(dir, "typo-findings.ndjson")
	stdout, stderr, code := runTP(t, dir, "audit", specPath, "--affected-files", aPath, "--findings", missing)
	assert.Equal(t, 3, code, "a missing findings file is a file error, not an empty finding set")
	assert.Contains(t, stderr, "findings file not found", "stderr says what went wrong")
	assert.Contains(t, stderr, missing, "stderr names the path tp could not find")
	assert.NotContains(t, stdout, "\"checklist\"", "no checklist is emitted from findings tp never read")
}
