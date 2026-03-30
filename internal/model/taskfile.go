package model

import "time"

// TaskFile is the root structure of a .tasks.json file.
type TaskFile struct {
	Version   int       `json:"version"`
	Spec      string    `json:"spec"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Workflow  Workflow  `json:"workflow"`
	Coverage  Coverage  `json:"coverage"`
	Tasks     []Task    `json:"tasks"`
}
