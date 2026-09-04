package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// §8a.5 / test 31: an arity violation is a usage error, not a run that failed.
// Every command's Args validator therefore exits 2, so exit 2 means uniformly
// "tp did not run the request" and exit 1 stays "it ran and failed" — the
// distinction an unattended driver branches on. Before this, cobra's validators
// returned a plain error that the dispatcher classified as exit 1.
func TestArity_EveryCommandExitsTwo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cases := []struct {
		name string
		args []string
	}{
		{"show with no id", []string{"show"}},
		{"show with two ids", []string{"show", "a", "b"}},
		{"remove with no id", []string{"remove"}},
		{"reopen with no id", []string{"reopen"}},
		{"close with no id", []string{"close"}},
		{"commit with three positionals", []string{"commit", "a", "b", "c"}},
		{"lint with no spec", []string{"lint"}},
		{"import with no file", []string{"import"}},
		{"init with two specs", []string{"init", "a.md", "b.md"}},
		{"add with two positionals", []string{"add", "{}", "{}"}},
		{"use with two files", []string{"use", "a.tasks.json", "b.tasks.json"}},
		{"brief with two ids", []string{"brief", "a", "b"}},
		{"resume with two specs", []string{"resume", "a.md", "b.md"}},
		{"run with two specs", []string{"run", "a.md", "b.md"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr, code := runTP(t, dir, tc.args...)
			e := errJSON(t, stderr)
			assert.Equal(t, 2, code, "an arity violation is a usage error")
			assert.Equal(t, float64(2), e["code"], "the error object carries the same code")
			assert.NotEmpty(t, e["hint"], "a usage error must carry a hint (§13.2)")
		})
	}
}

// §8a.5 / test 31: "including cobra's own built-ins" — the help and completion
// commands cobra registers lazily inside Execute carry Args validators of their
// own (cobra.NoArgs on each completion shell). They are registered after
// NewRootCmd returns, so a wrap that ran at construction time would miss them
// and leave the built-ins on exit 1.
func TestArity_CobraBuiltinExitsTwo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, stderr, code := runTP(t, dir, "completion", "bash", "extra")
	e := errJSON(t, stderr)
	assert.Equal(t, 2, code, "a cobra built-in's arity violation exits 2 too")
	assert.Equal(t, float64(2), e["code"])
	assert.Contains(t, e["hint"], "tp completion bash", "the hint names the command that refused")
}

// §8a.5: "tp show's arity hint names the missing argument rather than claiming
// to name the failing object." The exit-1 default hint pointed at the task file
// and at tp validate, neither of which is at fault when the id was never typed.
func TestArity_ShowHintNamesTheMissingArgument(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, stderr, code := runTP(t, dir, "show")
	e := errJSON(t, stderr)
	require.Equal(t, 2, code)
	hint, ok := e["hint"].(string)
	require.True(t, ok, "hint must be present")
	assert.Contains(t, hint, "<id>", "the hint names the argument that is missing")
	assert.Contains(t, hint, "tp show", "and the command it belongs to")
	assert.NotContains(t, hint, "tp validate", "it must not claim the task file is at fault")
}

// §8a.5 (negative): reclassifying arity must not turn a command that ran and
// failed into a usage error — the whole point of the split is that a driver can
// branch on it. A correct argument count that then fails keeps its own code.
func TestArity_CorrectArgCountKeepsItsOwnExitCode(t *testing.T) {
	t.Parallel()
	dir := initEntryProject(t)
	_, _, code := runTP(t, dir, "show", "does-not-exist")
	assert.Equal(t, 4, code, "a well-formed invocation still reports its own failure")
}
