package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// logChildScript is the runner §3.5's two arms are observed through. It is a
// real script rather than the fake runner because the fake's seam template
// always carries {log_path}, and the arm that matters most here is the one
// whose template omits it.
//
// It writes to both streams, records the log path it was handed when it was
// handed one, and reads stdin to completion — the four things §3.5 and §3.1.1
// say about a child's file descriptors, observed from the child's side.
const logChildScript = `#!/bin/sh
marker="$1"
if [ -n "$2" ]; then printf '%s\n' "$2" > "$marker/received-log-path"; fi
printf 'to-stdout\n'
printf 'to-stderr\n' >&2
cat > "$marker/stdin"
printf 'read-returned\n' > "$marker/after"
`

// logChildProject wires a temp project to that script as its runner. args is
// the runner's own argument template beyond the marker directory, which is the
// whole difference between §3.5's two arms: one carrying {log_path} and one
// omitting it.
func logChildProject(t *testing.T, args ...string) (root, spec, taskFile, marker string, wf *model.Workflow) {
	t.Helper()
	root, spec, taskFile = setupResumeProject(t, oneOpenTask)

	script := filepath.Join(t.TempDir(), "child.sh")
	require.NoError(t, os.WriteFile(script, []byte(logChildScript), 0o700)) //nolint:gosec // a test fixture that has to be executable
	marker = filepath.Join(t.TempDir(), "marker")
	require.NoError(t, os.MkdirAll(marker, 0o750))

	runner, err := json.Marshal(map[string]any{"cmd": script, "args": append([]string{marker}, args...)})
	require.NoError(t, err)
	wf = driverWorkflow()
	wf.Runner = runner
	return root, spec, taskFile, marker, wf
}

// unitLogRow returns the run state's single unit row. The log path is read back
// from there rather than rebuilt by the test wherever one path has to equal
// another, so the assertion compares what the driver recorded against what the
// child received instead of comparing two copies of the test's own arithmetic.
func unitLogRow(t *testing.T, root, taskFile string) RunUnitRow {
	t.Helper()
	st := readRunStateFile(t, root, taskFile)
	require.Len(t, st.Units, 1, "one open task, one attempt")
	return st.Units[0]
}

// assertChildSawStdinEOF is §10.1 test 62 from the child's side: a runner that
// reads stdin sees EOF rather than hanging. The marker written after the read
// is what separates "read nothing" from "never returned" — a child whose stdin
// were an open pipe would still have an empty capture, and no marker.
func assertChildSawStdinEOF(t *testing.T, marker string) {
	t.Helper()
	assert.FileExists(t, filepath.Join(marker, "after"),
		"the child's read of stdin returned rather than blocking on an inherited descriptor")
	stdin, err := os.ReadFile(filepath.Join(marker, "stdin"))
	require.NoError(t, err)
	assert.Empty(t, string(stdin), "stdin is closed, so the child reads no bytes at all")
}

// §3.5: a template that omits {log_path} has its child's stdout and stderr
// redirected by the driver to $TP_RUN_DIR/<seq>-<kind>-<id>.jsonl.
func TestRunDriver_TemplateOmittingLogPathIsRedirected(t *testing.T) {
	root, spec, taskFile, marker, wf := logChildProject(t)

	res := driveOnce(t, root, spec, taskFile, wf)
	require.Equal(t, StopUnitFailure, res.StopReason,
		"the child writes nothing durable, so the run makes one attempt and stops")

	row := unitLogRow(t, root, taskFile)
	assert.Equal(t, filepath.Join(RunDir(root, res.RunID), "1-implement-alpha.jsonl"), row.LogPath,
		"§3.5 names the log $TP_RUN_DIR/<seq>-<kind>-<id>.jsonl")

	log, err := os.ReadFile(row.LogPath)
	require.NoError(t, err, "the driver owns the file for a template that omits the placeholder")
	assert.Contains(t, string(log), "to-stdout", "the child's stdout is redirected there")
	assert.Contains(t, string(log), "to-stderr", "and so is its stderr, into the same file")

	assert.NoFileExists(t, filepath.Join(marker, "received-log-path"),
		"control: this arm's template never received the path")
	assertChildSawStdinEOF(t, marker)
}

// §3.5's other arm: a template using {log_path} receives that same path and
// owns the file, and the driver does not redirect.
//
// The child deliberately writes nothing to the log it was handed, which is what
// makes the absence of the file the discriminating observation: a driver that
// redirected as well would have created it before the child ever ran.
func TestRunDriver_TemplateUsingLogPathOwnsTheFile(t *testing.T) {
	root, spec, taskFile, marker, wf := logChildProject(t, "{log_path}")

	res := driveOnce(t, root, spec, taskFile, wf)
	require.Equal(t, StopUnitFailure, res.StopReason)

	row := unitLogRow(t, root, taskFile)
	received, err := os.ReadFile(filepath.Join(marker, "received-log-path"))
	require.NoError(t, err, "the placeholder expands to the unit's log path")
	assert.Equal(t, row.LogPath, strings.TrimSpace(string(received)),
		"the child is handed the same path the run state records")

	assert.NoFileExists(t, row.LogPath,
		"the child owns the file, so a child that wrote nothing leaves nothing behind")
	assertChildSawStdinEOF(t, marker)
}
