package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"time"
)

// Found request states (spec/1.31.0.md §1-§2). A request is filed unapplied;
// tp found apply turns it applied once it has an outcome, or undecided while it
// waits on the decision unit.
const (
	FoundStateUnapplied = "unapplied"
	FoundStateApplied   = "applied"
	FoundStateUndecided = "undecided"
)

// FoundFinder names who filed a request: the task a unit worked, or the audit
// round and row key an audit-role unit reported from (spec/1.31.0.md §3).
type FoundFinder struct {
	Task       string `json:"task,omitempty"`
	AuditRound int    `json:"audit_round,omitempty"`
	RowKey     string `json:"row_key,omitempty"`
}

// FoundRequest is one defect a unit ran into outside its own task
// (spec/1.31.0.md §1): the four Fix fields, the question and alternatives a
// request that needs a choice carries, whether the finder's task cannot pass
// without it, and the bookkeeping the store assigns — id, arrival order, filing
// time and state.
type FoundRequest struct {
	ID           string      `json:"id"`
	Seq          int         `json:"seq"`
	Location     string      `json:"location"`
	Reproduction string      `json:"reproduction"`
	Expected     string      `json:"expected,omitempty"`
	Observed     string      `json:"observed,omitempty"`
	Question     string      `json:"question,omitempty"`
	Alternatives []string    `json:"alternatives,omitempty"`
	Blocks       bool        `json:"blocks,omitempty"`
	Finder       FoundFinder `json:"finder"`
	FiledAt      string      `json:"filed_at"`
	State        string      `json:"state"`
}

// foundStore is the store file's shape.
type foundStore struct {
	Requests []FoundRequest `json:"requests"`
}

// FoundRequestsPath returns a cycle's found request store: part of the spec's
// run state under .tp/, keyed by CycleKey like RunStatePath, and not the task
// file (a request changes no task). It is a sibling of run-<key>.json rather
// than a field of it, because the driver rewrites that file whole from memory
// and replaces it on every new run, while requests are filed by child
// processes and must outlive a restart.
//
// The name still matches run-*.json, so it is git-ignored and a .tp/ holding
// only it stays a side-effect directory (isProjectTPDir). The "%found" suffix
// cannot be another cycle's run state: CycleKey is path-escaped, so a "%" in a
// key is always followed by two hex digits, never by "fo".
func FoundRequestsPath(root, taskFile string) string {
	return absoluteUnder(root, filepath.Join(".tp", "run-"+CycleKey(root, taskFile)+"%found.json"))
}

// ReadFoundRequests returns a cycle's requests in arrival order, or none when
// no request was ever filed. Readers take no lock: writes are atomic renames.
func ReadFoundRequests(root, taskFile string) ([]FoundRequest, error) {
	return readFoundStore(FoundRequestsPath(root, taskFile))
}

func readFoundStore(path string) ([]FoundRequest, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []FoundRequest{}, nil
	}
	if err != nil {
		return nil, err
	}
	var st foundStore
	if unmarshalErr := json.Unmarshal(data, &st); unmarshalErr != nil {
		return nil, fmt.Errorf("parse found requests: %w", unmarshalErr)
	}
	if st.Requests == nil {
		st.Requests = []FoundRequest{}
	}
	return st.Requests, nil
}

// AddFoundRequest records req as the cycle's next request and returns it as
// stored. The id, arrival sequence, filing time and state are the store's, set
// under the store's lock, so concurrent finders never share a sequence and a
// new request is always unapplied whatever the caller passed.
func AddFoundRequest(root, taskFile string, req *FoundRequest) (FoundRequest, error) {
	var stored FoundRequest
	err := UpdateFoundRequests(root, taskFile, func(reqs []FoundRequest) ([]FoundRequest, error) {
		seq := 1
		for i := range reqs {
			seq = max(seq, reqs[i].Seq+1)
		}
		stored = *req
		stored.Alternatives = slices.Clone(req.Alternatives)
		stored.Seq = seq
		stored.ID = "F" + strconv.Itoa(seq)
		stored.FiledAt = time.Now().UTC().Format(time.RFC3339Nano)
		stored.State = FoundStateUnapplied
		return append(reqs, stored), nil
	})
	return stored, err
}

// UpdateFoundRequests runs fn over the cycle's requests under the store's own
// lock and writes back what it returns. The lock is the store's, not the task
// file's: tp found apply holds the task-file lock while it updates requests.
func UpdateFoundRequests(root, taskFile string, fn func([]FoundRequest) ([]FoundRequest, error)) error {
	path := FoundRequestsPath(root, taskFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create found request dir: %w", err)
	}
	return WithFileLock(path, func() error {
		reqs, err := readFoundStore(path)
		if err != nil {
			return err
		}
		next, err := fn(reqs)
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(foundStore{Requests: next}, "", "  ")
		if err != nil {
			return err
		}
		return writeFileAtomic(path, append(data, '\n'))
	})
}
