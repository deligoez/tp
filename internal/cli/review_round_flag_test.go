package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// roundFlagFixture is a fresh project holding a spec, an empty findings file
// --record accepts, a one-row findings file the positional modes accept, the
// docs/ and tests/ directories the documentation and testing perspectives
// require, and the base.md baseline standalone regression diffs against, so
// every refusing invocation would run if not refused.
func roundFlagFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n\n## One\n\ntext\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "base.md"), []byte("# Spec\n\n## One\n\nold\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "empty.ndjson"), nil, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "findings.ndjson"),
		[]byte(`{"evidence":"read the cited section","severity":"low","location":"§1","class":"c","finding":"f"}`+"\n"), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "docs"), 0o750))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "tests"), 0o750))
	return dir
}

// topLevelBytes lists dir's entries with each file's bytes, so a new file, a
// new directory, or a rewritten fixture all show up as a difference.
func topLevelBytes(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	listing := make(map[string]string, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			listing[e.Name()+"/"] = ""
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, e.Name()))
		require.NoError(t, readErr)
		listing[e.Name()] = string(data)
	}
	return listing
}

// roundRefusal is one tp review invocation that refuses --round: its own
// arguments, the refusal it must print, and the word the flag's help line uses
// to name it.
type roundRefusal struct {
	args     []string
	message  string
	helpWord string
}

// roundRefusals lists every tp review invocation that refuses --round, each
// with the arguments it needs to run if the refusal were missing.
var roundRefusals = map[string]roundRefusal{
	"merge":       {[]string{"review", "--merge", "findings.ndjson", "-o", "merged.ndjson"}, "--merge is mutually exclusive with --round", "--merge"},
	"resolve":     {[]string{"review", "findings.ndjson", "--resolve", "0", "fixed", "e"}, "--resolve is mutually exclusive with --round", "--resolve"},
	"resolve-all": {[]string{"review", "findings.ndjson", "--resolve-all", "fixed", "e"}, "--resolve-all is mutually exclusive with --round", "--resolve-all"},
	"report":      {[]string{"review", "--report", "findings.ndjson"}, "--report is mutually exclusive with --round", "--report"},
	"record":      {[]string{"review", "spec.md", "--record", "empty.ndjson"}, "--record is mutually exclusive with --round", "--record"},
	"status":      {[]string{"review", "spec.md", "--status"}, "--status is mutually exclusive with --round", "--status"},
	"verify":      {[]string{"review", "--verify", "spec.md", "--findings", "findings.ndjson"}, "--verify is mutually exclusive with --round", "--verify"},
	"perspective-documentation": {[]string{"review", "spec.md", "--perspective", "documentation", "--docs-path", "docs"},
		"--perspective is mutually exclusive with --round/--findings (except code-audit)", "documentation"},
	"perspective-testing": {[]string{"review", "spec.md", "--perspective", "testing", "--test-path", "tests"},
		"--perspective is mutually exclusive with --round/--findings (except code-audit)", "testing"},
	"perspective-regression": {[]string{"review", "spec.md", "--perspective", "regression", "--diff-from", "base.md", "--findings", "findings.ndjson"},
		"--perspective regression is mutually exclusive with --round", "regression"},
}

// TestReviewModesRefuseRoundWhenPassed pins that an invocation refusing
// --round refuses the flag being passed, not a value other than the default.
// The refusal used to test `round != 1`, so `--record f --round 1` recorded
// round 1 and exited 0 while `--round 9` was refused: --round 1 is the value
// that separates the two rules, and --round 5 keeps the old refusal pinned.
func TestReviewModesRefuseRoundWhenPassed(t *testing.T) {
	t.Parallel()
	for name, tc := range roundRefusals {
		for _, value := range []string{"1", "5"} {
			t.Run(name+"/round="+value, func(t *testing.T) {
				t.Parallel()
				dir := roundFlagFixture(t)
				before := topLevelBytes(t, dir)

				_, stderr, code := runTP(t, dir, append(slices.Clone(tc.args), "--round", value)...)

				assert.Equal(t, 2, code, "%s must refuse a passed --round; stderr: %s", name, stderr)
				assert.Contains(t, stderr, tc.message)
				assert.Equal(t, before, topLevelBytes(t, dir), "a refused --round must write nothing")
				assert.Nil(t, stateTreeBytes(t, dir), "a refused --round must record no round")
			})
		}
	}
}

// TestReviewRoundHelpNamesTheRefusingModes binds the flag's help text to the
// table above: the --round line names every invocation that refuses the flag.
func TestReviewRoundHelpNamesTheRefusingModes(t *testing.T) {
	t.Parallel()
	stdout, stderr, code := runTP(t, t.TempDir(), "review", "--help")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	var roundLine string
	for line := range strings.SplitSeq(stdout, "\n") {
		if strings.Contains(line, "--round int") {
			roundLine = line
		}
	}
	require.NotEmpty(t, roundLine, "tp review --help lists --round")
	named := map[string]bool{}
	for _, token := range regexp.MustCompile(`(?:--)?[a-z][a-z-]*`).FindAllString(roundLine, -1) {
		named[token] = true
	}
	require.True(t, named["--perspective"], "the --round help names --perspective: %s", roundLine)
	for name, tc := range roundRefusals {
		assert.True(t, named[tc.helpWord], "the --round help names %s (%q) as refusing it: %s", name, tc.helpWord, roundLine)
	}
}

// TestReviewEmissionsStillEmit is the control: the default panel and
// --perspective code-audit take --round, and on a fresh spec --round 1 agrees
// with the state-derived round; the documentation, testing and regression
// perspectives still emit without it. Refusing too much fails here.
func TestReviewEmissionsStillEmit(t *testing.T) {
	t.Parallel()
	cases := map[string][]string{
		"default --round 1":    {"review", "spec.md", "--round", "1"},
		"code-audit --round 1": {"review", "spec.md", "--perspective", "code-audit", "--affected-files", "spec.md", "--round", "1"},
		"documentation":        {"review", "spec.md", "--perspective", "documentation", "--docs-path", "docs"},
		"testing":              {"review", "spec.md", "--perspective", "testing", "--test-path", "tests"},
		"regression":           {"review", "spec.md", "--perspective", "regression", "--diff-from", "base.md", "--findings", "findings.ndjson"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stdout, stderr, code := runTP(t, roundFlagFixture(t), args...)

			require.Equal(t, 0, code, "%s must emit; stderr: %s", name, stderr)
			var out struct {
				Prompts []any `json:"prompts"`
			}
			require.NoError(t, json.Unmarshal([]byte(stdout), &out))
			assert.NotEmpty(t, out.Prompts, "%s emits its prompts", name)
		})
	}
}

// TestReviewEmissionAcceptsRoundOne pins the default panel's round number:
// the accepted --round 1 is the round the payload reports.
func TestReviewEmissionAcceptsRoundOne(t *testing.T) {
	t.Parallel()
	stdout, stderr, code := runTP(t, roundFlagFixture(t), "review", "spec.md", "--round", "1")

	require.Equal(t, 0, code, "plain emission must accept --round 1; stderr: %s", stderr)
	var out struct {
		ReviewLoop struct {
			Round int `json:"round"`
		} `json:"review_loop"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, 1, out.ReviewLoop.Round)
}
