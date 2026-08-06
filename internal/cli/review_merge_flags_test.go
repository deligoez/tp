package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReviewMergeRejectsForeignFlags: `tp review --merge` records no round and
// reads no state, so a flag belonging to another mode must be rejected by name
// in ONE call. --harness-note used to exit 2 with "supply --harness-note only
// together with --record <file>" — obeying that hint produced a second failure
// ("--merge and --record are mutually exclusive"), two failed calls for one
// mistake. --force and --no-state were accepted outright and silently ignored,
// while tp audit's exhaustive --merge list rejects the equivalents.
//
// The list was not actually exhaustive despite the comment claiming so:
// --perspective/--docs-path/--test-path still fell through, so
// `tp review --merge in.ndjson -o out --perspective testing` exited 0 and wrote
// the merge with the flag silently ignored — while --record/--status reject
// that same flag. --merge generates no prompt, so every prompt-generation flag
// belongs in the list.
func TestReviewMergeRejectsForeignFlags(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.ndjson")
	require.NoError(t, os.WriteFile(in, []byte(""), 0o600))

	for _, tc := range []struct {
		name  string
		extra []string
	}{
		{"harness-note", []string{"--harness-note", "x"}},
		{"force", []string{"--force"}},
		{"no-state", []string{"--no-state"}},
		{"perspective", []string{"--perspective", "testing"}},
		{"docs-path", []string{"--docs-path", "docs"}},
		{"test-path", []string{"--test-path", "tests"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			merged := filepath.Join(dir, tc.name+"-merged.ndjson")
			args := append([]string{"review", "--merge", in, "-o", merged}, tc.extra...)
			_, stderr, code := runTP(t, dir, args...)
			assert.Equal(t, 2, code, "--merge with --%s is a usage error", tc.name)
			assert.Contains(t, stderr, "--merge cannot be combined",
				"the message names the actual constraint, not a rule the caller cannot satisfy")
			assert.Contains(t, stderr, "--"+tc.name, "the offending flag is named")
			assert.NotContains(t, stderr, "requires --record",
				"never prescribe --record: --merge and --record are mutually exclusive")
			assert.NoFileExists(t, merged, "review refuses before writing the merge output")
		})
	}
}
