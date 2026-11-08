package engine

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/deligoez/tp/internal/model"
)

// EpicNotFoundError reports a write naming an epic id the graph does not hold.
type EpicNotFoundError struct {
	ID string
}

func (e *EpicNotFoundError) Error() string {
	return fmt.Sprintf("epic %q not found", e.ID)
}

// EpicGraphSetKeys are the top-level keys tp epic set replaces whole (§3).
var EpicGraphSetKeys = []string{"milestones", "gates", "tag_checks"}

// EpicFieldSetKeys are the epic fields tp epic set <id> writes. title,
// milestone and acceptance take the value as written; tags and depends_on
// take JSON. The id is not settable: it is the epic's identity.
var EpicFieldSetKeys = []string{"title", "milestone", "tags", "depends_on", "acceptance"}

// ParseEpicGraphSet decodes value for one of EpicGraphSetKeys and returns the
// mutation that replaces that whole object. A bad key or value is an error
// before any lock is taken.
func ParseEpicGraphSet(key, value string) (func(*model.EpicGraph) error, error) {
	switch key {
	case "milestones":
		var m [][]string
		if err := decodeStrict(value, &m); err != nil {
			return nil, err
		}
		return func(g *model.EpicGraph) error { g.Milestones = m; return nil }, nil
	case "gates":
		var gates map[string]model.EpicGate
		if err := decodeStrict(value, &gates); err != nil {
			return nil, err
		}
		return func(g *model.EpicGraph) error { g.Gates = gates; return nil }, nil
	case "tag_checks":
		var checks map[string]model.EpicTagCheck
		if err := decodeStrict(value, &checks); err != nil {
			return nil, err
		}
		return func(g *model.EpicGraph) error { g.TagChecks = checks; return nil }, nil
	}
	return nil, fmt.Errorf("unknown key %q: one of %v", key, EpicGraphSetKeys)
}

// ParseEpicFieldSet decodes value for one of EpicFieldSetKeys and returns the
// mutation that writes it on epic id; the mutation returns an
// *EpicNotFoundError when the graph holds no such epic.
func ParseEpicFieldSet(id, field, value string) (func(*model.EpicGraph) error, error) {
	apply, err := epicFieldSetter(field, value)
	if err != nil {
		return nil, err
	}
	return func(g *model.EpicGraph) error {
		i := epicIndex(g, id)
		if i < 0 {
			return &EpicNotFoundError{ID: id}
		}
		apply(&g.Epics[i])
		return nil
	}, nil
}

func epicFieldSetter(field, value string) (func(*model.Epic), error) {
	switch field {
	case "title":
		return func(e *model.Epic) { e.Title = value }, nil
	case "milestone":
		return func(e *model.Epic) { e.Milestone = value }, nil
	case "acceptance":
		return func(e *model.Epic) { e.Acceptance = value }, nil
	case "tags":
		var tags []string
		if err := decodeStrict(value, &tags); err != nil {
			return nil, err
		}
		return func(e *model.Epic) { e.Tags = tags }, nil
	case "depends_on":
		var deps []model.EpicDep
		if err := decodeStrict(value, &deps); err != nil {
			return nil, err
		}
		return func(e *model.Epic) { e.DependsOn = deps }, nil
	}
	return nil, fmt.Errorf("unknown field %q: one of %v", field, EpicFieldSetKeys)
}

// decodeStrict decodes one JSON value, refusing keys outside the schema.
func decodeStrict(value string, v any) error {
	dec := json.NewDecoder(bytes.NewReader([]byte(value)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("invalid JSON value: %w", err)
	}
	return nil
}
