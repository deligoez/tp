package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/engine"
)

// The two documents that additionally render the audit prompt's body order and
// the prompts[].role contract verbatim (§4). The four that state the routing
// contract itself are repoRootDocs, shared with the per-spec lever guard.
var auditPromptOrderDocs = []string{
	"skills/tp/SKILL.md",
	"skills/tp/REFERENCE.md",
}

const (
	routingSubstring = "spec-coverage is the only auditor id that changes routing"
	upgradeSubstring = "ejected role files are not rewritten on upgrade"

	sharedArmOrder    = "Role → Role Rules → Project Context → JSON-array Checklist → Disposition → Affected Files → Output Schema"
	specCoverageOrder = "Role → Role Rules → Spec Excerpt → JSON-array Checklist → Affected Files → Output Schema"

	roleValueSubstring = "any active role id from the corpus"
	itemIDSubstring    = "file-<role-id>-<slug>"
)

// TestDocsStateOneAuditRoutingContract guards §4 (pinned by §7 item 15): the
// four documents that state part of the audit routing contract must tell one
// story, and every §4 requirement is guarded by a required substring rather
// than only by the absence of a superseded one — so deleting the old wording
// without writing the new one fails here too.
func TestDocsStateOneAuditRoutingContract(t *testing.T) {
	for _, doc := range repoRootDocs {
		text := readRepoDoc(t, doc)
		assert.Contains(t, text, routingSubstring, "%s states the routing rule", doc)
		assert.Contains(t, text, upgradeSubstring, "%s records that an ejected corpus keeps its old copies", doc)
		assert.NotContains(t, text, "file-sec-", "%s carries no superseded item-id prefix", doc)
		assert.NotContains(t, text, "file-maint-", "%s carries no superseded item-id prefix", doc)
	}

	for _, doc := range auditPromptOrderDocs {
		text := readRepoDoc(t, doc)
		assert.Contains(t, text, sharedArmOrder, "%s renders the shared-arm prompt body order", doc)
		assert.Contains(t, text, specCoverageOrder, "%s renders the spec-coverage prompt body order", doc)
		assert.Contains(t, text, roleValueSubstring, "%s states prompts[].role as a corpus role id", doc)
		assert.NotContains(t, text, "always `\"implementation-auditor\"`",
			"%s no longer renders the superseded prompts[].role value", doc)
		for _, enum := range []string{
			"`spec-coverage` \\| `security` \\| `maintainability-conventions`",
			"`spec-coverage` | `security` | `maintainability-conventions`",
		} {
			assert.NotContains(t, text, enum, "%s no longer renders prompts[].role as a three-id enum", doc)
		}
	}

	assert.Contains(t, readRepoDoc(t, "skills/tp/REFERENCE.md"), itemIDSubstring,
		"REFERENCE.md documents the deterministic item ids in the role-id slug form")
}

// convergenceSummaryDocs are the two driver-facing documents §4 asks for both
// summary substrings. CLAUDE.md and REFERENCE.md carry requirements of their
// own, which is why this set is not repoRootDocs.
var convergenceSummaryDocs = []string{
	"README.md",
	"skills/tp/SKILL.md",
}

const (
	divergenceCountingSubstring = "audit convergence still counts every non-PASS row"
	checkRetirementSubstring    = "a registered check retires its mechanize candidate"

	claudeSignalFieldSubstring = "tp audit now reports spec_coverage_clean_rounds"
)

// referenceSignalSentences are the first four of §4's five verbatim sentences
// for skills/tp/REFERENCE.md.
//
// The fifth — the divergence.hint constant — is deliberately absent here and is
// asserted against engine.DivergenceHint instead. §4 pins that sentence so the
// code constant, the documentation and the guard test cannot drift apart; a
// literal copy in this file would only pin the document against a second copy,
// leaving the constant free to drift away from both. Comparing the document
// against the shipped constant is the assertion that actually holds them
// together.
var referenceSignalSentences = []string{
	"A role with no rows in a round is not clean in it, so its streak ends.",
	"spec_coverage_clean_rounds is null, not 0, when the latest recorded round holds no spec-coverage row.",
	"role_streaks, spec_coverage_clean_rounds and divergence all survive --compact, divergence with every field.",
	"mechanized_classes names the candidate classes withheld because they are mechanized, and is [] when none were.",
}

// TestDocsCarryTheConvergenceSignalWording guards §4 (pinned by §6 test 46):
// the four documents this release changes must carry their required substrings
// verbatim, so a release that ships the fields without the wording — or that
// reworders the wording later — fails here.
func TestDocsCarryTheConvergenceSignalWording(t *testing.T) {
	for _, doc := range convergenceSummaryDocs {
		text := readRepoDoc(t, doc)
		assert.Contains(t, text, divergenceCountingSubstring,
			"%s states that audit convergence still counts every non-PASS row", doc)
		assert.Contains(t, text, checkRetirementSubstring,
			"%s states that a registered check retires its mechanize candidate", doc)
	}

	assert.Contains(t, readRepoDoc(t, "CLAUDE.md"), claudeSignalFieldSubstring,
		"CLAUDE.md's audit-scope rule names the field carrying the spec-coverage streak")

	reference := readRepoDoc(t, "skills/tp/REFERENCE.md")
	for _, sentence := range referenceSignalSentences {
		assert.Contains(t, reference, sentence,
			"REFERENCE.md carries §4's verbatim sentence %q", sentence)
	}
	assert.Contains(t, reference, engine.DivergenceHint,
		"REFERENCE.md quotes the shipped engine.DivergenceHint verbatim, so the constant, the document and this test cannot drift apart")
}
