package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// roleBatchWriteOps is every codedbpro write either role agent's matcher names,
// derived from the definitions rather than listed, for the reason
// batchWriteOps gives.
func roleBatchWriteOps(t *testing.T) []string {
	t.Helper()
	var ops []string
	for _, rel := range []string{agentReviewerFile, agentAuditorFile} {
		groups := readAgentDefinition(t, rel).Hooks["PreToolUse"]
		require.Len(t, groups, 1, "%s", rel)
		for _, op := range codedbproWritesIn(t, groups[0].Matcher) {
			if !slices.Contains(ops, op) {
				ops = append(ops, op)
			}
		}
	}
	return ops
}

// outsideTheRole are paths a role unit may read but never write: the spec it
// was spawned to review, source, the round's merged file, a sibling role's
// findings, its own findings under the final name the driver renames to, and
// tp's own state.
func outsideTheRole(t *testing.T) []string {
	t.Helper()
	round := absoluteUnit(t).roundDir
	return []string{
		"spec/0.35.0.md",
		"internal/engine/runnertemplate.go",
		filepath.Join(round, "merged.ndjson"),
		filepath.Join(round, "role-tester.ndjson.part"),
		filepath.Join(round, "role-implementer.ndjson"),
		".tp/config.json",
		"spec/.tp-review/0.35.0/snapshot-round-3.md",
	}
}

var batchOrders = []struct {
	name string
	op   func(string, map[string]any) any
}{{"tool first", toolFirst}, {"args first", argsFirst}}

// TestRoleWriteHookPassesABatchThatOnlyReads is the role half of the batch
// defect (measurements case R1): inside a role unit, a batch that read the
// spec it was spawned to review was refused as "not this unit's to write",
// while the same read as its own call never reaches the hook. A batch whose
// every operation reads passes, whatever it names.
func TestRoleWriteHookPassesABatchThatOnlyReads(t *testing.T) {
	t.Parallel()
	unit := absoluteUnit(t)

	t.Run("the measured case: a batched read of the spec", func(t *testing.T) {
		run := runRoleWriteHook(t, unit, batchTool, map[string]any{"ops": []any{
			toolFirst("read", map[string]any{"file": "spec/0.35.0.md"}),
		}})
		assert.Zero(t, run.exitCode, "a read is not a write; stderr=%q", run.stderr)
		assert.Empty(t, run.stderr)
	})

	for _, order := range batchOrders {
		for _, op := range batchReadOps {
			t.Run(order.name+" "+op, func(t *testing.T) {
				var ops []any
				for _, path := range outsideTheRole(t) {
					ops = append(ops, order.op(op, batchPathArgs(op, path)))
				}
				run := runRoleWriteHook(t, unit, batchTool, map[string]any{"ops": ops})

				assert.Zero(t, run.exitCode, "%s writes nothing; stderr=%q", op, run.stderr)
				assert.Empty(t, run.stderr, "a passed call is silent")
			})
		}
	}

	// A read made alone never reaches the hook, whatever the environment says,
	// so a batched one does not depend on the environment either. The
	// allowlist that cannot build itself still refuses every write (the
	// fail-closed test beside the single-call ones pins that).
	t.Run("without the round environment", func(t *testing.T) {
		run := runRoleWriteHook(t, roleUnitEnv{}, batchTool, map[string]any{"ops": []any{
			toolFirst("read", map[string]any{"file": "spec/0.35.0.md"}),
			argsFirst("faster_search", map[string]any{"path": "internal", "pattern": "tp"}),
		}})
		assert.Zero(t, run.exitCode, "stderr=%q", run.stderr)
	})
}

// TestRoleWriteHookRefusesABatchedWriteOutsideItsFiles is the other direction:
// one write outside the unit's two files, among reads, refuses the batch and
// names the path, for every write the role matchers name and in either key
// order.
func TestRoleWriteHookRefusesABatchedWriteOutsideItsFiles(t *testing.T) {
	t.Parallel()
	unit := absoluteUnit(t)
	own := filepath.Join(unit.roundDir, "role-implementer.ndjson.part")

	for _, order := range batchOrders {
		for _, op := range roleBatchWriteOps(t) {
			for _, path := range outsideTheRole(t) {
				t.Run(order.name+" "+op+" "+path, func(t *testing.T) {
					run := runRoleWriteHook(t, unit, batchTool, map[string]any{"ops": []any{
						order.op("read", map[string]any{"file": "spec/0.35.0.md"}),
						order.op("create", map[string]any{"file": own, "content": "{}\n"}),
						order.op(op, batchPathArgs(op, path)),
					}})

					assert.True(t, run.denied(), "%s into %s must be refused; exit=%d stderr=%q", op, path, run.exitCode, run.stderr)
					assert.Contains(t, run.stderr, path, "the refusal names the path it refused")
					assert.Empty(t, run.stdout)
				})
			}
		}
	}

	t.Run("replace with a paths list reaching past the unit's file", func(t *testing.T) {
		run := runRoleWriteHook(t, unit, batchTool, map[string]any{"ops": []any{
			toolFirst("replace", map[string]any{"paths": []string{own, "spec/0.35.0.md"}, "old": "a", "new": "b"}),
		}})
		assert.True(t, run.denied(), "exit=%d stderr=%q", run.exitCode, run.stderr)
		assert.Contains(t, run.stderr, "spec/0.35.0.md")
	})

	t.Run("an ops-shaped array inside a write's args is not a batch", func(t *testing.T) {
		run := runRoleWriteHook(t, unit, batchTool, map[string]any{"ops": []any{
			toolFirst("edit", map[string]any{
				"file":  "spec/0.35.0.md",
				"edits": []any{toolFirst("read", map[string]any{"file": own})},
			}),
		}})
		assert.True(t, run.denied(), "exit=%d stderr=%q", run.exitCode, run.stderr)
	})
}

// TestRoleWriteHookRefusesABatchOpItCannotClassify keeps the role reading
// fail-closed exactly as the fence's is: an op whose tool is not a known read
// is judged by its paths, so a path outside the unit's files refuses it.
func TestRoleWriteHookRefusesABatchOpItCannotClassify(t *testing.T) {
	t.Parallel()
	const outside = "spec/0.35.0.md"
	for _, tc := range []struct{ name, op string }{
		{"an unknown tool", `{"tool":"frobnicate","args":{"file":"` + outside + `"}}`},
		{"no tool key", `{"args":{"file":"` + outside + `"}}`},
		{"a tool that is not a string", `{"tool":["read"],"args":{"file":"` + outside + `"}}`},
		{"a write then a read under two tool keys", `{"tool":"edit","tool":"read","args":{"file":"` + outside + `"}}`},
		{"a second tool key spelled with an escape", `{"tool":"read","to` + `\` + `u006fl":"edit","args":{"file":"` + outside + `"}}`},
		{"a tool name that only starts like a read", `{"tool":"reader","args":{"file":"` + outside + `"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := `{"session_id":"7c1d2e3f","hook_event_name":"PreToolUse","tool_name":"` + batchTool +
				`","tool_input":{"ops":[{"tool":"read","args":{"file":"README.md"}},` + tc.op + `]}}`
			require.True(t, json.Valid([]byte(payload)), "the case must be JSON a harness could send")

			run := runRoleWriteHookRaw(t, absoluteUnit(t), payload)
			assert.True(t, run.denied(), "exit=%d stderr=%q", run.exitCode, run.stderr)
			assert.Contains(t, run.stderr, outside)
		})
	}
}

// TestRoleWriteHookPassesABatchedWriteToItsOwnFiles mirrors the single-call
// allow test: the unit's findings .part and its escalation record are its to
// write through a batch too, beside reads of anything (measurements case R3,
// a batch that reads the spec and creates the unit's own findings file).
func TestRoleWriteHookPassesABatchedWriteToItsOwnFiles(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		unit       roleUnitEnv
		findings   string
		escalation string
	}{
		{
			name:       "absolute round dir",
			unit:       absoluteUnit(t),
			findings:   filepath.Join(absoluteUnit(t).roundDir, "role-implementer.ndjson.part"),
			escalation: ".tp/runs/01JB0000000000000000000000/7-escalation.json",
		},
		{
			name:       "relative round dir",
			unit:       relativeUnit(),
			findings:   "./.tp/rounds/0.35.0/audit-r2/role-go-safety.ndjson.part",
			escalation: ".tp/runs/01JB0000000000000000000001/12-escalation.json",
		},
	} {
		for _, order := range batchOrders {
			t.Run(tc.name+" "+order.name, func(t *testing.T) {
				run := runRoleWriteHook(t, tc.unit, batchTool, map[string]any{"ops": []any{
					order.op("read", map[string]any{"file": "spec/0.35.0.md"}),
					order.op("create", map[string]any{"file": tc.findings, "content": `{"status":"FAIL","note":"a \"quoted\" {x}"}` + "\n"}),
					order.op("faster_search", map[string]any{"path": "internal/engine", "pattern": "roleUnits"}),
					order.op("edit", map[string]any{"file": tc.escalation, "content": "{}"}),
				}})
				assert.Zero(t, run.exitCode, "the unit's own files through a batch; stderr=%q", run.stderr)
				assert.Empty(t, run.stderr)
			})
		}
	}
}

// TestBatchReadingFailsClosedWithoutTheSharedClassifier is the experiment
// behind sharing one classifier between two hooks: a hook shipped without it
// must fall back to judging every path in the batch, never to passing it.
// Each hook is copied alone into an empty directory and handed a batch that
// only reads a path the hook refuses to let be written.
func TestBatchReadingFailsClosedWithoutTheSharedClassifier(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		hook string
		env  []string
		path string
	}{
		{preToolUseHookPath, []string{"PATH=/usr/bin:/bin"}, "spec/.tp-review/x/state.json"},
		{roleWriteHookPath, absoluteUnit(t).env(), "spec/0.35.0.md"},
	} {
		t.Run(tc.hook, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(tc.hook)))
			require.NoError(t, err)
			alone := filepath.Join(t.TempDir(), filepath.Base(tc.hook))
			require.NoError(t, os.WriteFile(alone, src, 0o700)) //nolint:gosec // an executable copy of the hook under test

			payload, err := json.Marshal(map[string]any{
				"hook_event_name": "PreToolUse",
				"tool_name":       batchTool,
				"tool_input":      map[string]any{"ops": []any{toolFirst("read", map[string]any{"file": tc.path})}},
			})
			require.NoError(t, err)

			cmd := exec.Command(alone) //nolint:gosec // a copy of a fixed script from the repo under test
			cmd.Env = tc.env
			cmd.Dir = repoRoot(t)
			cmd.Stdin = strings.NewReader(string(payload))
			var stderr strings.Builder
			cmd.Stderr = &stderr
			runErr := cmd.Run()

			var exitErr *exec.ExitError
			require.ErrorAs(t, runErr, &exitErr, "without the classifier the batch is judged whole; stderr=%q", stderr.String())
			assert.Equal(t, 2, exitErr.ExitCode(), "stderr=%q", stderr.String())
			assert.Contains(t, stderr.String(), tc.path)
		})
	}
}
