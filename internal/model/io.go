package model

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// ReadTaskFile reads and parses a .tasks.json file.
func ReadTaskFile(path string) (*TaskFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read task file: %w", err)
	}

	// First pass: extract version with lenient parsing (accepts int, string, or missing).
	var raw struct {
		Version json.RawMessage `json:"version"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse task file: %w", err)
	}

	version := 0
	if len(raw.Version) > 0 {
		// Try int first
		if err := json.Unmarshal(raw.Version, &version); err != nil {
			// Try string (e.g., "0.16.0" or "1")
			var vs string
			if err2 := json.Unmarshal(raw.Version, &vs); err2 != nil {
				return nil, fmt.Errorf("parse task file: version must be an integer or string, got %s", string(raw.Version))
			}
			// String version: default to 1 (version field is a schema version, not a release version)
			version = 1
		}
	}

	// Second pass: unmarshal into TaskFile, but skip the version field conflict
	// by temporarily using a type alias that makes version flexible.
	type taskFileAlias TaskFile
	var tf taskFileAlias
	if err := json.Unmarshal(data, &tf); err != nil {
		// If it fails due to version type mismatch, retry without version
		var rawMap map[string]json.RawMessage
		if err2 := json.Unmarshal(data, &rawMap); err2 != nil {
			return nil, fmt.Errorf("parse task file: %w", err)
		}
		// Remove version and re-marshal for clean parse
		delete(rawMap, "version")
		cleaned, _ := json.Marshal(rawMap)
		if err3 := json.Unmarshal(cleaned, &tf); err3 != nil {
			return nil, fmt.Errorf("parse task file: %w", err3)
		}
	}

	result := TaskFile(tf)
	result.Version = version

	// Default version to 1 when missing
	if result.Version == 0 {
		result.Version = 1
	}

	// Default empty status to "open"
	for i := range result.Tasks {
		if result.Tasks[i].Status == "" {
			result.Tasks[i].Status = StatusOpen
		}
	}

	return &result, nil
}

// WriteTaskFile writes a TaskFile to disk as pretty-printed JSON.
func WriteTaskFile(path string, tf *TaskFile) error {
	tf.UpdatedAt = time.Now().UTC()

	data, err := json.MarshalIndent(tf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal task file: %w", err)
	}

	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write task file: %w", err)
	}

	return nil
}

// FindTask returns the task with the given ID and its index, or an error.
func FindTask(tf *TaskFile, id string) (*Task, int, error) {
	for i := range tf.Tasks {
		if tf.Tasks[i].ID == id {
			return &tf.Tasks[i], i, nil
		}
	}
	return nil, -1, fmt.Errorf("task not found: %s", id)
}
