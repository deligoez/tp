package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nextUnitsRepo writes a spec and its adjacent task file in a fresh git-rooted
// directory and returns (root, taskFile, spec).
func nextUnitsRepo(t *testing.T) (root, taskFile, spec string) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	spec = filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(spec, []byte("# S\n"), 0o600))
	taskFile = filepath.Join(dir, "spec.tasks.json")
	require.NoError(t, os.WriteFile(taskFile, []byte(`{"spec":"spec.md","tasks":[]}`), 0o600))
	return dir, taskFile, spec
}

// writeRoundFile writes one file into a round's own directory (§3.1.1's
// .tp/rounds/<base>/<phase>-r<round>) and returns its path, creating the
// directory tree the way the driver would.
func writeRoundFile(t *testing.T, root, taskFile, phase string, round int, name, body string) string {
	t.Helper()
	dir := RoundDir(root, taskFile, phase, round)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

// ndjson joins rows into an NDJSON body with a trailing newline.
func ndjson(rows ...string) string {
	return strings.Join(rows, "\n") + "\n"
}

func unitKinds(units []NextUnit) []UnitKind {
	out := make([]UnitKind, 0, len(units))
	for _, u := range units {
		out = append(out, u.Kind)
	}
	return out
}

func unitIDs(units []NextUnit) []string {
	out := make([]string, 0, len(units))
	for _, u := range units {
		out = append(out, u.ID)
	}
	return out
}

// TestBuildNextUnits_RoundIsNilOutsideRoundBasedPhases: §4.1 — round is the
// number of the round the returned units belong to, and null outside a
// round-based phase. That is what makes the driver leave TP_ROUND unset rather
// than empty (§3.1.1).
func TestBuildNextUnits_RoundIsNilOutsideRoundBasedPhases(t *testing.T) {
	root, taskFile, spec := nextUnitsRepo(t)
	tf := &model.TaskFile{Spec: spec, Tasks: []model.Task{
		{ID: "t1", Status: model.StatusOpen},
	}}

	for _, phase := range []string{PhaseImplement, PhaseDecompose, PhaseRelease} {
		_, round := BuildNextUnits(root, taskFile, spec, phase, tf, nil, nil)
		assert.Nil(t, round, "%s is not a round-based phase", phase)
	}

	for _, phase := range []string{PhaseReview, PhaseAudit} {
		_, round := BuildNextUnits(root, taskFile, spec, phase, tf, nil, nil)
		require.NotNil(t, round, "%s is round-based", phase)
		assert.Equal(t, 1, *round, "no round recorded yet: the phase is collecting round 1")
	}
}

// TestBuildNextUnits_RoundIsTheOneBeingCollected: with rounds recorded, the
// reported round is the next one — the round the returned role units are
// collecting, not the last one recorded.
func TestBuildNextUnits_RoundIsTheOneBeingCollected(t *testing.T) {
	root, taskFile, spec := nextUnitsRepo(t)
	tf := &model.TaskFile{Spec: spec, Tasks: []model.Task{}}
	st := &ReviewState{
		ReviewRounds: []ReviewRound{{Round: 1}, {Round: 2}},
		AuditRounds:  []ReviewRound{{Round: 1}},
	}

	_, reviewRound := BuildNextUnits(root, taskFile, spec, PhaseReview, tf, st, nil)
	require.NotNil(t, reviewRound)
	assert.Equal(t, 3, *reviewRound)

	_, auditRound := BuildNextUnits(root, taskFile, spec, PhaseAudit, tf, st, nil)
	require.NotNil(t, auditRound)
	assert.Equal(t, 2, *auditRound)
}

// TestBuildNextUnits_RolePanelIsTheCorpus: a round-based phase returns one role
// unit per active role, in corpus order, each carrying its kind's brief command.
func TestBuildNextUnits_RolePanelIsTheCorpus(t *testing.T) {
	root, taskFile, spec := nextUnitsRepo(t)
	tf := &model.TaskFile{Spec: spec, Tasks: []model.Task{}}

	review, _ := BuildNextUnits(root, taskFile, spec, PhaseReview, tf, nil, nil)
	assert.Equal(t, []string{"implementer", "tester", "architect"}, unitIDs(review))
	for _, u := range review {
		assert.Equal(t, UnitReviewRole, u.Kind)
		assert.Equal(t, "tp review "+spec+" --role "+u.ID, u.BriefCommand,
			"each role unit asks for its own prompt (v0.36.0 §4.2.3)")
	}

	audit, _ := BuildNextUnits(root, taskFile, spec, PhaseAudit, tf, nil, nil)
	assert.Equal(t, []string{"spec-coverage", "security", "maintainability-conventions"}, unitIDs(audit))
	for _, u := range audit {
		assert.Equal(t, UnitAuditRole, u.Kind)
		assert.Equal(t, "tp audit "+spec+" --role "+u.ID, u.BriefCommand,
			"each role unit asks for its own prompt (v0.36.0 §4.2.3)")
	}
}

// TestBuildNextUnits_RolePanelHonoursSpecDeactivation: the oracle resolves the
// panel the emitting command would resolve, so a role this spec deactivates is
// not a unit the driver spawns.
func TestBuildNextUnits_RolePanelHonoursSpecDeactivation(t *testing.T) {
	root, taskFile, spec := nextUnitsRepo(t)
	require.NoError(t, os.WriteFile(spec,
		[]byte("---\ntp:\n  review_roles:\n    tester:\n      enabled: false\n---\n# S\n"), 0o600))
	tf := &model.TaskFile{Spec: spec, Tasks: []model.Task{}}

	units, _ := BuildNextUnits(root, taskFile, spec, PhaseReview, tf, nil, nil)
	assert.Equal(t, []string{"implementer", "architect"}, unitIDs(units))
}

// TestBuildNextUnits_ImplementAndDecomposeSubjects: §3.1.1's ids — the task id
// for implement (the WIP task first, so a crashed unit is resumed), the spec
// base name for decompose.
func TestBuildNextUnits_ImplementAndDecomposeSubjects(t *testing.T) {
	root, taskFile, spec := nextUnitsRepo(t)
	tf := &model.TaskFile{Spec: spec, Tasks: []model.Task{
		{ID: "ready", Status: model.StatusOpen},
		{ID: "in-progress", Status: model.StatusWIP},
	}}

	units, _ := BuildNextUnits(root, taskFile, spec, PhaseImplement, tf, nil, nil)
	require.Len(t, units, 1)
	assert.Equal(t, NextUnit{Kind: UnitImplement, ID: "in-progress", BriefCommand: "tp next --brief"}, units[0])

	units, _ = BuildNextUnits(root, taskFile, spec, PhaseDecompose, tf, nil, nil)
	require.Len(t, units, 1)
	assert.Equal(t, NextUnit{Kind: UnitDecompose, ID: "spec", BriefCommand: "tp resume"}, units[0])
}

// TestBuildNextUnits_EmptyCases: §4.1's three empty cases — release, a phase
// whose work is blocked, and a phase awaiting an operator decision (the
// escalate blocker class). Never nil: an empty next_units serializes as [].
func TestBuildNextUnits_EmptyCases(t *testing.T) {
	root, taskFile, spec := nextUnitsRepo(t)
	tf := &model.TaskFile{Spec: spec, Tasks: []model.Task{{ID: "t1", Status: model.StatusOpen}}}

	units, _ := BuildNextUnits(root, taskFile, spec, PhaseRelease, tf, nil, nil)
	assert.NotNil(t, units)
	assert.Empty(t, units, "release is the releasable condition itself")

	escalate := []Blocker{{Code: "review-budget-exhausted", Class: ClassEscalate}}
	units, round := BuildNextUnits(root, taskFile, spec, PhaseReview, tf, nil, escalate)
	assert.Empty(t, units, "raising the cap is a user decision, so the oracle offers no unit")
	require.NotNil(t, round, "the round is a property of the phase, not of the returned slice")
	assert.Equal(t, 1, *round)

	clearable := []Blocker{{Code: "unexplained-changes", Class: ClassAgentClearable}}
	units, _ = BuildNextUnits(root, taskFile, spec, PhaseImplement, tf, nil, clearable)
	assert.Len(t, units, 1, "an agent-clearable blocker does not stop the phase")

	blocked := &model.TaskFile{Spec: spec, Tasks: []model.Task{
		{ID: "t1", Status: model.StatusOpen, DependsOn: []string{"missing"}},
	}}
	units, _ = BuildNextUnits(root, taskFile, spec, PhaseImplement, blocked, nil, nil)
	assert.Empty(t, units, "no ready task: the phase's work is blocked")
}

// TestBuildNextUnits_NonConcurrentKindIsNeverAlongsideAnother is §4.1's
// concurrency invariant, checked over every kind rather than over the phases
// that happen to emit one today: only the two role kinds may share an array.
func TestBuildNextUnits_NonConcurrentKindIsNeverAlongsideAnother(t *testing.T) {
	for _, kind := range UnitKinds() {
		pair := []NextUnit{{Kind: kind, ID: "a"}, {Kind: UnitReviewRole, ID: "b"}}
		got := soloIfNotConcurrent(pair)
		if kind.Concurrent() {
			assert.Len(t, got, 2, "%s runs beside a sibling role", kind)
			continue
		}
		require.Len(t, got, 1, "%s runs alone", kind)
		assert.Equal(t, kind, got[0].Kind)
	}

	// A concurrent unit followed by a non-concurrent one is also trimmed: the
	// invariant is about the array, not about its first entry.
	mixed := soloIfNotConcurrent([]NextUnit{{Kind: UnitReviewRole, ID: "a"}, {Kind: UnitReviewRecord, ID: "1"}})
	assert.Equal(t, []UnitKind{UnitReviewRole}, unitKinds(mixed))
}

// TestRenderNextAction_RendersFirstUnit: §4.1 — next_action is a rendering of
// next_units[0], not a second opinion, so its payload names that unit. An empty
// array leaves next_action untouched.
func TestRenderNextAction_RendersFirstUnit(t *testing.T) {
	base := NextAction{Summary: "s", Payload: map[string]any{"round": 2}}

	unchanged := renderNextAction(base, nil)
	assert.Equal(t, map[string]any{"round": 2}, unchanged.Payload)

	rendered := renderNextAction(base, []NextUnit{
		{Kind: UnitReviewRole, ID: "implementer", BriefCommand: "tp review spec.md"},
		{Kind: UnitReviewRole, ID: "tester", BriefCommand: "tp review spec.md"},
	})
	assert.Equal(t, map[string]any{"kind": "review-role", "id": "implementer"}, rendered.Payload["unit"])
	assert.Equal(t, 2, rendered.Payload["round"], "the phase payload survives")
	assert.Equal(t, map[string]any{"round": 2}, base.Payload, "the caller's payload is not mutated")
}

// TestBuildNextUnits_OmitsRolesWhoseFindingsSatisfyThePredicate is test 45's
// oracle half and test 52: a resumed round omits every role whose findings file
// already satisfies §3.3's predicate — present and wholly parseable — so a role
// that left a malformed file is re-run rather than skipped.
func TestBuildNextUnits_OmitsRolesWhoseFindingsSatisfyThePredicate(t *testing.T) {
	root, taskFile, spec := nextUnitsRepo(t)
	tf := &model.TaskFile{Spec: spec, Tasks: []model.Task{}}

	writeRoundFile(t, root, taskFile, PhaseReview, 1, "role-implementer.ndjson",
		ndjson(`{"id":"f1","severity":"major"}`))
	writeRoundFile(t, root, taskFile, PhaseReview, 1, "role-tester.ndjson",
		ndjson(`{"id":"f2","severity":"minor"}`, `not json`))

	units, round := BuildNextUnits(root, taskFile, spec, PhaseReview, tf, nil, nil)
	assert.Equal(t, []string{"tester", "architect"}, unitIDs(units),
		"implementer finished; tester left a malformed file; architect wrote none")
	require.NotNil(t, round)
	assert.Equal(t, 1, *round, "the omission is inside the round being collected")

	// The predicate the oracle applied is the kind's own, which is what makes
	// the oracle, the driver and the Stop hook agree on a malformed file.
	dir := RoundDir(root, taskFile, PhaseReview, 1)
	assert.True(t, UnitReviewRole.DurableWrite(UnitTarget{RoundDir: dir, ID: "implementer"}))
	assert.False(t, UnitReviewRole.DurableWrite(UnitTarget{RoundDir: dir, ID: "tester"}))
}

// TestBuildNextUnits_OmissionCountsACleanRoleAsFinished: a role file of nothing
// but blank lines is a clean role, not a failed unit (§3.3), so it is omitted
// rather than re-run.
func TestBuildNextUnits_OmissionCountsACleanRoleAsFinished(t *testing.T) {
	root, taskFile, spec := nextUnitsRepo(t)
	tf := &model.TaskFile{Spec: spec, Tasks: []model.Task{}}

	writeRoundFile(t, root, taskFile, PhaseReview, 1, "role-implementer.ndjson", "\n\n")

	units, _ := BuildNextUnits(root, taskFile, spec, PhaseReview, tf, nil, nil)
	assert.Equal(t, []string{"tester", "architect"}, unitIDs(units))
}

// TestBuildNextUnits_OmissionAppliesToTheAuditPanel mirrors the review half:
// the rule is the role kinds', not the review phase's.
func TestBuildNextUnits_OmissionAppliesToTheAuditPanel(t *testing.T) {
	root, taskFile, spec := nextUnitsRepo(t)
	tf := &model.TaskFile{Spec: spec, Tasks: []model.Task{}}

	writeRoundFile(t, root, taskFile, PhaseAudit, 1, "role-spec-coverage.ndjson",
		ndjson(`{"role":"spec-coverage","item_id":"i1","status":"PASS"}`))

	units, _ := BuildNextUnits(root, taskFile, spec, PhaseAudit, tf, nil, nil)
	assert.Equal(t, []string{"security", "maintainability-conventions"}, unitIDs(units))
}

// TestBuildNextUnits_OmissionIsScopedToTheCollectingRound: the findings file
// read is the collecting round's own, so a role that answered round 1 is still
// asked for round 2.
func TestBuildNextUnits_OmissionIsScopedToTheCollectingRound(t *testing.T) {
	root, taskFile, spec := nextUnitsRepo(t)
	tf := &model.TaskFile{Spec: spec, Tasks: []model.Task{}}
	st := &ReviewState{ReviewRounds: []ReviewRound{{Round: 1}}}

	writeRoundFile(t, root, taskFile, PhaseReview, 1, "role-implementer.ndjson",
		ndjson(`{"id":"f1"}`))
	writeRoundFile(t, root, taskFile, PhaseReview, 2, "role-tester.ndjson",
		ndjson(`{"id":"f2"}`))

	units, round := BuildNextUnits(root, taskFile, spec, PhaseReview, tf, st, nil)
	assert.Equal(t, []string{"implementer", "architect"}, unitIDs(units),
		"round 1's role files say nothing about round 2; round 2's own do")
	require.NotNil(t, round)
	assert.Equal(t, 2, *round)
}

// TestBuildNextUnits_ReviewResolveWhileAFindingLacksDisposition: §4.1 — after a
// round is recorded the oracle returns a single review-resolve unit while any
// finding in that round's merged file lacks a disposition, carrying the round
// just recorded rather than the next one, and returns to collecting once every
// finding carries one.
func TestBuildNextUnits_ReviewResolveWhileAFindingLacksDisposition(t *testing.T) {
	root, taskFile, spec := nextUnitsRepo(t)
	tf := &model.TaskFile{Spec: spec, Tasks: []model.Task{}}
	st := &ReviewState{ReviewRounds: []ReviewRound{{Round: 1}}}

	units, round := BuildNextUnits(root, taskFile, spec, PhaseReview, tf, st, nil)
	assert.Equal(t, []string{"implementer", "tester", "architect"}, unitIDs(units),
		"no merged file: the recorded round holds no finding to dispose")
	require.NotNil(t, round)
	assert.Equal(t, 2, *round)

	merged := writeRoundFile(t, root, taskFile, PhaseReview, 1, "merged.ndjson", ndjson(
		`{"id":"f1","resolved":{"disposition":"fixed"}}`,
		`{"id":"f2"}`,
	))

	units, round = BuildNextUnits(root, taskFile, spec, PhaseReview, tf, st, nil)
	require.Len(t, units, 1, "review-resolve runs alone")
	assert.Equal(t, NextUnit{
		Kind:         UnitReviewResolve,
		ID:           "spec",
		BriefCommand: "tp review " + spec + " --status",
	}, units[0])
	require.NotNil(t, round)
	assert.Equal(t, 1, *round, "the unit acts on the findings of the round just recorded")

	require.NoError(t, os.WriteFile(merged, []byte(ndjson(
		`{"id":"f1","resolved":{"disposition":"fixed"}}`,
		`{"id":"f2","resolved":{"disposition":"wontfix"}}`,
	)), 0o600))

	units, round = BuildNextUnits(root, taskFile, spec, PhaseReview, tf, st, nil)
	assert.Equal(t, []string{"implementer", "tester", "architect"}, unitIDs(units),
		"every finding disposed: the phase collects the next round")
	require.NotNil(t, round)
	assert.Equal(t, 2, *round)
}

// TestBuildNextUnits_AuditFixOneRowAtATime: §4.1 — a single audit-fix unit for
// the first row in the recorded round's merged file that is neither PASS nor
// already disposed, keyed role:item_id so the unit can name its own row, and
// one at a time because the kind runs alone.
func TestBuildNextUnits_AuditFixOneRowAtATime(t *testing.T) {
	root, taskFile, spec := nextUnitsRepo(t)
	tf := &model.TaskFile{Spec: spec, Tasks: []model.Task{}}
	st := &ReviewState{AuditRounds: []ReviewRound{{Round: 1}}}

	pass := `{"role":"spec-coverage","item_id":"i1","status":"PASS"}`
	done := `{"role":"spec-coverage","item_id":"i2","status":"FAIL","resolved":{"disposition":"wontfix"}}`
	open1 := `{"role":"security","item_id":"i3","status":"FAIL"}`
	open2 := `{"role":"security","item_id":"i4","status":"PARTIAL"}`
	merged := writeRoundFile(t, root, taskFile, PhaseAudit, 1, "merged.ndjson",
		ndjson(pass, done, open1, open2))
	dir := filepath.Dir(merged)

	units, round := BuildNextUnits(root, taskFile, spec, PhaseAudit, tf, st, nil)
	require.Len(t, units, 1, "audit-fix runs alone: one finding at a time")
	assert.Equal(t, NextUnit{
		Kind:         UnitAuditFix,
		ID:           "security:i3",
		BriefCommand: "tp audit " + spec + " --status",
	}, units[0], "PASS rows and disposed rows are skipped")
	require.NotNil(t, round)
	assert.Equal(t, 1, *round)
	assert.False(t, UnitAuditFix.DurableWrite(UnitTarget{RoundDir: dir, ID: "security:i3"}),
		"the id the oracle emits is the one the kind's own predicate selects")

	fixed := `{"role":"security","item_id":"i3","status":"FAIL","resolved":{"disposition":"fixed"}}`
	require.NoError(t, os.WriteFile(merged, []byte(ndjson(pass, done, fixed, open2)), 0o600))
	assert.True(t, UnitAuditFix.DurableWrite(UnitTarget{RoundDir: dir, ID: "security:i3"}))

	units, _ = BuildNextUnits(root, taskFile, spec, PhaseAudit, tf, st, nil)
	require.Len(t, units, 1)
	assert.Equal(t, "security:i4", units[0].ID, "the next undisposed row, one at a time")

	closed := `{"role":"security","item_id":"i4","status":"PARTIAL","resolved":{"disposition":"duplicate"}}`
	require.NoError(t, os.WriteFile(merged, []byte(ndjson(pass, done, fixed, closed)), 0o600))
	units, round = BuildNextUnits(root, taskFile, spec, PhaseAudit, tf, st, nil)
	assert.Equal(t, []string{"spec-coverage", "security", "maintainability-conventions"}, unitIDs(units),
		"every non-PASS row disposed: the phase collects the next round")
	require.NotNil(t, round)
	assert.Equal(t, 2, *round)
}

// TestBuildNextUnits_RecordUnitOnceEveryRoleAnswered is test 45a: §4.1's record
// emission. A round whose panel is non-empty and whose every role satisfies
// §3.3's predicate returns exactly one record unit for that round — the step
// between collecting a round and acting on it, and the one point at which
// next_units would otherwise empty and stop the run with no-units while the
// round's own work is unfinished. The unit's id is its round number (§3.1.1).
func TestBuildNextUnits_RecordUnitOnceEveryRoleAnswered(t *testing.T) {
	for _, tc := range []struct {
		phase string
		roles []string
		kind  UnitKind
		verb  string
	}{
		{PhaseReview, []string{"implementer", "tester", "architect"}, UnitReviewRecord, "review"},
		{PhaseAudit, []string{"spec-coverage", "security", "maintainability-conventions"}, UnitAuditRecord, "audit"},
	} {
		t.Run(tc.phase, func(t *testing.T) {
			root, taskFile, spec := nextUnitsRepo(t)
			tf := &model.TaskFile{Spec: spec, Tasks: []model.Task{}}

			last := tc.roles[len(tc.roles)-1]
			for _, role := range tc.roles[:len(tc.roles)-1] {
				writeRoundFile(t, root, taskFile, tc.phase, 1, "role-"+role+".ndjson", ndjson(`{"id":"f"}`))
			}
			units, _ := BuildNextUnits(root, taskFile, spec, tc.phase, tf, nil, nil)
			require.Equal(t, []string{last}, unitIDs(units),
				"one role has still not answered, so the round is not collected yet")

			writeRoundFile(t, root, taskFile, tc.phase, 1, "role-"+last+".ndjson", ndjson(`{"id":"f"}`))

			units, round := BuildNextUnits(root, taskFile, spec, tc.phase, tf, nil, nil)
			require.Len(t, units, 1, "a record unit runs alone")
			assert.Equal(t, NextUnit{
				Kind: tc.kind,
				ID:   "1",
				BriefCommand: "[ -f $TP_ROUND_DIR/merged.ndjson ] || tp " + tc.verb +
					" --merge $TP_ROUND_DIR/role-*.ndjson -o $TP_ROUND_DIR/merged.ndjson; tp " +
					tc.verb + " " + spec + " --record $TP_ROUND_DIR/merged.ndjson",
			}, units[0])
			require.NotNil(t, round)
			assert.Equal(t, 1, *round, "the record unit belongs to the round it records")
		})
	}
}

// TestBuildNextUnits_NoRecordUnitForAnEmptyPanel is test 45a's other half and
// §4.1's non-empty guard: a panel that could not be resolved, or one a spec's
// frontmatter has wholly deactivated, satisfies "every role in it" vacuously.
// A record unit there would merge an unmatched glob and freeze a round holding
// zero role files, so the emptiness stays a no-units stop a human sees.
func TestBuildNextUnits_NoRecordUnitForAnEmptyPanel(t *testing.T) {
	t.Run("wholly deactivated", func(t *testing.T) {
		root, taskFile, spec := nextUnitsRepo(t)
		require.NoError(t, os.WriteFile(spec, []byte("---\ntp:\n  review_roles:\n"+
			"    implementer:\n      enabled: false\n"+
			"    tester:\n      enabled: false\n"+
			"    architect:\n      enabled: false\n---\n# S\n"), 0o600))
		tf := &model.TaskFile{Spec: spec, Tasks: []model.Task{}}

		units, round := BuildNextUnits(root, taskFile, spec, PhaseReview, tf, nil, nil)
		assert.Empty(t, units, "no role was asked, so no role file exists to merge")
		require.NotNil(t, round, "the round stays a property of the phase")
		assert.Equal(t, 1, *round)
	})

	t.Run("unresolvable", func(t *testing.T) {
		root, taskFile, spec := nextUnitsRepo(t)
		corpus := filepath.Join(root, ".tp", PhaseReviewers)
		require.NoError(t, os.MkdirAll(corpus, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(corpus, "broken.json"), []byte("{not json"), 0o600))
		tf := &model.TaskFile{Spec: spec, Tasks: []model.Task{}}

		units, _ := BuildNextUnits(root, taskFile, spec, PhaseReview, tf, nil, nil)
		assert.Empty(t, units, "a panel the oracle could not resolve is not a collected round")
	})
}

// TestBuildNextUnits_NoRecordUnitWhenTheRoundIsAlreadyRecorded: §4.1's "and the
// round has no recorded entry". The condition is the round file's own presence
// rather than the review state's count of rounds, so the crash window tp's
// writers leave open — the round file written before state.json names it — never
// re-records a recorded round.
func TestBuildNextUnits_NoRecordUnitWhenTheRoundIsAlreadyRecorded(t *testing.T) {
	root, taskFile, spec := nextUnitsRepo(t)
	tf := &model.TaskFile{Spec: spec, Tasks: []model.Task{}}
	for _, role := range []string{"implementer", "tester", "architect"} {
		writeRoundFile(t, root, taskFile, PhaseReview, 1, "role-"+role+".ndjson", ndjson(`{"id":"f"}`))
	}

	units, _ := BuildNextUnits(root, taskFile, spec, PhaseReview, tf, nil, nil)
	require.Len(t, units, 1, "the round is collected and unrecorded")
	assert.Equal(t, UnitReviewRecord, units[0].Kind)

	// No merged.ndjson: the record kind's own durable write is still absent, so
	// only the recorded entry can suppress the unit.
	require.NoError(t, os.MkdirAll(ReviewStateDir(spec), 0o755))
	require.NoError(t, os.WriteFile(roundFilePath(spec, PhaseReview, 1),
		[]byte(ndjson(`{"id":"f"}`)), 0o600))
	require.False(t, UnitReviewRecord.DurableWrite(UnitTarget{
		Spec: spec, RoundDir: RoundDir(root, taskFile, PhaseReview, 1), Round: 1,
	}), "the merged file is absent, so the kind's own predicate is still false")

	units, _ = BuildNextUnits(root, taskFile, spec, PhaseReview, tf, nil, nil)
	assert.Empty(t, units, "round 1 already has its recorded entry")
}
