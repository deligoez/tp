package cli

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsSpecLookingPath covers §4.1: the spec positional detector is the basis
// for --merge and --resolve/--resolve-all rejecting a spec-looking positional.
// Markdown extensions (.md, .markdown, case-insensitive) identify a spec; NDJSON
// and extensionless data files do not.
func TestIsSpecLookingPath(t *testing.T) {
	for _, p := range []string{
		"spec.md",
		"spec.markdown",
		"SPEC.MD",
		"Spec.Markdown",
		filepath.Join("dir", "notes.md"),
	} {
		assert.True(t, isSpecLookingPath(p), "expected spec-looking: %s", p)
	}
	for _, p := range []string{
		"findings.ndjson",
		"r1.json",
		"findings",
		"data.txt",
		"README",
	} {
		assert.False(t, isSpecLookingPath(p), "expected not spec-looking: %s", p)
	}
}
