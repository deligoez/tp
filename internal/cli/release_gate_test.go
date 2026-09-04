package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReleaseGate_PanelEqualityAcrossEverySpec is v0.36.0 §7: property 7's
// whole-panel equality, holding for every spec in the repository rather than
// for a fixture. Failing it blocks the release.
//
// Why breadth. Every mechanism in this release hides, regroups or narrows what
// a reviewer sees -- `--role` most of all -- which is the shape that cost
// v0.34.0 §7.1 eight rounds of suppressed findings. What the repository buys is
// narrower than a draft of the spec claimed and was measured: zero specs carry
// `enabled: false` and zero carry a `domain`, so the gate does not exercise
// those skips. What it does exercise is section structures no fixture author
// would think to write, and both sides of the one round-dependent skip --
// `regression`, absent at round 1 and emitted after -- because some specs here
// carry recorded rounds and some carry none.
//
// Every spec is relocated first. §6.1 S1 forbids the suite mutating live review
// state, and emission writes the round snapshot; the copy carries the spec's
// own `.tp-review/<base>` so the round number and the panel travel with it.
func TestReleaseGate_PanelEqualityAcrossEverySpec(t *testing.T) {
	t.Parallel()
	root := repoRootDir(t)
	specs, err := filepath.Glob(filepath.Join(root, "spec", "*.md"))
	require.NoError(t, err)
	require.NotEmpty(t, specs, "the repository must carry specs for this gate to mean anything")

	var (
		outOfScope []string
		inScope    int
	)

	for _, abs := range specs {
		rel := "spec/" + filepath.Base(abs)
		t.Run(filepath.Base(abs), func(t *testing.T) {
			local := relocatedSpec(t, rel)
			dir := filepath.Dir(local)
			base := filepath.Base(local)

			// A file to audit, inside the sandbox. §7: `tp audit <spec>` on a
			// clean tree exits 4 with "no changed files detected" -- it does
			// not pass vacuously, it does not run at all, and a release tree is
			// clean exactly when this gate fires. Naming the files is what
			// makes the audit half measure anything.
			require.NoError(t, os.WriteFile(filepath.Join(dir, "subject.go"),
				[]byte("package subject\n"), 0o600))

			for _, cmd := range []struct {
				name string
				args []string
			}{
				{"review", []string{"review", base}},
				{"audit", []string{"audit", base, "--affected-files", "subject.go"}},
			} {
				full, ok := emitOrSkip(t, dir, cmd.args...)
				if !ok {
					outOfScope = append(outOfScope, rel+" ("+cmd.name+")")
					continue
				}
				inScope++

				want, order := promptsOf(t, full)
				got := make([]string, 0, len(order))
				for _, role := range order {
					one, oneOK := emitOrSkip(t, dir, append(append([]string{}, cmd.args...), "--role", role)...)
					require.True(t, oneOK, "%s %s --role %s must emit", rel, cmd.name, role)

					bodies, oneOrder := promptsOf(t, one)
					require.Equal(t, []string{role}, oneOrder,
						"%s %s --role %s emits exactly that role", rel, cmd.name, role)
					got = append(got, bodies[role])
				}

				wantOrdered := make([]string, 0, len(order))
				for _, role := range order {
					wantOrdered = append(wantOrdered, want[role])
				}
				assert.Equal(t, wantOrdered, got,
					"%s (%s): the per-role emissions concatenated in the payload's order equal the payload",
					rel, cmd.name)
			}
		})
	}

	// A spec emitting nothing is named, not silently dropped: the gate's own
	// scope is a fact about this repository, and a gate whose coverage shrank
	// without anyone noticing is the failure mode §7 exists to prevent.
	sort.Strings(outOfScope)
	t.Logf("release gate: %d emissions in scope; %d out of scope:\n  %s",
		inScope, len(outOfScope), strings.Join(outOfScope, "\n  "))
	require.Positive(t, inScope, "the gate must measure at least one emission")
}

// emitOrSkip runs one emission and reports whether it produced prompts.
//
// A non-zero exit or an empty prompts[] is not a failure -- some files under
// spec/ are candidate lists and release notes rather than reviewable specs --
// but it IS out of scope, and the caller records it by name.
func emitOrSkip(t *testing.T, dir string, args ...string) (payload map[string]any, ok bool) {
	t.Helper()
	stdout, _, code := runTPIn(t, dir, append(args, "--json")...)
	if code != 0 {
		return nil, false
	}
	if json.Unmarshal([]byte(stdout), &payload) != nil {
		return nil, false
	}
	prompts, isList := payload["prompts"].([]any)
	if !isList || len(prompts) == 0 {
		return nil, false
	}
	return payload, true
}
