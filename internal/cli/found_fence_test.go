package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// spec/1.31.0.md §9: under tp run, a unit of any kind but decompose that calls
// tp add, tp import, tp set depends_on or tp found settle exits 2 naming
// tp found add; tp found settle is refused under decompose too.

func fenceUnitEnv(kind string) []string {
	return []string{"TP_UNATTENDED=1", "TP_UNIT_KIND=" + kind, "TP_UNIT_ID=t1"}
}

const fenceAddTask = `{"id":"t3","title":"T3","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["# S"]}`

// assertFenced holds one refusal: exit 2, tp found add named, nothing written.
func assertFenced(t *testing.T, dir string, before []byte, stdout, stderr string, code int) {
	t.Helper()
	assert.Equal(t, 2, code, "stdout=%s stderr=%s", stdout, stderr)
	assert.Contains(t, stdout+stderr, "tp found add", "the refusal names the route out")
	assertTaskFileUnchanged(t, dir, before)
}

// §10 row "The fence": the same tp add is refused under an implement unit and
// lands under a decompose unit, so a fence keyed on TP_UNATTENDED alone fails.
func TestFence_TpAddIsRefusedUnderImplementAndLandsUnderDecompose(t *testing.T) {
	dir, before := foundFixture(t)
	stdout, stderr, code := runTPEnv(t, dir, fenceUnitEnv("implement"), "add", fenceAddTask)
	assertFenced(t, dir, before, stdout, stderr, code)

	stdout, stderr, code = runTPEnv(t, dir, fenceUnitEnv("decompose"), "add", fenceAddTask)
	assert.Equal(t, 0, code, "a decompose unit writes the task graph: stdout=%s stderr=%s", stdout, stderr)
}

func TestFence_OtherUnitKindsAreRefusedTpAdd(t *testing.T) {
	for _, kind := range []string{"review-role", "audit-role", "audit-fix", "review-resolve"} {
		t.Run(kind, func(t *testing.T) {
			dir, before := foundFixture(t)
			stdout, stderr, code := runTPEnv(t, dir, fenceUnitEnv(kind), "add", fenceAddTask)
			assertFenced(t, dir, before, stdout, stderr, code)
		})
	}
}

func TestFence_TpAddBulkIsRefusedUnderImplement(t *testing.T) {
	dir, before := foundFixture(t)
	bulk := filepath.Join(dir, "bulk.ndjson")
	require.NoError(t, os.WriteFile(bulk, []byte(fenceAddTask+"\n"), 0o600))
	stdout, stderr, code := runTPEnv(t, dir, fenceUnitEnv("implement"), "add", "--bulk", bulk)
	assertFenced(t, dir, before, stdout, stderr, code)
}

func TestFence_TpImportIsRefusedUnderImplementAndLandsUnderDecompose(t *testing.T) {
	dir, _ := foundFixture(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.md"), []byte("# B\n"), 0o600))
	doc := filepath.Join(dir, "b.json")
	require.NoError(t, os.WriteFile(doc, []byte(`{"version":1,"spec":"b.md","tasks":[
		{"id":"b1","title":"B1","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["# B"]}
	]}`), 0o600))

	stdout, stderr, code := runTPEnv(t, dir, fenceUnitEnv("implement"), "import", doc)
	assert.Equal(t, 2, code, "stdout=%s stderr=%s", stdout, stderr)
	assert.Contains(t, stdout+stderr, "tp found add")
	assert.NoFileExists(t, filepath.Join(dir, "b.tasks.json"), "a refused import writes nothing")

	stdout, stderr, code = runTPEnv(t, dir, fenceUnitEnv("decompose"), "import", doc)
	assert.Equal(t, 0, code, "decompose units end in tp import: stdout=%s stderr=%s", stdout, stderr)
}

func TestFence_TpSetDependsOnIsRefusedUnderImplement(t *testing.T) {
	dir, before := foundFixture(t)
	stdout, stderr, code := runTPEnv(t, dir, fenceUnitEnv("implement"), "set", "t2", `depends_on=["t1"]`)
	assertFenced(t, dir, before, stdout, stderr, code)

	bulk := filepath.Join(dir, "set.ndjson")
	require.NoError(t, os.WriteFile(bulk, []byte(`{"id":"t2","field":"depends_on","value":["t1"]}`+"\n"), 0o600))
	stdout, stderr, code = runTPEnv(t, dir, fenceUnitEnv("implement"), "set", "--bulk", bulk)
	assertFenced(t, dir, before, stdout, stderr, code)

	// Only the edge is fenced: another field of the same task is still the unit's to write.
	stdout, stderr, code = runTPEnv(t, dir, fenceUnitEnv("implement"), "set", "t2", "title=Renamed")
	assert.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)

	stdout, stderr, code = runTPEnv(t, dir, fenceUnitEnv("decompose"), "set", "t2", `depends_on=["t1"]`)
	assert.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
}

// Round-5 finding: tp found settle is the person's step, so no unit kind may
// call it, decompose included.
func TestFence_TpFoundSettleIsRefusedUnderEveryUnitKind(t *testing.T) {
	for _, kind := range []string{"implement", "decompose", "review-role", "audit-fix"} {
		t.Run(kind, func(t *testing.T) {
			dir, before := foundFixture(t)
			stdout, stderr, code := runTPEnv(t, dir, fenceUnitEnv(kind), "found", "settle", "--reject", "Q1=not a defect")
			assertFenced(t, dir, before, stdout, stderr, code)
		})
	}
}

// Outside a run every fenced command behaves as before.
func TestFence_OutsideARunNothingIsFenced(t *testing.T) {
	dir, _ := foundFixture(t)
	_, stderr, code := runTPEnv(t, dir, nil, "add", fenceAddTask)
	assert.Equal(t, 0, code, stderr)
	_, stderr, code = runTPEnv(t, dir, nil, "set", "t2", `depends_on=["t1"]`)
	assert.Equal(t, 0, code, stderr)

	// tp found settle keeps its own answer: Q1 is unknown, which is its
	// pre-existing refusal, not the fence's.
	stdout, stderr, code := runTPEnv(t, dir, nil, "found", "settle", "--reject", "Q1=not a defect")
	assert.Equal(t, 2, code)
	assert.NotContains(t, stdout+stderr, "tp found add", "outside a run the refusal is settle's own")

	// TP_UNATTENDED with no unit kind is not a unit.
	_, stderr, code = runTPEnv(t, dir, []string{"TP_UNATTENDED=1"}, "add",
		`{"id":"t4","title":"T4","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["# S"]}`)
	assert.Equal(t, 0, code, stderr)

	// Last: a second task file makes discovery ambiguous for the calls above.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.md"), []byte("# B\n"), 0o600))
	doc := filepath.Join(dir, "b.json")
	require.NoError(t, os.WriteFile(doc, []byte(`{"version":1,"spec":"b.md","tasks":[
		{"id":"b1","title":"B1","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["# B"]}
	]}`), 0o600))
	_, stderr, code = runTPEnv(t, dir, nil, "import", doc)
	assert.Equal(t, 0, code, stderr)
}
