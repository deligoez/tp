package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTheEmissionReportsItsCountsAsNumbers pins the two figures the emission
// computes and, before this test, spent only on English.
//
// runGround derives the floor size and the carried count to build the prompt's
// ask, so both exist at the moment the envelope is written. Neither reached it:
// floor_size cost an operator a second process (--status's `emitted`), and
// `carried` was in no JSON at all until the round was recorded -- while
// --record's own envelope has carried `rows` and `carried` as integers since it
// shipped, with a comment explaining why two counts and not one. The asymmetry
// is the finding: the same two numbers, machine-readable from one mode and
// prose-only from the other.
//
// The second clause is what makes this more than tidiness. A carry that FAILS
// -- an unreadable round N-1 -- leaves the emission at exit 0 with an
// unchanged-shape envelope asking for every unit, and output.Notice on stderr
// is the only signal, which --quiet removes. `carried: 0` beside a floor that
// did not change is a fact a reader can branch on.
func TestTheEmissionReportsItsCountsAsNumbers(t *testing.T) {
	t.Parallel()
	dir := writeGroundFixture(t)

	// Round 1: nothing precedes it, so the round owes its whole floor.
	first := groundEmit(t, dir)
	floorSize, ok := first["floor_size"].(float64)
	require.True(t, ok, "the emission must report floor_size as a number, got %#v", first["floor_size"])
	carried, ok := first["carried"].(float64)
	require.True(t, ok, "the emission must report carried as a number, got %#v", first["carried"])
	assert.Equal(t, float64(0), carried, "round 1 has no preceding round to carry from")

	// The floor size is the units the round owes a disposition for, not the
	// index's row count -- the fixture's cut unit is in the index and not in
	// the ask, so a test that let the two coincide would not tell them apart.
	rows := groundEmittedFloor(t, dir, 1)
	require.Greater(t, len(rows), int(floorSize),
		"the fixture must hold a cut unit, or floor_size and the index row count cannot be told apart")

	// Round 2 on an unedited spec: section 8 carries every disposition, so the
	// ask is zero and `carried` equals the floor size. Both numbers move, and
	// they move in opposite directions, which is what makes the pair readable.
	emitted := make([]string, 0, len(rows))
	for _, unit := range rows {
		if unit.TextSHA == "" {
			continue // a cut unit owes nothing
		}
		emitted = append(emitted, groundCarryRowFor(unit, 1))
	}
	_, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", writeGroundRows(t, dir, emitted...))
	require.Equal(t, 0, code, "stderr: %s", stderr)

	second := groundEmit(t, dir)
	assert.Equal(t, floorSize, second["floor_size"],
		"floor_size is the floor, not the ask -- nobody edited the spec, so it must not move")
	assert.Equal(t, floorSize, second["carried"],
		"and every one of them carries what round 1 decided, which is what makes the ask zero")

	// The pair, not either number, is what a reader branches on: the ask is
	// their difference, and it is zero here. A first draft of this test asserted
	// floor_size == 0 in round 2, which would have passed only if floor_size
	// meant the ask -- it does not, and the prompt below is the artifact that
	// settles which reading is right.
	require.Contains(t, second["prompt"], "0 of the",
		"the prompt's ask is floor_size minus carried, and here that is zero")
}

// TestUnitsSaysSoWhenItIgnoresJSON pins the one thing wrong with --units
// --json, and not the thing that is right about it.
//
// --units prints text on purpose: row 4b's assertion is about LINES, and
// jsonMode is on for every piped invocation, so an envelope would leave the
// shipped shape reachable only from a terminal. That decision is documented and
// this test does not contest it. What was wrong is the silence -- exit 0, TSV
// on stdout, and zero bytes on stderr, so a caller that asked for JSON and got
// TSV had nothing to read. output.Notice's own doc names "a flag ignored" as
// exactly what the channel is for.
func TestUnitsSaysSoWhenItIgnoresJSON(t *testing.T) {
	t.Parallel()
	dir := writeGroundFixture(t)

	stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--units", "--json")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	// Still text, still tab-separated: the notice must not have changed what
	// the mode prints.
	require.NotEmpty(t, stdout)
	assert.Contains(t, stdout, "\t", "--units prints TSV whatever --json says")
	var envelope any
	assert.Error(t, json.Unmarshal([]byte(stdout), &envelope),
		"--units must not start emitting JSON: the notice is the repair, not an envelope")

	assert.Contains(t, stderr, "--json", "the ignored flag must be named on stderr")

	// The control: without --json there is nothing to announce, so stderr is
	// silent. Without it, a notice printed unconditionally would pass.
	//
	// It cannot go through runTP, which prepends --json to every invocation --
	// so the control has to build its own command, and a first draft of this
	// test that used the helper reported the notice firing on a bare --units
	// when what it had actually measured was the helper's own flag. The
	// distinction under test is Changed("json") against IsJSON(), and a helper
	// that always passes the flag collapses exactly that distinction.
	bare := exec.Command(binaryPath, "ground", "spec.md", "--units")
	bare.Dir = dir
	bare.Env = append(os.Environ(), "NO_COLOR=1", "TP_HC=0")
	var bareErr bytes.Buffer
	bare.Stderr = &bareErr
	bareOut, err := bare.Output()
	require.NoError(t, err)
	require.NotEmpty(t, bareOut, "the control must reach the same code path, not fail early")
	assert.NotContains(t, bareErr.String(), "--json",
		"--units alone ignores nothing, so it announces nothing")
}
