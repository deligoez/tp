package cli

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/output"
)

var (
	version = "dev"

	flagFile    string
	flagJSON    bool
	flagCompact bool
	flagQuiet   bool
	flagNoColor bool
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tp",
		Short: "tp — task-plan: spec-to-task lifecycle for AI agents",
		Long: `tp — spec-to-task lifecycle for AI agents

WORKFLOW (2 calls per session):
  1. tp plan --json          Get full execution plan
  2. [implement each task]
  3. tp done --batch f.ndjson Close all tasks at once

INCREMENTAL (1 task at a time):
  tp next                    Get/resume next task
  [implement]
  tp done <id> "reason"      Close with verification`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringVar(&flagFile, "file", "", "explicit task file (default: auto-detect *.tasks.json)")
	cmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "force JSON output")
	cmd.PersistentFlags().BoolVar(&flagCompact, "compact", false, "minimal JSON: omit descriptions, source_lines, metadata")
	cmd.PersistentFlags().BoolVar(&flagQuiet, "quiet", false, "suppress info-level output")
	cmd.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "disable colored output")

	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
	}
	cmd.Version = version
	cmd.SetVersionTemplate("tp version {{.Version}}\n")

	cmd.PersistentPreRun = func(_ *cobra.Command, _ []string) {
		output.Configure(flagJSON, flagQuiet, flagNoColor)
	}

	// Command groups
	planGroup := &cobra.Group{ID: "plan", Title: "Plan Commands (primary workflow):"}
	stateGroup := &cobra.Group{ID: "state", Title: "Task State Commands:"}
	queryGroup := &cobra.Group{ID: "query", Title: "Query Commands:"}
	dataGroup := &cobra.Group{ID: "data", Title: "Data Commands:"}
	cmd.AddGroup(planGroup, stateGroup, queryGroup, dataGroup)

	// Plan commands
	planCmd := newPlanCmd()
	planCmd.GroupID = "plan"
	doneCmd := newDoneCmd()
	doneCmd.GroupID = "plan"
	nextCmd := newNextCmd()
	nextCmd.GroupID = "plan"

	// State commands
	claimCmd := newClaimCmd()
	claimCmd.GroupID = "state"
	closeCmd := newCloseCmd()
	closeCmd.GroupID = "state"
	reopenCmd := newReopenCmd()
	reopenCmd.GroupID = "state"
	removeCmd := newRemoveCmd()
	removeCmd.GroupID = "state"
	setCmd := newSetCmd()
	setCmd.GroupID = "state"

	// Query commands
	listCmd := newListCmd()
	listCmd.GroupID = "query"
	statusCmd := newStatusCmd()
	statusCmd.GroupID = "query"
	readyCmd := newReadyCmd()
	readyCmd.GroupID = "query"
	blockedCmd := newBlockedCmd()
	blockedCmd.GroupID = "query"
	showCmd := newShowCmd()
	showCmd.GroupID = "query"
	graphCmd := newGraphCmd()
	graphCmd.GroupID = "query"
	statsCmd := newStatsCmd()
	statsCmd.GroupID = "query"
	reportCmd := newReportCmd()
	reportCmd.GroupID = "query"
	lintCmd := newLintCmd()
	lintCmd.GroupID = "query"
	validateCmd := newValidateCmd()
	validateCmd.GroupID = "query"

	// Data commands
	initCmd := newInitCmd()
	initCmd.GroupID = "data"
	addCmd := newAddCmd()
	addCmd.GroupID = "data"
	importCmd := newImportCmd()
	importCmd.GroupID = "data"

	cmd.AddCommand(planCmd, doneCmd, nextCmd)
	cmd.AddCommand(claimCmd, closeCmd, reopenCmd, removeCmd, setCmd)
	cmd.AddCommand(listCmd, statusCmd, readyCmd, blockedCmd, showCmd, graphCmd, statsCmd, reportCmd, lintCmd, validateCmd)
	cmd.AddCommand(initCmd, addCmd, importCmd)

	return cmd
}

func Execute() {
	if os.Getenv("NO_COLOR") != "" {
		flagNoColor = true
	}

	cmd := NewRootCmd()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func GetFileFlag() string    { return flagFile }
func IsJSONOutput() bool     { return flagJSON }
func IsQuiet() bool          { return flagQuiet }
func IsNoColor() bool        { return flagNoColor }
func IsCompact() bool        { return flagCompact }
