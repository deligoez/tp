package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeLines writes a .go file of n distinct lines after the package clause.
func writeLines(t *testing.T, dir, name string, n int, tag string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("package main\n")
	for i := range n {
		fmt.Fprintf(&b, "// %s line %d\n", tag, i)
	}
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(b.String()), 0o600))
}

// affectedPaths returns one prompt's affected_files paths in emitted order.
func affectedPaths(t *testing.T, prompt map[string]any) []string {
	t.Helper()
	raw, ok := prompt["affected_files"].([]any)
	require.True(t, ok, "affected_files is present")
	out := make([]string, 0, len(raw))
	for _, e := range raw {
		out = append(out, e.(map[string]any)["path"].(string))
	}
	return out
}

// TestAuditCodeFiles_RankByChurnOverTheRealDiff: decision 1 measured through
// git. Twelve files change against the tag; their path order is the reverse
// of their churn order, so the shipped alphabetical cut keeps the ten
// least-changed and the churn cut keeps the ten most-changed.
func TestAuditCodeFiles_RankByChurnOverTheRealDiff(t *testing.T) {
	t.Parallel()
	dir, specPath := newAuditRepo(t)
	for i := range 12 {
		writeLines(t, dir, fmt.Sprintf("f%02d.go", i), 1, "base")
	}
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "base")
	git(t, dir, "tag", "v0")
	for i := range 12 {
		// f00 churns least, f11 most: path order is the reverse of churn order.
		writeLines(t, dir, fmt.Sprintf("f%02d.go", i), 2*(i+1), "next")
	}
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "change")

	stdout, stderr, code := runTP(t, dir, "audit", specPath)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	got := affectedPaths(t, auditPromptsByRole(t, stdout)["security"])
	want := []string{"f11.go", "f10.go", "f09.go", "f08.go", "f07.go", "f06.go", "f05.go", "f04.go", "f03.go", "f02.go"}
	assert.Equal(t, want, got, "the ten most-changed files, most first")
}

// TestAuditCodeFiles_RenameRanksByItsOwnChurn: a renamed file ranks by the
// edit it carries, under its new path, and its diff annotation is that edit.
// Before rename-aware numstat parsing the line read "old => new", matched no
// universe path, and the renamed file was annotated +0/-0.
func TestAuditCodeFiles_RenameRanksByItsOwnChurn(t *testing.T) {
	t.Parallel()
	dir, specPath := newAuditRepo(t)
	writeLines(t, dir, "big.go", 40, "body")
	writeLines(t, dir, "small.go", 1, "base")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "base")
	git(t, dir, "tag", "v0")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "pkg"), 0o755))
	git(t, dir, "mv", "big.go", filepath.Join("pkg", "moved.go"))
	f, err := os.OpenFile(filepath.Join(dir, "pkg", "moved.go"), os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	for i := range 10 {
		_, err = fmt.Fprintf(f, "// appended %d\n", i)
		require.NoError(t, err)
	}
	require.NoError(t, f.Close())
	writeLines(t, dir, "small.go", 6, "next")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "move and edit")

	stdout, stderr, code := runTP(t, dir, "audit", specPath)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	sec := auditPromptsByRole(t, stdout)["security"]
	assert.Equal(t, []string{"pkg/moved.go", "small.go"}, affectedPaths(t, sec),
		"the rename's own +10 outranks small.go's +6/-1, under its new path")
	assert.Contains(t, sec["prompt"].(string), "- pkg/moved.go (diff: +10/-0)\n", "the rename's own edit, not the whole file")
	assert.Contains(t, sec["prompt"].(string), "- small.go (diff: +6/-1)\n")
}
