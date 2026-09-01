package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deligoez/tp/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRelocatedSpecKeepsStateOutOfTheRepository guards §6.1's first condition
// for every test that emits from one of this repository's own specs.
//
// `tp review <spec>` and `tp audit <spec>` write the round's snapshot as a side
// effect, and engine.ReviewStateDir derives that directory from the spec's own
// path with no flag or environment override. A test that pointed at spec/*.md
// in place would therefore rewrite spec/.tp-review/ and advance rounds it does
// not own — while reporting green.
//
// --no-state is not the answer and is deliberately not used anywhere in this
// suite: it disables review-state reads as well as writes, and that state is
// the subject. The round number, skipped_roles, the consecutive-clean count and
// whether `regression` is appended all come from it, so a suite run under it
// would pass while measuring a machine that had been switched off.
//
// Relocating the spec is the only mechanism that preserves the behaviour and
// moves the writes, because the state directory follows the spec.
func TestRelocatedSpecKeepsStateOutOfTheRepository(t *testing.T) {
	root := repoRootDir(t)
	spec := relocatedSpec(t, "spec/0.36.0.md")

	assert.False(t, strings.HasPrefix(spec, root+string(filepath.Separator)),
		"the relocated spec must not sit under the repository root, or its state directory will too")

	stateDir := engine.ReviewStateDir(spec)
	assert.False(t, strings.HasPrefix(stateDir, root+string(filepath.Separator)),
		"engine.ReviewStateDir(%q) resolved to %q, inside the repository", spec, stateDir)

	body, err := os.ReadFile(spec)
	require.NoError(t, err)
	original, err := os.ReadFile(filepath.Join(root, "spec", "0.36.0.md"))
	require.NoError(t, err)
	assert.Equal(t, string(original), string(body),
		"the relocated copy must be byte-identical, or the emission under test is not the repository's")
}
