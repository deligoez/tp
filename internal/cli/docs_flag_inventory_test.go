package cli

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// flagInventoryHeading is the SKILL.md section §8.1 made the canonical home of
// the command forms, and §8.3 the checked inventory of every registered flag.
const flagInventoryHeading = "## Command & flag inventory"

// inventoryFlagToken matches a long flag as it is written in the inventory.
// Shorthands are not collected: a shorthand only exists alongside its long
// name, so requiring the long name is the same requirement.
var inventoryFlagToken = regexp.MustCompile(`--[a-z][a-z0-9-]*`)

// cobraGeneratedFlags are added by cobra during execution rather than declared
// by tp, so the inventory does not record them.
var cobraGeneratedFlags = map[string]bool{"help": true, "version": true}

// cobraGeneratedCommands are likewise cobra's, not tp's.
var cobraGeneratedCommands = map[string]bool{"help": true, "completion": true}

// documentedFlags reads the §8.3 inventory out of SKILL.md and returns, per
// command name, the long flags its rows mention. The empty command name holds
// the global-flags table.
//
// A row is attributed to the command its first cell names, and every flag
// token anywhere in the row — form cell or purpose cell — counts as documented
// for that command. That is what makes the purpose column load-bearing: a flag
// mentioned only in prose there is still documented, while a flag mentioned
// under a command that does not register it is drift this file reports.
func documentedFlags(t *testing.T, registered map[string]map[string]bool) map[string]map[string]bool {
	t.Helper()
	return parseInventory(t, readRepoDoc(t, "skills/tp/SKILL.md"), registered)
}

// parseInventory is documentedFlags over an inventory passed as text.
func parseInventory(t *testing.T, doc string, registered map[string]map[string]bool) map[string]map[string]bool {
	t.Helper()

	lines := strings.Split(doc, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == flagInventoryHeading {
			start = i + 1
			break
		}
	}
	require.NotEqual(t, -1, start, "SKILL.md carries the %q section", flagInventoryHeading)

	documented := map[string]map[string]bool{}
	for _, line := range lines[start:] {
		if strings.HasPrefix(line, "## ") {
			break
		}
		cells := strings.Split(line, "|")
		if !strings.HasPrefix(line, "|") || len(cells) < 2 {
			continue
		}
		first := strings.TrimLeft(strings.TrimSpace(cells[1]), " `")

		var command string
		switch {
		case strings.HasPrefix(first, "tp "):
			command = inventoryCommand(strings.Fields(first), registered)
		case strings.HasPrefix(first, "--"):
			command = "" // the global-flags table
		default:
			continue // header or separator row
		}

		if documented[command] == nil {
			documented[command] = map[string]bool{}
		}
		for _, token := range inventoryFlagToken.FindAllString(line, -1) {
			documented[command][strings.TrimPrefix(token, "--")] = true
		}
	}
	return documented
}

// inventoryCommand names the command a row's form starts with. The name is
// followed by a backtick when the row's form ends there, as in `tp status`.
func inventoryCommand(fields []string, registered map[string]map[string]bool) string {
	command := strings.Trim(fields[1], "`")
	if len(fields) > 2 {
		// A parent's row is attributed to the subcommand it names, as in
		// `tp epic export --md`, so a flag under the wrong subcommand is drift.
		if sub := command + " " + strings.Trim(fields[2], "`"); registered[sub] != nil {
			return sub
		}
	}
	return command
}

// inventoryDrift lists every flag the inventory documents under a command that
// does not register it, and every command it documents that the CLI lacks.
func inventoryDrift(documented, registered map[string]map[string]bool, globals map[string]bool) []string {
	var drift []string
	for _, command := range sortedKeys(documented) {
		if command == "" {
			for _, name := range sortedKeys(documented[""]) {
				if !globals[name] {
					drift = append(drift, fmt.Sprintf(
						"SKILL.md's global-flags table documents --%s, which the CLI does not register as a global flag", name))
				}
			}
			continue
		}

		flags, known := registered[command]
		if !known {
			drift = append(drift, fmt.Sprintf("SKILL.md's inventory documents `tp %s`, which the CLI does not register", command))
			continue
		}
		for _, name := range sortedKeys(documented[command]) {
			if !flags[name] && !globals[name] {
				drift = append(drift, fmt.Sprintf(
					"SKILL.md's inventory documents `tp %s --%s`, which the CLI does not register", command, name))
			}
		}
	}
	return drift
}

// rootCmdMu serialises NewRootCmd across parallel tests, and newRootCmdForTest
// is the only way this package's tests should build a root command.
//
// The constructor BINDS tp's package-level flag variables: pflag's StringVar
// and BoolVar write through the pointer they are handed, so root.go's eight
// PersistentFlags lines each assign a default to a shared word every time a
// root command is built. Two parallel tests that build one therefore write the
// same addresses concurrently. Measured with -race: the two
// TestSkillFlagInventory tests run alone reported 30 data races per run, every
// one a write from NewRootCmd, and none of them a defect in either test. It
// reproduces at the commit that marked this package's tests parallel and is
// not visible without -race, which is why it survived as an intermittent
// suite failure rather than a reproducible one.
//
// The lock covers construction only. What callers read afterwards is the
// returned command tree and its *pflag.Flag values, which no other test writes.
var rootCmdMu sync.Mutex

func newRootCmdForTest() *cobra.Command {
	rootCmdMu.Lock()
	defer rootCmdMu.Unlock()
	return NewRootCmd()
}

// registeredFlags walks the command tree and returns the long flags tp declares
// on each command, plus the root's persistent (global) flags. A command's set
// excludes the globals, since LocalFlags() already drops inherited ones. A
// subcommand is its own entry, keyed by parent and name (`epic export`), the
// way the inventory attributes a `tp epic export --md` row.
func registeredFlags() (perCommand map[string]map[string]bool, globals map[string]bool) {
	root := newRootCmdForTest()

	globals = map[string]bool{}
	root.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		globals[f.Name] = true
	})

	perCommand = map[string]map[string]bool{}
	for _, cmd := range root.Commands() {
		if cobraGeneratedCommands[cmd.Name()] {
			continue
		}
		perCommand[cmd.Name()] = localFlags(cmd)
		for _, sub := range cmd.Commands() {
			if !cobraGeneratedCommands[sub.Name()] {
				perCommand[cmd.Name()+" "+sub.Name()] = localFlags(sub)
			}
		}
	}
	return perCommand, globals
}

// localFlags returns cmd's own long flags.
func localFlags(cmd *cobra.Command) map[string]bool {
	flags := map[string]bool{}
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if !cobraGeneratedFlags[f.Name] {
			flags[f.Name] = true
		}
	})
	return flags
}

// sortedKeys keeps the reported drift deterministic, so a failing run names the
// same first flag every time.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// TestSkillFlagInventoryIsComplete guards §8.3 in the direction that found the
// six missing flags: every flag the CLI registers appears in SKILL.md's
// inventory, under the command that registers it.
func TestSkillFlagInventoryIsComplete(t *testing.T) {
	t.Parallel()
	registered, globals := registeredFlags()
	documented := documentedFlags(t, registered)

	for _, command := range sortedKeys(registered) {
		for _, name := range sortedKeys(registered[command]) {
			assert.True(t, documented[command][name],
				"the CLI registers `tp %s --%s` and SKILL.md's inventory does not document it", command, name)
		}
	}

	for _, name := range sortedKeys(globals) {
		assert.True(t, documented[""][name],
			"the CLI registers the global flag --%s and SKILL.md's global-flags table does not document it", name)
	}
}

// TestSkillFlagInventoryRecordsOnlyWhatExists guards §8.3 in the other
// direction: the inventory records what the CLI has, so a flag it documents
// that no command registers — one renamed, removed, or never shipped — fails
// here.
func TestSkillFlagInventoryRecordsOnlyWhatExists(t *testing.T) {
	t.Parallel()
	registered, globals := registeredFlags()
	assert.Empty(t, inventoryDrift(documentedFlags(t, registered), registered, globals))
}

// TestSkillFlagInventoryReportsAFlagUnderTheWrongEpicSubcommand pins the
// attribution grain: `tp epic remove` registers --force and `tp epic set` does
// not, so an inventory row documenting `tp epic set --force` is drift even
// though some epic subcommand registers the flag.
func TestSkillFlagInventoryReportsAFlagUnderTheWrongEpicSubcommand(t *testing.T) {
	t.Parallel()
	registered, globals := registeredFlags()
	doc := flagInventoryHeading + "\n\n" +
		"| Form | Purpose |\n|---|---|\n" +
		"| `tp epic set <id> <field>=<value> --force` | Set one field |\n"

	drift := inventoryDrift(parseInventory(t, doc, registered), registered, globals)

	assert.Equal(t, []string{
		"SKILL.md's inventory documents `tp epic set --force`, which the CLI does not register",
	}, drift)
}
