package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// goInstallRef matches the version suffix of a `go install <module>@<ref>` in a
// workflow file. Only `go install` is examined: those are the gate's own tools,
// which is what this test is about.
var goInstallRef = regexp.MustCompile(`go install\s+(\S+)@(\S+)`)

// TestWorkflowToolsArePinned: no workflow installs a gate tool at a floating
// ref. A gate whose tools float is a gate whose result is a function of the day
// it ran, not of the code it ran on — the same finding set can appear or vanish
// with no commit in between.
//
// The failure this guards against was observed on a sibling repository rather
// than here, which is why it is written down instead of remembered:
// golangci-lint v2.12.2's staticcheck panics building IR for Go 1.27's stdlib
// ("Panic: buildir: package \"poll\": unexpected expr: *ast.KeyValueExpr"). The
// message names a package the repository does not contain — it is the stdlib's
// internal/poll — so it reads like a defect in your own code. tp was passing CI
// only because `@latest` happened to resolve to a version with Go 1.27 support.
// Passing by luck and failing by luck are the same mechanism.
//
// `latest` is not the only floating ref, so the test rejects anything that is
// not a vN.N.N tag rather than matching `latest` alone: a branch name, a bare
// major, or a commit-ish would drift the same way.
func TestWorkflowToolsArePinned(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(repoRoot(t), ".github", "workflows")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.NotEmpty(t, entries, ".github/workflows must exist")

	semver := regexp.MustCompile(`^v\d+\.\d+\.\d+$`)
	checked := 0

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}
		body, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		require.NoError(t, readErr)

		for _, match := range goInstallRef.FindAllStringSubmatch(string(body), -1) {
			module, ref := match[1], match[2]
			checked++
			assert.Regexp(t, semver, ref,
				"%s installs %s at the floating ref %q: pin it to the exact version the gate is measured at",
				entry.Name(), module, ref)
		}
	}

	assert.Positive(t, checked,
		"no `go install` found in any workflow — the regex or the workflows moved, and this guard is measuring nothing")
}
