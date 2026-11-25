package engine

import "github.com/deligoez/tp/internal/model"

// FoundQueueCount is what tp resume says of a spec's queue (spec/1.31.0.md
// §6): how many entries are open, and how many tasks wait on them.
type FoundQueueCount struct {
	Open  int `json:"open"`
	Holds int `json:"holds"`
}

// FoundOpenCount is the number of open entries in the queue beside taskFile:
// the count a converged payload carries. No task file or no queue file counts
// none; a queue file that cannot be read or parsed is ReadFoundQueue's error,
// never an empty queue counted as zero.
func FoundOpenCount(taskFile string) (int, error) {
	if taskFile == "" {
		return 0, nil
	}
	entries, err := ReadFoundQueue(taskFile)
	if err != nil {
		return 0, err
	}
	open := 0
	for i := range entries {
		if entries[i].State == FoundQueueOpen {
			open++
		}
	}
	return open, nil
}

// FoundQueueCounts is tp resume's queue line for the queue beside taskFile,
// nil when no entry is open and FoundOpenCount's error when the queue cannot
// be read. Holds counts the tasks of tf an open entry keeps out of the ready
// set (model.Task.Held through QueueEntry), each once, so it agrees with
// tp ready: a finder already done waits on nothing.
func FoundQueueCounts(taskFile string, tf *model.TaskFile) (*FoundQueueCount, error) {
	open, err := FoundOpenCount(taskFile)
	if err != nil || open == 0 {
		return nil, err
	}
	holds := 0
	for i := range tf.Tasks {
		if tf.Tasks[i].Status == model.StatusOpen && tf.Tasks[i].QueueEntry != "" {
			holds++
		}
	}
	return &FoundQueueCount{Open: open, Holds: holds}, nil
}
