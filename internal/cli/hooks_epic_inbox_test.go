package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicInboxStateDir is a slice state directory under .tp/epic-review/: its
// name is a SHA-256 in hex, as engine.EpicReviewStateDir makes it.
var epicInboxStateDir = ".tp/epic-review/" + strings.Repeat("0123456789abcdef", 4)

// runWriteDenyHookIn runs the deny hook from project with the given extra
// environment, as a session rooted there would. runPreToolUseHook always runs
// from this repository with no environment beyond PATH, so a case that needs
// a project holding real symlinks, or a writer inside a run, cannot use it.
func runWriteDenyHookIn(t *testing.T, project string, env []string, path string) preToolUseRun {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Write",
		"tool_input":      map[string]any{"file_path": path, "content": "{}\n"},
	})
	require.NoError(t, err)

	script := filepath.Join(repoRoot(t), filepath.FromSlash(preToolUseHookPath))
	cmd := exec.Command(script) //nolint:gosec // a fixed path inside the repo under test
	cmd.Env = append([]string{"PATH=/usr/bin:/bin", "CLAUDE_PROJECT_DIR=" + project}, env...)
	cmd.Dir = project
	cmd.Stdin = strings.NewReader(string(payload))
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	run := preToolUseRun{}
	if runErr := cmd.Run(); runErr != nil {
		var exitErr *exec.ExitError
		require.ErrorAs(t, runErr, &exitErr, "the hook must exit rather than fail to start")
		run.exitCode = exitErr.ExitCode()
	}
	run.stdout, run.stderr = stdout.String(), stderr.String()
	return run
}

// TestPreToolUseHookAllowsTheEpicReviewInbox is v1.7.0 §1's hook sentence: the
// one name an epic review role writes, <state dir>/inbox/<role>-r<N>.ndjson,
// passes the fence for any writer — a session outside tp run and a unit inside
// one alike — relative or absolute. Without the allowance every spelling here
// is refused as tp's own state.
func TestPreToolUseHookAllowsTheEpicReviewInbox(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	inbox := epicInboxStateDir + "/inbox/"
	paths := []string{
		inbox + "why-auditor-r1.ndjson",
		inbox + "dod-satisfiability-r12.ndjson",
		"./" + inbox + "phase-economist-r3.ndjson",
		filepath.Join(root, filepath.FromSlash(inbox), "why-auditor-r1.ndjson"),
	}
	writers := map[string][]string{
		"outside tp run": nil,
		"inside tp run":  {"TP_RUN_ID=01J0000000000000000000000A", "TP_ROUND_DIR=" + filepath.Join(root, ".tp", "rounds", "x", "review-r1")},
	}
	for writer, env := range writers {
		for _, path := range paths {
			t.Run(writer+" "+path, func(t *testing.T) {
				t.Parallel()
				run := runWriteDenyHookIn(t, root, env, path)
				assert.Zero(t, run.exitCode, "%s is the inbox name; stderr=%q", path, run.stderr)
			})
		}
	}
}

// TestPreToolUseHookRefusesEveryOtherEpicStatePath is the allowance's other
// edge: it lets exactly the inbox name through, so a round file, a `..` path,
// a nested or misnamed file in the inbox, and a state directory that is not a
// hash all stay refused.
func TestPreToolUseHookRefusesEveryOtherEpicStatePath(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	inbox := epicInboxStateDir + "/inbox/"
	refused := []string{
		epicInboxStateDir + "/round-1.ndjson",
		epicInboxStateDir + "/snapshot-round-1.json",
		inbox + "../round-1.ndjson",
		epicInboxStateDir + "/x/../inbox/why-auditor-r1.ndjson",
		inbox + "sub/why-auditor-r1.ndjson",
		inbox + "why-auditor-r1.ndjson.part",
		inbox + "why-auditor-r0.ndjson",
		inbox + "why-auditor.ndjson",
		inbox + "Why_Auditor-r1.ndjson",
		".tp/epic-review/b/inbox/why-auditor-r1.ndjson",
		".tp/epics.json",
	}
	for _, path := range refused {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			run := runWriteDenyHookIn(t, root, nil, path)
			assert.True(t, run.denied(), "%s must stay refused; exit=%d stderr=%q", path, run.exitCode, run.stderr)
		})
	}
}

// TestPreToolUseHookRefusesASymlinkedInboxName is the physical half: the name
// passes only when the physical path is that name too. A symlink at the inbox
// name pointing at a fenced file, and an inbox directory that is a symlink to
// the state directory, both spell the inbox name and write elsewhere. The
// plain file beside them is the control that shows the fixture's inbox passes.
func TestPreToolUseHookRefusesASymlinkedInboxName(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	state := filepath.Join(project, filepath.FromSlash(epicInboxStateDir))
	inbox := filepath.Join(state, "inbox")
	require.NoError(t, os.MkdirAll(inbox, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(project, ".tp", "config.json"), []byte("{}\n"), 0o600))
	require.NoError(t, os.Symlink(filepath.Join(project, ".tp", "config.json"), filepath.Join(inbox, "why-auditor-r1.ndjson")))

	other := filepath.Join(project, ".tp", "epic-review", strings.Repeat("fedcba9876543210", 4))
	require.NoError(t, os.MkdirAll(other, 0o755))
	require.NoError(t, os.Symlink(other, filepath.Join(other, "inbox")))

	control := runWriteDenyHookIn(t, project, nil, filepath.Join(inbox, "phase-economist-r1.ndjson"))
	require.Zero(t, control.exitCode, "the plain inbox name passes; stderr=%q", control.stderr)

	for _, path := range []string{
		filepath.Join(inbox, "why-auditor-r1.ndjson"),
		filepath.Join(other, "inbox", "why-auditor-r1.ndjson"),
	} {
		run := runWriteDenyHookIn(t, project, nil, path)
		assert.True(t, run.denied(), "%s writes elsewhere and must be refused; exit=%d stderr=%q", path, run.exitCode, run.stderr)
	}
}
