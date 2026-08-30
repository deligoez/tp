package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// escalationRecord is §5.2's per-unit record, read back from disk exactly as
// `tp escalate` wrote it. Options is a slice rather than a []string field with
// omitempty so a test can tell [] from absent.
type escalationRecord struct {
	Decision string   `json:"decision"`
	UnitKind string   `json:"unit_kind"`
	UnitID   string   `json:"unit_id"`
	Phase    string   `json:"phase"`
	Evidence string   `json:"evidence"`
	Options  []string `json:"options"`
	At       string   `json:"at"`
}

// escalateEnv builds a child environment with every TP_ variable stripped
// before the case's own are added. The strip is the point: a test for "outside
// a run" must see TP_RUN_DIR genuinely unset, and inheriting the harness's
// environment would leak whichever run happens to be driving this suite.
func escalateEnv(extra ...string) []string {
	env := []string{"NO_COLOR=1"}
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "TP_") || strings.HasPrefix(entry, "NO_COLOR=") {
			continue
		}
		env = append(env, entry)
	}
	return append(env, extra...)
}

// runEscalate runs `tp escalate` with an explicit environment and reports what
// it did. It does not use runTP: that helper inherits TP_ variables it cannot
// remove, which is exactly what these cases control.
func runEscalate(t *testing.T, dir string, env []string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	cmd := exec.Command(binaryPath, append([]string{"--json", "escalate"}, args...)...)
	cmd.Dir = dir
	cmd.Env = env

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("unexpected error running tp escalate: %v", err)
	}
	return outBuf.String(), errBuf.String(), exitCode
}

// escalateUnit is one unit's identity as the driver hands it to a child
// (§3.1.1), reduced to the five variables `tp escalate` reads.
type escalateUnit struct {
	runDir string
	seq    string
	kind   string
	id     string
	phase  string
}

// unitEnv seeds a run directory and returns the child environment for a unit in
// it.
func unitEnv(t *testing.T, unit *escalateUnit) []string {
	t.Helper()
	require.NoError(t, os.MkdirAll(unit.runDir, 0o755))
	return escalateEnv(
		"TP_RUN_DIR="+unit.runDir,
		"TP_UNIT_SEQ="+unit.seq,
		"TP_UNIT_KIND="+unit.kind,
		"TP_UNIT_ID="+unit.id,
		"TP_PHASE="+unit.phase,
	)
}

// sampleUnit is a review-role unit of a run, the kind §5.2's escalation is
// most often written from.
func sampleUnit(t *testing.T) *escalateUnit {
	t.Helper()
	return &escalateUnit{
		runDir: filepath.Join(t.TempDir(), ".tp", "runs", "01JB0000000000000000000000"),
		seq:    "7",
		kind:   "review-role",
		id:     "implementer",
		phase:  "review",
	}
}

// readEscalation reads the record the unit's escalate call should have written,
// returning both the decoded record and its raw bytes — the raw form is what
// distinguishes an empty options array from a null one.
func readEscalation(t *testing.T, unit *escalateUnit) (record escalationRecord, raw string) {
	t.Helper()
	path := filepath.Join(unit.runDir, unit.seq+"-escalation.json")
	data, err := os.ReadFile(path) //nolint:gosec // a path this test just built under t.TempDir()
	require.NoError(t, err, "tp escalate writes $TP_RUN_DIR/$TP_UNIT_SEQ-escalation.json")

	require.NoError(t, json.Unmarshal(data, &record), "the record is JSON: %s", data)
	return record, string(data)
}

// TestEscalateWritesThePerUnitRecord is test 22's first half: the documented
// path, the documented fields, and exit 2.
func TestEscalateWritesThePerUnitRecord(t *testing.T) {
	unit := sampleUnit(t)
	before := time.Now().UTC().Add(-time.Second)

	stdout, stderr, code := runEscalate(t, t.TempDir(), unitEnv(t, unit),
		"--decision", "skip-gate",
		"--evidence", "the gate fails on a pre-existing lint error in a file this task never touched",
		"--option", "fix the unrelated lint error first",
		"--option", "close with --skip-gate",
	)

	assert.Equal(t, 2, code, "§5.2: tp escalate exits 2; stdout=%s stderr=%s", stdout, stderr)

	record, raw := readEscalation(t, unit)
	assert.Equal(t, "skip-gate", record.Decision)
	assert.Equal(t, "review-role", record.UnitKind, "unit_kind comes from TP_UNIT_KIND")
	assert.Equal(t, "implementer", record.UnitID, "unit_id comes from TP_UNIT_ID")
	assert.Equal(t, "review", record.Phase, "phase comes from TP_PHASE")
	assert.Equal(t,
		"the gate fails on a pre-existing lint error in a file this task never touched",
		record.Evidence)
	assert.Equal(t, []string{
		"fix the unrelated lint error first",
		"close with --skip-gate",
	}, record.Options, "--option is repeatable and keeps its order")

	at, err := time.Parse(time.RFC3339Nano, record.At)
	require.NoError(t, err, "at is a timestamp: %s", raw)
	assert.WithinRange(t, at, before, time.Now().UTC().Add(time.Second))
}

// TestEscalateAcceptsTheFiveDecisions pins §5.2's closed set, in both
// directions: each documented value writes a record, and a value outside the
// set is a usage error that writes none.
func TestEscalateAcceptsTheFiveDecisions(t *testing.T) {
	for _, decision := range []string{"skip-gate", "raise-review-cap", "raise-audit-cap", "import-force", "other"} {
		t.Run(decision, func(t *testing.T) {
			unit := sampleUnit(t)
			_, stderr, code := runEscalate(t, t.TempDir(), unitEnv(t, unit),
				"--decision", decision, "--evidence", "a decision only the operator can make")
			assert.Equal(t, 2, code, "stderr=%s", stderr)

			record, _ := readEscalation(t, unit)
			assert.Equal(t, decision, record.Decision)
		})
	}

	for _, decision := range []string{"", "skipgate", "skip_gate", "SKIP-GATE", "raise-cap"} {
		t.Run("rejects "+decision, func(t *testing.T) {
			unit := sampleUnit(t)
			args := []string{"--evidence", "a decision only the operator can make"}
			if decision != "" {
				args = append(args, "--decision", decision)
			}
			_, stderr, code := runEscalate(t, t.TempDir(), unitEnv(t, unit), args...)

			assert.Equal(t, 2, code, "an undocumented decision is a usage error")
			assert.Contains(t, stderr, "decision", "the message names what was wrong")
			assert.NoFileExists(t, filepath.Join(unit.runDir, unit.seq+"-escalation.json"),
				"a rejected escalation writes no record")
		})
	}
}

// TestEscalateRequiresEvidence: the record exists to be read by an operator, so
// a record with nothing to read is refused rather than written empty.
func TestEscalateRequiresEvidence(t *testing.T) {
	for name, args := range map[string][]string{
		"missing":    {"--decision", "other"},
		"empty":      {"--decision", "other", "--evidence", ""},
		"whitespace": {"--decision", "other", "--evidence", "   "},
	} {
		t.Run(name, func(t *testing.T) {
			unit := sampleUnit(t)
			_, stderr, code := runEscalate(t, t.TempDir(), unitEnv(t, unit), args...)

			assert.Equal(t, 2, code, "evidence is required")
			assert.Contains(t, stderr, "evidence", "the message names what was wrong")
			assert.NoFileExists(t, filepath.Join(unit.runDir, unit.seq+"-escalation.json"))
		})
	}
}

// TestEscalateWithNoOptionsWritesAnEmptyArray guards the nil-slice rule at the
// one place it would reach an agent: options[] is documented as an array, and a
// null there is a shape a driver has to special-case.
func TestEscalateWithNoOptionsWritesAnEmptyArray(t *testing.T) {
	unit := sampleUnit(t)
	_, stderr, code := runEscalate(t, t.TempDir(), unitEnv(t, unit),
		"--decision", "other", "--evidence", "no option is obviously right here")
	require.Equal(t, 2, code, "stderr=%s", stderr)

	record, raw := readEscalation(t, unit)
	assert.NotNil(t, record.Options, "options is [] rather than null: %s", raw)
	assert.Empty(t, record.Options)
	assert.Contains(t, raw, `"options": []`, "the raw record carries an empty array: %s", raw)
}

// TestEscalateOutsideARunIsAUsageError is test 22's last clause and §5.2's
// fence: without TP_RUN_DIR there is no run to escalate within, so the command
// cannot be used to fabricate a record.
func TestEscalateOutsideARunIsAUsageError(t *testing.T) {
	dir := t.TempDir()
	stdout, stderr, code := runEscalate(t, dir, escalateEnv(),
		"--decision", "skip-gate", "--evidence", "the gate fails for an unrelated reason")

	assert.Equal(t, 2, code, "outside a run tp escalate is a usage error; stdout=%s", stdout)
	assert.Contains(t, stderr, "TP_RUN_DIR", "the message names the variable that was missing")

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "a refused escalation writes nothing anywhere")
}

// TestEscalateConcurrentSiblingsEachWriteTheirOwn is test 22's middle clause.
// The record is per unit precisely so the two role siblings of one round never
// clobber each other (§5.2).
func TestEscalateConcurrentSiblingsEachWriteTheirOwn(t *testing.T) {
	runDir := filepath.Join(t.TempDir(), ".tp", "runs", "01JB0000000000000000000001")
	siblings := []*escalateUnit{
		{runDir: runDir, seq: "3", kind: "review-role", id: "implementer", phase: "review"},
		{runDir: runDir, seq: "4", kind: "review-role", id: "tester", phase: "review"},
	}

	var wg sync.WaitGroup
	for _, unit := range siblings {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, stderr, code := runEscalate(t, t.TempDir(), unitEnv(t, unit),
				"--decision", "raise-review-cap", "--evidence", unit.id+" cannot converge within the cap")
			assert.Equal(t, 2, code, "stderr=%s", stderr)
		}()
	}
	wg.Wait()

	for _, unit := range siblings {
		record, _ := readEscalation(t, unit)
		assert.Equal(t, unit.id, record.UnitID, "each sibling's record carries its own identity")
		assert.Equal(t, unit.id+" cannot converge within the cap", record.Evidence)
	}
}
