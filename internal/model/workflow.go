package model

// Workflow defines quality gate and commit strategy defaults.
type Workflow struct {
	QualityGate    string `json:"quality_gate,omitempty"`
	CommitStrategy string `json:"commit_strategy,omitempty"`
}
