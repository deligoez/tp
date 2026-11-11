package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTPConfig writes tpDir/config.json and returns tpDir.
func writeTPConfig(t *testing.T, body string) string {
	t.Helper()
	tp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tp, "config.json"), []byte(body), 0o600))
	return tp
}

// TestProjectPlaceholderMatcher_UsesTheConfiguredPattern: the project's
// epic.threshold_placeholder is the pattern the matcher uses, read without a
// warning, and replaces the default.
func TestProjectPlaceholderMatcher_UsesTheConfiguredPattern(t *testing.T) {
	tp := writeTPConfig(t, `{"workflow":{},"epic":{"threshold_placeholder":"^draft:"}}`)
	pc, warns, err := LoadProjectConfig(tp)
	require.NoError(t, err)
	assert.Empty(t, warns, "epic is a known top-level key")
	require.NotNil(t, pc.Epic)
	require.NotNil(t, pc.Epic.ThresholdPlaceholder)
	assert.Equal(t, "^draft:", *pc.Epic.ThresholdPlaceholder)

	m, err := ProjectPlaceholderMatcher(tp)
	require.NoError(t, err)
	assert.True(t, m.IsPlaceholder("draft: 80% pass"))
	assert.False(t, m.IsPlaceholder("TBD"))
}

// TestProjectPlaceholderMatcher_WithoutTheKeyUsesTheDefault: a config without
// the key, or no config at all, uses the default pattern.
func TestProjectPlaceholderMatcher_WithoutTheKeyUsesTheDefault(t *testing.T) {
	for name, tp := range map[string]string{
		"no key":    writeTPConfig(t, `{"workflow":{"review_max_rounds":3}}`),
		"no epic":   writeTPConfig(t, `{"epic":{}}`),
		"no config": t.TempDir(),
	} {
		m, err := ProjectPlaceholderMatcher(tp)
		require.NoError(t, err, name)
		assert.True(t, m.IsPlaceholder("TBD"), name)
		assert.False(t, m.IsPlaceholder("draft: 80% pass"), name)
	}
}

// TestProjectPlaceholderMatcher_APatternThatDoesNotCompileNamesTheKey: the
// error names the key and the compile error.
func TestProjectPlaceholderMatcher_APatternThatDoesNotCompileNamesTheKey(t *testing.T) {
	tp := writeTPConfig(t, `{"epic":{"threshold_placeholder":"(["}}`)
	_, err := ProjectPlaceholderMatcher(tp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "epic.threshold_placeholder")
	assert.Contains(t, err.Error(), "missing closing")
}

// TestLoadProjectConfig_AnUnknownOrWrongTypedEpicKeyWarns: an epic key
// outside the schema and a non-string pattern warn and are ignored, as a
// workflow key is.
func TestLoadProjectConfig_AnUnknownOrWrongTypedEpicKeyWarns(t *testing.T) {
	tp := writeTPConfig(t, `{"epic":{"threshold_placeholder":3,"bogus":1}}`)
	pc, warns, err := LoadProjectConfig(tp)
	require.NoError(t, err)
	assert.Contains(t, warns, "unknown epic key: bogus")
	assert.Contains(t, warns, "epic.threshold_placeholder: expected a string, ignored")
	if pc.Epic != nil {
		assert.Nil(t, pc.Epic.ThresholdPlaceholder)
	}
}
