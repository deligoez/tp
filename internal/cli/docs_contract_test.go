package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/engine"
)

// The two SKILL.md lines §8a.2 names: the mechanize-candidate rule and the
// next_action precedence list's step 3. Both must carry engine's shipped
// qualifier verbatim, so the emitted advice and the documented rule cannot
// diverge. Anchoring per line rather than per document is what makes the
// assertion discriminating — the sentence sitting anywhere else in SKILL.md
// does not satisfy either anchor.
const (
	mechanizeRuleAnchor = "**Mechanization candidate:**"
	mechanizeStepAnchor = "3. **A `mechanize_candidates` class recurs"
)

// TestDocsCarryTheMechanizePhaseQualifier guards §8a.2 (pinned by §10.5 test 29).
func TestDocsCarryTheMechanizePhaseQualifier(t *testing.T) {
	seen := map[string]bool{}
	for _, line := range strings.Split(readRepoDoc(t, "skills/tp/SKILL.md"), "\n") {
		for _, anchor := range []string{mechanizeRuleAnchor, mechanizeStepAnchor} {
			if !strings.Contains(line, anchor) {
				continue
			}
			seen[anchor] = true
			assert.Contains(t, line, engine.MechanizePhaseQualifier,
				"the SKILL.md line carrying %q states the phase qualifier verbatim", anchor)
		}
	}
	for _, anchor := range []string{mechanizeRuleAnchor, mechanizeStepAnchor} {
		assert.True(t, seen[anchor], "SKILL.md still carries the %q line", anchor)
	}
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
// document that states the audit routing contract must tell one story, and
// every §4 requirement is guarded by a required substring rather than only by
// the absence of a superseded one — so deleting the old wording without writing
// the new one fails here too.
//
// v0.34.0 §8.1 narrowed the set from four documents to two, and the v0.34.0
// audit narrowed it again to the single owner: REFERENCE.md restated the
// routing rule and the eject-on-upgrade rule verbatim beside its schema, which
// is the duplication §8.1 forbids. The prompt body orders and the
// prompts[].role contract stay schema detail REFERENCE.md renders alone, and
// both documents still have to be free of the superseded item-id prefixes.
func TestDocsStateOneAuditRoutingContract(t *testing.T) {
	skill := readRepoDoc(t, roleContractDoc)
	assert.Contains(t, skill, routingSubstring, "%s states the routing rule", roleContractDoc)
	assert.Contains(t, skill, upgradeSubstring,
		"%s records that an ejected corpus keeps its old copies", roleContractDoc)

	reference := readRepoDoc(t, "skills/tp/REFERENCE.md")
	for doc, text := range map[string]string{
		roleContractDoc:          skill,
		"skills/tp/REFERENCE.md": reference,
	} {
		assert.NotContains(t, text, "file-sec-", "%s carries no superseded item-id prefix", doc)
		assert.NotContains(t, text, "file-maint-", "%s carries no superseded item-id prefix", doc)
	}

	assert.Contains(t, reference, sharedArmOrder, "REFERENCE.md renders the shared-arm prompt body order")
	assert.Contains(t, reference, specCoverageOrder, "REFERENCE.md renders the spec-coverage prompt body order")
	assert.Contains(t, reference, roleValueSubstring, "REFERENCE.md states prompts[].role as a corpus role id")
	assert.NotContains(t, reference, "always `\"implementation-auditor\"`",
		"REFERENCE.md no longer renders the superseded prompts[].role value")
	for _, enum := range []string{
		"`spec-coverage` \\| `security` \\| `maintainability-conventions`",
		"`spec-coverage` | `security` | `maintainability-conventions`",
	} {
		assert.NotContains(t, reference, enum, "REFERENCE.md no longer renders prompts[].role as a three-id enum")
	}

	assert.Contains(t, reference, itemIDSubstring,
		"REFERENCE.md documents the deterministic item ids in the role-id slug form")
}

const (
	divergenceCountingSubstring = "audit convergence counts every non-PASS row the resolved policy does not accept"

	// The SKILL.md line that must carry it: the divergence paragraph itself,
	// not the document. Anchored for the reason
	// TestDocsCarryTheMechanizePhaseQualifier anchors — measured on this
	// change, the sentence pasted into an unrelated section kept a
	// document-wide assertion green while the divergence paragraph still said
	// convergence counts every non-PASS row full stop.
	divergenceParagraphAnchor = "**`divergence` gates nothing — it is reporting only.**"
	checkRetirementSubstring  = "a registered check retires its mechanize candidate"

	claudeSignalFieldSubstring = "spec_coverage_clean_rounds"
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
// the documents that describe the convergence signals must carry their required
// substrings verbatim, so a release that ships the fields without the wording —
// or that rewords them later — fails here.
//
// v0.34.0 §8.1 moved the driver-facing summary into SKILL.md alone: README.md
// now points at the skill for the audit loop rather than restating it, so
// asserting the sentences there would pin a duplicate the sweep removed.
// CLAUDE.md's audit-scope rule still has to name the field carrying the
// spec-coverage streak, because that rule is this repository's own and its
// meaning depends on the field.
func TestDocsCarryTheConvergenceSignalWording(t *testing.T) {
	skill := readRepoDoc(t, "skills/tp/SKILL.md")
	anchored := false
	for _, line := range strings.Split(skill, "\n") {
		if !strings.Contains(line, divergenceParagraphAnchor) {
			continue
		}
		anchored = true
		assert.Contains(t, line, divergenceCountingSubstring,
			"SKILL.md's divergence paragraph states what audit convergence counts, in terms true under both values of audit_converge_on")
	}
	assert.True(t, anchored, "SKILL.md still carries the %q paragraph", divergenceParagraphAnchor)
	assert.Contains(t, skill, checkRetirementSubstring,
		"SKILL.md states that a registered check retires its mechanize candidate")

	assert.Contains(t, readRepoDoc(t, "CLAUDE.md"), claudeSignalFieldSubstring,
		"CLAUDE.md's audit-scope rule names the field carrying the spec-coverage streak")

	reference := readRepoDoc(t, "skills/tp/REFERENCE.md")
	for _, sentence := range referenceSignalSentences {
		assert.Contains(t, reference, sentence,
			"REFERENCE.md carries §4's verbatim sentence %q", sentence)
	}
	assert.Contains(t, reference, engine.DivergenceHint,
		"REFERENCE.md quotes the shipped engine.DivergenceHint verbatim, so the constant, the document and this test cannot drift apart")

	// §7 row 19's second mutant. The counting rule has to hold under both
	// values of audit_converge_on, so it is phrased over what the resolved
	// policy accepts and never over a severity name: §2 makes `error` the
	// blocking severity, so a sentence saying convergence counts the rows
	// whose severity is blocking would exclude the warning and info rows that
	// count under `all` — while reading as true to anyone checking it under
	// `blocking` alone. Both halves of the pair above are asserted, since that
	// rewording would be applied to both.
	for name, text := range map[string]string{
		"engine.DivergenceHint":   engine.DivergenceHint,
		"SKILL.md's own sentence": divergenceCountingSubstring,
	} {
		assert.NotContains(t, text, "severity",
			"%s states the counting rule over the resolved policy, not over a severity", name)
	}
}
