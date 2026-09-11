package engine

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rolePanelProject writes a project root holding a spec plus an optional
// reviewer corpus, and returns the spec path. Role bodies are passed verbatim so
// a test can write one that does not parse.
func rolePanelProject(t *testing.T, frontmatter string, roles map[string]string) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	if len(roles) > 0 {
		dir := filepath.Join(root, ".tp", PhaseReviewers)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		for stem, body := range roles {
			require.NoError(t, os.WriteFile(filepath.Join(dir, stem+".json"), []byte(body), 0o600))
		}
	}
	spec := "# Spec\n## 1. A\ncontent here.\n"
	if frontmatter != "" {
		spec = "---\n" + frontmatter + "\n---\n" + spec
	}
	specPath := filepath.Join(root, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte(spec), 0o600))
	return specPath
}

// captureStderr runs fn with os.Stderr replaced by a pipe and returns everything
// fn wrote to it. It swaps a process-global, so no test using it may run in
// parallel with another.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	saved := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = saved }()

	fn()

	require.NoError(t, w.Close())
	got, err := io.ReadAll(r)
	require.NoError(t, err)
	require.NoError(t, r.Close())
	return string(got)
}

// TestResolveRolePanel_ResolvesAnEmptiedPhaseWithoutRefusing pins the pure half
// of §7's first table row: a spec that deactivates every reviewer resolves to an
// empty panel and returns, where the cli wrapper exits 2 on the same input.
//
// Note the shape of the failure this guards against. If the refusal ever
// migrates back into the resolver, this test does not report a failed
// assertion — the os.Exit kills the package's whole test binary mid-run, which
// is the observable form of "a lint that called this would refuse a spec".
func TestResolveRolePanel_ResolvesAnEmptiedPhaseWithoutRefusing(t *testing.T) {
	specPath := rolePanelProject(t,
		"tp:\n  review_roles:\n    keeper:\n      enabled: false",
		map[string]string{"keeper": `{"id":"keeper","title":"K","instructions":"You review."}`})

	panel, err := ResolveRolePanel(specPath, PhaseReviewers)

	require.NoError(t, err)
	assert.Empty(t, panel.Roles, "the deactivated role is dropped and the embedded corpus does not refill the panel")
	assert.Equal(t, []string{"keeper"}, panel.Disabled, "the drop set survives, so the wrapper can name it in its refusal")
	assert.NotNil(t, panel.Frontmatter, "the frontmatter is part of the panel both phases carry forward")
}

// TestResolveRolePanel_ReturnsTheCorpusErrorInsteadOfExiting pins the pure half
// of §7's second table row. The wrapper turns this into exit 3 with a hint
// naming the corpus directory; the resolver hands the error back so a read-only
// caller can decide for itself.
func TestResolveRolePanel_ReturnsTheCorpusErrorInsteadOfExiting(t *testing.T) {
	specPath := rolePanelProject(t, "", map[string]string{"broken": `{ not json`})

	panel, err := ResolveRolePanel(specPath, PhaseReviewers)

	require.Error(t, err, "a malformed role file is reported, not swallowed")
	assert.Contains(t, err.Error(), "broken.json")
	assert.Empty(t, panel.Roles, "a panel that could not be resolved is not a panel with roles in it")
}

// TestResolveRolePanel_ReturnsWarningsWithoutWritingThem is §7's third table row
// at the engine layer, and the row a reader would miss: the one unconditional
// output.Notice loop belongs to the wrapper. The resolver returns the same
// strings as data and writes nothing, so a caller that must not gain an
// advisory stderr channel — tp lint — can have the panel without the notices.
//
// The order is asserted too, because the resolver concatenates the corpus
// warnings ahead of the override warnings and the wrapper's loop emits that
// slice as it stands, so this order is the byte sequence tp review writes.
func TestResolveRolePanel_ReturnsWarningsWithoutWritingThem(t *testing.T) {
	specPath := rolePanelProject(t,
		"tp:\n  domain: software\n  review_roles:\n    nosuch:\n      enabled: false",
		map[string]string{"prose-only": `{"id":"prose-only","title":"P","instructions":"You review.","domains":["prose"]}`})

	var panel RolePanel
	var err error
	stderr := captureStderr(t, func() { panel, err = ResolveRolePanel(specPath, PhaseReviewers) })

	require.NoError(t, err)
	assert.Equal(t, []string{
		`domain "software" filtered out every reviewers role; using the embedded default panel`,
		`tp.review_roles override for "nosuch" matches no active reviewers role; ignored`,
	}, panel.Warnings, "corpus warnings first, override warnings second — the wrapper's two loops in one slice")
	assert.Empty(t, stderr, "the resolver writes nothing; emitting is the wrapper's job")
}

// TestResolveRolePanel_CarriesFrontmatterWarningsFirst pins the frontmatter's
// shape warnings into the panel's warnings, ahead of the corpus and override
// ones: they come from parsing, which precedes resolution. Without them a
// mistyped frontmatter key reached tp lint and never the emitting commands.
func TestResolveRolePanel_CarriesFrontmatterWarningsFirst(t *testing.T) {
	specPath := rolePanelProject(t,
		"tp:\n  review_role: {}\n  review_roles:\n    nosuch:\n      enabled: false", nil)

	panel, err := ResolveRolePanel(specPath, PhaseReviewers)

	require.NoError(t, err)
	assert.Equal(t, []string{
		`tp key "review_role" is unknown (known: domain, lens, review_roles, audit_roles); ignored`,
		`tp.review_roles override for "nosuch" matches no active reviewers role; ignored`,
	}, panel.Warnings)
}
