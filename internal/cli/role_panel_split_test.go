package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRolePanelWrapper_RefusalsAndNoticesStayOnTheWrapper is the caller-facing
// half of §7's table. The split moves resolution into engine.ResolveRolePanel;
// what must not move with it is everything in these four sub-tests, because
// tp review and tp audit are the two callers and neither changes.
//
// The last sub-test is §7's third row, the one a reader would miss, and it is
// asserted as a byte count on stderr rather than as a substring: the wrapper's
// single output.Notice loop, over the merged warning slice the resolver
// returns, writes 86 bytes here, and tp lint on the same tree writes none. A
// lint that called the wrapper would gain an advisory stderr channel it has
// never had, so the count is the fact the next task inherits.
//
// Both halves of that sub-test discriminate, and only one of them always did.
// The review byte count did from the start: moving the notice loop into
// engine.ResolveRolePanel doubles it to 172, measured. The lint half was a
// baseline pin when this was written, because tp lint resolved no panel at all;
// it became a measurement the moment lintReviewPanel started calling the
// resolver, and the same mutant now takes lint's stderr from 0 bytes to the
// same 86 — measured on this fixture, not carried forward. A second mutant
// reaches the lint half from the other side: lintReviewPanel calling
// resolveRolePanel, the wrapper below, gives lint the wrapper's notices too.
func TestRolePanelWrapper_RefusalsAndNoticesStayOnTheWrapper(t *testing.T) {
	t.Parallel()

	t.Run("review still refuses an emptied reviewer phase while audit is unaffected", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
		writeReviewerRole(t, dir, "keeper.json", `{"id":"keeper","title":"K","instructions":"You review."}`)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n"), 0o600))
		spec := "---\ntp:\n  review_roles:\n    keeper:\n      enabled: false\n---\n# Spec\n## 1. A\ncontent here.\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))

		stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--no-state")
		assert.Equal(t, 2, code)
		assert.Empty(t, stdout, "a refused run emits no prompt")
		assert.Contains(t, stderr, "every reviewers role is deactivated by this spec: keeper")

		_, auditErr, auditCode := runTP(t, dir, "audit", "spec.md", "--affected-files", "code.go")
		assert.Equal(t, 0, auditCode, "a reviewer-phase deactivation does not reach the auditor phase; stderr: %s", auditErr)
	})

	t.Run("audit still refuses a deactivated spec-coverage", func(t *testing.T) {
		t.Parallel()
		spec := "---\ntp:\n  audit_roles:\n    spec-coverage:\n      enabled: false\n---\n# Spec\n## 1. A\ncontent here.\n"
		dir := writeAuditorCorpusProject(t, spec, "spec-coverage", "keeper")

		stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "code.go")
		assert.Equal(t, 2, code)
		assert.Empty(t, stdout)
		assert.Contains(t, stderr, "spec-coverage cannot be deactivated")
		assert.NotContains(t, stderr, "every auditors role is deactivated",
			"the auditors-only refusal is decided before the emptiness one, and keeper is still active anyway")
	})

	t.Run("a role file that will not parse still exits 3 in both phases", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
		writeReviewerRole(t, dir, "broken.json", `{ not json`)
		audDir := filepath.Join(dir, ".tp", "auditors")
		require.NoError(t, os.MkdirAll(audDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(audDir, "broken.json"), []byte(`{ not json`), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
			[]byte("# Spec\n## 1. A\ncontent here.\n"), 0o600))

		_, stderr, code := runTP(t, dir, "review", "spec.md", "--no-state")
		assert.Equal(t, 3, code)
		assert.Contains(t, stderr, "repair or delete the offending role file under .tp/reviewers/")

		_, auditErr, auditCode := runTP(t, dir, "audit", "spec.md", "--affected-files", "code.go")
		assert.Equal(t, 3, auditCode)
		assert.Contains(t, auditErr, "repair or delete the offending role file under .tp/auditors/")
	})

	t.Run("the notices stay on tp review and tp lint writes none", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
		writeReviewerRole(t, dir, "prose-only.json",
			`{"id":"prose-only","title":"P","instructions":"You review.","domains":["prose"]}`)
		spec := "---\ntp:\n  domain: software\n---\n# Spec\n## 1. A\ncontent here.\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))

		_, reviewErr, reviewCode := runTP(t, dir, "review", "spec.md", "--no-state")
		require.Equal(t, 0, reviewCode, "stderr: %s", reviewErr)
		assert.Equal(t, `domain "software" filtered out every reviewers role; using the embedded default panel`+"\n", reviewErr)
		assert.Len(t, reviewErr, 86, "the measured notice, byte for byte")

		_, lintErr, lintCode := runTP(t, dir, "lint", "spec.md")
		require.Equal(t, 0, lintCode, "stderr: %s", lintErr)
		assert.Empty(t, lintErr, "lint has no advisory stderr channel and the split is what keeps it that way")
	})
}
