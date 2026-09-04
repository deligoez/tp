package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// v0.37.0 §7 rows 10 and 10b: under `blocking` a round can be stamped clean
// while still holding non-PASS rows, and `next_action` is the only place the
// operator learns that the round closed over them. Both branches §2's table
// names — converged, and clean-but-not-yet-converged — must render the count as
// a numeral, on BOTH audit sinks.
//
// The fixture carries THREE advisory rows rather than one, so the numeral under
// assertion cannot be confused with the round ordinal, the required clean-round
// count, or any other 1 the payload happens to hold. Its severities are mixed
// (`warning`, `info`) because §2 makes both advisory and a fixture of one kind
// cannot tell a count from a per-severity special case.
const auditThreeAdvisoryRound = `{"role":"spec-coverage","item_id":"i1","status":"PASS","severity":null}` + "\n" +
	`{"role":"spec-coverage","item_id":"i2","status":"PARTIAL","severity":"warning","finding":"a","suggestion":"note it"}` + "\n" +
	`{"role":"spec-coverage","item_id":"i3","status":"PARTIAL","severity":"info","finding":"b","suggestion":"note it"}` + "\n" +
	`{"role":"spec-coverage","item_id":"i4","status":"PARTIAL","severity":"warning","finding":"c","suggestion":"note it"}` + "\n"

// auditAllPassRound is the empty round the two rendered strings are compared
// against. It is what makes the assertion textual difference rather than mere
// presence: a mutant that branches on `clean` alone renders one string for both
// rounds, and every "contains a numeral" assertion phrased on its own would
// have to name the numeral it expects to be absent, which no such assertion can
// do reliably.
const auditAllPassRound = `{"role":"spec-coverage","item_id":"i1","status":"PASS","severity":null}` + "\n"

// The two strings an empty round produces, pinned verbatim: this release
// changes what a round HOLDING accepted rows says and leaves the empty round
// exactly where v0.36.0 left it.
const (
	auditNextRoundDirective = "run the next audit round: tp audit spec.md --record <file>"
	auditReleaseDirective   = "converged — implementation verified, proceed to release"
)

// auditNextActionRun records `rounds` identical rounds under `blocking` and
// returns the payload of the last --record beside the payload of a --status run
// over the same state. Both are returned from one helper because §7 rows 10 and
// 10b are each an assertion about the two sinks agreeing: a count carried on
// --record alone leaves `tp audit --status`, the invocation a gated driver
// actually runs, silent about the rows the round closed over.
func auditNextActionRun(t *testing.T, rows string, rounds int) (record, status map[string]any) {
	t.Helper()
	dir := auditStampingProject(t)
	setAuditConvergeOn(t, dir, "project", "blocking")
	for i := 1; i <= rounds; i++ {
		out, stderr, code := auditRecord(t, dir, rows)
		require.Equal(t, 0, code, "recording round %d: %s", i, stderr)
		require.Equal(t, true, out["clean"],
			"round %d must be stamped clean under blocking, or neither branch is reached", i)
		record = out
	}
	status, _ = auditStatusPayload(t, dir)
	return record, status
}

// nextActionOf reads the advisory string off an audit payload.
func nextActionOf(t *testing.T, payload map[string]any) string {
	t.Helper()
	s, ok := payload["next_action"].(string)
	require.True(t, ok, "next_action is an emitted string: %v", payload["next_action"])
	return s
}

// TestAuditNextAction_CleanRoundNamesTheAcceptedCount covers §7 row 10b: the
// default arm — the branch a `blocking` cycle takes on EVERY clean round, where
// the converged arm fires once per cycle. The named mutant is carrying the
// count into the converged arm only, which leaves the common branch silent.
func TestAuditNextAction_CleanRoundNamesTheAcceptedCount(t *testing.T) {
	t.Parallel()
	held, heldStatus := auditNextActionRun(t, auditThreeAdvisoryRound, 1)
	require.Equal(t, false, held["converged"],
		"one clean round is not the default two, so this is the clean-but-not-converged arm")
	none, noneStatus := auditNextActionRun(t, auditAllPassRound, 1)
	require.Equal(t, false, none["converged"])

	for _, tc := range []struct {
		sink       string
		held, none map[string]any
	}{
		{"--record", held, none},
		{"--status", heldStatus, noneStatus},
	} {
		t.Run(tc.sink, func(t *testing.T) {
			withRows, empty := nextActionOf(t, tc.held), nextActionOf(t, tc.none)
			assert.Equal(t, auditNextRoundDirective, empty,
				"an empty clean round says exactly what it said before this release")
			assert.Contains(t, withRows, "3 accepted rows",
				"the accepted count is rendered as a numeral, in the noun phrase")
			assert.Contains(t, withRows, "run the next audit round",
				"the clean round still takes the default arm, not the fix-and-re-audit branch")
			assert.NotEqual(t, empty, withRows,
				"a round closing over accepted rows reads differently from an empty one")
		})
	}
}

// TestAuditNextAction_ConvergedRoundNamesTheAcceptedCount covers §7 row 10: the
// converged arm. The named mutant is branching on `clean` alone, which makes
// the two strings equal — which is why the assertion is NotEqual against the
// empty round rather than a presence check.
func TestAuditNextAction_ConvergedRoundNamesTheAcceptedCount(t *testing.T) {
	t.Parallel()
	held, heldStatus := auditNextActionRun(t, auditThreeAdvisoryRound, 2)
	require.Equal(t, true, held["converged"],
		"two clean rounds meet the default audit_clean_rounds, so this is the converged arm")
	none, noneStatus := auditNextActionRun(t, auditAllPassRound, 2)
	require.Equal(t, true, none["converged"])

	for _, tc := range []struct {
		sink       string
		held, none map[string]any
	}{
		{"--record", held, none},
		{"--status", heldStatus, noneStatus},
	} {
		t.Run(tc.sink, func(t *testing.T) {
			withRows, empty := nextActionOf(t, tc.held), nextActionOf(t, tc.none)
			assert.Equal(t, auditReleaseDirective, empty,
				"an empty converged round says exactly what it said before this release")
			assert.Contains(t, withRows, "3 accepted rows",
				"the accepted count is rendered as a numeral, in the noun phrase")
			assert.Contains(t, withRows, "proceed to release",
				"the terminal marker survives; the count is added to it, not substituted for it")
			assert.NotContains(t, withRows, "tp audit",
				"the terminal marker still names no further tp command")
			assert.NotEqual(t, empty, withRows,
				"a release reached over accepted rows reads differently from one reached clean")
		})
	}
}
