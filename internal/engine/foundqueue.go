package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Queue entry states (spec/1.31.0.md §6). An open entry awaits the person; a
// decided one is a conforms answer, kept so it stays auditable.
const (
	FoundQueueOpen    = "open"
	FoundQueueDecided = "decided"
)

// FoundQueueEntry is one entry of a spec's queue (spec/1.31.0.md §6): the
// request it came from, its location, question and alternatives, the finders
// waiting on it, and for a decided entry the answer that decided it.
type FoundQueueEntry struct {
	ID           string   `json:"id"`
	State        string   `json:"state"`
	Request      string   `json:"request,omitempty"`
	Location     string   `json:"location"`
	Question     string   `json:"question,omitempty"`
	Alternatives []string `json:"alternatives,omitempty"`
	// Reproduction, Expected and Observed carry a Fix-shaped request queued
	// past the cap (spec/1.31.0.md §8): the request store is not committed,
	// so the entry holds the defect itself.
	Reproduction string   `json:"reproduction,omitempty"`
	Expected     string   `json:"expected,omitempty"`
	Observed     string   `json:"observed,omitempty"`
	Finders      []string `json:"finders,omitempty"`
	Answer       string   `json:"answer,omitempty"`
	// Cited and Quote are a decision answer's evidence (spec/1.31.0.md §5):
	// the file it cited and the quote tp found there.
	Cited string `json:"cited,omitempty"`
	Quote string `json:"quote,omitempty"`
}

// foundQueueFile is the queue file's shape.
type foundQueueFile struct {
	Entries []FoundQueueEntry `json:"entries"`
}

// FoundQueuePath returns the queue file beside a spec's task file:
// <base>.tasks.json holds its queue in <base>.queue.json. Unlike the found
// request store it is committed, so it survives the machine, and its name does
// not match *.tasks.json, so discovery never takes it for a task file.
func FoundQueuePath(taskFile string) string {
	return strings.TrimSuffix(taskFile, ".tasks.json") + ".queue.json"
}

// ReadFoundQueue returns a spec's queue in file order, or none when the file
// does not exist. Readers take no lock: writes are atomic renames.
func ReadFoundQueue(taskFile string) ([]FoundQueueEntry, error) {
	data, err := os.ReadFile(FoundQueuePath(taskFile))
	if errors.Is(err, os.ErrNotExist) {
		return []FoundQueueEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	var q foundQueueFile
	if unmarshalErr := json.Unmarshal(data, &q); unmarshalErr != nil {
		return nil, fmt.Errorf("parse found queue: %w", unmarshalErr)
	}
	if q.Entries == nil {
		q.Entries = []FoundQueueEntry{}
	}
	return q.Entries, nil
}

// UpdateFoundQueue runs fn over a spec's queue under the task-file lock and
// writes back what it returns. The lock is the task file's because settling an
// entry writes a task into that file in the same step. An entry fn returns
// without an id gets the next Q<n>, and one without a state is open.
// A caller that already holds the task-file lock must not call this.
func UpdateFoundQueue(taskFile string, fn func([]FoundQueueEntry) ([]FoundQueueEntry, error)) error {
	return WithFileLock(taskFile, func() error {
		entries, err := ReadFoundQueue(taskFile)
		if err != nil {
			return err
		}
		next, err := fn(entries)
		if err != nil {
			return err
		}
		return WriteFoundQueue(taskFile, next)
	})
}

// WriteFoundQueue numbers entries and writes them as the spec's queue. The
// caller holds the task-file lock.
func WriteFoundQueue(taskFile string, entries []FoundQueueEntry) error {
	numberQueueEntries(entries)
	data, err := json.MarshalIndent(foundQueueFile{Entries: entries}, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(FoundQueuePath(taskFile), append(data, '\n'))
}

// numberQueueEntries gives each entry without an id the next Q<n> after the
// highest one present, and each entry without a state the open state.
func numberQueueEntries(entries []FoundQueueEntry) {
	last := 0
	for i := range entries {
		if n, err := strconv.Atoi(strings.TrimPrefix(entries[i].ID, "Q")); err == nil && strings.HasPrefix(entries[i].ID, "Q") {
			last = max(last, n)
		}
	}
	for i := range entries {
		if entries[i].ID == "" {
			last++
			entries[i].ID = "Q" + strconv.Itoa(last)
		}
		if entries[i].State == "" {
			entries[i].State = FoundQueueOpen
		}
	}
}
