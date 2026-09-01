package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestUnknownRoleExitsTwoWithTheEmittedNames pins v0.36.0 §4.2.1's exit-2 case:
// a name tp recognises nowhere.
//
// The hint has to carry the names, not just say the role is unknown. A caller
// who misspells a role cannot see the panel any other way — the emission they
// asked for is the one that refused — so a bare "unknown role" leaves them
// running the command again without the flag to find out what to type.
func TestUnknownRoleExitsTwoWithTheEmittedNames(t *testing.T) {
	spec := relocatedSpec(t, "spec/0.36.0.md")
	dir := filepath.Dir(spec)

	// The names this invocation would have emitted, read from the invocation
	// itself rather than written down.
	_, emittedOrder := promptsOf(t, emitPayload(t, spec))

	for _, cmd := range []string{"review", "audit"} {
		t.Run(cmd, func(t *testing.T) {
			args := []string{cmd, filepath.Base(spec), "--role", "no-such-role"}
			if cmd == "audit" {
				args = append(args, "--affected-files", filepath.Base(spec))
			}
			_, stderr, code := runTPIn(t, dir, args...)

			assert.Equal(t, 2, code, "an unrecognised role is a usage error; stderr: %s", stderr)
			assert.Contains(t, stderr, "no-such-role", "the error repeats what the caller typed")
			if cmd == "review" {
				for _, name := range emittedOrder {
					assert.Contains(t, stderr, name,
						"the hint lists %q, one of the names this invocation would have emitted", name)
				}
			}
		})
	}
}

// TestUnknownRoleHintIsNotJustTheCorpus guards the direction that would make
// the hint misleading: `regression` is emitted and belongs to no corpus, so a
// hint built from the corpus would omit the one name a caller is most likely to
// be reaching for after reading §4.1.
func TestUnknownRoleHintIsNotJustTheCorpus(t *testing.T) {
	spec := relocatedSpec(t, "spec/0.36.0.md")
	dir := filepath.Dir(spec)

	_, emittedOrder := promptsOf(t, emitPayload(t, spec))
	if !containsName(emittedOrder, "regression") {
		t.Skip("this round does not emit the regression role, so the case is not present")
	}

	_, stderr, code := runTPIn(t, dir, "review", filepath.Base(spec), "--role", "nope")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "regression",
		"the hint is built from what the invocation emits, which includes the built-in regression role")
}

func containsName(names []string, want string) bool {
	for _, n := range names {
		if strings.EqualFold(n, want) {
			return true
		}
	}
	return false
}
