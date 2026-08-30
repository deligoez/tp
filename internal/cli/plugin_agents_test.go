package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/deligoez/tp/internal/engine"
)

// The plugin's agents are discovered by convention at the plugin root, beside
// skills/, hooks/ and .claude-plugin/ (v0.35.0 §6.1). The role write allowlist
// is a hook the two role agents register for themselves: §6.2's plugin-level
// PreToolUse fence is a different rule with a different matcher group, and this
// one is scoped to the agents that carry it rather than to every session.
const (
	pluginAgentsDir      = "agents"
	roleWriteHookPath    = "hooks/pre-tool-use-role-write-allow.sh"
	agentImplementerFile = "agents/tp-implementer.md"
	agentReviewerFile    = "agents/tp-reviewer.md"
	agentAuditorFile     = "agents/tp-auditor.md"
)

// agentDefinition is the frontmatter subset §6.3 constrains: identity, and the
// hooks that carry the write restriction. Claude Code's agent frontmatter has
// no per-path permission block, so the restriction is expressed the only way a
// definition can express it — a PreToolUse hook over the write tools.
type agentDefinition struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Hooks       map[string][]struct {
		Matcher string `yaml:"matcher"`
		Hooks   []struct {
			Type    string `yaml:"type"`
			Command string `yaml:"command"`
			Timeout int    `yaml:"timeout"`
		} `yaml:"hooks"`
	} `yaml:"hooks"`
}

// readAgentDefinition parses one agent file's YAML frontmatter. The body is
// deliberately not asserted on: §6.3 says the definitions carry tool
// restrictions only, because role content lives in the corpus and reaches the
// unit through the prompt tp review / tp audit already emits.
func readAgentDefinition(t *testing.T, rel string) agentDefinition {
	t.Helper()
	raw := readRepoDoc(t, rel)

	require.True(t, strings.HasPrefix(raw, "---\n"), "%s must open with YAML frontmatter", rel)
	end := strings.Index(raw[4:], "\n---")
	require.GreaterOrEqual(t, end, 0, "%s must close its frontmatter block", rel)

	var def agentDefinition
	require.NoError(t, yaml.Unmarshal([]byte(raw[4:4+end]), &def), "%s frontmatter must parse", rel)
	return def
}

// TestPluginAgentsDeclareTheThreeUnitAgents is §6.3's first sentence: agents/
// holds exactly the three agents the claude template dispatches to, each named
// by the string tp passes to --agent, and nothing else. The name is checked
// against engine.AgentForKind rather than against a literal, so the flag and
// the file it loads can never drift apart.
func TestPluginAgentsDeclareTheThreeUnitAgents(t *testing.T) {
	want := map[string]engine.UnitKind{
		engine.AgentImplementer: engine.UnitImplement,
		engine.AgentReviewer:    engine.UnitReviewRole,
		engine.AgentAuditor:     engine.UnitAuditRole,
	}

	entries, err := os.ReadDir(filepath.Join(repoRoot(t), pluginAgentsDir))
	require.NoError(t, err, "%s/ must exist at the plugin root", pluginAgentsDir)

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	assert.Equal(t, []string{"tp-auditor.md", "tp-implementer.md", "tp-reviewer.md"}, files,
		"§6.3 declares three agents; a fourth would run units under a restriction nobody chose")

	for _, rel := range []string{agentImplementerFile, agentReviewerFile, agentAuditorFile} {
		def := readAgentDefinition(t, rel)

		kind, known := want[def.Name]
		require.True(t, known, "%s declares an agent §6.3 does not name: %q", rel, def.Name)
		assert.Equal(t, strings.TrimSuffix(filepath.Base(rel), ".md"), def.Name,
			"the agent's name is its filename, so --agent %s loads this file", def.Name)
		assert.Equal(t, def.Name, engine.AgentForKind(kind),
			"the claude template dispatches %s to this agent", kind)
		assert.NotEmpty(t, def.Description, "%s must say when it is used", rel)
	}
}

// TestRoleAgentsRegisterTheWriteAllowlist is the manifest half of test 55: the
// reviewer and the auditor each register the allowlist hook over every write
// tool, with §6.4's timeout, and the implementer does not — an implement unit's
// whole job is writing code, and a restriction that fenced it would stop the
// work rather than scope it.
func TestRoleAgentsRegisterTheWriteAllowlist(t *testing.T) {
	command := "${CLAUDE_PLUGIN_ROOT}/" + roleWriteHookPath

	for _, rel := range []string{agentReviewerFile, agentAuditorFile} {
		def := readAgentDefinition(t, rel)

		groups := def.Hooks["PreToolUse"]
		require.Len(t, groups, 1, "%s: one allowlist, one matcher", rel)
		require.Len(t, groups[0].Hooks, 1, "%s", rel)

		entry := groups[0].Hooks[0]
		assert.Equal(t, "command", entry.Type, "%s", rel)
		assert.Equal(t, command, entry.Command,
			"%s resolves the hook through the plugin root rather than the session's cwd", rel)
		assert.Equal(t, hookTimeoutSeconds, entry.Timeout, "%s: §6.4 bounds every shipped hook", rel)

		matcher, matcherErr := regexp.Compile(groups[0].Matcher)
		require.NoError(t, matcherErr, "%s: %q must be a usable pattern", rel, groups[0].Matcher)
		for _, tool := range writeTools {
			assert.True(t, matcher.MatchString(tool),
				"%s: every write reaches the allowlist, including %s", rel, tool)
		}
	}

	implementer := readAgentDefinition(t, agentImplementerFile)
	assert.Empty(t, implementer.Hooks["PreToolUse"],
		"the implementer writes code; §6.3 restricts the reviewer and auditor, not it")

	info, err := os.Stat(filepath.Join(repoRoot(t), filepath.FromSlash(roleWriteHookPath)))
	require.NoError(t, err, "%s must exist", roleWriteHookPath)
	assert.NotZero(t, info.Mode().Perm()&0o111, "%s must be executable", roleWriteHookPath)
}

// roleUnitEnv is the environment the driver gives a role unit (§3.1), reduced
// to the four variables the allowlist is derived from.
type roleUnitEnv struct {
	roundDir string
	unitID   string
	runDir   string
	unitSeq  string
}

// unset is the same unit with one variable missing, for the fail-closed cases.
func (e roleUnitEnv) env() []string {
	out := []string{"PATH=/usr/bin:/bin"}
	for name, value := range map[string]string{
		"TP_ROUND_DIR": e.roundDir,
		"TP_UNIT_ID":   e.unitID,
		"TP_RUN_DIR":   e.runDir,
		"TP_UNIT_SEQ":  e.unitSeq,
	} {
		if value != "" {
			out = append(out, name+"="+value)
		}
	}
	return out
}

// runRoleWriteHook executes the allowlist hook against one tool call in one
// unit's environment and reports what it did with it.
func runRoleWriteHook(t *testing.T, unit roleUnitEnv, toolName string, toolInput map[string]any) preToolUseRun {
	t.Helper()

	payload, err := json.Marshal(map[string]any{
		"session_id":      "7c1d2e3f",
		"transcript_path": filepath.Join(t.TempDir(), "transcript.jsonl"),
		"cwd":             repoRoot(t),
		"hook_event_name": "PreToolUse",
		"tool_name":       toolName,
		"tool_input":      toolInput,
	})
	require.NoError(t, err)

	script := filepath.Join(repoRoot(t), filepath.FromSlash(roleWriteHookPath))
	cmd := exec.Command(script) //nolint:gosec // a fixed path inside the repo under test
	cmd.Env = unit.env()
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

// absoluteUnit is a role unit whose round and run directories are absolute, the
// form §3.1 documents for TP_RUN_DIR.
func absoluteUnit(t *testing.T) roleUnitEnv {
	t.Helper()
	root := repoRoot(t)
	return roleUnitEnv{
		roundDir: filepath.Join(root, ".tp", "rounds", "0.35.0", "review-r3"),
		unitID:   "implementer",
		runDir:   filepath.Join(root, ".tp", "runs", "01JB0000000000000000000000"),
		unitSeq:  "7",
	}
}

// relativeUnit is the same unit with the round directory named relative to the
// session's cwd, which is the other form the driver can hand a child.
func relativeUnit() roleUnitEnv {
	return roleUnitEnv{
		roundDir: ".tp/rounds/0.35.0/audit-r2",
		unitID:   "go-safety",
		runDir:   ".tp/runs/01JB0000000000000000000001",
		unitSeq:  "12",
	}
}

// TestRoleWriteHookAllowsTheRolesOwnFindingsAndEscalation is the half of test
// 55 that a deny-everything hook would fail: the role's own findings .part and
// its escalation record go through, however the tool spells the path. The
// `.part` suffix is the one the prompt names too (§6.3), so prompt and
// allowlist agree on one filename.
func TestRoleWriteHookAllowsTheRolesOwnFindingsAndEscalation(t *testing.T) {
	root := repoRoot(t)

	for _, tc := range []struct {
		name  string
		unit  roleUnitEnv
		paths []string
	}{
		{
			name: "absolute round dir",
			unit: absoluteUnit(t),
			paths: []string{
				filepath.Join(root, ".tp", "rounds", "0.35.0", "review-r3", "role-implementer.ndjson.part"),
				// The same file named relative to the session's cwd.
				".tp/rounds/0.35.0/review-r3/role-implementer.ndjson.part",
				"./.tp/rounds/0.35.0/review-r3/role-implementer.ndjson.part",
				filepath.Join(root, ".tp", "runs", "01JB0000000000000000000000", "7-escalation.json"),
				".tp/runs/01JB0000000000000000000000/7-escalation.json",
			},
		},
		{
			name: "relative round dir",
			unit: relativeUnit(),
			paths: []string{
				".tp/rounds/0.35.0/audit-r2/role-go-safety.ndjson.part",
				"./.tp/rounds/0.35.0/audit-r2/role-go-safety.ndjson.part",
				filepath.Join(root, ".tp", "rounds", "0.35.0", "audit-r2", "role-go-safety.ndjson.part"),
				".tp/runs/01JB0000000000000000000001/12-escalation.json",
			},
		},
	} {
		for _, tool := range writeTools {
			for _, key := range pathArguments(tool) {
				for _, path := range tc.paths {
					t.Run(tc.name+" "+tool+" "+key+" "+path, func(t *testing.T) {
						run := runRoleWriteHook(t, tc.unit, tool, map[string]any{
							key:       path,
							"content": `{"role":"implementer","status":"FAIL"}` + "\n",
						})

						assert.Zero(t, run.exitCode,
							"%s is the unit's own durable write; stderr=%q", path, run.stderr)
						assert.Empty(t, run.stderr, "an allowed write is silent")
					})
				}
			}
		}
	}
}

// TestRoleWriteHookDeniesEveryOtherPath is test 55's other half. The cases that
// matter are the near misses: the final `.ndjson` name the driver renames to,
// a sibling role's file in the same round, the same role's file in another
// round, the round's merged.ndjson, and another unit's escalation record. Each
// is one character away from the allowed path and none of them is this unit's
// to write.
func TestRoleWriteHookDeniesEveryOtherPath(t *testing.T) {
	root := repoRoot(t)
	unit := absoluteUnit(t)
	round := filepath.Join(root, ".tp", "rounds", "0.35.0", "review-r3")

	denied := []string{
		// The name the driver renames the .part to when the child exits 0.
		filepath.Join(round, "role-implementer.ndjson"),
		// A sibling role's findings, in the same round.
		filepath.Join(round, "role-tester.ndjson.part"),
		".tp/rounds/0.35.0/review-r3/role-tester.ndjson.part",
		// The same role, another round and another phase.
		filepath.Join(root, ".tp", "rounds", "0.35.0", "review-r2", "role-implementer.ndjson.part"),
		filepath.Join(root, ".tp", "rounds", "0.35.0", "audit-r3", "role-implementer.ndjson.part"),
		// The merged file, which §6.3 puts deliberately outside the role glob.
		filepath.Join(round, "merged.ndjson"),
		// Another unit's escalation record in the same run.
		filepath.Join(root, ".tp", "runs", "01JB0000000000000000000000", "8-escalation.json"),
		// Ordinary repository files: source, spec, docs and tp's own state.
		"internal/engine/runnertemplate.go",
		"spec/0.35.0.md",
		"README.md",
		".tp/config.json",
		"spec/0.35.0.tasks.json",
		// Somewhere else entirely.
		"/etc/hosts",
		filepath.Join(t.TempDir(), "scratch.txt"),
	}

	for _, tool := range writeTools {
		for _, key := range pathArguments(tool) {
			for _, path := range denied {
				t.Run(tool+" "+key+" "+path, func(t *testing.T) {
					run := runRoleWriteHook(t, unit, tool, map[string]any{
						key:       path,
						"content": "whatever the role wanted to write",
					})

					assert.True(t, run.denied(),
						"%s is not this unit's durable write; exit=%d stderr=%q", path, run.exitCode, run.stderr)
					assert.Contains(t, run.stderr, path, "the refusal names the path it refused")
					assert.Empty(t, run.stdout, "the reason reaches the agent on stderr")
				})
			}
		}
	}
}

// TestRoleWriteHookFailsClosedWithoutTheRoundEnvironment covers the unit whose
// environment does not say which file is its own. There is no allowed path to
// compute, so every write is refused: an allowlist that fell open when it could
// not build itself would be the prose fence all over again.
func TestRoleWriteHookFailsClosedWithoutTheRoundEnvironment(t *testing.T) {
	full := absoluteUnit(t)
	own := filepath.Join(full.roundDir, "role-implementer.ndjson.part")

	for name, unit := range map[string]roleUnitEnv{
		"no round dir": {unitID: full.unitID, runDir: full.runDir, unitSeq: full.unitSeq},
		"no unit id":   {roundDir: full.roundDir, runDir: full.runDir, unitSeq: full.unitSeq},
		"nothing set":  {},
	} {
		t.Run(name, func(t *testing.T) {
			run := runRoleWriteHook(t, unit, "Write", map[string]any{
				"file_path": own,
				"content":   "{}\n",
			})
			assert.True(t, run.denied(),
				"without the round environment the hook cannot know its own file; exit=%d stderr=%q",
				run.exitCode, run.stderr)
			assert.NotEmpty(t, run.stderr, "the refusal says why")
		})
	}
}

// TestPluginAgentsValidateWithClaudeCLI runs the runtime's own component
// validator over agents/, which is what would catch a frontmatter shape the
// tests above happily parse and the harness silently drops. It is skipped when
// the claude CLI is absent, so the tests above stay the durable guard and this
// one is the confirmation on a machine that has the tool.
func TestPluginAgentsValidateWithClaudeCLI(t *testing.T) {
	claude, err := exec.LookPath("claude")
	if err != nil {
		t.Skip("claude CLI not on PATH")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, claude, "plugin", "validate",
		filepath.Join(repoRoot(t), pluginAgentsDir), "--strict")
	out, runErr := cmd.CombinedOutput()
	require.NoError(t, runErr, "claude plugin validate agents/ --strict failed:\n%s", out)
}

// A payload naming no file at all is not a write to refuse. §6.4 wants every
// hook to exit rather than hang or error, and a hook that objected on every
// unrelated call would put a diagnostic in front of the agent constantly.
func TestRoleWriteHookSurvivesAPayloadWithoutAPath(t *testing.T) {
	run := runRoleWriteHook(t, absoluteUnit(t), "Edit", map[string]any{})

	assert.Zero(t, run.exitCode, "stderr=%q", run.stderr)
	assert.Empty(t, run.stderr)
	assert.Empty(t, run.stdout)
}
