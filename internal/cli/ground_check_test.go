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

// TestCheckExitsOneWhileTheRoundCarriesARefutedClaim is the defect this gate
// shipped with: a round of nothing but FAILs is fully covered, so a check that
// gated on coverage alone exited 0 over it, and a driver stopping on exit 0
// stopped with the false claims standing in the spec.
//
// Every arm is FULLY covered — `dispositioned == emitted` is required before the
// code is read — so no exit 1 below can come from the coverage condition, and
// each arm's verdict pair is what decides it. Only FAIL blocks. PARTIAL does
// not: a PARTIAL is often true-when-written (a count over a growing
// population), which no repair makes permanently true, so a gate on it could
// leave the loop with no end. The payload counts it and Step 1.5 still says to
// repair it. A gate keyed on "any non-PASS" reddens every non-FAIL arm, and a
// gate keyed on FAIL or PARTIAL reddens the PARTIAL arms.
//
// The payload is compared whole against `--status`'s, because the gate adds a
// bit to the exit status and nothing to what tp prints: `by_verdict` already
// carries the counts the code turns on, so the code is reconstructible from the
// payload it just printed.
func TestCheckExitsOneWhileTheRoundCarriesARefutedClaim(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		verdicts []string
		want     int
	}{
		{"two FAIL rows", []string{"FAIL", "FAIL"}, 1},
		{"a FAIL beside an UNVERIFIABLE", []string{"FAIL", "UNVERIFIABLE"}, 1},
		{"a FAIL beside a PARTIAL", []string{"FAIL", "PARTIAL"}, 1},
		{"a PARTIAL beside a PASS", []string{"PARTIAL", "PASS"}, 0},
		{"a PARTIAL beside a QUESTION", []string{"PARTIAL", "QUESTION"}, 0},
		{"a QUESTION beside an UNVERIFIABLE", []string{"QUESTION", "UNVERIFIABLE"}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := writeGroundFixture(t)
			groundEmit(t, dir)
			emitted, _ := groundFloorIDs(t, dir, 1)
			require.Len(t, emitted, 2)
			lines := make([]string, 0, len(emitted))
			for i, id := range emitted {
				switch tc.verdicts[i] {
				case "QUESTION":
					lines = append(lines, groundQuestionRow(t, dir, id))
				case "PARTIAL":
					// §7.2 requires partial_kind on a PARTIAL row; the
					// shared builder carries no such field.
					row := groundVerdictRow(t, dir, id, "PARTIAL")
					lines = append(lines, strings.TrimSuffix(row, "}")+`,"partial_kind":"two-readings"}`)
				default:
					lines = append(lines, groundVerdictRow(t, dir, id, tc.verdicts[i]))
				}
			}
			_, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", writeGroundRows(t, dir, lines...))
			require.Equal(t, 0, code, "stderr: %s", stderr)

			payload, code := groundStatusCheck(t, dir)
			require.Equal(t, payload["emitted"], payload["dispositioned"],
				"the floor is fully dispositioned, so coverage cannot be what decides the code")
			assert.Equal(t, groundStatus(t, dir), payload, "--check changes nothing tp prints")
			assert.Equal(t, tc.want, code,
				"--check exits 1 while the round holds a FAIL, and 0 over PARTIAL, QUESTION and UNVERIFIABLE")
		})
	}
}

// TestCheckStillExitsOneWhenTheRefutedClaimIsCarried closes the path a driver
// takes after round 1: re-emit without repairing, hand in the empty payload the
// carry makes legal, record. The FAIL is not re-asked — it is carried into round
// 2's own file — so a gate that read only what the latest payload handed in
// would exit 0 over a claim nobody repaired.
func TestCheckStillExitsOneWhenTheRefutedClaimIsCarried(t *testing.T) {
	t.Parallel()
	dir := writeGroundFixture(t)
	groundEmit(t, dir)
	emitted, _ := groundFloorIDs(t, dir, 1)
	require.Len(t, emitted, 2)
	recordGroundRound(t, dir, emitted, []string{"FAIL", "PASS"})

	require.Equal(t, float64(2), groundEmit(t, dir)["round"])
	_, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", writeGroundRows(t, dir))
	require.Equal(t, 0, code, "an empty payload is legal when every unit carries: %s", stderr)

	payload, code := groundStatusCheck(t, dir)
	require.Equal(t, float64(2), payload["round"])
	require.Equal(t, payload["emitted"], payload["dispositioned"], "every unit carried, so the floor is covered")
	require.Equal(t, float64(1), groundStatusVerdicts(t, payload)["FAIL"], "the FAIL carried into round 2")
	assert.Equal(t, 1, code, "an unrepaired FAIL carries forward, and so does the exit 1")
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
