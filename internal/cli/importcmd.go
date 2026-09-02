package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

var (
	importForce bool
	importSpec  string
)

func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import a complete task file (validates, then writes)",
		Args:  cobra.ExactArgs(1),
		RunE:  runImport,
	}
	cmd.Flags().BoolVar(&importForce, "force", false, "overwrite existing task file")
	cmd.Flags().StringVar(&importSpec, "spec", "", "spec path (required for bare JSON arrays)")
	return cmd
}

func runImport(_ *cobra.Command, args []string) error {
	// §5.1: --force bypasses the convergence guard and overwrites a task file
	// with real tasks in it, which is a user-approved decision.
	if importForce && engine.Unattended() {
		refuseUnattended("tp import --force", "import-force")
		return nil
	}

	data, err := os.ReadFile(args[0])
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		output.Error(ExitFile, "empty file")
		os.Exit(ExitFile)
		return nil
	}

	var tf *model.TaskFile

	switch trimmed[0] {
	case '[':
		// Bare JSON array — wrap into TaskFile
		if importSpec == "" {
			output.Error(ExitUsage, "bare JSON array detected; use --spec <path> to specify the spec file",
				`or wrap in TaskFile format: {"version":1,"spec":"...","tasks":[...]}`)
			os.Exit(ExitUsage)
			return nil
		}
		var tasks []model.Task
		if err := json.Unmarshal(trimmed, &tasks); err != nil {
			output.Error(ExitFile, fmt.Sprintf("invalid JSON array: %v", err))
			os.Exit(ExitFile)
			return nil
		}
		// Default empty status to "open", mirroring model.ReadTaskFile
		for i := range tasks {
			if tasks[i].Status == "" {
				tasks[i].Status = model.StatusOpen
			}
		}
		now := time.Now().UTC()
		tf = &model.TaskFile{
			Version:   1,
			Spec:      importSpec,
			CreatedAt: now,
			UpdatedAt: now,
			Workflow:  model.WorkflowOverride{},
			Tasks:     tasks,
		}
	case '{':
		// Full TaskFile format (existing behavior)
		var err error
		tf, err = model.ReadTaskFile(args[0])
		if err != nil {
			// Check if this might be NDJSON
			lines := bytes.Split(trimmed, []byte("\n"))
			ndjsonLines := 0
			for _, l := range lines {
				l = bytes.TrimSpace(l)
				if len(l) > 0 && l[0] == '{' {
					ndjsonLines++
				}
			}
			if ndjsonLines >= 2 {
				output.Error(ExitFile, fmt.Sprintf("invalid task file: %v; if this is NDJSON, use tp add --bulk", err))
			} else {
				output.Error(ExitFile, err.Error())
			}
			os.Exit(ExitFile)
			return nil
		}
	default:
		output.Error(ExitFile, "invalid JSON: expected '[' or '{'")
		os.Exit(ExitFile)
		return nil
	}

	// Override spec from --spec flag if provided (even for TaskFile format)
	if importSpec != "" && tf.Spec == "" {
		tf.Spec = importSpec
	}

	// Entry validation (§6.1): the same per-task rules tp add applies, shared
	// via engine.ValidateTaskEntry. Runs before convergence/file-exists guards
	// so a malformed task is rejected up front with a field-specific hint. The
	// deeper checks (atomicity, cycles, coverage) still run via engine.Validate
	// below. Duplicate-id detection stays with engine.Validate's validateUniqueness.
	resolvable := make(map[string]bool, len(tf.Tasks))
	for i := range tf.Tasks {
		resolvable[tf.Tasks[i].ID] = true
	}
	for i := range tf.Tasks {
		if f := engine.ValidateTaskEntry(&tf.Tasks[i], resolvable); f != nil {
			output.Error(ExitValidation, f.Msg, f.Hint)
			os.Exit(ExitValidation)
			return nil
		}
	}

	// Derive target path from spec field
	base := strings.TrimSuffix(filepath.Base(tf.Spec), filepath.Ext(tf.Spec))
	dir := filepath.Dir(tf.Spec)
	targetPath := filepath.Join(dir, base+".tasks.json")

	// §3: the whole read-modify-write runs under the task-file write lock, on
	// the same terms as every other write command. WriteTaskFile is atomic, but
	// atomicity is not mutual exclusion — an unlocked import racing a
	// concurrent add lost the add's task in 1 of 20 runs, both processes
	// exiting 0. engine.WithFileLock honours the resolved
	// lock_timeout_seconds; a lock held past it returns *LockTimeoutError,
	// which Execute maps to exit 4 (STATE) with a hint naming the lock path and
	// the elapsed wait. The success path is unchanged.
	if lockErr := engine.WithFileLock(targetPath, func() error {
		// Workflow preservation (§9.3): when the target exists and the imported
		// document carries no top-level workflow key (raw-JSON key check, before
		// struct defaulting), the existing file's workflow block is carried over.
		// Bare arrays cannot carry workflow, so they always preserve.
		if _, statErr := os.Stat(targetPath); statErr == nil && !importedHasWorkflowKey(trimmed) {
			if existing, readErr := model.ReadTaskFile(targetPath); readErr == nil {
				tf.Workflow = existing.Workflow
			}
		}

		// Import convergence enforcement (§9.1/§9.2); runs before the file-exists
		// guard so a stale or unconverged spec blocks with exit 1 even when the
		// target already holds tasks. --force bypasses both checks.
		if !importForce {
			enforceImportConvergence(targetPath, tf)
		}

		// Check if exists — a zero-task init shell may be overwritten without
		// --force; --force is reserved for overwriting a file with real tasks
		if _, err := os.Stat(targetPath); err == nil && !importForce {
			existing, readErr := model.ReadTaskFile(targetPath)
			if readErr != nil || len(existing.Tasks) > 0 {
				output.Error(ExitFile, fmt.Sprintf("task file already exists: %s (use --force to overwrite)", targetPath))
				os.Exit(ExitFile)
				return nil
			}
		}

		// v0.37.0 §3's change rule. It runs after the exists-guard above and
		// not before it, which §7 row 13b pins: over a tasks-bearing target
		// tp import exits 3 there, so only a zero-task shell reaches the fence
		// — and an import row built over a populated target would pass green
		// with no fence built at all.
		//
		// tf.Workflow is read after the preservation step above has decided
		// it, so a document omitting the top-level workflow key is compared as
		// the carried-forward block it will actually become rather than as the
		// empty one it parsed to.
		fenceAuditConvergeOnImport(targetPath, &tf.Workflow)

		// Resolve spec, normalize source_sections to canonical form (lenient — accepts
		// plain-text headings from tp lint output), then auto-fill coverage.
		specPath, specExists := engine.ResolveSpecPath(targetPath, tf.Spec)
		if specExists {
			headings, perr := engine.ParseHeadings(specPath)
			if perr == nil && len(headings) > 0 {
				if nerr := engine.NormalizeSourceSections(tf.Tasks, headings); nerr != nil {
					output.Error(ExitValidation, nerr.Error())
					os.Exit(ExitValidation)
					return nil
				}
			}
		}
		if specExists && tf.Coverage.TotalSections == 0 {
			engine.AutoFillCoverage(tf, specPath)
		}

		// Validate (strict atomicity unless --force)
		result := engine.Validate(tf, specPath, specExists, !importForce)
		if !result.Valid {
			if err := output.JSON(result); err != nil {
				output.Error(ExitFile, err.Error())
			}
			output.Error(ExitValidation, "import failed: validation errors found")
			os.Exit(ExitValidation)
			return nil
		}

		if err := model.WriteTaskFile(targetPath, tf); err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}
		return nil
	}); lockErr != nil {
		return lockErr
	}

	output.Success(fmt.Sprintf("imported %d tasks to %s", len(tf.Tasks), targetPath))
	return output.JSON(map[string]any{"imported": len(tf.Tasks), "path": targetPath})
}
