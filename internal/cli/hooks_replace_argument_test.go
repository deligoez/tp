package cli

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Both fences declare `mcp__codedbpro__replace` in their matcher and in
// writeTools, and both read the write target out of the payload with one grep
// over `file_path`, `notebook_path` and `file`. replace carries neither: its
// target is `path` (one file or directory) or `paths` (a list). So the tool was
// matched, nothing was extracted, the loop over the extracted paths never ran,
// and the hook exited 0 — a fence failing open on a tool it names itself.
//
// The existing coverage could not see it. TestPreToolUseHookDeniesTheFencedPaths
// walks all eight tools but sends every one of them a `file_path` through
// pathArguments, so what it proved for replace is that the fence refuses an
// argument this tool cannot send. The fixture's incidental property — one key
// for every tool — was carrying the verdict.
//
// Measured against the unfixed hooks on spec/.tp-review/0.35.0/round-3/merged.ndjson:
// Write and mcp__codedbpro__create exit 2, mcp__codedbpro__replace exits 0 under
// `path` and again under `paths`; the role allowlist exits 2 for a Write outside
// the unit's one permitted file and 0 for the identical replace.
func TestWriteDenyFenceReadsReplacesOwnPathArgument(t *testing.T) {
	t.Parallel()
	fenced := "spec/.tp-review/0.35.0/round-3/merged.ndjson"

	for name, input := range map[string]map[string]any{
		"path": {
			"pattern": "a", "replacement": "b", "path": fenced, "apply": true,
		},
		"paths": {
			"pattern": "a", "replacement": "b", "paths": []string{fenced}, "apply": true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			run := runPreToolUseHook(t, "mcp__codedbpro__replace", input)
			assert.Equal(t, 2, run.exitCode,
				"a fenced write through replace's %s argument must be refused; stderr=%q", name, run.stderr)
			assert.Contains(t, run.stderr, fenced,
				"the refusal must name the path it refused")
		})
	}
}

// The same hole in the sibling fence, and in the direction that matters more:
// the allowlist exists to hold a role unit to its one findings file, so a
// silently unextracted path lets that unit rewrite the spec, the source, or
// another role's round.
func TestRoleWriteAllowFenceReadsReplacesOwnPathArgument(t *testing.T) {
	t.Parallel()
	round := t.TempDir()
	unit := roleUnitEnv{
		roundDir: round,
		unitID:   "implementer",
		runDir:   t.TempDir(),
		unitSeq:  "1",
	}
	outside := filepath.Join(t.TempDir(), "not-this-units-file.md")

	// The control: the same unit, the same path, through a tool whose argument
	// the hook already reads. Without it a green result below could mean the
	// environment was wrong rather than that the fence held.
	control := runRoleWriteHook(t, unit, "Write", map[string]any{
		"file_path": outside,
		"content":   "x",
	})
	assert.Equal(t, 2, control.exitCode,
		"control: a Write outside the unit's one file must be refused; stderr=%q", control.stderr)

	run := runRoleWriteHook(t, unit, "mcp__codedbpro__replace", map[string]any{
		"pattern": "a", "replacement": "b", "path": outside, "apply": true,
	})
	assert.Equal(t, 2, run.exitCode,
		"the identical write through replace must be refused too; stderr=%q", run.stderr)
}
