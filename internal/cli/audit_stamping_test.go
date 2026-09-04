package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// auditAdvisoryRound is one audit round whose only non-PASS row carries an
// advisory severity. It is the fixture both stamping directions turn on,
// because it is the one round the two policies grade differently: `all` calls
// it unclean for the row's existence, `blocking` calls it clean because
// `warning` is below the audit vocabulary's blocking severity (§2). A round
// whose non-PASS row carried `error`, or none at all, grades identically under
// both and could not tell a stamp from a live re-grade.
const auditAdvisoryRound = `{"role":"spec-coverage","item_id":"i1","status":"PASS","severity":null}` + "\n" +
	`{"role":"spec-coverage","item_id":"i2","status":"PARTIAL","severity":"warning","finding":"advisory","suggestion":"note it"}` + "\n"

// auditStampingProject seeds a tree holding nothing but a spec and a .git
// marker. The marker is load-bearing: .tp/config.json is found by walking up
// to the repository root, so without it a temp-dir project has no root for the
// project layer to be written at.
func auditStampingProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
	return dir
}

// setAuditConvergeOn stores the policy at one named layer, writing the file a
// hand edit or a future write sink would write. The layer is a parameter and
// not a constant because a stamp that reads only one of the two stored layers
// passes the other layer's test: §2 resolves the field task override > project
// config > built-in, and both stored layers must reach the stamp.
func setAuditConvergeOn(t *testing.T, dir, layer, value string) {
	t.Helper()
	switch layer {
	case "project":
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tp"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "config.json"),
			[]byte(`{"workflow":{"audit_converge_on":"`+value+`"}}`), 0o600))
	case "override":
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.tasks.json"),
			[]byte(`{"spec":"spec.md","tasks":[],"workflow":{"audit_converge_on":"`+value+`"}}`), 0o600))
	default:
		t.Fatalf("unknown layer %q", layer)
	}
}

// auditStateBytes returns state.json verbatim. The comparison this feeds is on
// the bytes rather than on a decoded field, so a reporting path that re-grades
// a round AND writes the new verdict back is caught whatever else it changed.
func auditStateBytes(t *testing.T, dir string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".tp-review", "spec", "state.json"))
	require.NoError(t, err)
	return data
}

// auditStatusPayload runs `tp audit spec.md --status` and returns its payload
// with the recorded rounds already unpacked. Both are returned because §7 row 9
// names two observables on this one surface: the rounds the report carries and
// the convergence signals derived from them.
func auditStatusPayload(t *testing.T, dir string) (payload map[string]any, rounds []map[string]any) {
	t.Helper()
	stdout, stderr, code := runTPFence(t, dir, false, "audit", "spec.md", "--status")
	require.Equal(t, 0, code, "audit --status: %s", stderr)
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	raw, _ := payload["audit_rounds"].([]any)
	rounds = make([]map[string]any, 0, len(raw))
	for _, r := range raw {
		entry, ok := r.(map[string]any)
		require.True(t, ok, "audit_rounds holds round objects")
		rounds = append(rounds, entry)
	}
	return payload, rounds
}

// TestAuditRecord_StampSurvivesTighteningTheKnob covers v0.37.0 §7 row 9: two
// rounds recorded under `all`, then the knob set to `blocking` without
// re-recording, must leave every round's `clean` exactly where the stamp put it.
//
// Two rounds and not one, because audit_clean_rounds defaults to 2: a single
// round leaves `converged` false under either reading, so the derived signals
// could not discriminate. With two, the named mutant — re-grading each stored
// round from its rows under the currently resolved policy in the reporting path
// — turns an unconverged cycle into a converged one, which is the harm.
//
// The assertion is deliberately four observables, not one. The stored bytes
// alone cannot see the named mutant at all, since it moves the report and not
// the store; audit_rounds[].clean alone cannot see a mutant that re-grades only
// the derived signals and leaves the array honest, which is the same false
// convergence reached one step later.
func TestAuditRecord_StampSurvivesTighteningTheKnob(t *testing.T) {
	t.Parallel()
	dir := auditStampingProject(t)
	setAuditConvergeOn(t, dir, "project", "all")

	for round := 1; round <= 2; round++ {
		out, stderr, code := auditRecord(t, dir, auditAdvisoryRound)
		require.Equal(t, 0, code, "recording round %d: %s", round, stderr)
		require.Equal(t, false, out["clean"], "under all, a non-PASS row makes round %d unclean", round)
	}
	before := auditStateBytes(t, dir)

	setAuditConvergeOn(t, dir, "project", "blocking")

	payload, rounds := auditStatusPayload(t, dir)
	require.Len(t, rounds, 2, "both rounds are reported")
	for i, r := range rounds {
		assert.Equal(t, false, r["clean"],
			"round %d was stamped under all and the knob does not reach back", i+1)
	}
	assert.Equal(t, float64(0), payload["consecutive_clean"],
		"the streak is walked over stamps, not re-graded rows")
	assert.Equal(t, false, payload["converged"],
		"tightening cannot manufacture a convergence the recorded rounds never had")
	assert.Equal(t, before, auditStateBytes(t, dir),
		"reporting under the new policy rewrote nothing in the store")
}

// TestAuditRecord_StampSurvivesRelaxingTheKnob covers v0.37.0 §7 row 9b, the
// direction §2 says is the one to state: a round recorded under `blocking`
// keeps `clean: true` after the knob returns to `all`. A read-side floor —
// clean only when the round also holds no non-PASS row — satisfies row 9 and
// fails here, which is why the two directions are asserted separately.
//
// The `findings` assertion is part of the contract and not decoration: the
// count stays 1 while the verdict is true, so `clean` is visibly the policy's
// verdict over the rows rather than a restatement of the count. That is the
// equality the pre-release code held — `clean := findings == 0` — and the one
// this row breaks.
//
// Both stored layers are run because a stamp taking its policy from the task
// override alone, or from the project config alone, is green on one subtest and
// red on the other; a single-layer test would ship the half that was dropped.
func TestAuditRecord_StampSurvivesRelaxingTheKnob(t *testing.T) {
	t.Parallel()
	for _, layer := range []string{"project", "override"} {
		t.Run(layer, func(t *testing.T) {
			dir := auditStampingProject(t)
			setAuditConvergeOn(t, dir, layer, "blocking")

			out, stderr, code := auditRecord(t, dir, auditAdvisoryRound)
			require.Equal(t, 0, code, "audit --record: %s", stderr)
			assert.Equal(t, true, out["clean"],
				"under blocking, an advisory row does not block the round")
			assert.Equal(t, float64(1), out["findings"],
				"the non-PASS row is still counted; clean is the verdict, not the count")

			setAuditConvergeOn(t, dir, layer, "all")

			stored := roundEntries(t, dir, "audit_rounds")
			require.Len(t, stored, 1)
			assert.Equal(t, true, stored[0]["clean"],
				"the stamp is a fact about the round, recorded once")

			payload, rounds := auditStatusPayload(t, dir)
			require.Len(t, rounds, 1)
			assert.Equal(t, true, rounds[0]["clean"],
				"and the report reads the stamp rather than re-grading the rows")
			assert.Equal(t, float64(1), payload["consecutive_clean"],
				"one operator-approved blocking round relaxes the streak permanently")
		})
	}
}
