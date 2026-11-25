package engine

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// foundFixture writes a task file under a fresh project root and returns both.
// The root is a project (.tp/config.json), as a real one is, so the store's
// lock resolves to one .tp/locks/ however many artifacts .tp/ gains.
func foundFixture(t *testing.T) (root, taskFile string) {
	t.Helper()
	root = t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "config.json"), []byte("{}\n"), 0o600))
	taskFile = filepath.Join(root, "spec", "x.tasks.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(taskFile), 0o755))
	require.NoError(t, os.WriteFile(taskFile, []byte("{\"tasks\":[]}\n"), 0o600))
	return root, taskFile
}

// fullRequest sets every caller-supplied field, so the round trip below
// covers each one.
func fullRequest() *FoundRequest {
	return &FoundRequest{
		Location:     "internal/engine/x.go:42",
		Reproduction: "go test ./internal/engine -run TestX",
		Expected:     "exit 0",
		Observed:     "exit 1",
		Question:     "keep or drop the flag?",
		Alternatives: []string{"keep", "drop"},
		Blocks:       true,
		Finder:       FoundFinder{Task: "t1", AuditRound: 3, RowKey: "go-safety:7"},
	}
}

// assertNoZeroField fails on any zero-valued field of v, recursing into
// structs, so "every field" is a counted claim rather than a hopeful one.
func assertNoZeroField(t *testing.T, prefix string, v reflect.Value) {
	t.Helper()
	for i := range v.NumField() {
		f := v.Field(i)
		name := prefix + v.Type().Field(i).Name
		if f.Kind() == reflect.Struct {
			assertNoZeroField(t, name+".", f)
			continue
		}
		assert.False(t, f.IsZero(), "field %s is zero in the fixture", name)
	}
}

func TestFoundRequestWithEveryFieldRoundTrips(t *testing.T) {
	root, taskFile := foundFixture(t)

	added, err := AddFoundRequest(root, taskFile, fullRequest())
	require.NoError(t, err)
	require.NoError(t, UpdateFoundRequests(root, taskFile, func(reqs []FoundRequest) ([]FoundRequest, error) {
		reqs[0].State = FoundStateUndecided
		reqs[0].Outcome = "found-f1"
		return reqs, nil
	}))
	added.State = FoundStateUndecided
	added.Outcome = "found-f1"
	assertNoZeroField(t, "", reflect.ValueOf(added))

	loaded, err := ReadFoundRequests(root, taskFile)
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	assert.Equal(t, added, loaded[0])
}

func TestFoundRequestsKeepArrivalOrderAndStartUnapplied(t *testing.T) {
	root, taskFile := foundFixture(t)

	for _, loc := range []string{"a.go:1", "b.go:2", "c.go:3"} {
		req := fullRequest()
		req.Location = loc
		req.State = FoundStateApplied // a caller cannot file an applied request
		got, err := AddFoundRequest(root, taskFile, req)
		require.NoError(t, err)
		assert.Equal(t, FoundStateUnapplied, got.State)
	}

	loaded, err := ReadFoundRequests(root, taskFile)
	require.NoError(t, err)
	require.Len(t, loaded, 3)
	for i, want := range []string{"a.go:1", "b.go:2", "c.go:3"} {
		assert.Equal(t, want, loaded[i].Location)
		assert.Equal(t, i+1, loaded[i].Seq)
		assert.Equal(t, FoundStateUnapplied, loaded[i].State)
	}
	assert.Equal(t, []string{"F1", "F2", "F3"}, []string{loaded[0].ID, loaded[1].ID, loaded[2].ID})
}

func TestFoundRequestsWithNoStoreReadEmpty(t *testing.T) {
	root, taskFile := foundFixture(t)
	loaded, err := ReadFoundRequests(root, taskFile)
	require.NoError(t, err)
	assert.Empty(t, loaded)
}

func TestFoundStoreLivesInRunStateNotTheTaskFile(t *testing.T) {
	root, taskFile := foundFixture(t)
	before, err := os.ReadFile(taskFile)
	require.NoError(t, err)

	_, err = AddFoundRequest(root, taskFile, fullRequest())
	require.NoError(t, err)

	after, err := os.ReadFile(taskFile)
	require.NoError(t, err)
	assert.Equal(t, before, after, "the task file is byte-identical after an add")

	path := FoundRequestsPath(root, taskFile)
	assert.Equal(t, filepath.Join(root, ".tp"), filepath.Dir(path))
	_, err = os.Stat(path)
	require.NoError(t, err)

	// git-ignored by the run-state entry, and a .tp/ holding only the store
	// stays a side-effect directory, never a project anchor.
	assert.True(t, strings.HasPrefix(filepath.Base(path), "run-") && strings.HasSuffix(path, ".json"))
	bare := filepath.Join(t.TempDir(), ".tp")
	require.NoError(t, os.MkdirAll(bare, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bare, filepath.Base(path)), []byte("{}\n"), 0o600))
	assert.False(t, isProjectTPDir(bare))
}

func TestFoundStoreNeverSharesAnotherCyclesRunStateFile(t *testing.T) {
	root := t.TempDir()
	mine := filepath.Join(root, "x.tasks.json")
	// The spelling that would collide under a ".found" suffix.
	other := filepath.Join(root, "x.found.tasks.json")
	assert.NotEqual(t, FoundRequestsPath(root, mine), RunStatePath(root, other))
	assert.NotEqual(t, FoundRequestsPath(root, mine), RunStatePath(root, mine))
	assert.NotEqual(t, FoundRequestsPath(root, mine), FoundRequestsPath(root, other))
}

func TestConcurrentFoundAddsLoseNone(t *testing.T) {
	root, taskFile := foundFixture(t)
	const n = 8
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for range n {
		wg.Go(func() {
			_, err := AddFoundRequest(root, taskFile, fullRequest())
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	loaded, err := ReadFoundRequests(root, taskFile)
	require.NoError(t, err)
	require.Len(t, loaded, n)
	for i, r := range loaded {
		assert.Equal(t, i+1, r.Seq)
	}
}
