package cli_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNextHelpSaysPeekIgnoresWIP: `tp next --peek` previews the next READY
// task and ignores a WIP one (spec/0.1.0.md), while plain `tp next`, `tp
// brief` and `tp resume` put WIP first. A field user tripped on the
// difference, so the --peek line of `tp next --help` states it.
func TestNextHelpSaysPeekIgnoresWIP(t *testing.T) {
	t.Parallel()
	stdout, stderr, code := runTP(t, t.TempDir(), "next", "--help")
	require.Equal(t, 0, code, stderr)

	var peekLine string
	for line := range strings.SplitSeq(stdout, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--peek") {
			peekLine = line
			break
		}
	}
	require.NotEmpty(t, peekLine, "tp next --help lists --peek: %s", stdout)
	assert.Contains(t, peekLine, "next ready task", "--peek's usage names what it shows")
	assert.Contains(t, peekLine, "ignores WIP", "--peek's usage says it skips a WIP task, unlike plain tp next")
}
