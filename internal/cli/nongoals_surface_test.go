package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// v034Surface is the command-and-flag surface tp shipped at the v0.34.2 tag:
// one line per command — its name, a colon, then its long flags in byte order.
// The first line, whose name is empty, carries the root's persistent (global)
// flags. Shorthands are not listed, since a shorthand only exists alongside its
// long name.
//
// The baseline exists so a surface change is always deliberate. It began as
// spec/0.34.0.md §11 Non-Goal 3 — that release added no command and no flag —
// and it keeps its value for a release that does add surface: every task still
// to come is measured against it at every close, because the quality gate runs
// the suite, so an accidental flag is caught by the task after the one that
// added it.
//
// A release that does add a command or a flag updates this baseline in the same
// task that adds it, and renames the constant to the tag it then pins.
// spec/0.35.0.md §3.1 adds exactly one command, `tp run`, which is listed below
// with the flags it ships, and §3.3 adds the three audit-side resolve flags
// (`--resolve`, `--resolve-all`, `--force`) an audit-fix unit disposes its row
// with; nothing else about the surface moves in that release.
const v034Surface = `
: color compact file json no-color no-compact no-quiet quiet
add: bulk spec stdin
audit: affected-files affected-from-tasks base check findings force harness-note merge output record resolve resolve-all status
blocked:
brief: prior
claim: all-ready
close: reason-file skip-gate stdin
commit: files
config: dry-run extract force resolved
done: auto-commit batch commit covered-by files gate-passed reason-file skip-gate stdin
graph: from tag
import: force spec
init: commit-strategy domain eject-roles force quality-gate
keep: list remove
lint:
list: ids status tag
next: brief minimal peek
plan: from level minimal
ready: count first ids
remove: force
reopen:
report:
resume:
review: affected-files check diff-from docs-path final-round findings force harness-note merge no-state output perspective record report resolve resolve-all round spec-inline status test-path verify
run: status
set: bulk local project workflow
show:
stats:
status:
use: clear
validate: project strict
`

// parseSurface reads v034Surface into per-command long-flag sets, keyed by
// command name with "" holding the globals.
func parseSurface(t *testing.T) map[string]map[string]bool {
	t.Helper()

	surface := map[string]map[string]bool{}
	for line := range strings.SplitSeq(strings.TrimSpace(v034Surface), "\n") {
		name, flags, ok := strings.Cut(line, ":")
		require.True(t, ok, "every baseline line names a command before a colon: %q", line)
		set := map[string]bool{}
		for flag := range strings.FieldsSeq(flags) {
			set[flag] = true
		}
		surface[name] = set
	}
	return surface
}

// TestNonGoals_NoCommandOrFlagIsAdded guards §11 Non-Goal 3 in the direction it
// can be crossed — a task registering one more command or one more flag — and
// in the other direction too, since removing a command or a flag is a behaviour
// change outside the sections §11 Non-Goal 1 enumerates.
func TestNonGoals_NoCommandOrFlagIsAdded(t *testing.T) {
	baseline := parseSurface(t)
	registered, globals := registeredFlags()
	registered[""] = globals

	for _, command := range sortedKeys(registered) {
		shipped, known := baseline[command]
		if !assert.True(t, known,
			"the CLI registers `tp %s`, which v0.34.2 did not ship: this release adds no command (§11 Non-Goal 3)", command) {
			continue
		}
		for _, name := range sortedKeys(registered[command]) {
			assert.True(t, shipped[name],
				"the CLI registers `tp %s --%s`, which v0.34.2 did not ship: this release adds no flag (§11 Non-Goal 3)", command, name)
		}
	}

	for _, command := range sortedKeys(baseline) {
		flags, known := registered[command]
		if !assert.True(t, known,
			"v0.34.2 shipped `tp %s` and the CLI no longer registers it: dropping a command is a behaviour change outside the sections §11 Non-Goal 1 enumerates", command) {
			continue
		}
		for _, name := range sortedKeys(baseline[command]) {
			assert.True(t, flags[name],
				"v0.34.2 shipped `tp %s --%s` and the CLI no longer registers it: dropping a flag is a behaviour change outside the sections §11 Non-Goal 1 enumerates", command, name)
		}
	}
}
