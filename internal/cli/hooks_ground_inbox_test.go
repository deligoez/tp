package cli

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPreToolUseHookAllowsTheGroundInbox is the plugin half of
// spec/backlog/ground-scratch-in-state-dir.md: the ground scratch file moved
// into the spec's state directory, which the fence denies as tp's own state,
// so its one name, <spec dir>/.tp-review/<base>/ground-inbox/round-<N>.ndjson,
// passes, relative or absolute. Without the allowance every spelling here is
// refused, and the unit cannot write the file its prompt names.
func TestPreToolUseHookAllowsTheGroundInbox(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	paths := []string{
		"spec/.tp-review/0.35.0/ground-inbox/round-1.ndjson",
		".tp-review/a/ground-inbox/round-12.ndjson",
		"./x/.tp-review/a/ground-inbox/round-3.ndjson",
		filepath.Join(root, "spec", ".tp-review", "a", "ground-inbox", "round-1.ndjson"),
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			run := runWriteDenyHookIn(t, root, nil, path)
			assert.Zero(t, run.exitCode, "%s is the ground inbox name; stderr=%q", path, run.stderr)
		})
	}
}

// TestPreToolUseHookAllowsTheRoleInboxes is spec/backlog/role-files-in-state-
// dir.md's plugin half: a review or audit role writes its file in the phase's
// inbox in the spec's state directory, which the fence denies as tp's own
// state, so exactly that name passes and the recorded rounds stay refused.
func TestPreToolUseHookAllowsTheRoleInboxes(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	allowed := []string{
		"spec/.tp-review/0.35.0/review-inbox/round-1-implementer.ndjson",
		".tp-review/a/audit-inbox/round-12-spec-coverage.ndjson",
		filepath.Join(root, "spec", ".tp-review", "a", "review-inbox", "round-3-tester.ndjson"),
	}
	refused := []string{
		".tp-review/a/review-round-1.ndjson",
		".tp-review/a/review-inbox/round-0-tester.ndjson",
		".tp-review/a/review-inbox/round-1.ndjson",
		".tp-review/a/review-inbox/round-1-tester.ndjson.part",
		".tp-review/a/review-inbox/../state.json",
		".tp-review/a/ground-inbox/round-1-tester.ndjson",
	}
	for _, path := range allowed {
		t.Run("allowed "+path, func(t *testing.T) {
			t.Parallel()
			run := runWriteDenyHookIn(t, root, nil, path)
			assert.Zero(t, run.exitCode, "%s is a role inbox name; stderr=%q", path, run.stderr)
		})
	}
	for _, path := range refused {
		t.Run("refused "+path, func(t *testing.T) {
			t.Parallel()
			run := runWriteDenyHookIn(t, root, nil, path)
			assert.True(t, run.denied(), "%s must stay refused; exit=%d stderr=%q", path, run.exitCode, run.stderr)
		})
	}
}

// TestPreToolUseHookRefusesEveryOtherGroundStatePath is the allowance's other
// edge: exactly the inbox name passes, so the recorded round, a round 0, a
// nested or suffixed file, a `..` escape and an inbox with no state directory
// above it all stay refused.
func TestPreToolUseHookRefusesEveryOtherGroundStatePath(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	refused := []string{
		".tp-review/a/ground-round-1.ndjson",
		".tp-review/a/ground-inbox/round-0.ndjson",
		".tp-review/a/ground-inbox/round-1.ndjson.part",
		".tp-review/a/ground-inbox/sub/round-1.ndjson",
		".tp-review/a/ground-inbox/../state.json",
		".tp-review/ground-inbox/round-1.ndjson",
	}
	for _, path := range refused {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			run := runWriteDenyHookIn(t, root, nil, path)
			assert.True(t, run.denied(), "%s must stay refused; exit=%d stderr=%q", path, run.exitCode, run.stderr)
		})
	}
}

// TestPreToolUseHookRefusesTheAuditPools: the pools file an audit emission
// keeps in its inbox (engine.AuditPoolsPath) is tp's own state, written by tp
// and read back at --record, so no role may write it. It sits beside the role
// files the fence lets through and must not ride their allowance.
func TestPreToolUseHookRefusesTheAuditPools(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	for _, path := range []string{
		".tp-review/a/audit-inbox/round-1.pools.json",
		filepath.Join(root, "spec", ".tp-review", "a", "audit-inbox", "round-12.pools.json"),
	} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			run := runWriteDenyHookIn(t, root, nil, path)
			assert.True(t, run.denied(), "%s must stay refused; exit=%d stderr=%q", path, run.exitCode, run.stderr)
		})
	}
}
