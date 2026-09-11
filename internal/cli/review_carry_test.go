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

// TestReviewCarry_AcceptedRowsHaveTheirOwnHeader: a finding resolved wontfix
// with evidence is carried into the next round under the accepted header, with
// its reason, and not under "UNRESOLVED ... DO NOT re-report", which keeps
// only the finding still open. Before, both sat under the UNRESOLVED header.
func TestReviewCarry_AcceptedRowsHaveTheirOwnHeader(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))

	_, stderr, code := recordRound(t, dir,
		`{"evidence":"read the cited section","severity":"high","category":"consistency","location":"spec.md:5","finding":"Finding A about exit codes","suggestion":"s"}`+"\n"+
			`{"evidence":"read the cited section","severity":"high","category":"completeness","location":"spec.md:6","finding":"Finding B about parser","suggestion":"s"}`+"\n")
	require.Equal(t, 0, code, "record failed: %s", stderr)

	roundFile := filepath.Join(dir, ".tp-review", "spec", "review-round-1.ndjson")
	_, stderr, code = runTP(t, dir, "review", roundFile, "--resolve", "0", "wontfix", "the spec intends this; operator accepted")
	require.Equal(t, 0, code, "resolve failed: %s", stderr)

	stdout, stderr, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code, "round 2 emission failed: %s", stderr)
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	carried := 0
	for _, p := range result["prompts"].([]any) {
		prompt := p.(map[string]any)["prompt"].(string)
		unresolved := carrySection(prompt, "UNRESOLVED findings from previous rounds")
		if unresolved == "" {
			continue
		}
		carried++
		assert.Contains(t, unresolved, "Finding B about parser")
		assert.NotContains(t, unresolved, "Finding A", "an accepted row is not listed as unresolved")

		accepted := carrySection(prompt, "ACCEPTED findings from previous rounds")
		assert.Contains(t, accepted, "Finding A about exit codes")
		assert.Contains(t, accepted, "the spec intends this; operator accepted")
		assert.NotContains(t, accepted, "Finding B")
	}
	require.Positive(t, carried, "round 2 carries the previous findings")
}

// carrySection returns the block of prompt from the line that starts with
// header up to the first blank line after it, or "" when no line starts with
// header.
func carrySection(prompt, header string) string {
	lines := strings.Split(prompt, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, header) {
			continue
		}
		end := i + 1
		for end < len(lines) && strings.TrimSpace(lines[end]) != "" {
			end++
		}
		return strings.Join(lines[i:end], "\n")
	}
	return ""
}
