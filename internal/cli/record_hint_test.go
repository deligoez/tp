package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRecordRowHintNamesTheNDJSON: a malformed --record row is §9.2's worked
// example. Both record paths answer it with exit 1, and a hintless exit-1 site
// inherits the code-1 default — "run 'tp validate' to audit the task file", an
// unrelated command over an unrelated file, when the object at fault is the
// NDJSON the reviewers or auditors just wrote.
//
// The two paths reached that default differently, which is why this is a
// behavioural test and not only an enumeration. tp audit --record passed no
// hint at all, the statically visible form TestFileErrorsCarryAHint now
// enumerates. tp review --record passed parseRecordRows' hint — a variable,
// empty for every row rule that carries no advice of its own, so no static
// enumeration of the call site can see it. Reading the hint the binary actually
// emitted covers both.
func TestRecordRowHintNamesTheNDJSON(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{"review", "audit"} {
		t.Run(phase, func(t *testing.T) {
			dir := t.TempDir()
			specPath := filepath.Join(dir, "spec.md")
			require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n\n## 1. One\n\nDo the thing.\n"), 0o600))
			recordPath := filepath.Join(dir, "results.ndjson")
			require.NoError(t, os.WriteFile(recordPath, []byte("not json\n"), 0o600))

			_, stderr, code := runTP(t, dir, phase, specPath, "--record", recordPath)
			require.Equal(t, 1, code, "a malformed row is a validation error")

			var payload map[string]any
			require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
			message, _ := payload["error"].(string)
			assert.Contains(t, message, "line 1", "the message names the offending line")

			hint, _ := payload["hint"].(string)
			assert.Contains(t, hint, "--record", "the hint names the flag whose file is at fault")
			assert.NotContains(t, hint, "tp validate",
				"task-file advice is the wrong object for a malformed results file")
		})
	}
}
