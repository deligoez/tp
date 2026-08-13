package engine

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// envMap turns ChildEnv's "KEY=VALUE" slice into a map, so a test can assert a
// variable's value and — just as importantly — its absence.
func envMap(t *testing.T, entries []string) map[string]string {
	t.Helper()
	out := make(map[string]string, len(entries))
	for _, entry := range entries {
		key, value, ok := strings.Cut(entry, "=")
		require.True(t, ok, "every entry carries a '=': %q", entry)
		_, dup := out[key]
		require.False(t, dup, "%s appears once", key)
		out[key] = value
	}
	return out
}

// roundUnit builds a unit in a round-based phase inside root.
func roundUnit(root string, round int) *UnitEnv {
	return &UnitEnv{
		RunID:    NewULID(),
		Root:     root,
		TaskFile: filepath.Join(root, "spec", "0.35.0.tasks.json"),
		Phase:    PhaseReview,
		Round:    &round,
		Kind:     UnitReviewRole,
		ID:       "architect",
		Seq:      7,
	}
}

// Test 37: every child receives the documented variables in their documented
// forms.
func TestChildEnv_DocumentedVariables(t *testing.T) {
	root := t.TempDir()
	unit := roundUnit(root, 3)

	env := envMap(t, ChildEnv([]string{"PATH=/usr/bin", "HOME=/home/x"}, nil, unit))

	assert.Equal(t, "/usr/bin", env["PATH"], "the parent's environment survives")
	assert.Equal(t, unit.RunID, env[EnvRunID])
	assertULID(t, env[EnvRunID])

	assert.True(t, filepath.IsAbs(env[EnvRunDir]), "TP_RUN_DIR is absolute")
	assert.Equal(t, filepath.Join(root, ".tp", "runs", unit.RunID), env[EnvRunDir])

	assert.Equal(t, filepath.Join(root, ".tp", "rounds", "0.35.0", "review-r3"), env[EnvRoundDir],
		"TP_ROUND_DIR is derived from the task file's base, the phase and the round")
	assert.Equal(t, "3", env[EnvRound])

	assert.Equal(t, unit.TaskFile, env[EnvTaskFile], "the child works the file the driver resolved")
	assert.Equal(t, "architect", env[EnvUnitID])
	assert.Equal(t, "review-role", env[EnvUnitKind])
	assert.Equal(t, "7", env[EnvUnitSeq])
	assert.Equal(t, "review", env[EnvPhase])
	assert.Equal(t, "1", env[EnvUnattended])
}

// Test 46: TP_ROUND and TP_ROUND_DIR are unset — not empty — outside a
// round-based phase, even when something upstream set them.
func TestChildEnv_RoundVariablesUnsetOutsideARound(t *testing.T) {
	root := t.TempDir()
	unit := roundUnit(root, 3)
	unit.Round = nil
	unit.Phase = PhaseImplement
	unit.Kind = UnitImplement
	unit.ID = "child-env"

	parent := []string{"TP_ROUND=9", "TP_ROUND_DIR=" + filepath.Join(root, "stale")}
	runner := map[string]string{"TP_ROUND": "11"}

	env := ChildEnv(parent, runner, unit)
	got := envMap(t, env)

	_, hasRound := got[EnvRound]
	_, hasRoundDir := got[EnvRoundDir]
	assert.False(t, hasRound, "TP_ROUND is absent, not empty")
	assert.False(t, hasRoundDir, "TP_ROUND_DIR is absent, not empty")
	for _, entry := range env {
		assert.NotEqual(t, "TP_ROUND=", entry)
		assert.NotEqual(t, "TP_ROUND_DIR=", entry)
	}
	assert.Equal(t, "implement", got[EnvUnitKind], "the rest of the identity is unaffected")
}

// Test 20: runner.env is merged over the parent, and the driver's own variables
// are applied after that merge, so an entry for a driver variable never reaches
// the child.
func TestChildEnv_DriverVariablesOutrankRunnerEnv(t *testing.T) {
	root := t.TempDir()
	unit := roundUnit(root, 2)
	runner := map[string]string{
		EnvUnattended:     "0",
		EnvRunID:          "forged",
		EnvRoundDir:       filepath.Join(root, "elsewhere"),
		EnvUnitKind:       "implement",
		"ANTHROPIC_MODEL": "some-model",
		"PATH":            "/runner/bin",
	}

	env := envMap(t, ChildEnv([]string{"PATH=/usr/bin", "SHELL=/bin/sh"}, runner, unit))

	assert.Equal(t, "1", env[EnvUnattended], "the fence cannot be turned off by runner.env")
	assert.Equal(t, unit.RunID, env[EnvRunID])
	assert.Equal(t, filepath.Join(root, ".tp", "rounds", "0.35.0", "review-r2"), env[EnvRoundDir])
	assert.Equal(t, "review-role", env[EnvUnitKind])

	assert.Equal(t, "some-model", env["ANTHROPIC_MODEL"], "runner.env still extends the environment")
	assert.Equal(t, "/runner/bin", env["PATH"], "and still overrides a non-driver parent variable")
	assert.Equal(t, "/bin/sh", env["SHELL"])
}

func TestChildEnv_SortedAndParentEntriesWithoutSeparatorDropped(t *testing.T) {
	root := t.TempDir()
	env := ChildEnv([]string{"ZZZ=1", "AAA=2", "NOT_AN_ENTRY"}, nil, roundUnit(root, 1))

	assert.True(t, sortedStrings(env), "the same unit always produces the same environment")
	for _, entry := range env {
		assert.NotEqual(t, "NOT_AN_ENTRY", entry)
		assert.Contains(t, entry, "=")
	}
}

func sortedStrings(entries []string) bool {
	for i := 1; i < len(entries); i++ {
		if entries[i-1] > entries[i] {
			return false
		}
	}
	return true
}

func TestRunBase_StripsTheTaskFileSuffix(t *testing.T) {
	cases := map[string]string{
		filepath.Join("spec", "0.35.0.tasks.json"): "0.35.0",
		"spec.tasks.json":                          "spec",
		filepath.Join("a", "b", "app.tasks.json"):  "app",
	}
	for path, want := range cases {
		assert.Equal(t, want, RunBase(path), "base of %s", path)
	}
}

func TestRoundDir_KeyedByCycleNotRun(t *testing.T) {
	root := t.TempDir()
	taskFile := filepath.Join(root, "spec.tasks.json")

	first := RoundDir(root, taskFile, PhaseAudit, 4)
	second := RoundDir(root, taskFile, PhaseAudit, 4)
	assert.Equal(t, first, second, "two runs collecting the same round address one directory")
	assert.Equal(t, filepath.Join(root, ".tp", "rounds", "spec", "audit-r4"), first)
	assert.NotEqual(t, first, RoundDir(root, taskFile, PhaseReview, 4), "the phase is part of the key")
	assert.NotEqual(t, first, RoundDir(root, taskFile, PhaseAudit, 5), "and so is the round")
}

// assertULID checks the canonical text form: 26 Crockford base32 characters
// whose first is never above '7', since the 48-bit timestamp pads into a 50-bit
// field.
func assertULID(t *testing.T, s string) {
	t.Helper()
	require.Len(t, s, ulidTimeChars+ulidRandChars, "a ULID is 26 characters")
	for i := range s {
		assert.True(t, strings.IndexByte(crockfordAlphabet, s[i]) >= 0,
			"character %d of %q is in Crockford's alphabet", i, s)
	}
	assert.LessOrEqual(t, s[0], byte('7'), "the timestamp's high bits are zero padding")
}

func TestNewULID_UniqueSortableAndTimestamped(t *testing.T) {
	seen := make(map[string]bool, 200)
	for range 200 {
		id := NewULID()
		assertULID(t, id)
		assert.False(t, seen[id], "%s is unique", id)
		seen[id] = true
	}

	// The leading 10 characters decode to the generation time, which is what
	// makes run directories sort by start time.
	before := time.Now().UTC().UnixMilli()
	id := NewULID()
	after := time.Now().UTC().UnixMilli()
	ms := decodeCrockford(t, id[:ulidTimeChars])
	assert.GreaterOrEqual(t, ms, before)
	assert.LessOrEqual(t, ms, after)
}

// decodeCrockford reads a base32 string back into the number it encodes.
func decodeCrockford(t *testing.T, s string) int64 {
	t.Helper()
	var value int64
	for i := range s {
		digit := strings.IndexByte(crockfordAlphabet, s[i])
		require.GreaterOrEqual(t, digit, 0, "%q is a base32 character", s[i])
		value = value<<5 | int64(digit)
	}
	return value
}
