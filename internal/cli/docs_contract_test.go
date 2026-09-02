package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

const (
	honestSignalsHeading = "## Honest Convergence Signals (v0.33.0)"

	// What that section must say instead, asserted positively so that deleting
	// the false promise rather than replacing it does not pass here.
	honestSignalsGovernedBy = "governed by `audit_converge_on` (v0.37.0), " +
		"which changes what the stored per-round `clean` flag records"

	storedCleanFlagPhrase = "`clean` flag"
)

// v032ParityMarkers are the ways a sentence can promise a signal still behaves
// as it always has. The section's original wording used the first two about the
// stored per-round `clean` flag, naming the version it claimed parity with; the
// markers are deliberately phrased without that version, because §6.4's defect
// is the promise and not its wording. Measured while writing this: with the
// first marker written as the full "exactly as in v0.32.0", a paragraph that
// re-listed the flag as behaving "exactly as they always did" passed green.
var v032ParityMarkers = []string{
	"exactly as",
	"untouched",
	"unchanged",
	"as before",
	"as it always",
	"as they always",
}

// TestReferenceDoesNotPromiseTheStoredCleanFlagIsUnchanged guards §6.4, pinned
// by §7 row 20. REFERENCE.md's "Honest Convergence Signals" paragraph listed
// the stored per-round `clean` flag among the signals behaving exactly as in
// v0.32.0, and §2 changes what that flag records. Nothing read this paragraph
// before this test — the guard extension is part of the work, not a bonus —
// and re-adding the flag to that list must redden it.
func TestReferenceDoesNotPromiseTheStoredCleanFlagIsUnchanged(t *testing.T) {
	section := docSectionBody(t, readRepoDoc(t, "skills/tp/REFERENCE.md"), honestSignalsHeading)

	assert.Contains(t, section, honestSignalsGovernedBy,
		"the section states what governs the stored per-round clean flag now")

	for _, sentence := range strings.Split(section, ". ") {
		if !strings.Contains(sentence, storedCleanFlagPhrase) {
			continue
		}
		for _, marker := range v032ParityMarkers {
			assert.NotContains(t, sentence, marker,
				"no sentence naming the stored clean flag may promise it is unmoved: %q", sentence)
		}
	}
}

// auditSideDenials are the ways a shipped document can deny that tp has an
// audit-side counterpart to review_converge_on. v0.37.0 §2 ships one, so every
// such sentence is false — and this class is the reason §6 refuses to state a
// count: the sentence these markers catch survived several sweeps of this
// release because it wraps across two source lines, which a single-line search
// misses and a whole-document NotContains does not.
//
// "audit never reads it", said of review_converge_on, is deliberately not a
// marker: that one is true and is what the Workflow Fields table says.
var auditSideDenials = []string{
	"no audit-side equivalent",
	"has no audit-side",
	"no equivalent on the audit side",
}

// auditScopeAnchor opens SKILL.md's Workflow D audit-scope paragraph — the one
// that carried the denial. Anchored rather than asserted document-wide for the
// reason TestDocsCarryTheConvergenceSignalWording anchors: the field is named
// in several places, so a document-wide positive assertion would stay green
// with this paragraph silent about it.
const auditScopeAnchor = "**Scope the audit, or it will not converge"

// divergenceSectionHeading and divergenceCarriedForward pin REFERENCE.md's
// divergence section against the §6.4 defect in its second form. That section
// said next_action reads the fix-and-re-audit directive on every round emitting
// divergence. Measured on this tree before the sentence was rewritten: two
// rounds under audit_converge_on=blocking with audit_clean_rounds=2 — an error
// row then a warning row, spec-coverage clean throughout — emit divergence on
// round 2 beside clean:true and next_action "1 accepted row carried forward".
const (
	divergenceSectionHeading = "### `divergence` (audit, §2.4)"
	divergenceCarriedForward = "carried forward"
)

// TestDocsStateTheConvergenceRuleUnderBothPolicies guards v0.37.0 §6's sweep
// where it has no other reader. §6.4's lesson was that nothing read the
// paragraph it repaired, so the two repairs most likely to be silently undone
// get a guard here: the denial that the field exists, and the divergence
// section's claim about next_action.
func TestDocsStateTheConvergenceRuleUnderBothPolicies(t *testing.T) {
	skill := readRepoDoc(t, "skills/tp/SKILL.md")
	for _, path := range []string{"skills/tp/SKILL.md", "skills/tp/REFERENCE.md", "README.md"} {
		doc := readRepoDoc(t, path)
		for _, denial := range auditSideDenials {
			assert.NotContains(t, doc, denial,
				"%s may not deny tp has an audit-side counterpart to review_converge_on; v0.37.0 ships audit_converge_on", path)
		}
	}

	anchored := false
	for _, para := range strings.Split(skill, "\n\n") {
		if !strings.Contains(para, auditScopeAnchor) {
			continue
		}
		anchored = true
		assert.Contains(t, para, "audit_converge_on",
			"SKILL.md's audit-scope paragraph states the counting rule in terms of the field that decides it")
	}
	assert.True(t, anchored, "SKILL.md still carries the %q paragraph", auditScopeAnchor)

	section := docSectionBody(t, readRepoDoc(t, "skills/tp/REFERENCE.md"), divergenceSectionHeading)
	assert.Contains(t, section, divergenceCarriedForward,
		"the divergence section says what next_action reads under blocking, where a divergent round can be clean")
}

// docSectionBody returns a markdown section's own body — the text between its
// heading and the next heading of any level — with whitespace collapsed, so an
// assertion over a sentence is not hostage to where the document happens to
// wrap its lines.
func docSectionBody(t *testing.T, doc, heading string) string {
	t.Helper()
	_, after, found := strings.Cut(doc, heading)
	require.True(t, found, "the document still carries the %q heading", heading)
	if end := strings.Index(after, "\n#"); end >= 0 {
		after = after[:end]
	}
	body := strings.Join(strings.Fields(after), " ")
	require.NotEmpty(t, body, "the %q section has a body to assert over", heading)
	return body
}

// TestDocsScopeTheAuditConvergeOnFence used to live here, and it is deleted
// rather than repaired. It anchored on a phrase in each document's fence
// paragraph and then asserted that two qualifier substrings appeared in the
// same paragraph — which pins that a topic is still discussed and nothing at
// all about what is claimed of it. An auditor built two paragraphs that keep
// the anchor AND both qualifiers while asserting the opposite of what the guard
// was written to deny, and the guard passed green on both. No phrase exists
// that a negation cannot restate, so no rewording of it would have helped.
//
// What replaced it is behaviour, in
// internal/cli/auditconvergeon_fence_test.go: the fence's rule is asserted by
// running the commands over eight trees, including the two bases the deleted
// prose used to carve out.
//
// The guards above are kept, but NOT because a negation cannot satisfy them.
// That was this comment's earlier claim and an auditor falsified both halves of
// it by running them. TestReferenceDoesNotPromiseTheStoredCleanFlagIsUnchanged,
// the "asserts a phrase is ABSENT" half, misses two REFERENCE.md variants that
// re-add the exact promise §6.4 deleted: one crosses a ". " split boundary, one
// is worded outside the marker blacklist. And the "compares against a constant
// the code ships" half, TestDocsCarryTheConvergenceSignalWording, misses a
// paragraph inserted right after the quoted constant declaring that sentence
// obsolete.
//
// The dividing line is SCOPE, not polarity. Contains and a windowed NotContains
// are both local assertions inside an unbounded text, so the complement is free
// and a negation simply goes there. These survive it: a subject that is the
// whole of a BOUNDED artifact (assert.NotContains over engine.DivergenceHint,
// where there is no elsewhere), and a verdict that rests on BOUNDED read-backs
// rather than on matching text — the eight-tree fence test named above decides
// on the process exit code, the envelope's code field and os.IsNotExist. Note
// what that does NOT say: the same test also runs five Contains/NotContains
// over the generated message, and those are the unbounded shape. What rescues
// it is that they are not what the verdict rests on.
//
// Not ci_gate_test.go, which an earlier draft of this comment cited and which
// an auditor caught: it derives the step list from .tp/config.json and then
// asserts Contains over the whole of ci.yml, so only the left side is derived.
// The gate-sequence release records that shape measured — `if: false` on a step, or
// continue-on-error: true, leaves every guard green. No claim is made here
// about shapes nobody has enumerated.
//
// So what the guards above catch is a document that stops saying what it says
// today, or drifts from the shipped constant at the matched site. What they do
// not catch is a document that keeps every matched phrase and contradicts it
// somewhere else — which is the failure the deleted guard shipped. They are
// left alone rather than reworded, because no phrase-based assertion over prose
// can be made sound by rewording; a derivation-based replacement is a new
// mechanism rather than an audit repair, and is routed to a later version.
