package cli

import (
	"errors"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

var (
	version = "dev"

	flagFile      string
	flagJSON      bool
	flagCompact   bool
	flagQuiet     bool
	flagNoColor   bool
	flagNoCompact bool
	flagNoQuiet   bool
	flagColor     bool
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
	cmd.PersistentFlags().BoolVar(&flagNoCompact, "no-compact", false, "force full JSON, overriding a compact default")
	cmd.PersistentFlags().BoolVar(&flagNoQuiet, "no-quiet", false, "force info output, overriding a quiet default")
	cmd.PersistentFlags().BoolVar(&flagColor, "color", false, "force colored output, overriding a no_color default")

	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
	}
	cmd.Version = version
	cmd.SetVersionTemplate("tp version {{.Version}}\n")

	cmd.PersistentPreRun = func(c *cobra.Command, _ []string) {
		applyFlagDefaults(c)
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
	briefCmd := newBriefCmd()
	briefCmd.GroupID = "plan"

	commitCmd := newCommitCmd()
	commitCmd.GroupID = "plan"
	resumeCmd := newResumeCmd()
	resumeCmd.GroupID = "plan"

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
	keepCmd := newKeepCmd()
	keepCmd.GroupID = "state"

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
	reviewCmd := newReviewCmd()
	reviewCmd.GroupID = "query"
	auditCmd := newAuditCmd()
	auditCmd.GroupID = "query"
	validateCmd := newValidateCmd()
	validateCmd.GroupID = "query"

	// Data commands
	initCmd := newInitCmd()
	initCmd.GroupID = "data"
	addCmd := newAddCmd()
	addCmd.GroupID = "data"
	importCmd := newImportCmd()
	importCmd.GroupID = "data"
	useCmd := newUseCmd()
	useCmd.GroupID = "data"

	cmd.AddCommand(planCmd, doneCmd, nextCmd, briefCmd, commitCmd, resumeCmd)
	cmd.AddCommand(claimCmd, closeCmd, reopenCmd, removeCmd, setCmd, keepCmd)
	cmd.AddCommand(listCmd, statusCmd, readyCmd, blockedCmd, showCmd, graphCmd, statsCmd, reportCmd, lintCmd, reviewCmd, auditCmd, validateCmd)
	cmd.AddCommand(initCmd, addCmd, importCmd, useCmd)
	cmd.AddCommand(newConfigCmd())

	return cmd
}

// runtimeFailureHint answers the dispatcher's last-resort exit-1 site. Unlike
// every other hint in this package it cannot name a flag or a path, because the
// error was returned by whichever command cobra ran and Execute knows only that
// it failed. What it can do is stop asserting an object: the code-1 default
// says the task file is at fault, which is wrong for every command that takes a
// spec, an NDJSON or a config file (§9.2).
const runtimeFailureHint = "the error message names the object that failed — check that one; 'tp validate' applies only when it is the task file"

func Execute() {
	if os.Getenv("NO_COLOR") != "" {
		flagNoColor = true
	}

	cmd := NewRootCmd()
	wrapFlagErrors(cmd)
	// §9.1: an unrecognized command or subcommand is a usage error (exit 2).
	// The check runs before dispatch because cobra classifies it itself
	// otherwise — as an exit-1 error at the top level, or as an exit-0 help
	// dump for an unknown subcommand.
	err := unknownCommandUsage(cmd, os.Args[1:])
	if err == nil {
		_, err = cmd.ExecuteC()
	}
	if err != nil {
		// A cobra-level error aborted before PersistentPreRun configured output
		// mode, so set it now (else a piped run would get TTY text, not JSON).
		output.EnsureConfigured(os.Args)
		var fe flagUsageError
		if errors.As(err, &fe) {
			// §13.1: a flag-parse failure is a usage error (exit 2) emitted as the
			// standard tp error object {error, code, hint}, never bare cobra text.
			msg, hint := usageErrorDetail(fe.cmd, fe.err)
			output.Error(ExitUsage, msg, hint)
			os.Exit(ExitUsage)
		}
		// §12.2: write-lock contention that retried past lock_timeout_seconds is a
		// state error (exit 4) with a hint naming the lock path and elapsed wait.
		var lockErr *engine.LockTimeoutError
		if errors.As(err, &lockErr) {
			output.Error(ExitState, lockErr.Error(), lockErr.Hint())
			os.Exit(ExitState)
		}
		// §3.1.1: a second tp run over a task file another run already drives is
		// the same class of answer — the state is not yours — so it exits 4 too.
		// It is a distinct error from the write-lock timeout because its hint
		// must not offer raising lock_timeout_seconds: the holder is a whole
		// run, and no timeout outlasts it.
		var runLockErr *engine.RunLockBusyError
		if errors.As(err, &runLockErr) {
			output.Error(ExitState, runLockErr.Error(), runLockErr.Hint())
			os.Exit(ExitState)
		}
		// Rare: any other RunE-returned error emits as the standard tp error
		// object with exit 1 (validation). The dispatcher cannot name the failing
		// object — the error came from whichever command ran — so it points at
		// the message instead of at the task file the code-1 default assumes
		// (§9.2).
		output.Error(ExitValidation, err.Error(), runtimeFailureHint)
		os.Exit(ExitValidation)
	}
}

func IsJSONOutput() bool { return flagJSON }
func IsCompact() bool    { return flagCompact }
