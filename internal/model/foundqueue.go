package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// FoundQueueOpen is the state of a queue entry that awaits the person
// (spec/1.31.0.md §6). An open entry holds its finders out of the ready set.
const FoundQueueOpen = "open"

// FoundQueuePath returns the queue file beside a spec's task file:
// <base>.tasks.json holds its queue in <base>.queue.json. Unlike the found
// request store it is committed, so it survives the machine, and its name does
// not match *.tasks.json, so discovery never takes it for a task file.
func FoundQueuePath(taskFile string) string {
	return strings.TrimSuffix(taskFile, ".tasks.json") + ".queue.json"
}

// markQueueHolds sets QueueEntry on every task an open entry of the queue
// beside taskFile names among its finders (spec/1.31.0.md §4): the finder of a
// blocking request with no task outcome waits, open and not ready, until its
// entry leaves the open state. Only the finders are held; every other task
// reads as it would without the queue. No queue file holds nothing.
func markQueueHolds(tf *TaskFile, taskFile string) error {
	data, err := os.ReadFile(FoundQueuePath(taskFile)) //nolint:gosec // the queue beside the task file being read
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read found queue: %w", err)
	}
	var q struct {
		Entries []struct {
			ID      string   `json:"id"`
			State   string   `json:"state"`
			Finders []string `json:"finders"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(data, &q); err != nil {
		return fmt.Errorf("parse found queue: %w", err)
	}
	holds := map[string]string{}
	for _, e := range q.Entries {
		if e.State != FoundQueueOpen {
			continue
		}
		for _, f := range e.Finders {
			if _, held := holds[f]; !held {
				holds[f] = e.ID
			}
		}
	}
	for i := range tf.Tasks {
		tf.Tasks[i].QueueEntry = holds[tf.Tasks[i].ID]
	}
	return nil
}
