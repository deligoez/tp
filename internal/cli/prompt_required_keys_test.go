package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// promptContractFixture seeds the four files the five commands of §5 row 8 need
// between them, in one temp dir outside the repository.
//
// Two properties of it are deliberate rather than incidental, and the test
// asserts both. The spec prose contains the word "evidence" — as this release's
// own specs do — and sample.go carries it in a comment. Measured, those two
// reach different prompts and neither reaches all five: a panel prompt names
// the spec by path instead of inlining it, so its copy of the word comes from
// tp's own instruction text; the regression prompt embeds changed-section
// bodies, so it gets the spec's; the code-audit prompt embeds the affected file,
// so it gets the excerpt's. The test's decoy table records which is which.
//
// No line of any fixture file is itself a JSON object, so a line that parses as
// one came from the prompt tp generated.
func promptContractFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	spec := "# Prompt Contract Spec\n\n" +
		"## 1. Alpha\n\n" +
		"A finding is believed once it names the evidence behind it.\n\n" +
		"## 2. Beta\n\n" +
		"A second rule, unrelated to the first.\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(spec), 0o600))

	// Alpha's body differs and Beta is absent, so the regression perspective's
	// section diff is non-empty and the prompt is not refused as vacuous.
	baseline := "# Prompt Contract Spec\n\n" +
		"## 1. Alpha\n\n" +
		"An older sentence that said nothing about how a finding is believed.\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "baseline.md"), []byte(baseline), 0o600))

	sample := "package sample\n\n" +
		"// Sample exists so the code-audit excerpt carries the word evidence.\n" +
		"func Sample() int { return 1 }\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sample.go"), []byte(sample), 0o600))

	// One row, resolved fixed: --verify has something to verify and the
	// standalone regression pass has something to guard.
	row := `{"severity":"high","finding":"a bound is unstated","location":"## 1. Alpha",` +
		`"evidence":"read the cited section","category":"completeness",` +
		`"resolved":{"status":"fixed","evidence":"the bound now appears in Alpha"}}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "findings.ndjson"), []byte(row), 0o600))

	return dir
}

// jsonObjectLines returns every line of s that parses as a JSON object, with the
// lines themselves alongside so a caller can excise them.
func jsonObjectLines(s string) (objects []map[string]any, lines []string) {
	for _, line := range strings.Split(s, "\n") {
		var obj map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &obj); err == nil && obj != nil {
			objects = append(objects, obj)
			lines = append(lines, line)
		}
	}
	return objects, lines
}

// TestReviewPromptOutputFormatsNameRequiredKeys is §5 row 8. Every command that
// emits a review finding contract must show the reviewer a key list that names
// all four keys both gates require, because the row a reviewer copies out of
// that line is the row that reaches tp review --merge and tp review <spec>
// --record: one missing key is skipped by the first and refuses the whole file
// at the second.
//
// The fixture is the five commands the row names, not a count of them, and each
// is emitted and its object line PARSED. A substring search of the prompt is not
// the check and could not be: the assertion at the end of each iteration shows
// that with the object line removed the word "evidence" is still in the prompt,
// so strings.Contains would be green against a prompt carrying no contract at
// all. A listed command whose prompt holds no object line fails on the Len
// below rather than being quietly skipped.
func TestReviewPromptOutputFormatsNameRequiredKeys(t *testing.T) {
	t.Parallel()

	// Each case names the exact sentence that carries the word "evidence" into
	// its prompt from somewhere other than the output-format object line, and
	// the test requires that sentence to survive the line being excised. That
	// is what makes a whole-prompt substring search demonstrably useless as a
	// check — and it was already true at HEAD, where no listed format but
	// --verify named the key. Naming the sentence rather than the bare word
	// means an edit that removes the decoy fails here loudly, instead of
	// leaving the rebuttal to be satisfied by whatever text happens to be near.
	//
	// Which sentence it is differs by path, measured rather than assumed: the
	// panel prompts reference the spec by path instead of inlining it, so their
	// copy comes from tp's own read-only instruction; the code-audit prompt
	// embeds the affected file; the regression prompt embeds changed-section
	// bodies; the verify prompt renders the resolution evidence of the fixed
	// finding it was given.
	const harnessDecoy = "report it with its evidence instead of making the edit"
	const excerptDecoy = "excerpt carries the word evidence"
	const specDecoy = "names the evidence behind it"
	const resolvedDecoy = "(evidence: the bound now appears in Alpha)"

	cases := []struct {
		label  string
		args   []string
		decoys []string
	}{
		{"tp review <spec>", []string{"review", "spec.md"}, []string{harnessDecoy}},
		{"tp review <spec> --role <r>", []string{"review", "spec.md", "--role", "tester"}, []string{harnessDecoy}},
		{"tp review <spec> --perspective code-audit",
			[]string{"review", "spec.md", "--perspective", "code-audit", "--affected-files", "sample.go"},
			[]string{excerptDecoy}},
		{"tp review <spec> --perspective regression",
			[]string{"review", "spec.md", "--perspective", "regression", "--diff-from", "baseline.md", "--findings", "findings.ndjson"},
			[]string{specDecoy}},
		{"tp review <spec> --verify", []string{"review", "spec.md", "--verify", "--findings", "findings.ndjson"}, []string{resolvedDecoy}},
	}

	graded := make(map[string]int, len(cases))

	for _, tc := range cases {
		dir := promptContractFixture(t)

		stdout, stderr, code := runTP(t, dir, tc.args...)
		require.Equal(t, 0, code, "%s: %s", tc.label, stderr)

		var result map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &result), tc.label)
		prompts, ok := result["prompts"].([]any)
		require.True(t, ok, "%s: emitted no prompts array", tc.label)
		require.NotEmpty(t, prompts, "%s: emitted no prompt to grade", tc.label)

		for _, p := range prompts {
			entry := p.(map[string]any)
			role := entry["role"].(string)
			prompt := entry["prompt"].(string)

			objects, objectLines := jsonObjectLines(prompt)
			require.Len(t, objects, 1,
				"%s (%s): the prompt carries exactly one JSON object line — the output format a reviewer copies — so the keys asserted below are provably that line's",
				tc.label, role)

			keys := make([]string, 0, len(objects[0]))
			for k := range objects[0] {
				keys = append(keys, k)
			}
			for _, required := range []string{"severity", "finding", "location", "evidence"} {
				assert.Contains(t, keys, required,
					"%s (%s): the output format names every key both gates require; a row copied without it is skipped by --merge and refuses the file at --record",
					tc.label, role)
			}

			// Why the assertion above is on the parsed keys and not on the
			// prompt text: with the object line excised the word is still
			// there on every listed path, so a strings.Contains(prompt,
			// "evidence") is green against a prompt whose contract names no
			// such key. The sentence carrying it is named per case above
			// rather than searched for as a bare word: since this release the
			// format block itself names the four keys in prose beneath the
			// object line, so `rest` contains "evidence" whatever the fixture
			// does, and a bare-word assertion here could no longer fail.
			rest := strings.Replace(prompt, objectLines[0], "", 1)
			for _, decoy := range tc.decoys {
				require.Contains(t, rest, decoy,
					"%s (%s): the fixture must keep the word reachable outside the object line, or this test proves nothing about substring searches",
					tc.label, role)
			}

			graded[tc.label]++
		}
	}

	// Every listed command reached the assertions rather than being skipped by
	// an early return, and the two panel entries are distinct paths: the
	// default emits the whole panel, --role narrows it to the one asked for.
	require.Len(t, graded, len(cases), "every listed command emitted at least one prompt and was graded")
	require.Greater(t, graded["tp review <spec>"], 1,
		"the default panel emits more than one role prompt, so the --role case is a narrowing rather than a repeat of it")
	require.Equal(t, 1, graded["tp review <spec> --role <r>"], "--role emits exactly the role it names")
}
