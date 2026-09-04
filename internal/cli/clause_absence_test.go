package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// quotedPerspective matches the single-quoted names in tp's own refusal text.
var quotedPerspective = regexp.MustCompile(`'([^']+)'`)

// acceptedPerspectives asks tp what it accepts instead of restating it.
//
// §6.2 property 2 requires the mode list to be derived from the code. A live
// refusal is that derivation: invalidPerspectiveMessage renders it from the
// same slice runReview branches on, and an internal test pins the round-trip.
// A perspective added to the code therefore joins this loop on its own, and one
// added without a required-input entry below fails loudly rather than being
// skipped.
func acceptedPerspectives(t *testing.T, dir, spec string) []string {
	t.Helper()
	_, stderr, code := runTPIn(t, dir, "review", spec, "--perspective", "zzz-not-a-perspective")
	require.Equal(t, 2, code, "an unaccepted perspective is refused")

	var refusal struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(stderr), &refusal), "the refusal is JSON: %s", stderr)

	out := make([]string, 0, 4)
	for _, m := range quotedPerspective.FindAllStringSubmatch(refusal.Error, -1) {
		out = append(out, m[1])
	}
	require.NotEmpty(t, out, "the refusal names the accepted perspectives: %s", refusal.Error)
	return out
}

// perspectiveInputs are the flags each perspective requires before it will
// emit. The requirement belongs to the code -- each is enforced with its own
// exit-2 refusal -- and the pairing is this test's fixture.
//
// A perspective the code accepts and this map does not cover produces no entry
// and fails the require below, which is the direction that matters: a new mode
// cannot slip past the property by being silently skipped.
var perspectiveInputs = map[string][]string{
	"documentation": {"--docs-path", "."},
	"testing":       {"--test-path", "."},
	"code-audit":    {"--affected-files"}, // the spec's own name is appended
	"regression":    nil,
}

// TestNoPromptWithoutAnOutputPathEndsWithTheSuffix is §6.2 property 2.
//
// It is POSITIONAL -- "does not end with" -- and not a containment check, for
// the reason property 3 gives: a containment form is forgeable by the document
// under review, because tp embeds changed spec sections verbatim and this
// release's spec quotes both clauses. A reviewer's claim that the containment
// form is false today did not survive checking (measured: zero occurrences in
// the perspective prompts), so the form is replaced for the hazard rather than
// for the failure.
func TestNoPromptWithoutAnOutputPathEndsWithTheSuffix(t *testing.T) {
	t.Parallel()
	spec := relocatedSpec(t, "spec/0.36.0.md")
	dir := filepath.Dir(spec)
	base := filepath.Base(spec)
	suffix := clauseSuffixFromSpec(t)

	modes := map[string][]string{}
	for _, p := range acceptedPerspectives(t, dir, base) {
		extra, known := perspectiveInputs[p]
		require.True(t, known, "perspective %q has no required-input entry; add one", p)
		args := append([]string{"--perspective", p}, extra...)
		if p == "code-audit" {
			args = append(args, base)
		}
		modes["perspective="+p] = args
	}
	modes["verify"] = []string{"--verify", "--findings", writeOneFinding(t, dir)}

	for name, extra := range modes {
		t.Run(name, func(t *testing.T) {
			stdout, stderr, code := runTPIn(t, dir,
				append([]string{"review", base, "--json"}, extra...)...)
			require.Equal(t, 0, code, "the mode must emit, not refuse: %s", stderr)

			var payload map[string]any
			require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
			bodies, order := promptsOf(t, payload)
			require.NotEmpty(t, order, "the mode emits at least one prompt")

			empties := 0
			raw, _ := payload["prompts"].([]any)
			for i, entry := range raw {
				p, _ := entry.(map[string]any)
				out, _ := p["output_path"].(string)
				if out != "" {
					continue
				}
				empties++
				assert.False(t, strings.HasSuffix(bodies[order[i]], suffix),
					"%s: the %s prompt names no output file, so it does not end with the clause suffix",
					name, order[i])
			}
			require.NotZero(t, empties,
				"%s must produce at least one prompt with an empty output_path, or it measures nothing", name)
		})
	}
}

// writeOneFinding produces the minimal --findings input --verify needs.
func writeOneFinding(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "findings.ndjson")
	row := `{"id":"f1","severity":"high","location":"§1","claim":"placeholder"}` + "\n"
	require.NoError(t, os.WriteFile(path, []byte(row), 0o600))
	return path
}
