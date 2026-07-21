package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAudit_FindingItemCarriesLocation: §3.2 says a `finding` checklist item's
// Section carries the review finding's `location` field.
func TestAudit_FindingItemCarriesLocation(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## S\ntext\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0o600))
	findings := filepath.Join(dir, "f.ndjson")
	require.NoError(t, os.WriteFile(findings, []byte(
		`{"finding":"citation drift in section 3","location":"## 3. Widgets","severity":"high"}`+"\n"+
			`{"finding":"finding without a location field"}`+"\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "a.go", "--findings", findings)
	require.Equal(t, 0, code, "stderr: %s", stderr)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))

	sections := map[string]string{}
	for _, e := range out["checklist"].([]any) {
		m := e.(map[string]any)
		if m["type"] == "finding" {
			sections[m["id"].(string)] = m["section"].(string)
		}
	}
	assert.Equal(t, "## 3. Widgets", sections["finding-0"], "location becomes the item Section")
	assert.Equal(t, "Review Findings", sections["finding-1"], "missing location falls back")
}
