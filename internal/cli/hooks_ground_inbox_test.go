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
