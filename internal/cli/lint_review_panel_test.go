package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deligoez/tp/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The two reviewer roles §6 row 2's fixture is built from, plus the domain its
// spec declares. Exactly one role declares `domains`, and the test asserts that
// rather than trusting these literals: a second `domains` role would leave the
// surviving panel empty, tp would fall back to the embedded default corpus, and
// the row would be decided on the wrong population.
//
// The spec's domain must NOT be the one ParseFrontmatter supplies by default.
// An earlier fixture had the spec declare `software` and the role `prose`, and
// `software` is exactly the value a spec carrying no frontmatter at all resolves
// to, so the frontmatter block was live but not load-bearing: a mutant resolving
// the corpus against a hardcoded "software" instead of the spec's own domain
// left this row green. Polarising the fixture puts the red on the field the row
// is about under that mutant too. The polarity is asserted against
// engine.DomainSoftware rather than restated in prose, so it cannot drift back
// onto the default.
const (
	domainFixtureSpecDomain    = "prose"
	domainFixtureSoftwareRole  = `{"id":"software-only","title":"S","instructions":"You review.","domains":["software"]}`
	domainFixtureUniversalRole = `{"id":"universal","title":"U","instructions":"You review."}`
)

// writeDomainMismatchFixture builds §6 row 2's fixture: a temporary corpus whose
// only `domains`-declaring reviewer asks for `software`, beside a spec whose
// frontmatter says `tp.domain: prose`. The domain filter therefore drops
// `software-only` and leaves `universal`, which is the whole of what the row
// turns on.
//
// `prose` is a domain tp ships an embedded corpus for, so the mismatch draws no
// unknown-domain notice and the row's `lintErr` assertion still measures the
// absence of the wrapper's advisory channel rather than the absence of a warning
// about the fixture itself.
func writeDomainMismatchFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	writeReviewerRole(t, dir, "software-only.json", domainFixtureSoftwareRole)
	writeReviewerRole(t, dir, "universal.json", domainFixtureUniversalRole)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("---\ntp:\n  domain: "+domainFixtureSpecDomain+"\n---\n# Spec\n## 1. A\ncontent here.\n"), 0o600))

	require.NotEqual(t, engine.DomainSoftware, domainFixtureSpecDomain,
		"the spec's domain must differ from the parser's default, or a mutant that ignores the frontmatter resolves the same panel and the row stays green under it")
	require.True(t, strings.Contains(domainFixtureSoftwareRole, `"domains"`),
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
// A third mutant is why the fixture is polarised the way it is: resolving the
// corpus against a hardcoded "software" rather than the spec's own domain, so
// that the frontmatter is not read at all. Measured, with the fixture the way
// it is now: `["universal"]` at HEAD and `["software-only", "universal"]` under
// each of the two mutants that touch the domain, both red on this test's own
// NotContains.
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
	assert.NotContains(t, panel, "software-only",
		"and the domain-mismatched role is omitted from it")
	assert.NotContains(t, emitted, "software-only",
		"as it is from the emission the panel is compared against: the domain filter is what removes it from both")

	assert.Empty(t, lintErr,
		"lint describes a spec through the pure resolver, so it gains none of the wrapper's advisory stderr")
}

// TestLintReportsAnEmptyPanelWhereReviewRefuses is §7's first table row across
// all three of its columns, on one fixture: a spec that deactivates every
// reviewer in the corpus makes `tp review` exit 2 and emit nothing, while
// `tp lint` on the identical tree exits 0 and reports `review_panel` as an
// empty JSON list.
//
// One fixture rather than two, because the row is about a DIFFERENCE. Run on
// separate trees the two halves could not say the input was the same, and the
// difference is the whole of what §7's split buys: a read-only caller must be
// able to describe a panel the two emitting commands would refuse to run.
//
// `review_panel` is decoded and type-checked before it is checked for
// emptiness. A nil slice serialises as `null`, and `null` satisfies an
// emptiness assertion written on the decoded Go value while telling an agent
// reading the JSON that tp reports no list at all.
//
// `tp review` runs WITHOUT `--no-state` on purpose. The refusal is specified to
// happen ahead of every write the emission path performs, so the assertion that
// no round state was created is part of the row rather than a bonus.
//
// Watched red under two mutants, one per half of the difference:
//
//   - `lintReviewPanel` calling `resolveRolePanel`, the refusing wrapper in
//     role_panel.go, instead of `engine.ResolveRolePanel`. Lint exits 2 with
//     review's refusal on stderr and the lint half fails on the exit code.
//     This is the mutant the row exists for — a `tp lint` that refuses a spec.
//   - `engine.ResolveRolePanel` returning `roles` where it returns
//     `DropDisabledRoles(roles, disabled)`. Lint reports `["keeper"]` and the
//     empty-list assertion fails; `tp review` stops refusing, because the
//     wrapper's refusal keys on an empty post-drop panel, and the review half
//     fails on exit code 0. One mutant, both halves red, in opposite
//     directions — which is what a shared resolver looks like from here.
func TestLintReportsAnEmptyPanelWhereReviewRefuses(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	writeReviewerRole(t, dir, "keeper.json", `{"id":"keeper","title":"K","instructions":"You review."}`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("---\ntp:\n  review_roles:\n    keeper:\n      enabled: false\n---\n# Spec\n## 1. A\ncontent here.\n"), 0o600))

	lintOut, lintErr, lintCode := runTP(t, dir, "lint", "spec.md")
	require.Equal(t, 0, lintCode,
		"tp lint describes a spec it could not review rather than refusing it; stderr: %s", lintErr)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(lintOut), &payload))
	raw, ok := payload["review_panel"]
	require.True(t, ok, "tp lint reports review_panel even where the panel is empty: %s", lintOut)
	list, ok := raw.([]any)
	require.True(t, ok, "review_panel is a JSON list, not %T — a nil slice serialises as null here", raw)
	assert.Empty(t, list,
		"every reviewer is deactivated, so the panel a round-1 emission would carry is empty")
	assert.Empty(t, lintErr, "and lint gains none of the wrapper's advisory stderr on the way")

	reviewOut, reviewErr, reviewCode := runTP(t, dir, "review", "spec.md")
	assert.Equal(t, 2, reviewCode, "tp review refuses the same spec; stderr: %s", reviewErr)
	assert.Empty(t, reviewOut, "a refused run emits no prompt")
	assert.Contains(t, reviewErr, "every reviewers role is deactivated by this spec: keeper")

	_, statErr := os.Stat(filepath.Join(dir, ".tp-review"))
	assert.True(t, os.IsNotExist(statErr),
		"the refusal precedes every write the emission path performs, so a refused run leaves nothing on disk")
}

// TestLintReviewPanelOmitsADeactivatedReviewer is §6 row 3: when a spec's
// frontmatter deactivates a reviewer, `tp lint` reports a `review_panel` that
// does not contain that role id.
//
// The assertion is deliberately NOT an equality against a `tp review` emission,
// unlike row 2's. The row's named mutant — ignore the frontmatter override and
// return the corpus's full role list — lives in the resolver both commands
// share, so it moves the emission and the panel together and an equality would
// stay green under exactly the mutant the row exists for. Row 2 can afford the
// comparison because it also carries a `NotContains` that fails inside the
// shared resolver; this row's whole assertion is the literal panel.
//
// The control lint is the fixture's precondition and it is asserted rather than
// assumed: `dropped` must actually be in the corpus, or the row is decided on a
// role that was never there. It is checked through a second spec against the
// SAME corpus, so a role file that failed to land fails here instead of passing
// the requirement vacuously. The precondition survives the mutant — under it
// the control still reports both — so the red lands on the row's own assertion
// rather than on the fixture.
//
// The deactivated role must be a REVIEWER. A deactivation naming an auditor id
// matches no active reviewers role, is ignored with a warning, and leaves the
// panel whole, which an earlier version of this row mistook for a measurement.
//
// Measured: `["keeper"]` at HEAD, `["dropped", "keeper"]` under the mutant.
func TestLintReviewPanelOmitsADeactivatedReviewer(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	writeReviewerRole(t, dir, "dropped.json", `{"id":"dropped","title":"D","instructions":"You review."}`)
	writeReviewerRole(t, dir, "keeper.json", `{"id":"keeper","title":"K","instructions":"You review."}`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "control.md"),
		[]byte("# Spec\n## 1. A\ncontent here.\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("---\ntp:\n  review_roles:\n    dropped:\n      enabled: false\n---\n# Spec\n## 1. A\ncontent here.\n"), 0o600))

	control, controlErr := lintReviewPanelOf(t, dir, "control.md")
	require.Equal(t, []string{"dropped", "keeper"}, control,
		"the corpus must hold both roles, or the row is decided on a role that was never in it")
	require.Empty(t, controlErr)

	panel, lintErr := lintReviewPanelOf(t, dir, "spec.md")

	assert.Equal(t, []string{"keeper"}, panel,
		"the deactivated reviewer is dropped and the rest of the same corpus stands")
	assert.NotContains(t, panel, "dropped",
		"which is the row's own requirement: the panel does not contain the deactivated role id")
	assert.Empty(t, lintErr,
		"lint describes a spec through the pure resolver, so it gains none of the wrapper's advisory stderr")
}
