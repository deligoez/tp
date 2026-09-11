package cli

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// batchTool is the carrier the write-deny matcher names alongside the write
// tools. Its tool_input is {"ops":[{"tool":<name>,"args":{...}}]}, the shape
// codedbpro's own batch schema declares, and `tool` is one of read,
// faster_search, meta_search, edit, patch, create, diff, memo, replace, lint.
const batchTool = "mcp__codedbpro__batch"

// batchReadOps are the batch operations that cannot put bytes on disk. Each is
// a codedbpro tool the matcher does not name, so made as its own call it never
// reaches the hook; a batch must judge it the same way, or whether a read is
// refused depends on whether the agent batched it.
var batchReadOps = []string{"read", "faster_search", "meta_search", "diff", "lint", "memo"}

// batchWriteOps derives the batch write operations from the matcher itself:
// every codedbpro tool the matcher names, the carrier aside. Deriving rather
// than listing is what keeps the hook's per-operation reading and the matcher's
// per-call reading the same classification.
func batchWriteOps(t *testing.T) []string {
	t.Helper()
	return codedbproWritesIn(t, pluginWriteMatcher(t))
}

// pluginWriteMatcher is the write-deny hook's matcher in hooks/hooks.json.
func pluginWriteMatcher(t *testing.T) string {
	t.Helper()
	var manifest pluginHooksManifest
	require.NoError(t, json.Unmarshal([]byte(readRepoDoc(t, pluginHooksManifestPath)), &manifest))
	groups := manifest.Hooks["PreToolUse"]
	require.Len(t, groups, 1)
	return groups[0].Matcher
}

// codedbproWritesIn lists the codedbpro tools a matcher names, the carrier
// aside: the operations a batch carries that the matcher would judge alone.
func codedbproWritesIn(t *testing.T, matcher string) []string {
	t.Helper()
	var ops []string
	for _, alt := range strings.Split(matcher, "|") {
		name, ok := strings.CutPrefix(alt, "mcp__codedbpro__")
		if ok && alt != batchTool {
			ops = append(ops, name)
		}
	}
	require.GreaterOrEqual(t, len(ops), 4, "the matcher names codedbpro's create, edit, patch and replace")
	return ops
}

// batchOp is one operation with `tool` serialized before `args`. A map would
// serialize them the other way round, since json.Marshal sorts map keys, so the
// two constructors below give the hook both key orders.
type batchOp struct {
	Tool string         `json:"tool"`
	Args map[string]any `json:"args"`
}

func toolFirst(tool string, args map[string]any) any { return batchOp{Tool: tool, Args: args} }

func argsFirst(tool string, args map[string]any) any {
	return map[string]any{"tool": tool, "args": args}
}

// batchPathArgs is the argument each operation names its target under.
func batchPathArgs(op, path string) map[string]any {
	switch op {
	case "faster_search", "meta_search":
		return map[string]any{"path": path, "pattern": "tp"}
	case "replace":
		return map[string]any{"path": path, "old": "a", "new": "b"}
	case "memo":
		// memo names no file; a fenced path can only reach it as a value.
		return map[string]any{"action": "store", "value": path, "file": path}
	default:
		return map[string]any{"file": path, "content": "whatever the agent wanted to write"}
	}
}

// fencedReadTargets are paths in each of the four fenced classes. Reading any
// of them is what a grader, an auditor or an orchestrator is told to do.
var fencedReadTargets = []string{
	"spec/.tp-review/x/state.json",
	"spec/.tp-review/1.1.0/snapshot-round-1.md",
	"spec/.tp-review/1.1.0",
	".tp/config.json",
	".tp/local.json",
	"spec/0.25.0.tasks.json",
}

// TestPreToolUseHookPassesABatchThatOnlyReads is the field report's case: a
// batch of reads naming tp's round state was refused with the hand-edit
// message, although the same reads made one call at a time never reach the
// hook. A batch whose every operation reads passes, whatever it names and in
// either key order.
func TestPreToolUseHookPassesABatchThatOnlyReads(t *testing.T) {
	t.Parallel()

	t.Run("the reported repro", func(t *testing.T) {
		run := runPreToolUseHook(t, batchTool, map[string]any{"ops": []any{
			toolFirst("read", map[string]any{"file": "spec/.tp-review/x/state.json"}),
			toolFirst("read", map[string]any{"file": "spec/.tp-review/x/state.json", "mode": "outline"}),
		}})
		assert.Zero(t, run.exitCode, "an all-read batch writes nothing; stderr=%q", run.stderr)
		assert.Empty(t, run.stderr)
	})

	for _, order := range []struct {
		name string
		op   func(string, map[string]any) any
	}{{"tool first", toolFirst}, {"args first", argsFirst}} {
		for _, op := range batchReadOps {
			t.Run(order.name+" "+op, func(t *testing.T) {
				ops := []any{order.op("read", map[string]any{"file": "internal/cli/run.go"})}
				for _, path := range fencedReadTargets {
					ops = append(ops, order.op(op, batchPathArgs(op, path)))
				}
				run := runPreToolUseHook(t, batchTool, map[string]any{"ops": ops})

				assert.Zero(t, run.exitCode, "%s writes nothing; stderr=%q", op, run.stderr)
				assert.Empty(t, run.stderr, "a passed call is silent")
			})
		}
	}
}

// TestPreToolUseHookRefusesABatchedWriteIntoTheFence is the other direction:
// one write operation into a fenced path among any number of reads refuses the
// batch and names the write's path, for every write the matcher names and in
// either key order. A read op sharing the batch changes nothing.
func TestPreToolUseHookRefusesABatchedWriteIntoTheFence(t *testing.T) {
	t.Parallel()
	for _, order := range []struct {
		name string
		op   func(string, map[string]any) any
	}{{"tool first", toolFirst}, {"args first", argsFirst}} {
		for _, op := range batchWriteOps(t) {
			for _, path := range []string{"spec/.tp-review/x/state.json", ".tp/config.json", "spec/0.35.0.tasks.json"} {
				t.Run(order.name+" "+op+" "+path, func(t *testing.T) {
					run := runPreToolUseHook(t, batchTool, map[string]any{"ops": []any{
						order.op("read", map[string]any{"file": "spec/1.1.0.md"}),
						order.op(op, batchPathArgs(op, path)),
						order.op("faster_search", map[string]any{"path": "spec/.tp-review/x", "pattern": "tp"}),
					}})

					assert.True(t, run.denied(), "%s into %s must be refused; exit=%d stderr=%q", op, path, run.exitCode, run.stderr)
					assert.Contains(t, run.stderr, path, "the refusal names the write's path")
					assert.Empty(t, run.stdout)
				})
			}
		}
	}

	t.Run("replace with a paths list", func(t *testing.T) {
		run := runPreToolUseHook(t, batchTool, map[string]any{"ops": []any{
			toolFirst("read", map[string]any{"file": "README.md"}),
			toolFirst("replace", map[string]any{"paths": []string{"README.md", "spec/1.1.0.tasks.json"}, "old": "a", "new": "b"}),
		}})
		assert.True(t, run.denied(), "exit=%d stderr=%q", run.exitCode, run.stderr)
		assert.Contains(t, run.stderr, "spec/1.1.0.tasks.json")
	})

	// A write's content is where an op boundary could be faked: text that
	// reads as `}},{"tool":"read"` ends the write early to a hook that splits
	// on braces. Inside a JSON string every quote arrives escaped, so the
	// content can never be taken for structure.
	t.Run("a write's content cannot fake an op boundary", func(t *testing.T) {
		fake := `x"}},{"tool":"read","args":{"file":"y"}},{"tool":"read","args":{"q":"` + `\` + `"}`
		run := runPreToolUseHook(t, batchTool, map[string]any{"ops": []any{
			toolFirst("create", map[string]any{"content": fake, "file": ".tp/local.json"}),
			argsFirst("edit", map[string]any{"content": fake, "file": "spec/.tp-review/x/state.json"}),
		}})
		assert.True(t, run.denied(), "exit=%d stderr=%q", run.exitCode, run.stderr)
	})

	// Only the elements of tool_input.ops are ops. An array of read-shaped
	// objects inside a write's own args is an argument, and reading it as a
	// nested batch would let its "read" stand for the write that carries it.
	t.Run("an ops-shaped array inside a write's args is not a batch", func(t *testing.T) {
		run := runPreToolUseHook(t, batchTool, map[string]any{"ops": []any{
			toolFirst("edit", map[string]any{
				"file":  "spec/.tp-review/x/state.json",
				"edits": []any{toolFirst("read", map[string]any{"file": "README.md"})},
			}),
		}})
		assert.True(t, run.denied(), "exit=%d stderr=%q", run.exitCode, run.stderr)
	})

	t.Run("a `tool` key inside a write's args is not the op's tool", func(t *testing.T) {
		run := runPreToolUseHook(t, batchTool, map[string]any{"ops": []any{
			toolFirst("edit", map[string]any{"tool": "read", "file": "spec/.tp-review/x/state.json"}),
		}})
		assert.True(t, run.denied(), "exit=%d stderr=%q", run.exitCode, run.stderr)
	})
}

// TestPreToolUseHookRefusesABatchOpItCannotClassify keeps the batch reading
// fail-closed: an operation whose tool is not a known read — a name codedbpro
// may add later, no tool at all, a tool that is not a string, or two tools —
// is judged by its paths exactly as before, so a fenced path refuses it.
func TestPreToolUseHookRefusesABatchOpItCannotClassify(t *testing.T) {
	t.Parallel()
	const fenced = "spec/.tp-review/x/state.json"
	for _, tc := range []struct{ name, op string }{
		{"an unknown tool", `{"tool":"frobnicate","args":{"file":"` + fenced + `"}}`},
		{"a write-sounding unknown tool", `{"args":{"file":"` + fenced + `"},"tool":"write"}`},
		{"no tool key", `{"args":{"file":"` + fenced + `"}}`},
		{"a tool that is not a string", `{"tool":["read"],"args":{"file":"` + fenced + `"}}`},
		{"a numeric tool", `{"tool":1,"args":{"file":"` + fenced + `"}}`},
		{"a read then a write under two tool keys", `{"tool":"read","tool":"edit","args":{"file":"` + fenced + `"}}`},
		// Which of two keys a parser keeps is its own choice, so neither order
		// may be read as the other: a first-wins parser runs this edit.
		{"a write then a read under two tool keys", `{"tool":"edit","tool":"read","args":{"file":"` + fenced + `"}}`},
		// A key spelled with a code-point escape for its `o` decodes to `tool`:
		// the op names two tools, and the hook, which compares spellings, would
		// see only the plain one. The escape is built from parts so that no
		// editor or encoder on the way to this file can decode it first.
		{"a second tool key spelled with an escape", `{"tool":"read","to` + `\` + `u006fl":"edit","args":{"file":"` + fenced + `"}}`},
		{"a tool name that only starts like a read", `{"tool":"reader","args":{"file":"` + fenced + `"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := `{"session_id":"0e1f2a3b","hook_event_name":"PreToolUse","tool_name":"` + batchTool +
				`","tool_input":{"ops":[{"tool":"read","args":{"file":"README.md"}},` + tc.op + `]}}`
			require.True(t, json.Valid([]byte(payload)), "the case must be JSON a harness could send")

			run := runPreToolUseHookRaw(t, payload)
			assert.True(t, run.denied(), "an op the hook cannot classify must be judged by its path; exit=%d stderr=%q", run.exitCode, run.stderr)
			assert.Contains(t, run.stderr, fenced)
		})
	}
}

// TestPreToolUseHookReadsOnlyAWholeBatchByOp pins the conditions under which
// the hook reads a payload op by op at all. Only the batch tool's own ops are
// read that way, and only in a payload it walked end to end; anything else is
// judged by every path it names, as before, so an ops-shaped argument on
// another tool, or a payload cut short, cannot shed a fenced path.
func TestPreToolUseHookReadsOnlyAWholeBatchByOp(t *testing.T) {
	t.Parallel()
	const fenced = "spec/.tp-review/x/state.json"
	readOp := `{"tool":"read","args":{"file":"` + fenced + `"}}`
	for _, tc := range []struct{ name, payload string }{
		{"another tool carrying an ops argument", `{"tool_name":"mcp__codedbpro__edit","tool_input":{"note":"` + batchTool +
			`","ops":[` + readOp + `],"file":"README.md"}}`},
		{"a batch tool_name overridden by a second one", `{"tool_name":"mcp__codedbpro__edit","tool_name":"` + batchTool +
			`","tool_input":{"ops":[` + readOp + `]}}`},
		{"a batch cut off after its reads", `{"tool_name":"` + batchTool + `","tool_input":{"ops":[` + readOp + `,`},
		{"a batch cut off inside a string", `{"tool_name":"` + batchTool + `","tool_input":{"ops":[` + readOp + `,{"tool":"re`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := runPreToolUseHookRaw(t, tc.payload)
			assert.True(t, run.denied(), "exit=%d stderr=%q", run.exitCode, run.stderr)
			assert.Contains(t, run.stderr, fenced)
		})
	}
}

// TestPreToolUseHookPassesABatchedWriteOutsideTheFence pins that reading a
// batch by operation does not narrow what the fence lets through: writes to
// ordinary files pass, alone or beside reads that name fenced paths.
func TestPreToolUseHookPassesABatchedWriteOutsideTheFence(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		ops  []any
	}{
		{"writes only", []any{
			toolFirst("create", map[string]any{"file": "docs/notes.md", "content": "notes"}),
			argsFirst("edit", map[string]any{"file": "internal/cli/run.go", "content": "package cli"}),
			toolFirst("replace", map[string]any{"paths": []string{"README.md", "spec/1.1.0.md"}, "old": "a", "new": "b"}),
		}},
		{"writes beside fenced reads", []any{
			toolFirst("read", map[string]any{"file": "spec/.tp-review/1.1.0/snapshot-round-1.md"}),
			toolFirst("edit", map[string]any{"file": "internal/cli/run.go", "content": "package cli"}),
			argsFirst("faster_search", map[string]any{"path": ".tp/config.json", "pattern": "gate"}),
		}},
		// Content full of escaped quotes and backslashes: a walk that ended a
		// string at an escaped quote would lose its place, fall back to judging
		// the whole payload, and refuse the fenced reads around it.
		{"quoted content beside fenced reads", []any{
			toolFirst("read", map[string]any{"file": ".tp/local.json"}),
			toolFirst("create", map[string]any{"file": "docs/notes.md", "content": `say "{x}" \ and \"y\" ]}`}),
			argsFirst("read", map[string]any{"file": "spec/.tp-review/x/state.json"}),
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := runPreToolUseHook(t, batchTool, map[string]any{"ops": tc.ops})
			assert.Zero(t, run.exitCode, "stderr=%q", run.stderr)
			assert.Empty(t, run.stderr)
		})
	}
}

// TestBatchReadOpsStayOutsideTheMatcher ties the hooks' read set to their
// matchers: a codedbpro tool a matcher names is a write, so it can never be a
// read inside a batch. A tool that gains the ability to write joins the
// matchers and fails here until the shared classifier stops treating it as a
// read. Both hooks read batches through that one classifier, so the plugin's
// matcher and each role agent's are checked alike.
func TestBatchReadOpsStayOutsideTheMatcher(t *testing.T) {
	t.Parallel()
	matchers := map[string]string{pluginHooksManifestPath: pluginWriteMatcher(t)}
	for _, rel := range []string{agentReviewerFile, agentAuditorFile} {
		groups := readAgentDefinition(t, rel).Hooks["PreToolUse"]
		require.Len(t, groups, 1, "%s", rel)
		matchers[rel] = groups[0].Matcher
	}

	for rel, raw := range matchers {
		matcher, err := regexp.Compile("^(?:" + raw + ")$")
		require.NoError(t, err, "%s", rel)
		for _, op := range batchReadOps {
			assert.False(t, matcher.MatchString("mcp__codedbpro__"+op),
				"%s: %s is in the write matcher, so a batch must not pass it as a read", rel, op)
		}
		for _, op := range codedbproWritesIn(t, raw) {
			assert.NotContains(t, batchReadOps, op, "%s", rel)
		}
	}
}
