package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// releaseRecordInboxConflicts is every tp audit flag --record-inbox without
// --task-audit refuses: it takes the spec, --inbox and --harness-note.
var releaseRecordInboxConflicts = []string{
	"affected-files", "affected-from-tasks", "base", "findings", "record", "status",
	"check", "merge", "resolve", "resolve-all", "force", "output", "role",
}

// runReleaseAuditRecordInbox routes tp audit <spec> --record-inbox.
func runReleaseAuditRecordInbox(cmd *cobra.Command, args []string) error {
	for _, name := range releaseRecordInboxConflicts {
		if cmd.Flags().Changed(name) {
			usageExit("--record-inbox cannot be combined with --"+name, "--record-inbox records every role of the emitted round; it takes only the spec, --inbox and --harness-note")
		}
	}
	if len(args) != 1 {
		usageExit("spec path required", "")
	}
	specPath := engine.ResolveSpecArg(args[0])
	if _, err := os.Stat(specPath); err != nil {
		output.Error(ExitFile, fmt.Sprintf("spec not found: %s", specPath), specFileMissingHint)
		os.Exit(ExitFile)
	}
	enterTree(emissionTarget(specPath), "tp audit")
	note, _ := cmd.Flags().GetString("harness-note")
	return runAuditRecordInbox(specPath, note)
}

// runAuditRecordInbox records the emitted release audit round from its roles'
// files (spec/backlog/audit-round-ergonomics.md §2.3): each emitted role's
// whole file and part files (engine.RoleInboxFiles), read and deduplicated as
// tp audit --merge reads them, every row stamped with its file's role, then
// recorded as --record records a file. It refuses, recording nothing, a round
// with no emission to read the prompted roles from and a prompted role with
// no file (exit 3), and a line the reader skips or a row naming another role
// than its file's (exit 1).
func runAuditRecordInbox(specPath, harnessNote string) error {
	if os.Getenv(engine.EnvRound) != "" {
		exitUsage("--record-inbox records the next round by hand and does not rewrite a recorded one", "unset "+engine.EnvRound+", or record a file with --record <file>")
	}
	round := nextAuditRound(specPath)
	pools, found, err := engine.ReadAuditPools(specPath, round)
	emit := "tp audit " + specPath
	if err != nil || !found {
		output.Error(ExitFile, fmt.Sprintf("audit round %d of %s has no emission to record from", round, specPath),
			"emit the round first with "+emit+", have each role write its prompt's output_path, then "+emit+" --record-inbox")
		os.Exit(ExitFile)
	}
	files := make(map[string][]string, len(pools))
	var missing []string
	owed := 0
	for _, p := range pools {
		if p.Unprompted {
			continue // asked nothing, so it owes no file
		}
		owed++
		files[p.Role] = engine.RoleInboxFiles(namedInbox(), specPath, engine.PhaseAudit, round, p.Role)
		if len(files[p.Role]) == 0 {
			missing = append(missing, p.Role)
		}
	}
	if len(missing) > 0 {
		roles := strings.Join(missing, ", ")
		hint := fmt.Sprintf("run the prompts of roles %s (%s --role <role>), each writing its file, then %s --record-inbox; a round emitted with --inbox <dir> is recorded with the same --inbox <dir>", roles, emit, emit)
		if len(missing) == owed {
			if flat := flatInboxHint(specPath, engine.PhaseAudit, round, emit); flat != "" {
				hint = flat
			}
		}
		output.Error(ExitFile, fmt.Sprintf("audit round %d cannot be recorded from the inbox %s: no results file from roles %s", round, absPath(engine.RoleInboxDirAt(namedInbox(), specPath, engine.PhaseAudit)), roles), hint)
		os.Exit(ExitFile)
	}
	data, conflicts, problems := auditInboxRows(pools, files)
	if len(problems) > 0 {
		output.Error(ExitValidation, fmt.Sprintf("audit round %d's inbox cannot be recorded: %s", round, strings.Join(problems, "; ")),
			"fix the named lines in the role files, then "+emit+" --record-inbox")
		os.Exit(ExitValidation)
	}
	merged := engine.AuditInboxMergedPath(specPath, round)
	if err := os.WriteFile(merged, data, 0o600); err != nil {
		output.Error(ExitFile, err.Error(), runtimeFailureHint)
		os.Exit(ExitFile)
	}
	if err := runAuditRecord(specPath, merged, harnessNote, namedInbox(), conflicts); err != nil {
		return err
	}
	removeNamedInboxFiles(files)
	return nil
}

// nextAuditRound is the audit round a record would write: one past the
// rounds recorded. A state index that is only absent, as before a spec's
// first round, counts none.
func nextAuditRound(specPath string) int {
	st, err := engine.LoadReviewState(specPath)
	if err != nil && !engine.IsRebuildableStateIndex(err) {
		exitStateError(err)
	}
	if st == nil {
		return 1
	}
	return len(st.AuditRounds) + 1
}

// auditInboxRows reads every prompted role's files through the reader
// tp audit --merge applies, stamps each row with its file's role, and keeps
// one row of each agreeing (role, item_id) group as --merge does; conflicts
// are the groups whose rows disagree, every row of them kept.
func auditInboxRows(pools []engine.AuditRolePool, files map[string][]string) (data []byte, conflicts []auditConflict, problems []string) {
	var rows []map[string]any
	for _, p := range pools {
		for _, path := range files[p.Role] {
			read, counts := loadMergeRows([]string{path}, auditMergeRule)
			for _, c := range counts {
				for _, s := range c.skips {
					problems = append(problems, fmt.Sprintf("%s (role %s) %s: %s", filepath.Base(path), p.Role, lineList(s.lines), s.reason))
				}
			}
			for i, row := range read {
				if named, _ := row["role"].(string); named != "" && named != p.Role {
					problems = append(problems, fmt.Sprintf("%s row %d names role %q, but the file is role %s's", filepath.Base(path), i, named, p.Role))
					continue
				}
				row["role"] = p.Role
				rows = append(rows, row)
			}
		}
	}
	unique, conflicts := dedupAuditRows(rows)
	var buf bytes.Buffer
	for _, row := range unique {
		line, err := json.Marshal(row)
		if err != nil {
			problems = append(problems, fmt.Sprintf("role %v item %v: %v", row["role"], row["item_id"], err))
			continue
		}
		buf.Write(append(line, '\n'))
	}
	return buf.Bytes(), conflicts, problems
}
