package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/cli"
)

// treeFixture is twoProjects plus what the ways of naming a task file need: a
// findings file and an import plan in the foreign project, and a project
// nested inside the working directory's, its task file named by the working
// directory's active pointer.
type treeFixture struct {
	here, there, taskFile, spec string
	nestedSpec, nestedTask      string
	plan, findings              string
}

func newTreeFixture(t *testing.T) treeFixture {
	t.Helper()
	here, there, taskFile := twoProjects(t)
	f := treeFixture{here: here, there: there, taskFile: taskFile, spec: filepath.Join(there, "spec.md")}

	f.findings = filepath.Join(there, "findings.ndjson")
	require.NoError(t, os.WriteFile(f.findings, nil, 0o600))

	// An import plan whose spec is the foreign one: import derives its target
	// task file from the plan's spec field.
	var plan map[string]any
	data, err := os.ReadFile(taskFile)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &plan))
	plan["spec"] = f.spec
	out, err := json.Marshal(plan)
	require.NoError(t, err)
	f.plan = filepath.Join(here, "plan.json")
	require.NoError(t, os.WriteFile(f.plan, out, 0o600))

	// A project nested in this one: its own .tp/config.json makes it a
	// project, and the active pointer is the one way that names its task file
	// with no path on the command line.
	nested := filepath.Join(here, "nested")
	require.NoError(t, os.MkdirAll(filepath.Join(nested, ".tp"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(nested, ".tp", "config.json"), []byte(`{"workflow":{"quality_gate":"touch nested-gate-ran"}}`), 0o600))
	f.nestedSpec = filepath.Join(nested, "spec.md")
	require.NoError(t, os.WriteFile(f.nestedSpec, []byte("# Spec\n## 1. A\ncontent\n"), 0o600))
	_, stderr, code := runTP(t, nested, "init", "spec.md")
	require.Equal(t, 0, code, "init nested: %s", stderr)
	_, stderr, code = runTP(t, nested, "add",
		`{"id":"t1","title":"T","estimate_minutes":5,"acceptance":"Done.","source_sections":["# Spec"]}`)
	require.Equal(t, 0, code, "add nested: %s", stderr)
	f.nestedTask = filepath.Join("nested", "spec.tasks.json")
	require.NoError(t, os.WriteFile(filepath.Join(here, ".tp", "local.json"), []byte(`{"active":"`+f.nestedTask+`"}`), 0o600))
	return f
}

// way is one way of giving a mode its task file: the arguments and the extra
// environment, or the reason the mode takes no task file that way.
type way struct {
	args []string
	env  []string
	na   string
}

// treeModeWays lists, for every tree mode, each way it can be given a task
// file. The test fails on a tree mode missing here, so a mode placed on the
// tree side is driven with a foreign task file before it ships.
func treeModeWays(f *treeFixture) map[string]map[string]way {
	fileEnv := []string{"TP_FILE=" + f.taskFile}
	noSpec := "takes task ids, not a spec"
	return map[string]map[string]way{
		"tp done": {
			"spec": {na: noSpec}, "--file": {args: []string{"done", "--file", f.taskFile, "t1", "implemented and verified"}},
			"TP_FILE": {args: []string{"done", "t1", "implemented and verified"}, env: fileEnv},
			"active":  {args: []string{"done", "t1", "implemented and verified"}},
		},
		"tp close": {
			"spec": {na: noSpec}, "--file": {args: []string{"close", "--file", f.taskFile, "t1", "implemented and verified"}},
			"TP_FILE": {args: []string{"close", "t1", "implemented and verified"}, env: fileEnv},
			"active":  {args: []string{"close", "t1", "implemented and verified"}},
		},
		"tp commit": {
			"spec": {na: noSpec}, "--file": {args: []string{"commit", "--file", f.taskFile, "t1", "implemented"}},
			"TP_FILE": {args: []string{"commit", "t1", "implemented"}, env: fileEnv},
			"active":  {args: []string{"commit", "t1", "implemented"}},
		},
		"tp import": {
			"spec":    {args: []string{"import", f.plan}},
			"--file":  {na: "the target is the plan's spec field, not a discovered task file"},
			"TP_FILE": {na: "the target is the plan's spec field, not a discovered task file"},
			"active":  {na: "the target is the plan's spec field, not a discovered task file"},
		},
		"tp review": {
			"spec":    {args: []string{"review", f.spec}},
			"--file":  {args: []string{"review", f.spec, "--file", f.taskFile}},
			"TP_FILE": {args: []string{"review", f.spec}, env: fileEnv},
			"active":  {args: []string{"review", f.nestedSpec}},
		},
		"tp review --perspective": {
			"spec":    {args: []string{"review", f.spec, "--perspective", "regression"}},
			"--file":  {args: []string{"review", f.spec, "--perspective", "regression", "--file", f.taskFile}},
			"TP_FILE": {args: []string{"review", f.spec, "--perspective", "regression"}, env: fileEnv},
			"active":  {args: []string{"review", f.nestedSpec, "--perspective", "regression"}},
		},
		"tp review --record": {
			"spec":    {args: []string{"review", f.spec, "--record", f.findings}},
			"--file":  {args: []string{"review", f.spec, "--record", f.findings, "--file", f.taskFile}},
			"TP_FILE": {args: []string{"review", f.spec, "--record", f.findings}, env: fileEnv},
			"active":  {args: []string{"review", f.nestedSpec, "--record", f.findings}},
		},
		"tp review --verify": {
			"spec":    {args: []string{"review", f.spec, "--verify", "--findings", f.findings}},
			"--file":  {args: []string{"review", f.spec, "--verify", "--findings", f.findings, "--file", f.taskFile}},
			"TP_FILE": {args: []string{"review", f.spec, "--verify", "--findings", f.findings}, env: fileEnv},
			"active":  {args: []string{"review", f.nestedSpec, "--verify", "--findings", f.findings}},
		},
		"tp audit": {
			"spec":    {args: []string{"audit", f.spec}},
			"--file":  {args: []string{"audit", f.spec, "--file", f.taskFile}},
			"TP_FILE": {args: []string{"audit", f.spec}, env: fileEnv},
			"active":  {args: []string{"audit", f.nestedSpec}},
		},
		"tp run": {
			"spec":    {args: []string{"run", f.spec}},
			"--file":  {args: []string{"run", "--file", f.taskFile}},
			"TP_FILE": {args: []string{"run"}, env: fileEnv},
			"active":  {args: []string{"run"}},
		},
		"tp run --dry-run": {
			"spec":    {args: []string{"run", f.spec, "--dry-run"}},
			"--file":  {args: []string{"run", "--dry-run", "--file", f.taskFile}},
			"TP_FILE": {args: []string{"run", "--dry-run"}, env: fileEnv},
			"active":  {args: []string{"run", "--dry-run"}},
		},
	}
}

// TestEveryTreeModeRefusesAForeignTaskFileEveryWay is the class test for the
// side check: a tree mode refuses through the task file it resolved itself,
// so for every tree mode and every way it can be given a task file — a
// positional spec, --file, TP_FILE, the active pointer — a task file of
// another project exits 3 and no gate runs in either tree.
func TestEveryTreeModeRefusesAForeignTaskFileEveryWay(t *testing.T) {
	t.Parallel()
	f := newTreeFixture(t)
	table := treeModeWays(&f)
	for _, mode := range cli.TreeModes() {
		ways, ok := table[mode]
		if !assert.True(t, ok, "tree mode %s has no row in treeModeWays: drive it with a foreign task file every way", mode) {
			continue
		}
		for _, name := range []string{"spec", "--file", "TP_FILE", "active"} {
			w, listed := ways[name]
			if !assert.True(t, listed, "%s: way %s is neither driven nor marked n/a", mode, name) || w.na != "" {
				continue
			}
			t.Run(mode+" via "+name, func(t *testing.T) {
				_, stderr, code := runTPEnv(t, f.here, w.env, w.args...)
				assert.Equal(t, 3, code, "a foreign task file is refused: %s", stderr)
				assert.Contains(t, stderr, "is not this project's")
			})
		}
	}
	for _, marker := range []string{"here-gate-ran", "there-gate-ran", filepath.Join("nested", "nested-gate-ran")} {
		assert.NoFileExists(t, filepath.Join(f.here, marker))
		assert.NoFileExists(t, filepath.Join(f.there, marker))
	}
}
