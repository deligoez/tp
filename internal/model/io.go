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

	var tf TaskFile
	if err := json.Unmarshal(data, &tf); err != nil {
		return nil, fmt.Errorf("parse task file: %w", err)
	}

	// Default empty status to "open"
	for i := range tf.Tasks {
		if tf.Tasks[i].Status == "" {
			tf.Tasks[i].Status = StatusOpen
		}
	}

	return &tf, nil
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
