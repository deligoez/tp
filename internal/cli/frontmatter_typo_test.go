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

// lensDeprecationNotice is the one stderr line a valid legacy lens earns from
// tp review: the shim's auto-translation notice.
const lensDeprecationNotice = "tp: lens is deprecated; migrate to tp.review_roles with a focus list — auto-translating for this run"

// frontmatterWarningsFromLint runs tp lint on the project's spec and returns
// the messages of its frontmatter-rule warnings.
func frontmatterWarningsFromLint(t *testing.T, dir string) []string {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "lint", "spec.md")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var res lintFMResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &res))
	msgs := make([]string, 0)
	for _, f := range res.Findings {
		if f.Rule == "frontmatter" && f.Severity == "warning" {
			msgs = append(msgs, f.Message)
		}
	}
	return msgs
}

// TestFrontmatterTypo_WarnsInLintAndOnReviewStderr is the BUGS.md reproduction:
// a mistyped key in the tp: frontmatter was dropped in silence by tp review, and
// tp lint caught only the one inside tp.lens. Each typo must now be named by
// lint and by review's stderr notice channel — the one that already carries the
// panel warnings, visible in the JSON mode every agent-driven run uses — while
// a valid lens earns nothing beyond the deprecation notice it always had.
func TestFrontmatterTypo_WarnsInLintAndOnReviewStderr(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		frontmatter string
		want        string // "" for the valid control
	}{
		{
			name:        "A unknown lens key",
			frontmatter: "tp:\n  lens:\n    implementr:\n      - \"typo question\"",
			want:        `tp.lens key "implementr" is unknown (known: implementer, tester, architect, all); ignored`,
		},
		{
			name:        "B unknown tp key lenz",
			frontmatter: "tp:\n  lenz:\n    implementer:\n      - \"typo question\"",
			want:        `tp key "lenz" is unknown (known: domain, lens, review_roles, audit_roles); ignored`,
		},
		{
			name:        "C unknown tp key review_role",
			frontmatter: "tp:\n  review_role:\n    implementer:\n      focus:\n        - \"typo question\"",
			want:        `tp key "review_role" is unknown (known: domain, lens, review_roles, audit_roles); ignored`,
		},
		{
			name:        "D valid lens control",
			frontmatter: "tp:\n  lens:\n    implementer:\n      - \"valid question\"",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
			spec := "---\n" + tc.frontmatter + "\n---\n# Spec\n## 1. A\ncontent here.\n"
			require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))

			lintWarnings := frontmatterWarningsFromLint(t, dir)

			stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--no-state")
			require.Equal(t, 0, code, "stderr: %s", stderr)
			var payload map[string]any
			require.NoError(t, json.Unmarshal([]byte(stdout), &payload), "stdout stays parseable JSON")

			if tc.want == "" {
				assert.Empty(t, lintWarnings, "a valid lens earns no lint warning")
				rest := strings.TrimSpace(strings.Replace(stderr, lensDeprecationNotice, "", 1))
				assert.Contains(t, stderr, lensDeprecationNotice, "the deprecation notice is unchanged")
				assert.Empty(t, rest, "a valid lens earns no notice beyond the deprecation one")
				return
			}
			assert.Equal(t, []string{tc.want}, lintWarnings, "lint names the typo and the known keys")
			assert.Contains(t, stderr, tc.want, "review names the typo on its notice channel")
			assert.NotContains(t, stdout, tc.want, "the notice belongs on stderr, never in the payload")
		})
	}
}
