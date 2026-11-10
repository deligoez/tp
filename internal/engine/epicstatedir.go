package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// EpicReviewStateRoot is the directory under .tp/ that holds one state
// directory per tp epic review slice (spec/1.3.0.md §5).
const EpicReviewStateRoot = "epic-review"

// EpicReviewStateDir returns key's state directory under tpDir: its name is
// the lowercase hex SHA-256 of the slice key with its kind (D13), so that
// --milestone A and --epic A never share one and no id reaches the file
// system. Round files and snapshots of the slice live there.
func EpicReviewStateDir(tpDir string, key EpicSliceKey) string {
	return filepath.Join(tpDir, EpicReviewStateRoot, epicHash(epicSliceKeyText(key)))
}

// EpicReviewInboxDir is the slice's inbox under its state directory: where a
// role of a hand-run round writes its rows (spec/1.7.0.md §1). It is
// git-ignored, and the plugin's write-deny hook lets the inbox name through.
func EpicReviewInboxDir(stateDir string) string {
	return filepath.Join(stateDir, "inbox")
}

// EpicReviewInboxPath is one role's file of round in the slice's inbox,
// <state dir>/inbox/<role>-r<N>.ndjson.
func EpicReviewInboxPath(stateDir, role string, round int) string {
	return filepath.Join(EpicReviewInboxDir(stateDir), fmt.Sprintf("%s-r%d.ndjson", role, round))
}

// epicSliceKeyText renders key with its kind and a NUL separator, which no
// milestone or epic id can make ambiguous. A named set is keyed by its name,
// an unnamed one by its sorted, de-duplicated ids, under two different kinds.
func epicSliceKeyText(key EpicSliceKey) string {
	if key.Name != "" {
		return "epics-named\x00" + key.Name
	}
	if len(key.Epics) > 0 {
		ids := slices.Compact(slices.Sorted(slices.Values(key.Epics)))
		return "epics\x00" + strings.Join(ids, "\x00")
	}
	if key.Milestone != "" {
		return "milestone\x00" + key.Milestone
	}
	return "epic\x00" + key.Epic
}

// epicSliceKeyFile names the file in a slice's state directory that records
// the slice's key, so the slices can be listed without inverting the hash.
const epicSliceKeyFile = "key.json"

// EpicSliceKeyRecord is a slice's key as its state directory records it
// (spec/1.5.0.md §7): a milestone or an epic by kind and value, a set by
// kind "epics" with its sorted, de-duplicated ids and its name when it has one.
type EpicSliceKeyRecord struct {
	Kind  string   `json:"kind"`
	Value string   `json:"value,omitempty"`
	IDs   []string `json:"ids,omitempty"`
	Name  string   `json:"name,omitempty"`
}

// EpicSliceKeyRecordOf renders key as its state directory records it.
func EpicSliceKeyRecordOf(key EpicSliceKey) EpicSliceKeyRecord {
	switch {
	case len(key.Epics) > 0:
		ids := slices.Compact(slices.Sorted(slices.Values(key.Epics)))
		return EpicSliceKeyRecord{Kind: "epics", IDs: ids, Name: key.Name}
	case key.Milestone != "":
		return EpicSliceKeyRecord{Kind: "milestone", Value: key.Milestone}
	default:
		return EpicSliceKeyRecord{Kind: "epic", Value: key.Epic}
	}
}

// SliceKey is the slice key the record names, as EpicSliceKeyRecordOf
// rendered it.
func (r *EpicSliceKeyRecord) SliceKey() EpicSliceKey {
	switch r.Kind {
	case "epics":
		return EpicSliceKey{Epics: r.IDs, Name: r.Name}
	case "milestone":
		return EpicSliceKey{Milestone: r.Value}
	}
	return EpicSliceKey{Epic: r.Value}
}

// EpicSliceDir is one slice state directory under .tp/epic-review/: its name,
// its path, and its key (key.json's, else the one its recorded rounds name;
// nil when neither says).
type EpicSliceDir struct {
	Name string
	Path string
	Key  *EpicSliceKeyRecord
}

// EpicReviewSliceDirs lists every slice state directory under tpDir, in name
// order, each with its key; no directory lists none.
func EpicReviewSliceDirs(tpDir string, g *model.EpicGraph) ([]EpicSliceDir, error) {
	root := filepath.Join(tpDir, EpicReviewStateRoot)
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return []EpicSliceDir{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []EpicSliceDir{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		key, err := ReadEpicSliceKey(dir)
		if err != nil {
			return nil, err
		}
		if key == nil {
			key = deriveEpicSliceKey(dir, g)
		}
		out = append(out, EpicSliceDir{Name: entry.Name(), Path: dir, Key: key})
	}
	return out, nil
}

// WriteEpicSliceKey records key in stateDir, creating the directory, through
// an atomic rename. Each emission writes it, so a slice recorded before
// v1.5.0 gains its key on its next emission.
func WriteEpicSliceKey(stateDir string, key EpicSliceKey) error {
	data, err := json.MarshalIndent(EpicSliceKeyRecordOf(key), "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(stateDir, epicSliceKeyFile), append(data, '\n'))
}

// ReadEpicSliceKey returns the key stateDir records, nil when the slice has
// no key file: one recorded before v1.5.0, whose key is unknown.
func ReadEpicSliceKey(stateDir string) (*EpicSliceKeyRecord, error) {
	data, err := os.ReadFile(filepath.Join(stateDir, epicSliceKeyFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rec EpicSliceKeyRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Join(stateDir, epicSliceKeyFile), err)
	}
	return &rec, nil
}
