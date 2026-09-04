package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEveryPromptWithAnOutputPathEndsWithTheSuffix is v0.36.0 §6.2 property 1,
// swept across both commands and the flag combinations that reach a different
// append point.
//
// The sibling test in clause_emission_test.go pins the *predicate* -- output
// path, not role -- on three emissions. This one pins the *position* across the
// matrix, because the append is applied at one place per command and every flag
// that reshapes the panel is a chance to bypass it: --role slices the payload
// after the append, --compact rewrites prompt fields, and audit assembles its
// prompts in a different function entirely.
//
// The suffix is rebuilt from the spec's own fenced blocks rather than from the
// implementation's constants, so a test asserting against the code it checks
// cannot pass by agreeing with a mistake.
func TestEveryPromptWithAnOutputPathEndsWithTheSuffix(t *testing.T) {
	t.Parallel()
	suffix := clauseSuffixFromSpec(t)
	spec := relocatedSpec(t, "spec/0.36.0.md")
	dir := filepath.Dir(spec)
	base := filepath.Base(spec)

	emissions := map[string][]string{
		"review":         {"review", base},
		"review/compact": {"review", base, "--compact"},
		"review/role":    {"review", base, "--role", "implementer"},
		"audit":          {"audit", base, "--affected-files", base},
		"audit/compact":  {"audit", base, "--affected-files", base, "--compact"},
		"audit/role":     {"audit", base, "--affected-files", base, "--role", "spec-coverage"},
	}

	for name, args := range emissions {
		t.Run(name, func(t *testing.T) {
			seen := 0
			for _, p := range emitFor(t, dir, args...) {
				if p.outputPath == "" {
					continue
				}
				seen++
				label := name + "/" + p.role

				assert.True(t, strings.HasSuffix(p.body, suffix),
					"%s names %s, so its body ends with §2.3's construction", label, p.outputPath)

				// The construction, not merely its length: §2.2's clause after
				// a blank line, §3.2's clause after a blank line, and nothing
				// after that. A 468-byte tail in the wrong order would satisfy
				// a length check.
				assert.False(t, strings.HasSuffix(p.body, "\n"),
					"%s: §2.3 fixes no trailing newline; the body ends on the clause", label)
			}
			require.Positive(t, seen,
				"%s must emit at least one prompt with an output_path, or it measures nothing", name)
		})
	}
}
