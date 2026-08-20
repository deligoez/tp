package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// preToolUseHookPath is the second hook the plugin ships, discovered by
// convention at the plugin root beside the session hook (v0.35.0 §6.1).
const preToolUseHookPath = "hooks/pre-tool-use-write-deny.sh"

// writeTools are the four tools §6.2 puts inside the matcher. shellTools are
// tools that must stay outside it: the denial exists to stop hand-editing, not
// to sandbox, and tp's own commands rewrite the fenced files through a shell.
var (
	writeTools = []string{"Write", "Edit", "MultiEdit", "NotebookEdit"}
	shellTools = []string{"Bash", "BashOutput", "KillShell", "Read", "Glob", "Grep", "Task"}
)

// TestPreToolUseHookIsRegisteredForTheWriteTools guards §6.2's second row and
// the half of test 25 that lives in the manifest: the matcher covers the four
// editing tools and no shell tool, so the fence is enforcement for a
// hand-editing agent and invisible to tp itself. §6.4's timeout is pinned here
// too, on the hook that ships with it.
func TestPreToolUseHookIsRegisteredForTheWriteTools(t *testing.T) {
	var manifest pluginHooksManifest
	raw := readRepoDoc(t, pluginHooksManifestPath)
	require.NoError(t, json.Unmarshal([]byte(raw), &manifest), "%s must be valid JSON", pluginHooksManifestPath)

	groups := manifest.Hooks["PreToolUse"]
	require.Len(t, groups, 1, "one PreToolUse group: one fence, one matcher")

	require.Len(t, groups[0].Hooks, 1)
	entry := groups[0].Hooks[0]
	assert.Equal(t, "command", entry.Type)
	assert.Equal(t, "${CLAUDE_PLUGIN_ROOT}/"+preToolUseHookPath, entry.Command,
		"the command resolves through the plugin root rather than the session's cwd")
	assert.Equal(t, hookTimeoutSeconds, entry.Timeout, "§6.4 bounds every shipped hook")

	matcher, err := regexp.Compile(groups[0].Matcher)
	require.NoError(t, err, "the matcher must be a usable pattern: %q", groups[0].Matcher)
	for _, tool := range writeTools {
		assert.True(t, matcher.MatchString(tool), "%s is inside the matcher (§6.2)", tool)
	}
	for _, tool := range shellTools {
		assert.False(t, matcher.MatchString(tool),
			"%s must stay outside the matcher: tp's own writes go through a shell (§6.2)", tool)
	}

	info, err := os.Stat(filepath.Join(repoRoot(t), filepath.FromSlash(preToolUseHookPath)))
	require.NoError(t, err, "%s must exist", preToolUseHookPath)
	assert.NotZero(t, info.Mode().Perm()&0o111, "%s must be executable", preToolUseHookPath)
}

// preToolUseRun is one execution of the deny hook against a synthetic payload.
type preToolUseRun struct {
	stdout   string
	stderr   string
	exitCode int
}

// denied reports whether the run blocked the tool call. Exit 2 is the mechanism
// Claude Code defines for a PreToolUse hook that refuses a call: the tool does
// not run and stderr is what the agent is told.
func (r preToolUseRun) denied() bool { return r.exitCode == 2 }

// runPreToolUseHook feeds the hook the payload shape Claude Code sends on
// stdin — the envelope plus the tool's own arguments under tool_input — and
// returns what the hook did with it.
func runPreToolUseHook(t *testing.T, toolName string, toolInput map[string]any) preToolUseRun {
	t.Helper()

	payload, err := json.Marshal(map[string]any{
		"session_id":      "0e1f2a3b",
		"transcript_path": filepath.Join(t.TempDir(), "transcript.jsonl"),
		"cwd":             repoRoot(t),
		"hook_event_name": "PreToolUse",
		"tool_name":       toolName,
		"tool_input":      toolInput,
	})
	require.NoError(t, err)

	script := filepath.Join(repoRoot(t), filepath.FromSlash(preToolUseHookPath))
	cmd := exec.Command(script) //nolint:gosec // a fixed path inside the repo under test
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	cmd.Dir = repoRoot(t)
	cmd.Stdin = bytes.NewReader(payload)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	run := preToolUseRun{}
	if runErr := cmd.Run(); runErr != nil {
		var exitErr *exec.ExitError
		require.ErrorAs(t, runErr, &exitErr, "the hook must exit rather than fail to start")
		run.exitCode = exitErr.ExitCode()
	}
	run.stdout, run.stderr = stdout.String(), stderr.String()
	return run
}

// pathArguments names the keys a tool can carry its target under. Notebook
// payloads have been seen with both spellings, so the hook reads both and the
// tests pin both: a fence that missed the live spelling would be prose again.
func pathArguments(tool string) []string {
	if tool == "NotebookEdit" {
		return []string{"file_path", "notebook_path"}
	}
	return []string{"file_path"}
}

// TestPreToolUseHookDeniesTheFencedPaths is test 25's first half: every path
// §6.2 fences is denied, through every tool in the matcher, whether it is named
// relatively, with a leading ./ or absolutely.
func TestPreToolUseHookDeniesTheFencedPaths(t *testing.T) {
	root := repoRoot(t)
	fenced := []string{
		// *.tasks.json — tp's own task state, rewritten on every close.
		"spec/0.35.0.tasks.json",
		"./spec/0.35.0.tasks.json",
		filepath.Join(root, "spec", "0.35.0.tasks.json"),
		"tp.tasks.json",
		// .tp/config.json and .tp/local.json — the project and local layers.
		".tp/config.json",
		"./.tp/config.json",
		filepath.Join(root, ".tp", "config.json"),
		".tp/local.json",
		filepath.Join(root, ".tp", "local.json"),
		// .tp-review/ contents — the recorded rounds.
		"spec/.tp-review/0.35.0/round-3/role-implementer.ndjson",
		"./spec/.tp-review/0.35.0/rounds.json",
		filepath.Join(root, "spec", ".tp-review", "0.35.0", "round-3", "merged.ndjson"),
	}

	for _, tool := range writeTools {
		for _, key := range pathArguments(tool) {
			for _, path := range fenced {
				t.Run(tool+" "+key+" "+path, func(t *testing.T) {
					run := runPreToolUseHook(t, tool, map[string]any{
						key:       path,
						"content": "whatever the agent wanted to write",
					})

					assert.True(t, run.denied(), "%s must be denied; exit=%d stderr=%q", path, run.exitCode, run.stderr)
					assert.Contains(t, run.stderr, path, "the refusal names the path it refused")
					assert.Contains(t, run.stderr, "tp", "the refusal points at the tp command that owns the file")
					assert.Empty(t, run.stdout, "the reason reaches the agent on stderr")
				})
			}
		}
	}
}

// TestPreToolUseHookAllowsOrdinaryWrites is the other direction, and the half a
// deny-only test would pass with a hook that refuses everything: source, specs,
// docs and the role corpus are all outside the fence, and so is the
// `.tp-review` directory itself — §6.2 fences its contents.
func TestPreToolUseHookAllowsOrdinaryWrites(t *testing.T) {
	root := repoRoot(t)
	allowed := []string{
		"internal/cli/run.go",
		"internal/engine/runstate.go",
		"spec/0.35.0.md",
		"README.md",
		"hooks/hooks.json",
		filepath.Join(root, "internal", "cli", "root.go"),
		// The reviewer/auditor corpus is tp state the agent may legitimately
		// edit; only the four classes in §6.2 are fenced.
		".tp/reviewers/implementer.json",
		".tp/auditors/go-safety.json",
		".tp/config.json.example",
		// The directory itself, not its contents.
		"spec/.tp-review",
	}

	for _, tool := range writeTools {
		for _, key := range pathArguments(tool) {
			for _, path := range allowed {
				t.Run(tool+" "+key+" "+path, func(t *testing.T) {
					run := runPreToolUseHook(t, tool, map[string]any{
						key:       path,
						"content": "package cli\n",
					})

					assert.Zero(t, run.exitCode, "%s is outside the fence; stderr=%q", path, run.stderr)
					assert.Empty(t, run.stderr, "an allowed write is silent")
				})
			}
		}
	}
}

// TestPreToolUseHookReadsTheArgumentNotTheContent pins why the hook can read
// its payload without a JSON parser: a fenced path quoted inside the file being
// written arrives escaped, so it can never be mistaken for the tool's own
// argument. Without this the hook would refuse to let anyone edit a test that
// merely mentions a task file — this one included.
func TestPreToolUseHookReadsTheArgumentNotTheContent(t *testing.T) {
	body := `payload := ` + "`" + `{"file_path": "spec/0.35.0.tasks.json"}` + "`" + `
also := "{\"notebook_path\": \".tp/local.json\"}"
and := ".tp-review/0.35.0/round-1/role-tester.ndjson"
`

	run := runPreToolUseHook(t, "Write", map[string]any{
		"file_path": "internal/cli/hooks_pre_tool_use_test.go",
		"content":   body,
	})

	assert.Zero(t, run.exitCode, "the fenced names are file content, not the write target; stderr=%q", run.stderr)
	assert.Empty(t, run.stderr)
}

// TestPreToolUseHookDoesNotFireForShellWrites is test 25's second half at the
// hook itself: even handed a shell payload, the hook has no write target to
// refuse. tp closes a task by rewriting `*.tasks.json` through exactly this
// path, and a fence that blocked it would stop the tool it exists to protect.
func TestPreToolUseHookDoesNotFireForShellWrites(t *testing.T) {
	for _, command := range []string{
		"tp done hook-write-deny --commit abc1234 -- '- did the thing'",
		"tp import spec/0.35.0.tasks.json",
		"tp use spec/0.35.0.tasks.json",
		"tp review spec/0.35.0.md --record spec/.tp-review/0.35.0/round-3/merged.ndjson",
		"printf '{}' > .tp/config.json",
	} {
		t.Run(command, func(t *testing.T) {
			run := runPreToolUseHook(t, "Bash", map[string]any{
				"command":     command,
				"description": "close the unit",
			})

			assert.Zero(t, run.exitCode, "tp's own writes go through a shell (§6.2); stderr=%q", run.stderr)
			assert.Empty(t, run.stderr)
		})
	}
}

// TestPreToolUseHookSurvivesAPayloadWithoutAPath covers the tool whose input
// names no file at all. §6.4 wants every hook to exit rather than hang or
// error, and a hook that failed open loudly would put a diagnostic in front of
// the agent on every unrelated call.
func TestPreToolUseHookSurvivesAPayloadWithoutAPath(t *testing.T) {
	run := runPreToolUseHook(t, "Edit", map[string]any{})

	assert.Zero(t, run.exitCode, "stderr=%q", run.stderr)
	assert.Empty(t, run.stderr)
	assert.Empty(t, run.stdout)
}
