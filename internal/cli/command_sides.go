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

// The side of a command, for spec/backlog/project-layer-from-task-file.md: the
// project layer is resolved from the task file, and a command whose work
// happens in the working tree is refused when the task file's project root is
// not the working directory's. What reads only the task file and its state is
// allowed from outside.
const (
	// sideTree is work in the working tree, by writing to it or by sending
	// agents to read it: the quality gate, a registered check, a commit, a
	// review or audit emission, a run.
	sideTree = "tree"
	// sideFile reads or writes only the task file and its state.
	sideFile = "file"
	// sideNone takes no task file, so discovery is the working directory's.
	sideNone = "none"
)

// commandSides places every command. TestEveryCommandHasASide enumerates the
// command tree and fails on a command missing here or an entry no command has,
// so a new command has to be placed rather than defaulting to a side.
var commandSides = map[string]string{
	"tp audit":  sideTree,
	"tp close":  sideTree,
	"tp commit": sideTree,
	"tp done":   sideTree,
	"tp import": sideTree,
	"tp review": sideTree,
	"tp run":    sideTree,

	"tp add":      sideFile,
	"tp blocked":  sideFile,
	"tp brief":    sideFile,
	"tp claim":    sideFile,
	"tp config":   sideFile,
	"tp export":   sideFile,
	"tp graph":    sideFile,
	"tp init":     sideFile,
	"tp list":     sideFile,
	"tp next":     sideFile,
	"tp plan":     sideFile,
	"tp ready":    sideFile,
	"tp remove":   sideFile,
	"tp reopen":   sideFile,
	"tp report":   sideFile,
	"tp resume":   sideFile,
	"tp set":      sideFile,
	"tp show":     sideFile,
	"tp stats":    sideFile,
	"tp status":   sideFile,
	"tp unclaim":  sideFile,
	"tp validate": sideFile,

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

// projectLayerReaders are the task-file commands outside sideTree that resolve
// the workflow from the project config; with the tree side they are the paths
// on which an unreadable config is an error.
var projectLayerReaders = map[string]bool{
	"tp brief": true, "tp config": true, "tp init": true, "tp next": true,
	"tp plan": true, "tp resume": true, "tp set": true,
}

// checkTaskFileProject runs before every command. For a command that takes a
// task file it refuses an unreadable project config of the task file's own
// project on the paths that use it, and refuses work in the working tree when
// the task file belongs to another project. A command no discovery finds a
// task file for is left to its own handling.
func checkTaskFileProject(c *cobra.Command) {
	path := c.CommandPath()
	side := commandSides[path]
	if side == "" || side == sideNone {
		return
	}
	taskFile, err := engine.DiscoverTaskFile(".", flagFile)
	if err != nil {
		return
	}
	if side == sideTree || projectLayerReaders[path] {
		refuseUnreadableProjectConfig(filepath.Dir(taskFile))
	}
	if side == sideTree {
		refuseForeignTree(taskFile, path)
	}
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

// refuseForeignTree exits 2 when taskFile's project root is not the working
// directory's: what would run here belongs to the task file's project, and
// running it here ran another project's gate, checks or panel against this
// tree and exited 0. what names the work for the message. A task file of this
// project, or none, passes.
func refuseForeignTree(taskFile, what string) {
	if taskFile == "" || engine.SameProjectRoot(filepath.Dir(taskFile), ".") {
		return
	}
	root := engine.CanonicalProjectRoot(filepath.Dir(taskFile))
	output.Error(ExitUsage,
		fmt.Sprintf("%s works in the working tree, and %s belongs to the project at %s, not to this one", what, taskFile, root),
		fmt.Sprintf("run it from %s, or name a task file of this project; reading the task file and its state (tp status, tp resume, tp show) works from here", root))
	os.Exit(ExitUsage)
}
