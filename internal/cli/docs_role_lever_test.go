package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// perSpecLeverSubstring is §4's pinned wording. It is asserted verbatim, so a
// document that wraps `enabled: false` in inline-code backticks inside the
// phrase — which reads identically but breaks the substring — fails here.
const perSpecLeverSubstring = "enabled: false deactivates a role for one spec"

// referenceTrimLeverSubstrings are the claims §4 requires REFERENCE.md to make
// about a trim_candidate's two levers. Each is a required substring rather than
// the absence of a wrong one, so dropping the guidance fails the test instead
// of passing unnoticed. None of them spans a markdown emphasis boundary, so
// rewrapping the surrounding prose does not break them.
var referenceTrimLeverSubstrings = []string{
	// The two levers exist and are guarded differently.
	"two levers, and they are guarded differently",
	// Lever 1 — enabled: false, one spec, guarded by the two refusals.
	"by the two emission-only refusals: it cannot empty a phase",
	"it cannot deactivate the `spec-coverage` auditor",
	// Lever 2 — deleting the role file, every spec, unguarded.
	"Deleting the role file",
	"nothing protects it from file deletion",
	// Lever 2's silent no-op on the phase's last role file.
	"role file removes nothing: an empty phase directory reads as unpopulated",
	"falls back to the embedded default corpus",
	// Neither lever is guarded against a role with open findings.
	"guard against deactivating a role that has open findings",
}

// TestDocsStateThePerSpecRoleLever guards §4 (pinned by §6 item 17): the
// documents that own the role-corpus rules must each carry the pinned
// substring, and REFERENCE.md must state both of a trim_candidate's levers and
// how differently they are guarded. v0.34.0 §8.1 narrowed the set from four
// documents to those two owners.
func TestDocsStateThePerSpecRoleLever(t *testing.T) {
	for _, doc := range roleContractDocs {
		assert.Contains(t, readRepoDoc(t, doc), perSpecLeverSubstring,
			"%s states the per-spec deactivation lever verbatim", doc)
	}

	ref := readRepoDoc(t, "skills/tp/REFERENCE.md")
	for _, want := range referenceTrimLeverSubstrings {
		assert.Contains(t, ref, want,
			"REFERENCE.md states both trim-candidate levers and how each is guarded")
	}
}
