package cli

import "testing"

// TestIsAuditableType_SkipsNonSourceMetadata pins the type filter's boundary.
//
// Field report (a Rust NLP project driving tp): --affected-from-tasks produced a
// 49-file list that included .gitignore and .tp/config.json. Both reach the list
// honestly — a task commits them like any other file — and both then cost every
// auditor role its attention on text no spec item is ever about.
//
// The guard runs in both directions on purpose. Skipping too much is the more
// expensive mistake: a file the filter drops is a file no auditor ever sees, and
// nothing downstream reports the omission. .golangci.yml and .github workflows
// are the cases that must survive — they are project configuration a spec can
// legitimately govern.
func TestIsAuditableType_SkipsNonSourceMetadata(t *testing.T) {
	t.Parallel()
	skipped := []string{
		".gitignore",
		".gitattributes",
		".dockerignore",
		".tp/config.json",
		".tp/local.json",
		".tp/reviewers/implementer.json",
		"nested/.tp/config.json",
	}
	for _, path := range skipped {
		if isAuditableType(path) {
			t.Errorf("isAuditableType(%q) = true, want false: not source the spec is audited against", path)
		}
	}

	audited := []string{
		".golangci.yml",
		".github/workflows/ci.yml",
		"internal/cli/audit.go",
		"cmd/tp/main.go",
		"scripts/check-deadcode.sh",
		"package.json",
		".tprc.json",
	}
	for _, path := range audited {
		if !isAuditableType(path) {
			t.Errorf("isAuditableType(%q) = false, want true: dropping it hides the file from every auditor", path)
		}
	}
}
