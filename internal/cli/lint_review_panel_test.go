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

// The two reviewer roles §6 row 2's fixture is built from. Exactly one of them
// declares `domains`, and the test asserts that rather than trusting these two
// literals: a second `domains` role would leave the surviving panel empty, tp
// would fall back to the embedded default corpus, and the row would be decided
// on the wrong population.
const (
	domainFixtureProseRole     = `{"id":"prose-only","title":"P","instructions":"You review.","domains":["prose"]}`
	domainFixtureUniversalRole = `{"id":"universal","title":"U","instructions":"You review."}`
)

// writeDomainMismatchFixture builds §6 row 2's fixture: a temporary corpus whose
// only `domains`-declaring reviewer asks for `prose`, beside a spec whose
// frontmatter says `domain: software`. The domain filter therefore drops
// `prose-only` and leaves `universal`, which is the whole of what the row turns
// on.
func writeDomainMismatchFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	writeReviewerRole(t, dir, "prose-only.json", domainFixtureProseRole)
	writeReviewerRole(t, dir, "universal.json", domainFixtureUniversalRole)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("---\ndomain: software\n---\n# Spec\n## 1. A\ncontent here.\n"), 0o600))

	require.True(t, strings.Contains(domainFixtureProseRole, `"domains"`),
		"the fixture's mismatching role must declare domains, or nothing is filtered")
	require.False(t, strings.Contains(domainFixtureUniversalRole, `"domains"`),
		"exactly one role may declare domains: a second one empties the panel and tp falls back to the embedded corpus")

	return dir
}

// lintReviewPanel runs `tp lint spec.md` in dir and returns its `review_panel`
// alongside the command's stderr.
func lintReviewPanel(t *testing.T, dir string) (panel []string, stderr string) {
	t.Helper()
	return lintReviewPanelOf(t, dir, "spec.md")
}

// lintReviewPanelOf runs `tp lint <specFile>` in dir and returns its
// `review_panel` alongside the command's stderr, decoding the field as a list
// of strings.
//
// The spec file is a parameter because §6 row 3's fixture lints two specs
// against ONE corpus: that row turns on the difference the frontmatter makes,
// and a second corpus could not tell that from a difference between corpora.
func lintReviewPanelOf(t *testing.T, dir, specFile string) (panel []string, stderr string) {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "lint", specFile)
	require.Equal(t, 0, code, "stderr: %s", stderr)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	raw, ok := payload["review_panel"]
	require.True(t, ok, "tp lint reports review_panel: %s", stdout)
	list, ok := raw.([]any)
	require.True(t, ok, "review_panel is a JSON list, not %T — a nil slice serialises as null here", raw)

	panel = make([]string, 0, len(list))
	for _, item := range list {
		id, ok := item.(string)
		require.True(t, ok, "review_panel carries role ids, not %T", item)
		panel = append(panel, id)
	}
	return panel, stderr
}

// reviewEmissionRoles runs a round-1 `tp review <spec>` — no `--diff-from` — in
// dir and returns the role ids its emission carries, in emission order.
//
// The emission is RUN rather than re-derived. Re-deriving it would assert that
// two calls of one resolver agree, while the requirement is about what the two
// commands report.
func reviewEmissionRoles(t *testing.T, dir string) []string {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	var payload struct {
		Prompts []struct {
			Role string `json:"role"`
		} `json:"prompts"`
		ReviewLoop struct {
			Round int `json:"round"`
		} `json:"review_loop"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	require.Equal(t, 1, payload.ReviewLoop.Round,
		"the panel under comparison must be a ROUND-1 emission's")
	require.NotEmpty(t, payload.Prompts, "an emission carrying no prompt names no role")

	roles := make([]string, 0, len(payload.Prompts))
	for _, p := range payload.Prompts {
		roles = append(roles, p.Role)
	}
	return roles
}

// TestLintReviewPanelOmitsADomainMismatchedRole is §6 row 2: when a role in the
// corpus declares `domains` the spec's frontmatter does not match, `tp lint`
// reports a `review_panel` that omits that role and equals the role ids of a
// round-1 `tp review` emission of the same spec.
//
// The row's named mutant is "resolve the corpus but skip the domain filter",
// and it is decided twice over, because the two assertions catch it in the two
// places it can be introduced. A mutant inside the shared resolver moves BOTH
// commands together, leaving the equality green — the NotContains is what fails
// there. A mutant that gives lint a second derivation of its own moves only one
// side, and the equality fails.
//
// The order matters and is asserted: the requirement is that the two lists are
// equal, not that they hold the same set, since an agent reading `review_panel`
// reads it as the panel the emission would carry.
//
// This repository's own corpus cannot serve as the fixture. No reviewer role in
// `.tp/reviewers/` declares `domains`, so the filter is inert here and the
// mutant changes nothing — measured as zero `domain-mismatch` skips across every
// spec in the repository. A test that cannot fail is not a test, which is why
// the corpus is built for the row instead.
func TestLintReviewPanelOmitsADomainMismatchedRole(t *testing.T) {
	t.Parallel()
	dir := writeDomainMismatchFixture(t)

	panel, lintErr := lintReviewPanel(t, dir)
	emitted := reviewEmissionRoles(t, dir)

	require.Contains(t, emitted, "universal",
		"the fixture must leave a role standing, or the panel is empty for a reason this row is not about")

	assert.Equal(t, emitted, panel,
		"`review_panel` is the role ids a round-1 `tp review` emission of the same spec carries")
	assert.NotContains(t, panel, "prose-only",
		"and the domain-mismatched role is omitted from it")
	assert.NotContains(t, emitted, "prose-only",
		"as it is from the emission the panel is compared against: the domain filter is what removes it from both")

	assert.Empty(t, lintErr,
		"lint describes a spec through the pure resolver, so it gains none of the wrapper's advisory stderr")
}
