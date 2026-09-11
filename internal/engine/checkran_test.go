package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCheckRan_ExitCodeContract pins the boundary of the checks exit-code
// contract: 0 and 1 are verdicts, and every other outcome — 2, the shell's 126
// and 127, a signal's -1, a start failure and a timeout, the last two with no
// exit code at all — is a check that could not run. The CLI test drives exit 2
// and 127 through a real emission; these are the outcomes it cannot cheaply
// produce.
func TestCheckRan_ExitCodeContract(t *testing.T) {
	t.Parallel()
	code := func(n int) *int { return &n }
	for name, tc := range map[string]struct {
		res  RunResult
		want bool
	}{
		"exit 0 passed":             {RunResult{Passed: true, ExitCode: code(0)}, true},
		"exit 1 found violations":   {RunResult{ExitCode: code(1)}, true},
		"exit 2 could not run":      {RunResult{ExitCode: code(2)}, false},
		"exit 126 cannot execute":   {RunResult{ExitCode: code(126)}, false},
		"exit 127 command missing":  {RunResult{ExitCode: code(127)}, false},
		"killed by a signal":        {RunResult{ExitCode: code(-1)}, false},
		"failed to start":           {RunResult{Message: "exec: not found"}, false},
		"stopped at its timeout":    {RunResult{TimedOut: true, Message: "timed out after 1s"}, false},
		"timeout despite exit code": {RunResult{TimedOut: true, ExitCode: code(0)}, false},
	} {
		assert.Equal(t, tc.want, CheckRan(&tc.res), name)
	}
}
