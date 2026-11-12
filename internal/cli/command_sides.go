package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// The side of a mode, for spec/backlog/project-layer-from-task-file.md: the
// project layer is resolved from the task file, and a mode whose work happens
// in the working tree is refused when the task file's project root is not the
// working directory's. What reads only the task file and its state is allowed
// from outside.
const (
	// sideTree is work in the working tree, by writing to it or by sending
	// agents to read it: the quality gate, a registered check, a commit, a
	// review or audit emission, a run, and the run oracle's reading of git.
	sideTree = "tree"
	// sideFile reads or writes only the task file and its state.
	sideFile = "file"
	// sideNone takes no task file, so discovery is the working directory's.
	sideNone = "none"
)

// commandSides places every mode: a command's path for its default mode, and
// "<path> --<flag>" for a mode a flag selects (commandModeFlags). The
// enumerating tests fail on a command or mode missing here or an entry no
// command has, so a new one has to be placed rather than defaulting to a side,
// and every tree mode is driven with a foreign task file every way it can be
// given one.
//
// A tree mode refuses through enterTree, with the task file it has already
// resolved by its own rule; nothing here resolves one, since a check that
// resolves by another route can approve a target the mode does not use.
var commandSides = map[string]string{
	"tp audit":           sideTree,
	"tp close":           sideTree,
	"tp commit":          sideTree,
	"tp done":            sideTree,
	"tp import":          sideTree,
	"tp review":          sideTree,
	"tp review --record": sideTree,
	"tp review --verify": sideTree,
	"tp run":             sideTree,
	// --dry-run computes next units through the oracle, which reads the
	// working tree's git status (the unexplained-changes blocker empties the
	// batch), so it reads the tree it runs in.
	"tp run --dry-run": sideTree,

	"tp audit --record":       sideFile,
	"tp audit --status":       sideFile,
	"tp review --merge":       sideFile,
	"tp review --report":      sideFile,
	"tp review --resolve":     sideFile,
	"tp review --resolve-all": sideFile,
	// --status --check runs registered checks only through runMechanicalChecks,
	// which refuses a foreign task file itself.
	"tp review --status": sideFile,
	// --status reads the run state file and the run lock and nothing else.
	"tp run --status": sideFile,
	"tp add":          sideFile,
	"tp blocked":      sideFile,
	"tp brief":        sideFile,
	"tp claim":        sideFile,
	"tp config":       sideFile,
	"tp export":       sideFile,
	"tp graph":        sideFile,
	"tp init":         sideFile,
	"tp list":         sideFile,
	"tp next":         sideFile,
	"tp plan":         sideFile,
	"tp ready":        sideFile,
	"tp remove":       sideFile,
	"tp reopen":       sideFile,
	"tp report":       sideFile,
	"tp resume":       sideFile,
	"tp set":          sideFile,
	"tp show":         sideFile,
	"tp stats":        sideFile,
	"tp status":       sideFile,
	"tp unclaim":      sideFile,
	"tp validate":     sideFile,

	"tp dashboard":          sideNone,
	"tp epic":               sideNone,
	"tp epic add":           sideNone,
	"tp epic apply":         sideNone,
	"tp epic claim":         sideNone,
	"tp epic done":          sideNone,
	"tp epic export":        sideNone,
	"tp epic gate":          sideNone,
	"tp epic import":        sideNone,
	"tp epic link":          sideNone,
	"tp epic lint":          sideNone,
	"tp epic milestone":     sideNone,
	"tp epic milestone add": sideNone,
	"tp epic ready":         sideNone,
	"tp epic remove":        sideNone,
	"tp epic review":        sideNone,
	"tp epic scope":         sideNone,
	"tp epic set":           sideNone,
	"tp epic status":        sideNone,
	"tp epic unlink":        sideNone,
	"tp escalate":           sideNone,
	"tp ground":             sideNone,
	"tp keep":               sideNone,
	"tp lint":               sideNone,
	"tp use":                sideNone,
}

// commandModeFlags names, per command, the flags that select a mode of their
// own side. Every one is placed in commandSides as "<path> --<flag>".
var commandModeFlags = map[string][]string{
	"tp audit":  {"record", "status"},
	"tp review": {"merge", "record", "report", "resolve", "resolve-all", "status", "verify"},
	"tp run":    {"dry-run", "status"},
}

// projectLayerReaders are the file-side commands that resolve the workflow
// from the project config; with the tree modes they are the paths on which an
// unreadable config is an error.
var projectLayerReaders = map[string]bool{
	"tp brief": true, "tp config": true, "tp init": true, "tp next": true,
	"tp plan": true, "tp resume": true, "tp set": true,
}

// checkProjectConfig runs before every command and refuses an unreadable
// project config on a file-side command that reads the project layer. The
// config is the task file's project's; for tp init, which creates its task
// file beside the spec it is given, the spec's directory's. It decides no
// side: a tree mode checks its config in enterTree, on the task file it
// resolved itself.
func checkProjectConfig(c *cobra.Command, args []string) {
	path := c.CommandPath()
	if !projectLayerReaders[path] {
		return
	}
	if path == "tp init" {
		if len(args) > 0 {
			refuseUnreadableProjectConfig(filepath.Dir(args[0]))
		}
		return
	}
	if taskFile, err := engine.DiscoverTaskFile(".", flagFile); err == nil {
		refuseUnreadableProjectConfig(filepath.Dir(taskFile))
	}
}

// enterTree is a tree mode's gate, called with the task file the mode has
// resolved by its own rule (or, for a review or audit with no task file, the
// spec): an unreadable project config of its project exits 3, and so does a
// task file whose project is not the working directory's. what names the mode
// for the message.
func enterTree(target, what string) {
	if target == "" {
		return
	}
	refuseUnreadableProjectConfig(filepath.Dir(target))
	refuseForeignTree(target, what)
}

// emissionTarget is what a review or audit of specPath works for: the task
// file ResolveWorkflow resolves for it, the rule the command reads its
// workflow by, or the spec itself when no task file resolves. A spec that is
// not there has no project yet; the command's own spec-not-found refusal
// answers for it.
func emissionTarget(specPath string) string {
	if _, err := os.Stat(specPath); err != nil {
		return ""
	}
	if _, taskFile := engine.ResolveWorkflow(specPath, flagFile); taskFile != "" {
		return taskFile
	}
	return specPath
}

// refuseUnreadableProjectConfig exits 3 when the project config discovered
// from start exists but cannot be read or parsed: running as if there were no
// project config would apply the built-in gate and caps to a project that set
// its own.
func refuseUnreadableProjectConfig(start string) {
	err := engine.ProjectConfigError(start)
	if err == nil {
		return
	}
	var mce *engine.MalformedConfigError
	if errors.As(err, &mce) {
		output.Error(ExitFile, mce.Error(), mce.Hint())
	} else {
		output.Error(ExitFile, err.Error(), "repair or remove the task file's project config, .tp/config.json, then run the command again")
	}
	os.Exit(ExitFile)
}

// refuseForeignTree exits 3 when target's project root is not the working
// directory's: the task file (or spec) is one of another project, so what
// would run here — its gate, its checks, its panel — belongs to that tree and
// running it here ran it against this one and exited 0. It is a file error in
// REFERENCE's sense, the file named belongs elsewhere, not an operator
// decision: under tp run exit 2 escalates one. what names the mode.
func refuseForeignTree(target, what string) {
	if target == "" || engine.SameProjectRoot(filepath.Dir(target), ".") {
		return
	}
	root := engine.CanonicalProjectRoot(filepath.Dir(target))
	shown := engine.CanonicalPath(target)
	if rel, err := filepath.Rel(root, shown); err == nil {
		shown = rel
	}
	output.Error(ExitFile,
		fmt.Sprintf("%s works in the working tree, and %s in the project at %s is not this project's", what, shown, root),
		fmt.Sprintf("the file belongs to another project: run it from %s, or name a task file of this project; modes that read only the task file and its state (tp status, tp show, tp plan, tp resume, tp run --status) work from here", root))
	os.Exit(ExitFile)
}
