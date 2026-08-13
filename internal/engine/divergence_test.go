package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// divergenceFixture is a repository-shaped fixture for §2.4: a spec, an auditor
// corpus that really hashes, and recorded audit rounds stamped with both hashes
// the way the record path stamps them. The tests that use it derive every
// condition input from it through the same helpers the audit outputs call, so a
// fixture state that could not arise in a real repository cannot be asserted
// through it either.
type divergenceFixture struct {
	root     string
	specPath string
	rounds   []ReviewRound
}

// newDivergenceFixture writes one recorded round per NDJSON body, oldest first.
// Each round carries the corpus hash and spec hash computed at fixture time and
// the `clean` flag the record path would have stored — findings == 0 — so
// engine.Converged and engine.StateStale see the state they would see in a real
// repository.
func newDivergenceFixture(t *testing.T, bodies ...string) *divergenceFixture {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o750))
	auditors := filepath.Join(root, ".tp", "auditors")
	require.NoError(t, os.MkdirAll(auditors, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(auditors, "spec-coverage.json"),
		[]byte(`{"id":"spec-coverage","title":"Spec coverage","instructions":"check the spec"}`), 0o600))

	specPath := filepath.Join(root, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# spec\n"), 0o600))

	rolesHash, err := ComputeRolesHash(root, PhaseAuditors)
	require.NoError(t, err)
	specHash, err := SpecHash(specPath)
	require.NoError(t, err)

	stateDir := ReviewStateDir(specPath)
	require.NoError(t, os.MkdirAll(stateDir, 0o750))

	f := &divergenceFixture{root: root, specPath: specPath}
	for i, body := range bodies {
		name := fmt.Sprintf("audit-round-%d.ndjson", i+1)
		require.NoError(t, os.WriteFile(filepath.Join(stateDir, name), []byte(body), 0o600))
		findings := countNonPassRows(t, body)
		f.rounds = append(f.rounds, ReviewRound{
			Round:     i + 1,
			Findings:  findings,
			Clean:     findings == 0,
			File:      name,
			SpecHash:  specHash,
			RolesHash: rolesHash,
		})
	}
	return f
}

// countNonPassRows is the record path's finding count over a round body.
func countNonPassRows(t *testing.T, body string) int {
	t.Helper()
	n := 0
	for line := range strings.SplitSeq(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var row map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &row))
		if !AuditRowIsPass(row) {
			n++
		}
	}
	return n
}

// inputs assembles DivergenceInputs the way an audit output assembles them:
// every condition's value comes from the helper that output already calls, with
// the corpus hash computed now (the --status source; on --record the same value
// is read back off the just-recorded round and the equality holds by
// construction).
func (f *divergenceFixture) inputs(t *testing.T, requiredCleanRounds int) *DivergenceInputs {
	t.Helper()
	specHash, err := SpecHash(f.specPath)
	require.NoError(t, err)
	rolesHash, _ := ComputeRolesHash(f.root, PhaseAuditors)

	var streaks []RoleStreak
	var latestRows []map[string]any
	captureAuditRoundNotices(t, func() {
		streaks, latestRows = ComputeAuditRoleStreaks(f.specPath, f.rounds)
	})

	return &DivergenceInputs{
		Rounds:                  f.rounds,
		LatestRows:              latestRows,
		SpecCoverageCleanRounds: SpecCoverageCleanRounds(streaks),
		RequiredCleanRounds:     requiredCleanRounds,
		Stale:                   StateStale(f.rounds, specHash),
		Converged:               Converged(f.rounds, requiredCleanRounds, specHash),
		CurrentRolesHash:        rolesHash,
	}
}

// divergenceOver is the object the fixture produces at one threshold.
func (f *divergenceFixture) divergenceOver(t *testing.T, requiredCleanRounds int) *Divergence {
	t.Helper()
	return ComputeAuditDivergence(f.inputs(t, requiredCleanRounds))
}

// assertKeyOmitted pins that a nil object is an omitted key rather than a JSON
// null, over a payload assembled the way a caller assembles one.
func assertKeyOmitted(t *testing.T, d *Divergence) {
	t.Helper()
	require.Nil(t, d)
	payload := map[string]any{"converged": false}
	if d != nil {
		payload["divergence"] = d
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "divergence",
		"the key is omitted, never emitted as null")
}

// Test 19 — the object fires and carries the first message form. The fixture's
// streak (4), open count (3) and threshold (2) are three different numbers, so
// the round slot can only be the streak and the finding slot only
// other_roles_open.
//
// The hint is asserted against §2.6's constant and not against a literal copy
// of its words. A copy here would pin the constant against a second copy in the
// same repository, failing on any reword and catching no defect;
// TestDocsCarryTheConvergenceSignalWording is what holds the words themselves,
// by requiring skills/tp/REFERENCE.md to quote the shipped constant.
func TestComputeAuditDivergence_FiresWithTheFirstMessageForm(t *testing.T) {
	clean := auditRows(auditRow("spec-coverage", "PASS"))
	f := newDivergenceFixture(t,
		clean,
		clean,
		clean,
		auditRows(
			auditRow("spec-coverage", "PASS"),
			auditRow("go-safety", "FAIL"),
			auditRow("go-safety", "PARTIAL"),
			auditRow("ax-contract", "FAIL"),
		),
	)

	d := f.divergenceOver(t, 2)
	require.NotNil(t, d)
	assert.Equal(t, 3, d.OtherRolesOpen)
	assert.Equal(t, []string{"ax-contract", "go-safety"}, d.OpenRoles)
	assert.Equal(t, 0, d.UnattributedOpen)
	assert.Equal(t, "spec-coverage clean 4 rounds; 3 findings open from other roles", d.Message)
	assert.Equal(t, DivergenceHint, d.Hint)

	// All five fields are present in the emitted object.
	data, err := json.Marshal(d)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"other_roles_open": 3,
		"open_roles": ["ax-contract","go-safety"],
		"unattributed_open": 0,
		"message": "spec-coverage clean 4 rounds; 3 findings open from other roles",
		"hint": `+jsonString(DivergenceHint)+`
	}`, string(data))
}

// Test 19, second half — both counted nouns inflect to the singular together.
func TestComputeAuditDivergence_SingularMessage(t *testing.T) {
	f := newDivergenceFixture(t, auditRows(
		auditRow("spec-coverage", "PASS"),
		auditRow("go-safety", "FAIL"),
	))

	d := f.divergenceOver(t, 1)
	require.NotNil(t, d)
	assert.Equal(t, "spec-coverage clean 1 round; 1 finding open from other roles", d.Message)
}

// Test 20 — the third message form: every open non-spec-coverage row carries no
// role, so open_roles is an emitted empty array and unattributed_open equals
// other_roles_open.
func TestComputeAuditDivergence_ThirdMessageForm(t *testing.T) {
	f := newDivergenceFixture(t,
		auditRows(auditRow("spec-coverage", "PASS")),
		auditRows(
			auditRow("spec-coverage", "PASS"),
			`{"status":"FAIL"}`,
			`{"role":"   ","status":"FAIL"}`,
			`null`,
		),
	)

	d := f.divergenceOver(t, 2)
	require.NotNil(t, d)
	assert.Equal(t, 3, d.OtherRolesOpen)
	assert.Equal(t, 3, d.UnattributedOpen)
	assert.Empty(t, d.OpenRoles)
	assert.Equal(t,
		"spec-coverage clean 2 rounds; 3 findings open, none attributed to a role (possibly spec-coverage's)",
		d.Message)

	data, err := json.Marshal(d)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"open_roles":[]`,
		"an emitted empty array, neither absent nor null")

	// One finding, same form: only the two counted nouns inflect.
	single := newDivergenceFixture(t, auditRows(
		auditRow("spec-coverage", "PASS"),
		`{"status":"FAIL"}`,
	))
	one := single.divergenceOver(t, 1)
	require.NotNil(t, one)
	assert.Equal(t,
		"spec-coverage clean 1 round; 1 finding open, none attributed to a role (possibly spec-coverage's)",
		one.Message)
}

// Test 21 — open_roles names exactly the non-spec-coverage roles holding open
// rows, each id once even when a role holds three of them, and ascending by the
// byte order role_streaks uses: "Go-Safety" leads with an uppercase byte and so
// sorts before "ax-contract", which a case-insensitive sort would reverse.
func TestComputeAuditDivergence_OpenRolesAreDedupedAndByteOrdered(t *testing.T) {
	f := newDivergenceFixture(t, auditRows(
		auditRow("spec-coverage", "PASS"),
		auditRow("ax-contract", "FAIL"),
		auditRow("Go-Safety", "FAIL"),
		auditRow("Go-Safety", "PARTIAL"),
		auditRow("Go-Safety", "FAIL"),
	))

	d := f.divergenceOver(t, 1)
	require.NotNil(t, d)
	assert.Equal(t, []string{"Go-Safety", "ax-contract"}, d.OpenRoles)
	assert.Equal(t, 4, d.OtherRolesOpen, "every open row is counted, not every role")
	assert.Equal(t, 0, d.UnattributedOpen)
}

// Test 22 — the object is withheld when spec-coverage's streak is below the
// threshold (condition 1), and when it meets the threshold with no other role
// holding an open finding (condition 2).
func TestComputeAuditDivergence_WithheldByConditionsOneAndTwo(t *testing.T) {
	t.Run("streak below the threshold", func(t *testing.T) {
		f := newDivergenceFixture(t, auditRows(
			auditRow("spec-coverage", "PASS"),
			auditRow("go-safety", "FAIL"),
		))
		require.NotNil(t, f.divergenceOver(t, 1), "the fixture fires at a threshold of 1")
		assertKeyOmitted(t, f.divergenceOver(t, 2))
	})

	// The latest round is clean for every role, so condition 2 fails while
	// conditions 1, 3, 4 and 5 all hold: the round below it is unclean, which
	// keeps the stored clean-round count under the threshold and the sequence
	// unconverged.
	t.Run("no other role holds an open finding", func(t *testing.T) {
		f := newDivergenceFixture(t,
			auditRows(
				auditRow("spec-coverage", "PASS"),
				auditRow("go-safety", "FAIL"),
			),
			auditRows(
				auditRow("spec-coverage", "PASS"),
				auditRow("go-safety", "PASS"),
			),
		)
		in := f.inputs(t, 2)
		require.NotNil(t, in.SpecCoverageCleanRounds)
		require.Equal(t, 2, *in.SpecCoverageCleanRounds, "condition 1 holds")
		require.False(t, in.Stale, "condition 3 holds")
		require.False(t, in.Converged, "condition 4 holds")
		require.Equal(t, in.Rounds[len(in.Rounds)-1].RolesHash, in.CurrentRolesHash, "condition 5 holds")
		assertKeyOmitted(t, ComputeAuditDivergence(in))
	})
}

// A row attributed to spec-coverage is never one of condition 2's rows, whatever
// its status. The state is not reachable through the resolver — a non-PASS
// spec-coverage row makes its streak 0, so condition 1 admits it only at a
// threshold of 0, where condition 4 withholds the object — so the inputs here
// are built by hand to pin the field definition rather than a reachable payload.
func TestComputeAuditDivergence_SpecCoverageRowsAreNeverCounted(t *testing.T) {
	clean := 2
	in := &DivergenceInputs{
		Rounds: []ReviewRound{{Round: 1, File: "audit-round-1.ndjson", RolesHash: "sha256:panel"}},
		LatestRows: []map[string]any{
			{"role": "spec-coverage", "status": "FAIL"},
			{"role": "spec-coverage", "status": "PARTIAL"},
			{"role": "go-safety", "status": "FAIL"},
		},
		SpecCoverageCleanRounds: &clean,
		RequiredCleanRounds:     2,
		CurrentRolesHash:        "sha256:panel",
	}

	d := ComputeAuditDivergence(in)
	require.NotNil(t, d)
	assert.Equal(t, 1, d.OtherRolesOpen, "spec-coverage's own open rows are outside the count")
	assert.Equal(t, []string{"go-safety"}, d.OpenRoles)
	assert.Equal(t, 0, d.UnattributedOpen)
}

// Test 23 — the threshold is the effective audit_clean_rounds, not a literal 2:
// at 3, a spec-coverage streak of exactly 2 withholds the object and one further
// clean-for-spec-coverage round makes it fire. Both halves keep condition 2
// satisfied, so the added round is clean for spec-coverage and not as a whole.
func TestComputeAuditDivergence_ThresholdIsTheEffectiveValue(t *testing.T) {
	open := auditRow("go-safety", "FAIL")
	bodies := []string{
		auditRows(auditRow("spec-coverage", "FAIL"), open),
		auditRows(auditRow("spec-coverage", "PASS"), open),
		auditRows(auditRow("spec-coverage", "PASS"), open),
	}

	f := newDivergenceFixture(t, bodies...)
	in := f.inputs(t, 3)
	require.NotNil(t, in.SpecCoverageCleanRounds)
	require.Equal(t, 2, *in.SpecCoverageCleanRounds)
	assertKeyOmitted(t, ComputeAuditDivergence(in))

	// One further round, spec-coverage still clean and go-safety still open.
	f = newDivergenceFixture(t, append(bodies, auditRows(auditRow("spec-coverage", "PASS"), open))...)
	d := f.divergenceOver(t, 3)
	require.NotNil(t, d)
	assert.Equal(t, "spec-coverage clean 3 rounds; 1 finding open from other roles", d.Message)
}

// Test 24 — condition 4 withholds the object where conditions 1, 2, 3 and 5 all
// hold: at a resolved audit_clean_rounds of 0, engine.Converged reduces to "not
// stale" and reports true while the latest round holds a non-spec-coverage open
// finding. An implementation omitting condition 4 emits the object beside an
// open gate. (The exit code of tp audit --status --check on this state is
// asserted by the audit-output wiring.)
func TestComputeAuditDivergence_ConditionFourWithholdsAtZeroThreshold(t *testing.T) {
	f := newDivergenceFixture(t,
		auditRows(auditRow("spec-coverage", "PASS")),
		auditRows(
			auditRow("spec-coverage", "PASS"),
			auditRow("go-safety", "FAIL"),
		),
	)

	in := f.inputs(t, 0)
	require.NotNil(t, in.SpecCoverageCleanRounds)
	require.GreaterOrEqual(t, *in.SpecCoverageCleanRounds, 0, "condition 1 holds at a threshold of 0")
	require.False(t, in.Stale, "condition 3 holds")
	require.Equal(t, in.Rounds[len(in.Rounds)-1].RolesHash, in.CurrentRolesHash, "condition 5 holds")
	require.True(t, in.Converged, "engine.Converged reduces to \"not stale\" at a threshold of 0")

	assertKeyOmitted(t, ComputeAuditDivergence(in))

	// The same fixture at the default threshold is not converged and fires, so
	// the withholding above is condition 4 and nothing else.
	require.NotNil(t, f.divergenceOver(t, 2))
}

// Test 25 — a stale spec suppresses the object on --status even where
// conditions 1, 2 and 4 hold: the spec file is edited after the last round is
// recorded and StateStale reads true.
func TestComputeAuditDivergence_StaleSpecSuppressesTheObject(t *testing.T) {
	f := newDivergenceFixture(t,
		auditRows(auditRow("spec-coverage", "PASS")),
		auditRows(
			auditRow("spec-coverage", "PASS"),
			auditRow("go-safety", "FAIL"),
		),
	)
	require.NotNil(t, f.divergenceOver(t, 2), "the fixture fires before the spec is edited")

	require.NoError(t, os.WriteFile(f.specPath, []byte("# spec\n\nedited\n"), 0o600))

	in := f.inputs(t, 2)
	require.True(t, in.Stale, "the recorded round's spec hash no longer matches the spec on disk")
	require.NotNil(t, in.SpecCoverageCleanRounds)
	require.Equal(t, 2, *in.SpecCoverageCleanRounds, "condition 1 still holds")
	assertKeyOmitted(t, ComputeAuditDivergence(in))
}

// Test 26 — a non-PASS row carrying no role is counted in other_roles_open,
// disclosed in unattributed_open and named inline in the message; with every
// open non-spec-coverage row attributed, unattributed_open is 0 and present,
// and the suffix does not appear.
func TestComputeAuditDivergence_UnattributedRowsAreDisclosed(t *testing.T) {
	f := newDivergenceFixture(t,
		auditRows(auditRow("spec-coverage", "PASS")),
		auditRows(
			auditRow("spec-coverage", "PASS"),
			auditRow("go-safety", "FAIL"),
			`{"status":"FAIL"}`,
		),
	)

	d := f.divergenceOver(t, 2)
	require.NotNil(t, d)
	assert.Equal(t, 2, d.OtherRolesOpen)
	assert.Equal(t, 1, d.UnattributedOpen)
	assert.Equal(t, []string{"go-safety"}, d.OpenRoles)
	assert.Equal(t,
		"spec-coverage clean 2 rounds; 2 findings open from other roles "+
			"(including 1 with no role, which may be spec-coverage's)",
		d.Message)

	// Every open row attributed: the count is 0 and present, and the suffix is
	// gone.
	attributed := newDivergenceFixture(t,
		auditRows(auditRow("spec-coverage", "PASS")),
		auditRows(
			auditRow("spec-coverage", "PASS"),
			auditRow("go-safety", "FAIL"),
		),
	)
	a := attributed.divergenceOver(t, 2)
	require.NotNil(t, a)
	assert.Equal(t, 0, a.UnattributedOpen)
	assert.NotContains(t, a.Message, "with no role")

	data, err := json.Marshal(a)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"unattributed_open":0`,
		"present with the value 0, never absent")
}

// Test 14 — a corpus change made after the latest recorded round suppresses the
// object through condition 5. Every stored hash still matches every other, so
// §2.1's fourth no-rows cause cannot reach this state; only the equality
// against the hash computed now catches it. roles_stale reads true over the
// same state, which is the field the --status payload already carries.
func TestComputeAuditDivergence_CorpusChangeAfterTheLatestRoundSuppresses(t *testing.T) {
	f := newDivergenceFixture(t,
		auditRows(auditRow("spec-coverage", "PASS")),
		auditRows(
			auditRow("spec-coverage", "PASS"),
			auditRow("go-safety", "FAIL"),
		),
	)
	require.NotNil(t, f.divergenceOver(t, 2), "conditions 1-4 hold before the corpus is edited")

	require.NoError(t, os.WriteFile(filepath.Join(f.root, ".tp", "auditors", "spec-coverage.json"),
		[]byte(`{"id":"spec-coverage","title":"Spec coverage","instructions":"check the spec harder"}`), 0o600))

	in := f.inputs(t, 2)
	require.NotEmpty(t, in.CurrentRolesHash)
	require.True(t, RolesStale(f.rounds, in.CurrentRolesHash))
	require.NotNil(t, in.SpecCoverageCleanRounds, "§2.1's fourth cause never fires: every stored hash matches")
	assertKeyOmitted(t, ComputeAuditDivergence(in))
}

// Test 15 — an unreadable corpus withholds the object by two routes on two
// different paths.
func TestComputeAuditDivergence_UnreadableCorpusWithholdsByBothRoutes(t *testing.T) {
	// Route 1 (--status and --record alike): the latest round's stored
	// roles_hash is empty, so §2.1's fourth cause makes that round contribute
	// no rows, spec_coverage_clean_rounds is null and condition 1 fails.
	// roles_stale reads false here, pinning that the withholding comes from §2
	// rather than from that field.
	t.Run("route 1: an empty stored hash on the latest round", func(t *testing.T) {
		f := newDivergenceFixture(t,
			auditRows(auditRow("spec-coverage", "PASS")),
			auditRows(
				auditRow("spec-coverage", "PASS"),
				auditRow("go-safety", "FAIL"),
			),
		)
		require.NotNil(t, f.divergenceOver(t, 2), "the fixture fires with the hash stamped")

		f.rounds[len(f.rounds)-1].RolesHash = ""

		in := f.inputs(t, 2)
		assert.Nil(t, in.SpecCoverageCleanRounds, "the latest round contributes no rows")
		assert.False(t, RolesStale(f.rounds, in.CurrentRolesHash),
			"an empty stored hash is not roles-stale")
		assertKeyOmitted(t, ComputeAuditDivergence(in))
	})

	// Route 2 (--status only): the stored hash is non-empty and the corpus is
	// unreadable at report time, so the freshly computed hash is empty and
	// condition 5's equality fails. --record cannot reach this state, since it
	// stamps and reads one hash in one invocation.
	t.Run("route 2: the corpus is unreadable at report time", func(t *testing.T) {
		f := newDivergenceFixture(t,
			auditRows(auditRow("spec-coverage", "PASS")),
			auditRows(
				auditRow("spec-coverage", "PASS"),
				auditRow("go-safety", "FAIL"),
			),
		)
		require.NotNil(t, f.divergenceOver(t, 2), "the fixture fires with a readable corpus")

		// A dangling symlink is a corpus file tp cannot read, whatever the
		// process's privileges are.
		require.NoError(t, os.Symlink(filepath.Join(f.root, "gone"),
			filepath.Join(f.root, ".tp", "auditors", "zz.json")))

		hash, err := ComputeRolesHash(f.root, PhaseAuditors)
		require.Error(t, err)
		require.Empty(t, hash, "an unreadable corpus hashes to the empty string")

		in := f.inputs(t, 2)
		require.NotNil(t, in.SpecCoverageCleanRounds, "condition 1 still holds")
		require.NotEmpty(t, in.Rounds[len(in.Rounds)-1].RolesHash)
		assertKeyOmitted(t, ComputeAuditDivergence(in))
	})
}

// jsonString renders a Go string as a JSON string literal for an inline JSONEq
// expectation.
func jsonString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}
