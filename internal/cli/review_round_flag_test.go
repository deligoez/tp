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
// --record accepts, and a one-row findings file the positional modes accept,
// so every refusing mode gets its own arguments and would run if not refused.
func roundFlagFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n\n## One\n\ntext\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "empty.ndjson"), nil, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "findings.ndjson"),
		[]byte(`{"evidence":"read the cited section","severity":"low","location":"§1","class":"c","finding":"f"}`+"\n"), 0o600))
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

// roundRefusingModes maps each tp review mode that refuses --round to its own
// arguments, so the mode would run if the refusal were missing.
var roundRefusingModes = map[string][]string{
	"merge":       {"review", "--merge", "findings.ndjson", "-o", "merged.ndjson"},
	"resolve":     {"review", "findings.ndjson", "--resolve", "0", "fixed", "e"},
	"resolve-all": {"review", "findings.ndjson", "--resolve-all", "fixed", "e"},
	"report":      {"review", "--report", "findings.ndjson"},
	"record":      {"review", "spec.md", "--record", "empty.ndjson"},
	"status":      {"review", "spec.md", "--status"},
	"verify":      {"review", "--verify", "spec.md", "--findings", "findings.ndjson"},
}

// TestReviewModesRefuseRoundWhenPassed pins that a mode refusing --round
// refuses the flag being passed, not a value other than the default. The
// refusal used to test `round != 1`, so `--record f --round 1` recorded round 1
// and exited 0 while `--round 9` was refused: --round 1 is the value that
// separates the two rules, and --round 2 keeps the old refusal pinned.
func TestReviewModesRefuseRoundWhenPassed(t *testing.T) {
	t.Parallel()
	for mode, args := range roundRefusingModes {
		for _, value := range []string{"1", "2"} {
			t.Run(mode+"/round="+value, func(t *testing.T) {
				t.Parallel()
				dir := roundFlagFixture(t)
				before := topLevelBytes(t, dir)

				_, stderr, code := runTP(t, dir, append(slices.Clone(args), "--round", value)...)

				assert.Equal(t, 2, code, "--%s must refuse a passed --round; stderr: %s", mode, stderr)
				assert.Contains(t, stderr, "--"+mode+" is mutually exclusive with --round")
				assert.Equal(t, before, topLevelBytes(t, dir), "a refused --round must write nothing")
				assert.Nil(t, stateTreeBytes(t, dir), "a refused --round must record no round")
			})
		}
	}
}

// TestReviewRoundHelpNamesTheRefusingModes binds the flag's help text to the
// table above: the --round line names every mode that refuses the flag.
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
	for _, token := range regexp.MustCompile(`--[a-z][a-z-]*`).FindAllString(roundLine, -1) {
		named[token] = true
	}
	for mode := range roundRefusingModes {
		assert.True(t, named["--"+mode], "the --round help names --%s as refusing it: %s", mode, roundLine)
	}
}

// TestReviewEmissionAcceptsRoundOne is the control: plain emission is the mode
// that takes --round, and on a fresh spec --round 1 agrees with the
// state-derived round, so refusing the flag everywhere would fail here.
func TestReviewEmissionAcceptsRoundOne(t *testing.T) {
	t.Parallel()
	dir := roundFlagFixture(t)

	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--round", "1")

	require.Equal(t, 0, code, "plain emission must accept --round 1; stderr: %s", stderr)
	var out struct {
		ReviewLoop struct {
			Round int `json:"round"`
		} `json:"review_loop"`
		Prompts []any `json:"prompts"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, 1, out.ReviewLoop.Round)
	assert.NotEmpty(t, out.Prompts, "the accepted --round 1 emits the round's prompts")
}
