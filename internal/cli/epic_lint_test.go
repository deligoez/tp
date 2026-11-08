package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicLintPayload is the JSON tp epic lint prints (spec/1.3.0.md §4).
type epicLintPayload struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Findings []struct {
		Rule     string   `json:"rule"`
		Severity string   `json:"severity"`
		Epics    []string `json:"epics"`
		Message  string   `json:"message"`
	} `json:"findings"`
}

// epicGraphDir returns a temporary project whose .git bounds .tp/ discovery,
// holding body as .tp/epics.json unless body is empty.
func epicGraphDir(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".tp"), 0o755))
	if body != "" {
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "epics.json"), []byte(body), 0o600))
	}
	return dir
}

// runEpicLint runs tp epic lint in dir and decodes its payload when it printed one.
func runEpicLint(t *testing.T, dir string) (out epicLintPayload, stderr string, code int) {
	t.Helper()
	var stdout string
	stdout, stderr, code = runTP(t, dir, "epic", "lint")
	if code == 0 || code == 1 {
		require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s stderr: %s", stdout, stderr)
	}
	return out, stderr, code
}

// oneDefectPerErrorRule is §9 row 3's hand-written fixture: each of the nine
// error rules has exactly one defect, and no defect trips a second error rule.
const oneDefectPerErrorRule = `{
  "version": 1,
  "milestones": [["A", "B"], ["L1"]],
  "gates": {"A": {"members": ["late"]}},
  "tag_checks": {"ui": {"provided_by": "p"}},
  "epics": [
    {"id": "x", "title": "X", "milestone": "A", "depends_on": [{"id": "y", "why": "x reads y"}]},
    {"id": "y", "title": "Y", "milestone": "A", "depends_on": [{"id": "x", "why": "y reads x"}]},
    {"id": "a", "title": "A step", "milestone": "A", "depends_on": [{"id": "zz", "why": "needs the absent one"}]},
    {"id": "dup", "title": "Dup one", "milestone": "A"},
    {"id": "dup", "title": "Dup two", "milestone": "A"},
    {"id": "k", "title": "K", "milestone": "A", "owner": "someone"},
    {"id": "w", "title": "W", "milestone": "A", "depends_on": [{"id": "k", "why": "   "}]},
    {"id": "nm", "title": "No milestone"},
    {"id": "early", "title": "Early", "milestone": "A", "depends_on": [{"id": "late", "why": "uses the later work"}]},
    {"id": "late", "title": "Late", "milestone": "B"},
    {"id": "p", "title": "Provider", "milestone": "A", "tags": ["ui"]},
    {"id": "screen", "title": "Screen", "milestone": "A", "tags": ["ui"]}
  ]
}
`

// TestEpicLint_OneDefectPerErrorRuleReportsEveryRuleOnce is §9 row 3: a lint
// that stopped at the first error would report one.
func TestEpicLint_OneDefectPerErrorRuleReportsEveryRuleOnce(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, oneDefectPerErrorRule)
	before, err := os.ReadFile(filepath.Join(dir, ".tp", "epics.json"))
	require.NoError(t, err)

	out, stderr, code := runEpicLint(t, dir)
	require.Equal(t, 1, code, "stderr: %s", stderr)

	rules := []string{
		"cycle", "dangling-reference", "duplicate-name", "unknown-key", "missing-why",
		"unknown-milestone", "later-milestone-dependency", "gate-member-outside-milestone",
		"unprovided-tag-check",
	}
	assert.Equal(t, len(rules), out.Errors)
	count := map[string]int{}
	for _, f := range out.Findings {
		if f.Severity == "error" {
			count[f.Rule]++
		}
	}
	for _, rule := range rules {
		assert.Equal(t, 1, count[rule], "error rule %q fires exactly once", rule)
	}

	after, err := os.ReadFile(filepath.Join(dir, ".tp", "epics.json"))
	require.NoError(t, err)
	assert.Equal(t, before, after, "tp epic lint writes nothing")
}

// TestEpicLint_NoGraphFileExits3 is §4's file error: no .tp/epics.json.
func TestEpicLint_NoGraphFileExits3(t *testing.T) {
	t.Parallel()
	_, stderr, code := runEpicLint(t, epicGraphDir(t, ""))
	assert.Equal(t, 3, code, "stderr: %s", stderr)
}

// TestEpicLint_CleanGraphExits0 lints the chain a → b → c → d in milestone A.
func TestEpicLint_CleanGraphExits0(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{
  "version": 1,
  "milestones": [["A", "B"], ["L1"]],
  "epics": [
    {"id": "a", "title": "A", "milestone": "A", "depends_on": [{"id": "b", "why": "a needs b"}]},
    {"id": "b", "title": "B", "milestone": "A", "depends_on": [{"id": "c", "why": "b needs c"}]},
    {"id": "c", "title": "C", "milestone": "A", "depends_on": [{"id": "d", "why": "c needs d"}]},
    {"id": "d", "title": "D", "milestone": "A"}
  ]
}
`)
	out, stderr, code := runEpicLint(t, dir)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Zero(t, out.Errors)
	assert.Zero(t, out.Warnings)
	assert.NotNil(t, out.Findings, "findings encodes as [] on a clean graph")
}
