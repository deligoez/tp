package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/engine"
)

// TestOutputContractSeverityEnumAgreesWithTheAuditSchema pins the pairing that
// reaches every auditor prompt: renderAuditOutputSchema writes the "Output
// Schema" block and outputContractInstruction stamps the contract immediately
// after it. Both name a severity enum, and while the stamp was phase-blind the
// two contradicted each other in the same prompt — error|warning|info above,
// critical|high|medium|low below — leaving the role to guess which tp meant.
//
// The enums differ by phase on purpose. An audit row states its verdict in
// `status`, so severity is only a qualifier and uses the audit vocabulary; a
// review finding has no status, so there severity IS the blocking predicate
// (review_converge_on blocks on critical/high) and keeps the four-level scale.
// This test is an in-package test because the stamp is unexported and no other
// test pinned it, which is how the contradiction survived.
func TestOutputContractSeverityEnumAgreesWithTheAuditSchema(t *testing.T) {
	auditPrompt := renderAuditOutputSchema() + outputContractInstruction("go-safety", engine.PhaseAuditors)

	assert.Contains(t, auditPrompt, "one of error|warning|info",
		"the Output Schema block states the audit severity enum")
	assert.Contains(t, auditPrompt, "- severity: one of error, warning, info",
		"the contract stamp must restate the SAME enum, not the review one")
	assert.Contains(t, auditPrompt, "- status: one of PASS, PARTIAL, FAIL",
		"status is the audit row's verdict; severity only qualifies it")
	for _, sev := range []string{"critical", "high", "medium", "low"} {
		assert.NotContains(t, auditPrompt, sev,
			"the review severity vocabulary must not appear anywhere in an auditor prompt's contract")
	}

	reviewStamp := outputContractInstruction("tester", engine.PhaseReviewers)
	assert.Contains(t, reviewStamp, "- severity: one of critical, high, medium, low",
		"review severity is the blocking predicate and keeps its four-level scale")
	assert.NotContains(t, reviewStamp, "status",
		"a review finding carries no status")
	assert.False(t, strings.Contains(reviewStamp, "warning"),
		"the audit severity vocabulary must not leak into a reviewer prompt")
}
