package cli_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicExportRaw runs tp epic export in dir and returns its top-level keys raw.
func epicExportRaw(t *testing.T, dir string) map[string]json.RawMessage {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "epic", "export")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var p map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &p), "stdout: %s", stdout)
	return p
}

// TestEpicExport_PrintsGeneratedAtInUTCAndKeepsVersion2 is spec/1.8.0.md §2:
// tp epic export prints generated_at, an RFC 3339 time in UTC, and its
// version stays 2, since the key is added rather than renamed. An export
// without the key, or stamped in local time, turns it red.
func TestEpicExport_PrintsGeneratedAtInUTCAndKeepsVersion2(t *testing.T) {
	t.Parallel()
	before := time.Now().UTC().Truncate(time.Second)
	p := epicExportRaw(t, boundEpicDir(t, epicChain))

	require.Contains(t, p, "generated_at")
	var raw string
	require.NoError(t, json.Unmarshal(p["generated_at"], &raw))
	at, err := time.Parse(time.RFC3339, raw)
	require.NoError(t, err, "generated_at is RFC 3339: %s", raw)
	assert.Equal(t, 'Z', rune(raw[len(raw)-1]), "generated_at is in UTC: %s", raw)
	assert.False(t, at.Before(before), "generated_at is the export's own time")
	assert.JSONEq(t, `2`, string(p["version"]))
}

// TestEpicExport_TwoExportsDifferInGeneratedAtAlone is §2's comparison rule:
// two exports of an unchanged repository are byte-identical once
// generated_at is left out, so a push that compares them sees no change.
func TestEpicExport_TwoExportsDifferInGeneratedAtAlone(t *testing.T) {
	t.Parallel()
	dir := exportWithBRecorded(t)
	first := epicExportRaw(t, dir)
	time.Sleep(10 * time.Millisecond)
	second := epicExportRaw(t, dir)

	require.Contains(t, first, "generated_at")
	require.Contains(t, second, "generated_at")
	assert.NotEqual(t, string(first["generated_at"]), string(second["generated_at"]),
		"generated_at is the time of each export")
	delete(first, "generated_at")
	delete(second, "generated_at")
	assert.Equal(t, first, second)
}
