package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type lintFMResult struct {
	Frontmatter struct {
		Present   bool     `json:"present"`
		Lines     *string  `json:"lines"`
		Domain    string   `json:"domain"`
		LensRoles []string `json:"lens_roles"`
	} `json:"frontmatter"`
	Findings []struct {
		Severity string `json:"severity"`
		Rule     string `json:"rule"`
		Message  string `json:"message"`
	} `json:"findings"`
}

func TestLintFrontmatter_ObjectShapes(t *testing.T) {
	t.Run("with frontmatter", func(t *testing.T) {
		dir := t.TempDir()
		spec := "---\ntp:\n  domain: prose\n  lens:\n    all:\n      - \"q1\"\n    tester:\n      - \"q2\"\n    architect: []\n---\n# Heading\ncontent\n"
		specPath := filepath.Join(dir, "spec.md")
		require.NoError(t, os.WriteFile(specPath, []byte(spec), 0o600))

		stdout, _, code := runTP(t, dir, "lint", "spec.md")
		require.Equal(t, 0, code, "lint output: %s", stdout)

		var res lintFMResult
		require.NoError(t, json.Unmarshal([]byte(stdout), &res))
		assert.True(t, res.Frontmatter.Present)
		require.NotNil(t, res.Frontmatter.Lines)
		assert.Equal(t, "1-10", *res.Frontmatter.Lines)
		assert.Equal(t, "prose", res.Frontmatter.Domain)
		assert.Equal(t, []string{"tester", "all"}, res.Frontmatter.LensRoles, "non-empty roles in fixed order")
	})

	t.Run("no frontmatter exact shape", func(t *testing.T) {
		dir := t.TempDir()
		specPath := filepath.Join(dir, "spec.md")
		require.NoError(t, os.WriteFile(specPath, []byte("# Heading\ncontent\n"), 0o600))

		stdout, _, code := runTP(t, dir, "lint", "spec.md")
		require.Equal(t, 0, code)

		var raw map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &raw))
		fm, ok := raw["frontmatter"].(map[string]any)
		require.True(t, ok, "frontmatter object present")
		assert.Equal(t, false, fm["present"])
		assert.Nil(t, fm["lines"])
		assert.Equal(t, "software", fm["domain"])
		assert.Equal(t, []any{}, fm["lens_roles"])
	})

	t.Run("malformed YAML is a lint error naming the failure", func(t *testing.T) {
		dir := t.TempDir()
		specPath := filepath.Join(dir, "spec.md")
		require.NoError(t, os.WriteFile(specPath, []byte("---\ntp: [broken\n---\n# Heading\ncontent\n"), 0o600))

		stdout, _, code := runTP(t, dir, "lint", "spec.md")
		assert.Equal(t, 1, code, "lint error exit")

		var res lintFMResult
		require.NoError(t, json.Unmarshal([]byte(stdout), &res))
		found := false
		for _, f := range res.Findings {
			if f.Rule == "frontmatter" && f.Severity == "error" {
				found = true
				assert.Contains(t, f.Message, "YAML parse failed")
			}
		}
		assert.True(t, found)
		assert.Equal(t, "software", res.Frontmatter.Domain, "defaults apply")
	})
}
