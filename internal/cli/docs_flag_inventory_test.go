package cli

import (
	"regexp"
	"sort"
	"strings"
	"testing"

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
func documentedFlags(t *testing.T) map[string]map[string]bool {
	t.Helper()

	lines := strings.Split(readRepoDoc(t, "skills/tp/SKILL.md"), "\n")
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
			// The name is followed by a backtick when the row's form ends
			// there, as in `tp status`.
			command = strings.Trim(strings.Fields(first)[1], "`")
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

// registeredFlags walks the command tree and returns the long flags tp declares
// on each command, plus the root's persistent (global) flags. A command's set
// excludes the globals, since LocalFlags() already drops inherited ones.
func registeredFlags() (perCommand map[string]map[string]bool, globals map[string]bool) {
	root := NewRootCmd()

	globals = map[string]bool{}
	root.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		globals[f.Name] = true
	})

	perCommand = map[string]map[string]bool{}
	for _, sub := range root.Commands() {
		if cobraGeneratedCommands[sub.Name()] {
			continue
		}
		flags := map[string]bool{}
		sub.LocalFlags().VisitAll(func(f *pflag.Flag) {
			if cobraGeneratedFlags[f.Name] {
				return
			}
			flags[f.Name] = true
		})
		perCommand[sub.Name()] = flags
	}
	return perCommand, globals
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
	documented := documentedFlags(t)
	registered, globals := registeredFlags()

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
	documented := documentedFlags(t)
	registered, globals := registeredFlags()

	for _, command := range sortedKeys(documented) {
		if command == "" {
			for _, name := range sortedKeys(documented[""]) {
				assert.True(t, globals[name],
					"SKILL.md's global-flags table documents --%s, which the CLI does not register as a global flag", name)
			}
			continue
		}

		flags, known := registered[command]
		if !assert.True(t, known, "SKILL.md's inventory documents `tp %s`, which the CLI does not register", command) {
			continue
		}
		for _, name := range sortedKeys(documented[command]) {
			assert.True(t, flags[name] || globals[name],
				"SKILL.md's inventory documents `tp %s --%s`, which the CLI does not register", command, name)
		}
	}
}
