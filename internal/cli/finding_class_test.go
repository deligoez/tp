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

func TestFindingClass_MergePreservesAndFirstNonEmptyWins(t *testing.T) {
	dir := t.TempDir()

	// Same identity key (category+location+finding); r1 row has no class,
	// r2 duplicate carries one, r2 second row keeps its own class.
	r1 := filepath.Join(dir, "r1.ndjson")
	require.NoError(t, os.WriteFile(r1, []byte(
		`{"severity":"low","category":"consistency","location":"L1","finding":"dup finding","suggestion":"fix"}`+"\n"), 0o600))
	r2 := filepath.Join(dir, "r2.ndjson")
	require.NoError(t, os.WriteFile(r2, []byte(
		`{"severity":"high","category":"consistency","location":"L1","finding":"dup finding","suggestion":"fix","class":"code-citation-drift"}`+"\n"+
			`{"severity":"low","category":"ambiguity","location":"L9","finding":"solo finding","suggestion":"fix","class":"vague-wording"}`+"\n"), 0o600))

	out := filepath.Join(dir, "merged.ndjson")
	_, stderr, code := runTP(t, dir, "review", "--merge", r1, r2, "-o", out)
	require.Equal(t, 0, code, "merge failed: %s", stderr)

	data, err := os.ReadFile(out)
	require.NoError(t, err)

	byFinding := map[string]map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &m))
		byFinding[m["finding"].(string)] = m
	}
	require.Len(t, byFinding, 2)

	// Dedup kept high severity from r2; class is the first non-empty in
	// merge order (r2's, since r1's row had none).
	dup := byFinding["dup finding"]
	assert.Equal(t, "high", dup["severity"])
	assert.Equal(t, "code-citation-drift", dup["class"])

	solo := byFinding["solo finding"]
	assert.Equal(t, "vague-wording", solo["class"], "merge preserves class on non-dedup rows")
}

func TestFindingClass_ResolvePreservesClass(t *testing.T) {
	dir := t.TempDir()
	findings := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(findings, []byte(
		`{"severity":"low","category":"consistency","location":"L1","finding":"classed finding","suggestion":"fix","class":"code-citation-drift"}`+"\n"), 0o600))

	_, stderr, code := runTP(t, dir, "review", "--resolve", findings, "0", "fixed", "evidence: adjusted the citation")
	require.Equal(t, 0, code, "resolve failed: %s", stderr)

	data, err := os.ReadFile(findings)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(string(data))), &m))
	assert.Equal(t, "code-citation-drift", m["class"], "resolve keeps the class field")
	require.NotNil(t, m["resolved"])
}
