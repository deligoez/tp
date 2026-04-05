package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

var validResolveStatuses = map[string]bool{
	"fixed":     true,
	"wontfix":   true,
	"duplicate": true,
}

// runReviewResolve marks a single finding as resolved.
// args: [file, index, status, evidence?]
func runReviewResolve(args []string, force bool) error {
	if len(args) < 3 {
		output.Error(ExitUsage, "usage: tp review --resolve <file> <index> <status> [evidence]")
		os.Exit(ExitUsage)
		return nil
	}

	filePath := args[0]
	indexStr := args[1]
	status := args[2]
	evidence := ""
	if len(args) >= 4 {
		evidence = args[3]
	}

	// Validate status
	if !validResolveStatuses[status] {
		output.Error(ExitUsage, fmt.Sprintf("invalid status: %s (must be fixed, wontfix, or duplicate)", status))
		os.Exit(ExitUsage)
		return nil
	}

	// Validate index
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		output.Error(ExitUsage, fmt.Sprintf("invalid index %q: must be an integer", indexStr))
		os.Exit(ExitUsage)
		return nil
	}

	// Check file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		output.Error(ExitFile, fmt.Sprintf("findings file not found: %s", filePath))
		os.Exit(ExitFile)
		return nil
	}

	var findings []map[string]any

	lockErr := engine.WithFileLock(filePath, func() error {
		// Read findings
		var readErr error
		findings, readErr = readNDJSON(filePath)
		if readErr != nil {
			return readErr
		}

		// Validate index range
		if index < 0 || index >= len(findings) {
			output.Error(ExitUsage, fmt.Sprintf("finding index %d out of range (0-%d)", index, len(findings)-1))
			os.Exit(ExitUsage)
			return nil
		}

		finding := findings[index]

		// Check already resolved
		if existingResolved, ok := finding["resolved"]; ok && !force {
			currentStatus := "unknown"
			if rm, ok := existingResolved.(map[string]any); ok {
				if s, ok := rm["status"].(string); ok {
					currentStatus = s
				}
			}
			output.Error(ExitValidation, fmt.Sprintf("finding %d already resolved as %s", index, currentStatus), "use --force to re-resolve")
			os.Exit(ExitValidation)
			return nil
		}

		// Apply resolution
		finding["resolved"] = map[string]any{
			"status":      status,
			"evidence":    evidence,
			"resolved_at": time.Now().UTC().Format(time.RFC3339),
		}

		// Write back
		return writeNDJSON(filePath, findings)
	})

	if lockErr != nil {
		output.Error(ExitFile, lockErr.Error())
		os.Exit(ExitFile)
		return nil
	}

	// Success output
	fmt.Fprintf(os.Stderr, "resolved finding %d as %s\n", index, status)
	result := map[string]any{
		"index":    index,
		"status":   status,
		"evidence": evidence,
		"file":     filePath,
	}

	// Check if all findings are now resolved — if so, include next_step
	if allFindingsResolved(findings) {
		result["next_step"] = fmt.Sprintf("tp review --verify <spec> --findings %s", filePath)
	}

	return output.JSON(result)
}

// runReviewResolveAll marks all unresolved findings with a status.
// args: [file, status, evidence?]
func runReviewResolveAll(args []string, force bool) error {
	if len(args) < 2 {
		output.Error(ExitUsage, "usage: tp review --resolve-all <file> <status> [evidence]")
		os.Exit(ExitUsage)
		return nil
	}

	filePath := args[0]
	status := args[1]
	evidence := ""
	if len(args) >= 3 {
		evidence = args[2]
	}

	// Validate status
	if !validResolveStatuses[status] {
		output.Error(ExitUsage, fmt.Sprintf("invalid status: %s (must be fixed, wontfix, or duplicate)", status))
		os.Exit(ExitUsage)
		return nil
	}

	// Check file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		output.Error(ExitFile, fmt.Sprintf("file not found: %s", filePath))
		os.Exit(ExitFile)
		return nil
	}

	resolvedCount := 0
	skippedCount := 0

	lockErr := engine.WithFileLock(filePath, func() error {
		// Read findings
		findings, readErr := readNDJSON(filePath)
		if readErr != nil {
			return readErr
		}

		now := time.Now().UTC().Format(time.RFC3339)

		for _, finding := range findings {
			if _, ok := finding["resolved"]; ok {
				if force {
					finding["resolved"] = map[string]any{
						"status":      status,
						"evidence":    evidence,
						"resolved_at": now,
					}
					resolvedCount++
				} else {
					skippedCount++
				}
			} else {
				finding["resolved"] = map[string]any{
					"status":      status,
					"evidence":    evidence,
					"resolved_at": now,
				}
				resolvedCount++
			}
		}

		// Write back
		return writeNDJSON(filePath, findings)
	})

	if lockErr != nil {
		output.Error(ExitFile, lockErr.Error())
		os.Exit(ExitFile)
		return nil
	}

	// Success output
	fmt.Fprintf(os.Stderr, "resolved %d findings as %s (%d already resolved, skipped)\n", resolvedCount, status, skippedCount)
	return output.JSON(map[string]any{
		"resolved_count": resolvedCount,
		"skipped_count":  skippedCount,
		"status":         status,
		"file":           filePath,
		"next_step":      fmt.Sprintf("tp review --verify <spec> --findings %s", filePath),
	})
}

// allFindingsResolved checks if all findings in the slice have a "resolved" field.
func allFindingsResolved(findings []map[string]any) bool {
	for _, f := range findings {
		if _, ok := f["resolved"]; !ok {
			return false
		}
	}
	return true
}

// readNDJSON reads a file as newline-delimited JSON into a slice of maps.
func readNDJSON(path string) ([]map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	findings := make([]map[string]any, 0)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	lineNum := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lineNum++
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			return nil, fmt.Errorf("line %d: invalid JSON: %w", lineNum, err)
		}
		findings = append(findings, m)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return findings, nil
}

// writeNDJSON writes a slice of maps as newline-delimited JSON to a file.
func writeNDJSON(path string, findings []map[string]any) error {
	var buf strings.Builder
	for _, f := range findings {
		data, err := json.Marshal(f)
		if err != nil {
			return fmt.Errorf("marshal finding: %w", err)
		}
		buf.Write(data)
		buf.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(buf.String()), 0o644)
}
