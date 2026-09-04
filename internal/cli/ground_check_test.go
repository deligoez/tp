package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// groundStatusCheck runs `tp ground spec.md --status --check` and returns the
// decoded payload beside the exit code.
//
// Both are returned because §7.1 makes --check a read-back rather than a mode:
// it is --status plus one bit, so the payload is asserted on the same run that
// asserts the code. Decoding it is itself an assertion — an implementation that
// exited before printing leaves stdout empty and fails here.
func groundStatusCheck(t *testing.T, dir string) (payload map[string]any, exitCode int) {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--status", "--check")
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload),
		"--status --check carries --status's payload whatever it exits: stdout %q stderr %q", stdout, stderr)
	return payload, code
}

// recordGroundRound records one round over ids with the verdicts named, one per
// id, and requires it to be accepted.
func recordGroundRound(t *testing.T, dir string, ids, verdicts []string) {
	t.Helper()
	require.Len(t, verdicts, len(ids), "one verdict per row")
	lines := make([]string, 0, len(ids))
	for i, id := range ids {
		lines = append(lines, groundVerdictRow(t, dir, id, verdicts[i]))
	}
	rows := writeGroundRows(t, dir, lines...)
	_, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows)
	require.Equal(t, 0, code, "stderr: %s", stderr)
}

// TestCheckExitsZeroOnCompleteCoverageAndOneOtherwise is §7.1's fifth
// invocation: `--status --check` exits 0 when every emitted floor unit carries a
// disposition, 1 otherwise.
//
// The three cases are 0-of-2, 1-of-2 and 2-of-2 against the same fixture, which
// is what puts the assertion AT the boundary: a comparison relaxed from `<` to
// `<=` reddens the complete case alone, and one dropped altogether reddens the
// two incomplete ones alone. 0-of-2 is not redundant with 1-of-2 — it is the
// state in which no row exists at all, which an implementation reading "no
// undispositioned rows" instead of "every unit dispositioned" reports as clean.
func TestCheckExitsZeroOnCompleteCoverageAndOneOtherwise(t *testing.T) {
	t.Parallel()
	t.Run("nothing recorded against an emitted round", func(t *testing.T) {
		dir := writeGroundFixture(t)
		groundEmit(t, dir)

		payload, code := groundStatusCheck(t, dir)
		require.Equal(t, float64(2), payload["emitted"])
		require.Equal(t, float64(0), payload["dispositioned"],
			"the round is emitted and not recorded, so nothing in it has been decided")
		assert.Equal(t, 1, code, "an emitted round nobody has dispositioned is not covered")
	})

	t.Run("one of the two floor units dispositioned", func(t *testing.T) {
		dir := writeGroundFixture(t)
		groundEmit(t, dir)
		emitted, _ := groundFloorIDs(t, dir, 1)
		require.Len(t, emitted, 2, "the fixture emits two floor units")
		recordGroundRound(t, dir, emitted[:1], []string{"PASS"})

		payload, code := groundStatusCheck(t, dir)
		require.Equal(t, float64(2), payload["emitted"])
		require.Equal(t, float64(1), payload["dispositioned"], "deliberately one short of the floor")
		assert.Equal(t, 1, code, "a floor unit carrying no disposition exits 1 (§7.1)")
	})

	t.Run("every floor unit dispositioned", func(t *testing.T) {
		dir := writeGroundFixture(t)
		groundEmit(t, dir)
		emitted, _ := groundFloorIDs(t, dir, 1)
		require.Len(t, emitted, 2)
		recordGroundRound(t, dir, emitted, []string{"PASS", "PASS"})

		payload, code := groundStatusCheck(t, dir)
		require.Equal(t, payload["emitted"], payload["dispositioned"],
			"every emitted floor unit carries a disposition")
		assert.Equal(t, 0, code, "complete coverage exits 0 (§7.1)")
	})
}

// TestCheckGatesOnCoverageAndNotOnWhatTheRoundFound is the acceptance's "gates
// nothing else", and §8's reason for it: coverage answers *did anyone look*,
// never *did the premises hold*. A spec whose every claim was refuted is 100%
// covered, so it exits 0.
//
// The round is FAIL and UNVERIFIABLE rather than a single verdict, so a gate
// keyed on either one alone still reddens here. The FAIL count is read back
// from the same payload, which is what makes the exit code's silence about it
// visible: the round tp reports as two failures is the round tp exits 0 on.
func TestCheckGatesOnCoverageAndNotOnWhatTheRoundFound(t *testing.T) {
	t.Parallel()
	dir := writeGroundFixture(t)
	groundEmit(t, dir)
	emitted, _ := groundFloorIDs(t, dir, 1)
	require.Len(t, emitted, 2)
	recordGroundRound(t, dir, emitted, []string{"FAIL", "UNVERIFIABLE"})

	payload, code := groundStatusCheck(t, dir)
	require.Equal(t, payload["emitted"], payload["dispositioned"], "the floor is fully dispositioned")

	byVerdict := groundStatusVerdicts(t, payload)
	require.Equal(t, float64(1), byVerdict["FAIL"], "the round refuted a claim")
	require.Equal(t, float64(1), byVerdict["UNVERIFIABLE"], "and could not reach another")
	assert.Equal(t, 0, code,
		"a FAIL is a disposition: --check gates on coverage and says nothing about the verdicts (§8)")
}

// TestNothingRefusesOnCoverage is Non-Goal 3 in the direction it can be
// crossed. `--check`'s exit code is a read-back an operator branches on; no tp
// invocation *refuses* because coverage is incomplete.
//
// The fixture is deliberately at 1-of-2 — the state the test above pins as
// exit 1 under --check — so every exit 0 below is measured against a spec
// grounding has not finished with, and not against a vacuous one.
func TestNothingRefusesOnCoverage(t *testing.T) {
	t.Parallel()
	dir := writeGroundFixture(t)
	groundEmit(t, dir)
	emitted, _ := groundFloorIDs(t, dir, 1)
	require.Len(t, emitted, 2)
	recordGroundRound(t, dir, emitted[:1], []string{"PASS"})

	uncovered, checkCode := groundStatusCheck(t, dir)
	require.Equal(t, 1, checkCode, "the fixture is incomplete, which is what makes the runs below a test")

	t.Run("--status alone reports it and exits 0", func(t *testing.T) {
		// groundStatus requires exit 0 itself, and the payloads are compared
		// as whole objects: --check adds one bit to the exit status and
		// changes nothing tp prints.
		assert.Equal(t, uncovered, groundStatus(t, dir),
			"--status --check carries exactly --status's payload")
	})

	t.Run("a second emission runs on an uncovered spec", func(t *testing.T) {
		require.Equal(t, float64(2), groundEmit(t, dir)["round"],
			"emitting round 2 does not require round 1 to have been covered")
	})

	t.Run("--record runs on an uncovered spec", func(t *testing.T) {
		emittedTwo, _ := groundFloorIDs(t, dir, 2)
		recordGroundRound(t, dir, emittedTwo[:1], []string{"PASS"})
	})
}

// TestCheckWithoutStatusIsAUsageErrorByTheRuleAndNotAnUnknownFlag is §7.1's
// second exit-2 input.
//
// Exit 2 alone cannot decide it: before this task `tp ground --check` already
// exited 2, because the flag was unregistered and cobra's flag-parse failure is
// routed to the same code. The verdict therefore rests on the FIRST require
// below — `--status --check` exiting 0 — which no unknown-flag path can
// produce, so the exit 2 that follows it is the rule refusing a combination tp
// understands. The message assertion is corroboration, not the verdict: a
// NotContains over prose is a presence assertion over a one-item blacklist.
func TestCheckWithoutStatusIsAUsageErrorByTheRuleAndNotAnUnknownFlag(t *testing.T) {
	t.Parallel()
	dir := writeGroundFixture(t)
	groundEmit(t, dir)
	emitted, _ := groundFloorIDs(t, dir, 1)
	require.Len(t, emitted, 2)
	recordGroundRound(t, dir, emitted, []string{"PASS", "PASS"})

	_, code := groundStatusCheck(t, dir)
	require.Equal(t, 0, code,
		"tp parses --check as its own flag, so an exit 2 below is the rule and not cobra's unknown-flag path")

	before := stateDirNames(t, dir)

	t.Run("--check alone", func(t *testing.T) {
		stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--check")
		require.Equal(t, 2, code, "stdout: %s stderr: %s", stdout, stderr)

		envelope := groundErrorEnvelope(t, stderr)
		assert.Equal(t, float64(2), envelope["code"])
		assert.NotContains(t, strings.ToLower(envelope["error"].(string)), "unknown flag",
			"the refusal is §7.1's rule, not a flag tp does not know")
		assert.Equal(t, before, stateDirNames(t, dir), "a usage refusal writes nothing")
	})

	t.Run("--check beside --record", func(t *testing.T) {
		rows := writeGroundRows(t, dir, groundVerdictRow(t, dir, emitted[0], "PASS"))
		stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows, "--check")
		require.Equal(t, 2, code, "stdout: %s stderr: %s", stdout, stderr)
		assert.Equal(t, float64(2), groundErrorEnvelope(t, stderr)["code"])
		assert.Equal(t, before, stateDirNames(t, dir),
			"--record is not --status, so this is the same refusal and it records nothing")
	})
}

// TestCheckOnASpecWithNoEmittedRoundExitsThree keeps the coverage gate apart
// from the state one. Exit 1 is this release's answer to *the floor is not
// covered*; a spec nobody has ever grounded has no floor to be uncovered, and
// §7.1 maps that to 3 through --status's own refusal.
//
// Without this, an implementation folding every non-complete state into 1 is
// indistinguishable from the shipped one on the tests above.
func TestCheckOnASpecWithNoEmittedRoundExitsThree(t *testing.T) {
	t.Parallel()
	dir := writeGroundFixture(t)

	stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--status", "--check")
	require.Equal(t, 3, code, "stdout: %s stderr: %s", stdout, stderr)
	assert.Equal(t, float64(3), groundErrorEnvelope(t, stderr)["code"])
}
