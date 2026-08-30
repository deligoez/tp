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
	runCmd := newRunCmd()
	runCmd.GroupID = "plan"
	escalateCmd := newEscalateCmd()
	escalateCmd.GroupID = "plan"

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

	cmd.AddCommand(planCmd, doneCmd, nextCmd, briefCmd, commitCmd, resumeCmd, runCmd, escalateCmd)
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

// dispatchError classifies an error the command dispatcher returned into the tp
// error contract's (code, message, hint). It is a function rather than a run of
// branches inside Execute because Execute ends in os.Exit: extracted, the
// classification every exit code depends on can be tested directly.
func dispatchError(err error) (code int, msg, hint string) {
	var fe flagUsageError
	if errors.As(err, &fe) {
		// §13.1: a flag-parse failure is a usage error (exit 2) emitted as the
		// standard tp error object {error, code, hint}, never bare cobra text.
		msg, hint = usageErrorDetail(fe.cmd, fe.err)
		return ExitUsage, msg, hint
	}
	// §8a.5: an Args validator's rejection is a usage error (exit 2) for the
	// same reason a flag-parse failure is — tp did not run the request. Its
	// hint names the argument the command wanted, which the code-1 default
	// could not.
	var ae arityUsageError
	if errors.As(err, &ae) {
		return ExitUsage, ae.Error(), arityHint(ae.cmd)
	}
	// §3.2: a runner value that is none of the three shapes — a map missing
	// default, a runner object missing cmd — is a usage error (exit 2), not
	// the exit 4 the run-lock precedent above uses: nothing about the run
	// state is wrong, the configuration simply does not say what to spawn, so
	// neither waiting nor retrying can change the answer. Its own hint names
	// the three shapes, which the code-1 default could not.
	var shapeErr *engine.RunnerShapeError
	if errors.As(err, &shapeErr) {
		return ExitUsage, shapeErr.Error(), shapeErr.Hint()
	}
	// §3.2.1: the same classification for the two failures of the layer above
	// the shapes — a template name that is not one tp ships, and a placeholder
	// the driver cannot resolve. Both are raised before any child is spawned,
	// so the run has done nothing to recover from; its hint lists the names or
	// the placeholders rather than the three shapes.
	var tmplErr *engine.RunnerTemplateError
	if errors.As(err, &tmplErr) {
		return ExitUsage, tmplErr.Error(), tmplErr.Hint()
	}
	// §12.2: write-lock contention that retried past lock_timeout_seconds is a
	// state error (exit 4) with a hint naming the lock path and elapsed wait.
	var lockErr *engine.LockTimeoutError
	if errors.As(err, &lockErr) {
		return ExitState, lockErr.Error(), lockErr.Hint()
	}
	// §3.1.1: a second tp run over a task file another run already drives is
	// the same class of answer — the state is not yours — so it exits 4 too.
	// It is a distinct error from the write-lock timeout because its hint
	// must not offer raising lock_timeout_seconds: the holder is a whole
	// run, and no timeout outlasts it.
	var runLockErr *engine.RunLockBusyError
	if errors.As(err, &runLockErr) {
		return ExitState, runLockErr.Error(), runLockErr.Hint()
	}
	// §3.4: a driver-side fatal error — a runner that will not exec, a run
	// directory or run state file the driver cannot write — is a state error
	// (exit 4) for the same reason. It is checked after the two runner
	// classifications above and never wraps one, so a configuration mistake
	// keeps its exit 2: what separates them is that the run really started
	// and its environment is not in the shape it needs, which is a report to
	// a human rather than a value to correct.
	var driverErr *engine.DriverError
	if errors.As(err, &driverErr) {
		return ExitState, driverErr.Error(), driverErr.Hint()
	}
	// Rare: any other RunE-returned error emits as the standard tp error
	// object with exit 1 (validation). The dispatcher cannot name the failing
	// object — the error came from whichever command ran — so it points at
	// the message instead of at the task file the code-1 default assumes
	// (§9.2).
	return ExitValidation, err.Error(), runtimeFailureHint
}

func Execute() {
	if os.Getenv("NO_COLOR") != "" {
		flagNoColor = true
	}

	cmd := NewRootCmd()
	wrapFlagErrors(cmd)
	// §8a.5: cobra registers help and completion (and its per-shell
	// subcommands, which carry cobra.NoArgs) lazily inside ExecuteC, so they
	// are put on the tree here — before wrapArityErrors walks it, which is the
	// only way their built-in validators are covered too. Both initializers
	// are idempotent, so cobra re-running them changes nothing.
	cmd.InitDefaultHelpCmd()
	cmd.InitDefaultCompletionCmd(os.Args[1:]...)
	wrapArityErrors(cmd)
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
		code, msg, hint := dispatchError(err)
		output.Error(code, msg, hint)
		os.Exit(code)
	}
}

func IsJSONOutput() bool { return flagJSON }
func IsCompact() bool    { return flagCompact }
