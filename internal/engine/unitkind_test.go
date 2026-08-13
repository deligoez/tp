package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeUnitFile writes content at path, creating parent directories.
func writeUnitFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

// unitTaskFile / unitSpec / unitRoundDir name the fixture artifacts a target
// points at inside a test's own directory.
func unitTaskFile(dir string) string { return filepath.Join(dir, "spec.tasks.json") }
func unitSpec(dir string) string     { return filepath.Join(dir, "spec.md") }
func unitRoundDir(dir string) string { return filepath.Join(dir, "rounds", "r3") }

// taskFileJSON renders a task file holding one task with the given status, or
// the empty init shell when status is "".
func taskFileJSON(status string) string {
	if status == "" {
		return `{"version":1,"tasks":[]}`
	}
	return `{"version":1,"tasks":[{"id":"t1","title":"T","status":"` + status + `"}]}`
}

func TestUnitKinds_EightKindsInTableOrder(t *testing.T) {
	want := []UnitKind{
		"implement", "review-role", "review-record", "review-resolve",
		"decompose", "audit-role", "audit-record", "audit-fix",
	}
	assert.Equal(t, want, UnitKinds(), "the eight §3.3 kinds, in table order")

	// The accessor hands out a copy: a caller cannot reorder the shared set.
	got := UnitKinds()
	got[0] = "clobbered"
	assert.Equal(t, want, UnitKinds())

	for _, k := range want {
		parsed, ok := ParseUnitKind(string(k))
		assert.True(t, ok, "%s parses", k)
		assert.Equal(t, k, parsed)
	}
	for _, bad := range []string{"", "Implement", "implement ", "review_role", "run", "role"} {
		_, ok := ParseUnitKind(bad)
		assert.False(t, ok, "%q is not a unit kind", bad)
	}
}

