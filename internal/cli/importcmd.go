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
		now := time.Now().UTC()
		tf = &model.TaskFile{
			Version:   1,
			Spec:      importSpec,
			CreatedAt: now,
			UpdatedAt: now,
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

	// Derive target path from spec field
	base := strings.TrimSuffix(filepath.Base(tf.Spec), filepath.Ext(tf.Spec))
	dir := filepath.Dir(tf.Spec)
	targetPath := filepath.Join(dir, base+".tasks.json")

	// Check if exists
	if _, err := os.Stat(targetPath); err == nil && !importForce {
		output.Error(ExitFile, fmt.Sprintf("task file already exists: %s (use --force to overwrite)", targetPath))
		os.Exit(ExitFile)
		return nil
	}

	// Auto-fill coverage if empty and spec exists
	specPath, specExists := engine.ResolveSpecPath(targetPath, tf.Spec)
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

	output.Success(fmt.Sprintf("imported %d tasks to %s", len(tf.Tasks), targetPath))
	return output.JSON(map[string]any{"imported": len(tf.Tasks), "path": targetPath})
}
