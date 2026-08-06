package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The four documents that each state part of the audit routing contract (§4).
var auditContractDocs = []string{
	"README.md",
	"skills/tp/SKILL.md",
	"skills/tp/REFERENCE.md",
	"CLAUDE.md",
}

// The two that additionally render the audit prompt's body order and the
// prompts[].role contract verbatim (§4).
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
	root := repoRoot(t)
	read := func(rel string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		require.NoError(t, err, "%s must exist at the repo root", rel)
		return string(data)
	}

	for _, doc := range auditContractDocs {
		text := read(doc)
		assert.Contains(t, text, routingSubstring, "%s states the routing rule", doc)
		assert.Contains(t, text, upgradeSubstring, "%s records that an ejected corpus keeps its old copies", doc)
		assert.NotContains(t, text, "file-sec-", "%s carries no superseded item-id prefix", doc)
		assert.NotContains(t, text, "file-maint-", "%s carries no superseded item-id prefix", doc)
	}

	for _, doc := range auditPromptOrderDocs {
		text := read(doc)
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

	assert.Contains(t, read("skills/tp/REFERENCE.md"), itemIDSubstring,
		"REFERENCE.md documents the deterministic item ids in the role-id slug form")
}
