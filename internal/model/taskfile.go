package model

import "time"

// TaskFile is the root structure of a .tasks.json file.
type TaskFile struct {
	Version int    `json:"version"`
	Spec    string `json:"spec"`
	// Epic names the one epic of the project's graph this file implements;
	// omitted when the file is bound to none.
	Epic string `json:"epic,omitempty"`
	// BoundAt is when the file was bound to Epic, and ImportedAt when tasks
	// were last imported into it (spec/1.8.0.md §15), both in UTC; nil, and
	// omitted, on a file written before either was recorded.
	BoundAt    *time.Time       `json:"bound_at,omitempty"`
	ImportedAt *time.Time       `json:"imported_at,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
	Workflow   WorkflowOverride `json:"workflow"`
	Coverage   Coverage         `json:"coverage"`
	Tasks      []Task           `json:"tasks"`
}
