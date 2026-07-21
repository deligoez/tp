package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseRoleBytes_Valid parses a fully-specified role and a minimal role.
func TestParseRoleBytes_Valid(t *testing.T) {
	full := `{
		"id": "security",
		"title": "Security Reviewer",
		"instructions": "Try to break the spec's trust boundaries.",
		"focus": ["Any command injection in the detector cmd?"],
		"domains": ["software"]
	}`
	role, err := ParseRoleBytes([]byte(full), "security")
	require.NoError(t, err)
	assert.Equal(t, "security", role.ID)
	assert.Equal(t, "Security Reviewer", role.Title)
	assert.Equal(t, []string{"Any command injection in the detector cmd?"}, role.Focus)
	assert.Equal(t, []string{"software"}, role.Domains)

	// Minimal: only the required fields; focus/domains absent.
	minimal := `{"id":"impl-detail","title":"T","instructions":"I"}`
	role, err = ParseRoleBytes([]byte(minimal), "impl-detail")
	require.NoError(t, err)
	assert.Equal(t, "impl-detail", role.ID)
	assert.Empty(t, role.Focus)
	assert.Empty(t, role.Domains)
}

// TestParseRoleBytes_IDRules covers each id-rule violation (§3.2).
func TestParseRoleBytes_IDRules(t *testing.T) {
	cases := []struct {
		name string
		json string
		stem string
		msg  string
	}{
		{"id != stem", `{"id":"security","title":"T","instructions":"I"}`, "auditor", "must equal the filename stem"},
		{"uppercase", `{"id":"Security","title":"T","instructions":"I"}`, "Security", "kebab-case"},
		{"underscore", `{"id":"sec_ops","title":"T","instructions":"I"}`, "sec_ops", "kebab-case"},
		{"leading hyphen", `{"id":"-sec","title":"T","instructions":"I"}`, "-sec", "kebab-case"},
		{"trailing hyphen", `{"id":"sec-","title":"T","instructions":"I"}`, "sec-", "kebab-case"},
		{"missing id", `{"title":"T","instructions":"I"}`, "sec", "missing required field \"id\""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseRoleBytes([]byte(c.json), c.stem)
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.msg)
		})
	}
}

// TestParseRoleBytes_ReservedID rejects a role file named regression (§3.2, §5.2).
func TestParseRoleBytes_ReservedID(t *testing.T) {
	_, err := ParseRoleBytes([]byte(`{"id":"regression","title":"T","instructions":"I"}`), "regression")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reserved")
}

// TestParseRoleBytes_UnknownKey rejects any top-level key outside §3.3, naming it.
func TestParseRoleBytes_UnknownKey(t *testing.T) {
	// An attempt to declare an output field is an unknown key.
	_, err := ParseRoleBytes([]byte(`{"id":"sec","title":"T","instructions":"I","severity":"high"}`), "sec")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown top-level key "severity"`)
}

// TestParseRoleBytes_DomainsShape rejects a non-array domains and missing
// required fields.
func TestParseRoleBytes_DomainsShape(t *testing.T) {
	_, err := ParseRoleBytes([]byte(`{"id":"sec","title":"T","instructions":"I","domains":"software"}`), "sec")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "domains")

	_, err = ParseRoleBytes([]byte(`{"id":"sec","title":"  ","instructions":"I"}`), "sec")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "title")
}

// TestParseRoleFile_StemFromFilename derives the id-defining stem from the file
// name and surfaces the path in the error.
func TestParseRoleFile_StemFromFilename(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "maintainability-conventions.json")
	require.NoError(t, os.WriteFile(good, []byte(`{"id":"maintainability-conventions","title":"T","instructions":"I"}`), 0o600))
	role, err := ParseRoleFile(good)
	require.NoError(t, err)
	assert.Equal(t, "maintainability-conventions", role.ID)

	// id disagreeing with the filename stem fails and the message names the file.
	bad := filepath.Join(dir, "tester.json")
	require.NoError(t, os.WriteFile(bad, []byte(`{"id":"implementer","title":"T","instructions":"I"}`), 0o600))
	_, err = ParseRoleFile(bad)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tester.json")
	assert.Contains(t, err.Error(), "must equal the filename stem")
}
