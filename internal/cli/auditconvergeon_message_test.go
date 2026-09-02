package cli_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// v0.37.0 §3: "The refusal message must name all three exits and the condition
// selecting each, because it has two audiences and the wrong pairing is
// silent." A unit that reads only "refused" stops a run over something one edit
// fixes; a unit that intended the relax and reads only the two passing
// documents writes "audit_converge_on": "all", passes, and reverts an
// operator-approved blocking with no escalation and no record.
//
// These are the clauses the refusal is asserted on. They are constants rather
// than literals repeated across four subtests so that the negative half — the
// exits a sink must NOT offer — is derived from the same strings as the
// positive half, and a reworded remedy cannot pass the negative assertion by
// no longer matching anything.
const (
	fenceCondUnintended = "if you did not mean to change it"
	fenceCondIntended   = "if you did, escalate"

	// The two import remedies. §7 row 14 is phrased on these: they are the
	// two documents §3 names, and they are remedies at tp import alone —
	// the document is the only one of the four sinks whose input the unit
	// can re-author into something that resolves the same value.
	fenceExitOmitKey    = `omit its top-level "workflow" key`
	fenceExitCarryValue = `write "audit_converge_on": "all" into it`

	// The other two sinks write the change themselves, so neither import
	// remedy exists there and the only honest unintended-case exit is not
	// to make the write.
	fenceExitDoNotWrite = "do not make this write"
	fenceExitDoNotHoist = "do not run this hoist"

	// Row 14's mutant: reusing refuseUnattendedCommandField, whose text
	// interpolates the field's own name and so satisfies any assertion
	// phrased as "the message names the field".
	fenceCommandFieldWording = "names a command the driver executes"

	// What each sink's message says about the change it refuses. The
	// change-rule sinks name the resolved value the write moves off, which
	// is what makes "carry the resolved value explicitly" actionable. tp
	// set --workflow --project applies a value rule over a layer every base
	// resolves through, so it has no single resolved value: a message
	// naming one would report one base's resolution as the tree's. The two
	// clauses are asserted against each other below, so a project message
	// that went back to the change rule's wording fails.
	fenceMsgFromAllToBlocking = "from all to blocking"
	fenceMsgEveryBase         = "the layer every base resolves through"
)

// fenceAllExits is every unintended-case exit any sink offers. Each sink names
// a subset; the assertion below is that it names its own subset and nothing
// else, which is what an over-general message fails.
var fenceAllExits = []string{
	fenceExitOmitKey, fenceExitCarryValue, fenceExitDoNotWrite, fenceExitDoNotHoist,
}

// fenceRefusal runs one write that §3's change rule refuses and returns the
// refusal's two agent-facing strings. The exit code is required rather than
// asserted: a subtest that went on to read the message of a command that
// succeeded would report a wording failure for a fence that never fired.
func fenceRefusal(t *testing.T, dir string, args ...string) (msg, hint string) {
	t.Helper()
	_, stderr, code := runTPFence(t, dir, true, args...)
	require.Equal(t, 2, code, "the fence refuses this write: %s", stderr)
	e := errJSON(t, stderr)
	msg, ok := e["error"].(string)
	require.True(t, ok, "the envelope carries a message")
	hint, ok = e["hint"].(string)
	require.True(t, ok, "the envelope carries a hint")
	return msg, hint
}

// TestAuditConvergeOnFence_RefusalNamesEveryExitAndItsCondition covers v0.37.0
// §7 row 14 at all four write sinks.
//
// Two things are asserted that a single "the message names the field" check
// cannot separate. First, the pairing: each exit is named together with the
// condition that selects it, so a unit can tell an authoring error it fixes
// itself from a decision it must escalate. Second, that the remedies are the
// sink's own — the two documents §3 names are remedies at tp import and at no
// other sink, because tp set and tp config --extract write the change
// themselves and no form of either lands while leaving the value alone. An
// over-general message that offered all four exits everywhere would satisfy
// every positive assertion here and tell a unit at tp set to go and edit a
// document that is not what refused.
func TestAuditConvergeOnFence_RefusalNamesEveryExitAndItsCondition(t *testing.T) {
	sinks := []struct {
		name  string
		build func(t *testing.T) (dir string, args []string)
		// msgClause is what this sink's message says about the change it
		// is refusing. Three sinks apply §3's change rule and name the
		// resolved value the write moves off; tp set --workflow --project
		// applies a value rule and has no single such value to name, so
		// it says what it does know — the layer the write lands in.
		msgClause string
		exits     []string
	}{
		{
			name: "set --workflow",
			build: func(t *testing.T) (string, []string) {
				return fenceShell(t, "{}", ""),
					[]string{"set", "--workflow", "audit_converge_on=blocking"}
			},
			msgClause: fenceMsgFromAllToBlocking,
			exits:     []string{fenceExitDoNotWrite},
		},
		{
			name: "set --workflow --project",
			build: func(t *testing.T) (string, []string) {
				return fenceShell(t, "{}", ""),
					[]string{"set", "--workflow", "--project", "audit_converge_on=blocking"}
			},
			msgClause: fenceMsgEveryBase,
			exits:     []string{fenceExitDoNotWrite},
		},
		{
			name: "import",
			build: func(t *testing.T) (string, []string) {
				dir := fenceShell(t, "{}", "")
				return dir, []string{"import", fenceImportDoc(t, dir, `{"audit_converge_on":"blocking"}`)}
			},
			msgClause: fenceMsgFromAllToBlocking,
			exits:     []string{fenceExitOmitKey, fenceExitCarryValue},
		},
		{
			name: "config --extract",
			build: func(t *testing.T) (string, []string) {
				return extractShell(t, "",
						extractBase{"a", extractBlockingBlock},
						extractBase{"b", extractBlockingBlock},
						extractBase{"c", ""}),
					[]string{"config", "--extract"}
			},
			msgClause: fenceMsgFromAllToBlocking,
			exits:     []string{fenceExitDoNotHoist},
		},
	}

	for _, sink := range sinks {
		t.Run(sink.name, func(t *testing.T) {
			dir, args := sink.build(t)
			msg, hint := fenceRefusal(t, dir, args...)

			// The message says which change it is refusing, in this
			// sink's own terms — and only in this sink's terms, so a
			// message that adopted the other rule's wording fails here
			// rather than passing a positive assertion by accident.
			assert.Contains(t, msg, "audit_converge_on", "the refusal names the field")
			assert.Contains(t, msg, sink.msgClause,
				"and says which change it refuses in this sink's own terms")
			for _, clause := range []string{fenceMsgFromAllToBlocking, fenceMsgEveryBase} {
				if clause == sink.msgClause {
					continue
				}
				assert.NotContains(t, msg, clause,
					"and not %q, which belongs to the other rule", clause)
			}

			// The three exits, each with its condition. Both conditions
			// have to be present: a message naming only the remedies
			// tells a unit that meant the relax to paper over it, and a
			// message naming only escalation stops a run over an edit.
			assert.Contains(t, hint, fenceCondUnintended,
				"the hint names the condition selecting the authoring exits")
			assert.Contains(t, hint, fenceCondIntended,
				"and the condition selecting escalation")
			assert.Contains(t, hint, "--decision audit-converge-on",
				"escalation is nameable under the field's own decision")
			assert.NotContains(t, hint, "--decision other", "not the fallback")

			// This sink's own exits, and only its own.
			for _, exit := range fenceAllExits {
				if slices.Contains(sink.exits, exit) {
					assert.Contains(t, hint, exit,
						"this sink offers %q", exit)
					continue
				}
				assert.NotContains(t, hint, exit,
					"and does not offer %q, which is not an exit here", exit)
			}

			// Row 14's mutant, checked on both strings rather than on
			// the message alone: a hint carrying the command-field
			// wording would reuse a claim that is false of this field.
			assert.NotContains(t, msg, fenceCommandFieldWording,
				"the refusal does not reuse the command-field wording")
			assert.NotContains(t, hint, fenceCommandFieldWording,
				"and neither does its hint")
		})
	}
}
