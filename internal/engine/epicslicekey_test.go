package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEpicSliceKey_EachKindReadsBackAsWritten: spec/1.5.0.md §7 — an emission
// records its slice's key, a milestone or an epic by kind and value, a set by
// its sorted, de-duplicated ids and its name when it has one.
func TestEpicSliceKey_EachKindReadsBackAsWritten(t *testing.T) {
	for _, tc := range []struct {
		key  EpicSliceKey
		want EpicSliceKeyRecord
	}{
		{EpicSliceKey{Milestone: "A"}, EpicSliceKeyRecord{Kind: "milestone", Value: "A"}},
		{EpicSliceKey{Epic: "b"}, EpicSliceKeyRecord{Kind: "epic", Value: "b"}},
		{EpicSliceKey{Epics: []string{"c", "a", "c"}}, EpicSliceKeyRecord{Kind: "epics", IDs: []string{"a", "c"}}},
		{EpicSliceKey{Epics: []string{"b", "a"}, Name: "pair"}, EpicSliceKeyRecord{Kind: "epics", IDs: []string{"a", "b"}, Name: "pair"}},
	} {
		stateDir := filepath.Join(t.TempDir(), "slice")
		require.NoError(t, WriteEpicSliceKey(stateDir, tc.key))
		got, err := ReadEpicSliceKey(stateDir)
		require.NoError(t, err)
		require.NotNil(t, got, "%+v", tc.key)
		assert.Equal(t, tc.want, *got)
	}
}

// TestEpicSliceKey_ASliceWithoutTheFileReadsAsUnknown: a slice recorded
// before v1.5.0 has no key file, and reads as key unknown rather than an error.
func TestEpicSliceKey_ASliceWithoutTheFileReadsAsUnknown(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "slice")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "snapshot-round-1.json"), []byte(`{"round": 1, "units": {}}`+"\n"), 0o600))

	got, err := ReadEpicSliceKey(stateDir)
	require.NoError(t, err)
	assert.Nil(t, got)
}
