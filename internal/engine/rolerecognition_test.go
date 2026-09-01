package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RoleIsRecognised is v0.36.0's only new function in this package, and a
// gremlins dry run found all five of its mutants NOT COVERED: every test that
// exercised it went through the CLI, so the engine package's own suite never
// reached it. End-to-end coverage of the behaviour is real coverage, but it
// leaves this package blind — a mutation run here could not have scored the
// function at all.

// writeScopedRole drops one role file into a phase's corpus directory.
//
// rolecorpus_test.go's writeRole is the plain version; this one carries a
// domains list, which is the field recognition must deliberately ignore.
func writeScopedRole(t *testing.T, root, phase, id, domains string) {
	t.Helper()
	dir := filepath.Join(root, ".tp", phase)
	require.NoError(t, os.MkdirAll(dir, 0o750))
	body := `{"id":"` + id + `","title":"` + id + `","instructions":"You work."`
	if domains != "" {
		body += `,"domains":[` + domains + `]`
	}
	body += "}"
	require.NoError(t, os.WriteFile(filepath.Join(dir, id+".json"), []byte(body), 0o600))
}

func TestRoleIsRecognised(t *testing.T) {
	root := t.TempDir()
	writeScopedRole(t, root, "reviewers", "my-reviewer", "")
	writeScopedRole(t, root, "auditors", "my-auditor", "")
	// A role the spec's domain excludes: still a name tp knows, which is the
	// point — recognition is deliberately unfiltered by domain.
	writeScopedRole(t, root, "reviewers", "prose-only", `"prose"`)

	for _, c := range []struct {
		name, role string
		want       bool
		why        string
	}{
		{"a user reviewer", "my-reviewer", true, "the phase's own corpus"},
		{"a user auditor", "my-auditor", true,
			"the OTHER phase's corpus: tp review never emits an auditor, so a one-phase set would refuse a name this repo defines"},
		{"domain-excluded", "prose-only", true,
			"unfiltered by domain: the role is dropped from the panel and named in skipped_roles, but it is still a name tp knows"},
		{"the regression built-in", RegressionRoleID, true,
			"emitted every round and belongs to no corpus, so it has to be matched directly"},
		{"an embedded default", "implementer", true, "the embedded corpus counts too — a repo with no role files runs on it"},
		{"an embedded auditor", "spec-coverage", true, "embedded, other phase"},
		{"unknown", "no-such-role", false, "recognised nowhere"},
		{"empty", "", false, `--role "" is a name, not an absent flag`},
	} {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, RoleIsRecognised(root, "software", c.role), c.why)
		})
	}
}

// TestRoleIsRecognisedFallsBackForAnUnknownDomain pins the branch that keeps
// recognition agreeing with ResolveActiveCorpus: a domain with no embedded
// corpus resolves against software rather than returning nothing.
func TestRoleIsRecognisedFallsBackForAnUnknownDomain(t *testing.T) {
	root := t.TempDir() // no user corpus at all

	assert.True(t, RoleIsRecognised(root, "not-a-domain", "implementer"),
		"an unknown domain falls back to the software corpus, the same fallback ResolveActiveCorpus applies")
	assert.True(t, RoleIsRecognised(root, "software", "implementer"),
		"and the known domain resolves directly")
	assert.False(t, RoleIsRecognised(root, "not-a-domain", "no-such-role"),
		"the fallback widens where the name is looked up, not what counts as a name")
}

// TestRoleIsRecognisedIgnoresAMalformedCorpus pins the error path: a corpus
// that will not parse is not this predicate's error to report, because emission
// has already failed on it before any caller here would care.
func TestRoleIsRecognisedIgnoresAMalformedCorpus(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".tp", "reviewers")
	require.NoError(t, os.MkdirAll(dir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "broken.json"), []byte("{not json"), 0o600))

	assert.False(t, RoleIsRecognised(root, "software", "broken"),
		"an unreadable corpus contributes no names")
	assert.True(t, RoleIsRecognised(root, "software", "implementer"),
		"and does not stop the embedded corpus from being consulted")
}
