package model

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReadWriteTaskFile_OriginRoundTrips: a found task's origin comes back
// from a read and a write unchanged, and a task without one gains no key.
func TestReadWriteTaskFile_OriginRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "found.tasks.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"version":1,"spec":"s.md","tasks":[`+
		`{"id":"f1","title":"t","status":"open","acceptance":"a","estimate_minutes":5,`+
		`"origin":{"finder_task":"t1","location":"internal/a.go:40"}},`+
		`{"id":"f2","title":"t","status":"open","acceptance":"a","estimate_minutes":5,`+
		`"origin":{"audit_round":3,"audit_row":"go-safety:file-internal/b.go","location":"internal/b.go:7"}},`+
		`{"id":"s1","title":"t","status":"open","acceptance":"a","estimate_minutes":5,"source_sections":["## A"]}`+
		`]}`), 0o600))

	tf, err := ReadTaskFile(path)
	require.NoError(t, err)
	require.NotNil(t, tf.Tasks[0].Origin)
	assert.Equal(t, Origin{FinderTask: "t1", Location: "internal/a.go:40"}, *tf.Tasks[0].Origin)
	require.NotNil(t, tf.Tasks[1].Origin)
	assert.Equal(t, Origin{AuditRound: 3, AuditRow: "go-safety:file-internal/b.go", Location: "internal/b.go:7"}, *tf.Tasks[1].Origin)
	assert.Nil(t, tf.Tasks[2].Origin)

	require.NoError(t, WriteTaskFile(path, tf))
	again, err := ReadTaskFile(path)
	require.NoError(t, err)
	assert.Equal(t, tf.Tasks[0].Origin, again.Tasks[0].Origin)
	assert.Equal(t, tf.Tasks[1].Origin, again.Tasks[1].Origin)

	plain := filepath.Join(t.TempDir(), "plain.tasks.json")
	require.NoError(t, WriteTaskFile(plain, &TaskFile{Version: 1, Spec: "s.md", Tasks: []Task{tf.Tasks[2]}}))
	data, err := os.ReadFile(plain)
	require.NoError(t, err)
	assert.NotContains(t, string(data), `"origin"`)
}
