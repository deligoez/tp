package cli_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deligoez/tp/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cutMarker opens the line both inline-spec sites append when they cut a spec.
const cutMarker = "\n[...spec truncated at "

// runeStraddlingSpec is a spec whose byte engine.SpecContentCap is the second
// byte of "ı" (\xc4\xb1): the field report's Turkish spec, reduced to the
// boundary. A byte slice at the cap keeps the lead byte alone, invalid UTF-8
// the JSON encoder turned into U+FFFD. It returns the spec and the byte count
// a rune-boundary cut keeps — everything before that "ı".
func runeStraddlingSpec() (spec string, kept int) {
	head := "# Spec\n## Table\n| C | D |\n|---|---|\n| a | b |\n\n"
	kept = engine.SpecContentCap - 1
	pad := strings.Repeat("a", kept-len(head))
	return head + pad + strings.Repeat("ı", 2000) + "\n", kept
}

// assertCutExcerpt checks one prompt that embedded the cut spec: what precedes
// the marker is exactly the spec's first kept bytes, valid UTF-8, and the
// marker line names the kept and total byte counts and the spec path.
func assertCutExcerpt(t *testing.T, prompt, spec string, kept int, specPath string) {
	t.Helper()
	if i := strings.IndexRune(prompt, utf8.RuneError); i >= 0 {
		assert.Failf(t, "a half rune reached the encoder", "U+FFFD at byte %d: %q", i, prompt[max(0, i-8):min(len(prompt), i+40)])
	}
	end := strings.Index(prompt, cutMarker)
	if end < 0 {
		at := max(0, strings.Index(prompt, "\n[...")-8)
		require.Failf(t, "the prompt names the cut with its counts", "found instead: %q", prompt[at:min(len(prompt), at+48)])
	}
	require.GreaterOrEqual(t, end, kept, "the prompt holds the kept bytes before the marker")
	excerpt := prompt[end-kept : end]
	assert.True(t, utf8.ValidString(excerpt), "the excerpt is valid UTF-8")
	assert.Equal(t, spec[:kept], excerpt, "the cut backs off to the last complete rune at or before the cap")

	line := prompt[end+1:]
	if nl := strings.Index(line, "\n"); nl >= 0 {
		line = line[:nl]
	}
	assert.Contains(t, line, strconv.Itoa(kept), "the marker names the kept byte count")
	assert.Contains(t, line, strconv.Itoa(len(spec)), "the marker names the total byte count")
	assert.Contains(t, line, specPath, "the marker names the spec path, so the role can read the rest")
	assert.Contains(t, line, "bytes")
	assert.NotContains(t, line, "chars", "the cap counts bytes")
}

// assertSpecTruncatedKey checks the payload's one key for a cut spec.
func assertSpecTruncatedKey(t *testing.T, result map[string]any, specPath string, kept, total int) {
	t.Helper()
	raw, ok := result["spec_truncated"]
	require.True(t, ok, "a cut spec is named in the payload")
	cut, ok := raw.(map[string]any)
	require.True(t, ok, "spec_truncated is an object: %v", raw)
	assert.Equal(t, specPath, cut["path"])
	assert.InDelta(t, float64(kept), cut["kept_bytes"], 0)
	assert.InDelta(t, float64(total), cut["total_bytes"], 0)
}

// TestAuditSpecCutOnRuneBoundaryAndNamed: loadAuditSpec cut the spec with a
// byte slice at SpecContentCap, so a spec whose cap fell inside a multi-byte
// rune reached spec-coverage ending in U+FFFD, under a bare marker, and
// nothing in the payload said the role saw part of the spec.
func TestAuditSpecCutOnRuneBoundaryAndNamed(t *testing.T) {
	t.Parallel()
	for _, compact := range []bool{false, true} {
		name := "full"
		if compact {
			name = "compact"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			specPath := filepath.Join(dir, "spec.md")
			spec, kept := runeStraddlingSpec()
			require.NoError(t, os.WriteFile(specPath, []byte(spec), 0o600))
			goPath := filepath.Join(dir, "x.go")
			require.NoError(t, os.WriteFile(goPath, []byte("package main\n"), 0o600))

			args := []string{"audit", specPath, "--affected-files", goPath}
			if compact {
				// The role saw part of the spec: that survives --compact.
				args = append(args, "--compact")
			}
			stdout, stderr, code := runTP(t, dir, args...)
			require.Equal(t, 0, code, "stderr: %s", stderr)

			prompt, ok := auditPromptsByRole(t, stdout)["spec-coverage"]["prompt"].(string)
			require.True(t, ok, "spec-coverage is emitted")
			assertCutExcerpt(t, prompt, spec, kept, specPath)
			assertSpecTruncatedKey(t, decodePayload(t, stdout), specPath, kept, len(spec))
		})
	}
}

// TestAuditSpecUnderCapCarriesNoCutKey: the key is conditional, absent when
// the whole spec reached the prompt.
func TestAuditSpecUnderCapCarriesNoCutKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n## Table\n| C | D |\n|---|---|\n| ı | b |\n"), 0o600))
	goPath := filepath.Join(dir, "x.go")
	require.NoError(t, os.WriteFile(goPath, []byte("package main\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "audit", specPath, "--affected-files", goPath)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.NotContains(t, decodePayload(t, stdout), "spec_truncated")
	assert.NotContains(t, stdout, "spec truncated")
}

// reviewSpecInlineFixture writes a project for every tp review mode that
// embeds the spec under --spec-inline, and returns the argument list per mode.
func reviewSpecInlineFixture(t *testing.T, dir, specPath string) map[string][]string {
	t.Helper()
	goPath := filepath.Join(dir, "g.go")
	require.NoError(t, os.WriteFile(goPath, []byte("package main\n"), 0o600))
	docs := filepath.Join(dir, "docs")
	require.NoError(t, os.MkdirAll(docs, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(docs, "a.md"), []byte("# Doc\n"), 0o600))
	tests := filepath.Join(dir, "tests")
	require.NoError(t, os.MkdirAll(tests, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(tests, "a_test.go"), []byte("package a\n"), 0o600))
	findings := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(findings, []byte(`{"severity":"high","category":"completeness","location":"## Table","finding":"f","evidence":"read it"}`+"\n"), 0o600))

	return map[string][]string{
		"panel":         {"review", specPath, "--spec-inline", "--no-state"},
		"panel-compact": {"review", specPath, "--spec-inline", "--no-state", "--compact"},
		"code-audit":    {"review", specPath, "--spec-inline", "--perspective", "code-audit", "--affected-files", goPath},
		"documentation": {"review", specPath, "--spec-inline", "--perspective", "documentation", "--docs-path", docs},
		"testing":       {"review", specPath, "--spec-inline", "--perspective", "testing", "--test-path", tests},
		"verify":        {"review", specPath, "--spec-inline", "--verify", "--findings", findings},
	}
}

// TestReviewSpecInlineCutOnRuneBoundaryAndNamed: readSpecContent had the same
// byte cut as tp audit, labelled "chars", and no payload key. Every mode that
// embeds the spec under --spec-inline now cuts on a rune boundary and names
// the cut in spec_truncated.
func TestReviewSpecInlineCutOnRuneBoundaryAndNamed(t *testing.T) {
	t.Parallel()
	spec, kept := runeStraddlingSpec()
	for _, mode := range []string{"panel", "panel-compact", "code-audit", "documentation", "testing", "verify"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			specPath := filepath.Join(dir, "spec.md")
			require.NoError(t, os.WriteFile(specPath, []byte(spec), 0o600))
			args, ok := reviewSpecInlineFixture(t, dir, specPath)[mode]
			require.True(t, ok, "the fixture has a %s invocation", mode)

			stdout, stderr, code := runTP(t, dir, args...)
			require.Equal(t, 0, code, "stderr: %s", stderr)

			result := decodePayload(t, stdout)
			prompts, ok := result["prompts"].([]any)
			require.True(t, ok)
			require.NotEmpty(t, prompts)
			for i, p := range prompts {
				// The first prompt embeds the spec in every mode; any other
				// that names a cut must name it the same way.
				text := p.(map[string]any)["prompt"].(string)
				if i == 0 || strings.Contains(text, "truncated at") {
					assertCutExcerpt(t, text, spec, kept, specPath)
				}
			}
			assertSpecTruncatedKey(t, result, specPath, kept, len(spec))
		})
	}
}

// TestReviewSpecInlineUnderCapCarriesNoCutKey: absent when nothing was cut.
func TestReviewSpecInlineUnderCapCarriesNoCutKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n## Table\n| C | D |\n|---|---|\n| ı | b |\n"), 0o600))
	args := reviewSpecInlineFixture(t, dir, specPath)["code-audit"]

	stdout, stderr, code := runTP(t, dir, args...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.NotContains(t, decodePayload(t, stdout), "spec_truncated")
	assert.NotContains(t, stdout, "spec truncated")
}
