package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clauseSuffixFromSpec rebuilds §2.3's suffix from the spec's own fenced blocks,
// so this external test asserts against the source of truth rather than against
// a literal that could drift with the implementation it is checking.
func clauseSuffixFromSpec(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRootDir(t), "spec", "0.36.0.md"))
	require.NoError(t, err)
	spec := string(body)

	fenced := func(heading string) string {
		i := strings.Index(spec, heading)
		require.GreaterOrEqual(t, i, 0, "spec must carry %q", heading)
		rest := spec[i:]
		open := strings.Index(rest, "```text\n")
		require.GreaterOrEqual(t, open, 0)
		rest = rest[open+len("```text\n"):]
		closeAt := strings.Index(rest, "\n```")
		require.GreaterOrEqual(t, closeAt, 0, "the fenced block under %q must be closed", heading)
		return rest[:closeAt]
	}
	return "\n\n" + fenced("### 2.2 The clause") + "\n\n" + fenced("### 3.2 The clause")
}

// emittedPrompt is one entry of an emission's prompts[], reduced to the three
// fields these tests compare.
type emittedPrompt struct{ role, outputPath, body string }

// emitFor runs one emission and returns its prompts as (role, output_path, body).
func emitFor(t *testing.T, dir string, args ...string) []emittedPrompt {
	t.Helper()
	stdout, stderr, code := runTPIn(t, dir, append(args, "--json")...)
	require.Equal(t, 0, code, "emission failed: %s", stderr)

	var payload struct {
		Prompts []struct {
			Role       string `json:"role"`
			Prompt     string `json:"prompt"`
			OutputPath string `json:"output_path"`
		} `json:"prompts"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))

	out := make([]emittedPrompt, 0, len(payload.Prompts))
	for _, p := range payload.Prompts {
		out = append(out, emittedPrompt{p.Role, p.OutputPath, p.Prompt})
	}
	return out
}

// TestClausesAppendedByOutputPath pins v0.36.0 §2.3's emission predicate: the
// clauses go on every prompt that names an output file, and on no other.
//
// The discriminator is output_path rather than the role, because the same role
// carries a non-empty output_path in one mode and an empty one in another — so
// a rule written against role names would classify the same prompt two ways.
func TestClausesAppendedByOutputPath(t *testing.T) {
	t.Parallel()
	suffix := clauseSuffixFromSpec(t)
	spec := relocatedSpec(t, "spec/0.36.0.md")
	dir := filepath.Dir(spec)
	base := filepath.Base(spec)

	subject := filepath.Join(dir, "subject.go")
	require.NoError(t, os.WriteFile(subject, []byte("package subject\n"), 0o600))

	emissions := map[string][]string{
		"review/default":     {"review", base},
		"review/perspective": {"review", base, "--perspective", "testing", "--test-path", "."},
		"audit/default":      {"audit", base, "--affected-files", "subject.go"},
	}

	sawWith, sawWithout := 0, 0
	for name, args := range emissions {
		for _, p := range emitFor(t, dir, args...) {
			label := name + "/" + p.role
			if p.outputPath == "" {
				sawWithout++
				assert.False(t, strings.HasSuffix(p.body, suffix),
					"%s names no output file, so it must not end with the suffix", label)
				continue
			}
			sawWith++
			assert.True(t, strings.HasSuffix(p.body, suffix),
				"%s names an output file, so it must end with §2.3's suffix", label)
			stripped := strings.TrimSuffix(p.body, suffix)
			assert.False(t, strings.HasSuffix(stripped, "\n"),
				"%s: §2.3 removes the body's trailing newline before appending", label)
			assert.False(t, strings.HasSuffix(stripped, suffix),
				"%s: the suffix is appended once, not twice", label)
		}
	}

	// Without both sides the run proves nothing: an implementation that
	// appended everywhere, or nowhere, would satisfy a one-sided sweep.
	assert.Positive(t, sawWith, "the fixture must produce at least one prompt with an output_path")
	assert.Positive(t, sawWithout, "the fixture must produce at least one prompt without one")
}
