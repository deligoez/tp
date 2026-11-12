package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEpicSet_GatePurposeIsWrittenLikeTheThreshold: gates.<m>.purpose is
// written as text, keeps the gate's other fields, an empty value clears it,
// and tp epic lint reports it as no unknown key.
func TestEpicSet_GatePurposeIsWrittenLikeTheThreshold(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicGateFieldGraph(`{"A": {"members": ["a1"], "threshold": "ships"}}`))
	setGateField(t, dir, "gates.A.purpose=prove the import path end to end")
	var g struct {
		Gates map[string]struct {
			Members   []string `json:"members"`
			Threshold string   `json:"threshold"`
			Purpose   *string  `json:"purpose"`
		} `json:"gates"`
	}
	require.NoError(t, json.Unmarshal(readEpicGraphFile(t, dir), &g))
	require.NotNil(t, g.Gates["A"].Purpose)
	assert.Equal(t, "prove the import path end to end", *g.Gates["A"].Purpose)
	assert.Equal(t, "ships", g.Gates["A"].Threshold)
	assert.Equal(t, []string{"a1"}, g.Gates["A"].Members)

	out, stderr, code := runTP(t, dir, "epic", "lint")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.NotContains(t, out, "unknown-key", "purpose is a schema key")

	setGateField(t, dir, "gates.A.purpose=")
	g.Gates = nil
	require.NoError(t, json.Unmarshal(readEpicGraphFile(t, dir), &g))
	assert.Nil(t, g.Gates["A"].Purpose, "an empty value clears the purpose")
}
