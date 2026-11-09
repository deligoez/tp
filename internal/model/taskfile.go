package model

import "time"

// TaskFile is the root structure of a .tasks.json file.
type TaskFile struct {
	Version int    `json:"version"`
	Spec    string `json:"spec"`
	// Epic names the one epic of the project's graph this file implements;
	// omitted when the file is bound to none.
	Epic      string           `json:"epic,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	Workflow  WorkflowOverride `json:"workflow"`
	Coverage  Coverage         `json:"coverage"`
	Tasks     []Task           `json:"tasks"`
}
