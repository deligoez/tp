package engine

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/deligoez/tp/internal/model"
)

// EpicGraphFile is the graph file's name inside the project's .tp/ directory,
// beside config.json.
const EpicGraphFile = "epics.json"

// ErrEpicGraphNotFound reports that the .tp/ directory holds no epics.json. It
// is distinct from a parse error, which is a *MalformedConfigError.
var ErrEpicGraphNotFound = errors.New("epic graph not found")

// knownEpicGraphKeys and knownEpicKeys are this release's schema; any other
// key is returned to the caller for the lint to report.
var (
	knownEpicGraphKeys = map[string]bool{
		"version": true, "milestones": true, "gates": true, "tag_checks": true, "epics": true,
	}
	knownEpicKeys = map[string]bool{
		"id": true, "title": true, "milestone": true, "tags": true, "depends_on": true, "acceptance": true,
	}
)

// EpicGraphUnknownKey is a key outside the schema. Path locates it
// ("owner" at the top level, "epics[1].trigger" on an epic); EpicID is the
// carrying epic's id, empty at the top level.
type EpicGraphUnknownKey struct {
	Path   string
	Key    string
	EpicID string
}

// LoadEpicGraph reads tpDir/epics.json. A missing file wraps
// ErrEpicGraphNotFound; an unreadable or unparsable one is a
// *MalformedConfigError. Unknown top-level and per-epic keys are returned with
// their paths, top-level first, then by epic index, each group sorted by key.
func LoadEpicGraph(tpDir string) (*model.EpicGraph, []EpicGraphUnknownKey, error) {
	path := filepath.Join(tpDir, EpicGraphFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("%w: %s", ErrEpicGraphNotFound, path)
		}
		return nil, nil, &MalformedConfigError{Path: path, Err: err}
	}
	var g model.EpicGraph
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, nil, &MalformedConfigError{Path: path, Err: err}
	}
	// The typed parse above already proved both shapes, so these cannot fail.
	var top map[string]json.RawMessage
	_ = json.Unmarshal(data, &top)
	var epics []map[string]json.RawMessage
	if rawEpics, ok := top["epics"]; ok {
		_ = json.Unmarshal(rawEpics, &epics)
	}
	unknown := unknownKeys(top, knownEpicGraphKeys, "", "")
	for i, e := range epics {
		var id string
		if raw, ok := e["id"]; ok {
			_ = json.Unmarshal(raw, &id)
		}
		unknown = append(unknown, unknownKeys(e, knownEpicKeys, fmt.Sprintf("epics[%d].", i), id)...)
	}
	return &g, unknown, nil
}

// unknownKeys returns obj's keys outside known, sorted, each prefixed into a path.
func unknownKeys(obj map[string]json.RawMessage, known map[string]bool, prefix, epicID string) []EpicGraphUnknownKey {
	var keys []string
	for k := range obj {
		if !known[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	out := make([]EpicGraphUnknownKey, 0, len(keys))
	for _, k := range keys {
		out = append(out, EpicGraphUnknownKey{Path: prefix + k, Key: k, EpicID: epicID})
	}
	return out
}

// WriteEpicGraph writes g to tpDir/epics.json as 2-space JSON with content
// left unescaped, through a temp file and a rename so a lock-free reader never
// sees a partial document. The caller holds the write lock.
func WriteEpicGraph(tpDir string, g *model.EpicGraph) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(g); err != nil {
		return fmt.Errorf("marshal epic graph: %w", err)
	}
	path := filepath.Join(tpDir, EpicGraphFile)
	tmp, err := os.CreateTemp(tpDir, EpicGraphFile+".*.tmp")
	if err != nil {
		return fmt.Errorf("write epic graph: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write epic graph: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write epic graph: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("write epic graph: %w", err)
	}
	return nil
}
