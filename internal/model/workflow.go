package model

import "encoding/json"

// Workflow defines quality gate, commit strategy, and convergence parameters.
type Workflow struct {
	QualityGate       string `json:"quality_gate,omitempty"`
	CommitStrategy    string `json:"commit_strategy,omitempty"`
	ReviewCleanRounds int    `json:"review_clean_rounds"`
	AuditCleanRounds  int    `json:"audit_clean_rounds"`
}

// workflowJSON is the raw JSON representation used by UnmarshalJSON
// to distinguish absent fields (nil) from explicit zero (non-nil, value 0).
type workflowJSON struct {
	QualityGate       string `json:"quality_gate,omitempty"`
	CommitStrategy    string `json:"commit_strategy,omitempty"`
	ReviewCleanRounds *int   `json:"review_clean_rounds,omitempty"`
	AuditCleanRounds  *int   `json:"audit_clean_rounds,omitempty"`
}

// UnmarshalJSON applies defaults for absent convergence fields.
// Absent fields (nil pointer) get default 2.
// Explicit zero is preserved as 0 for validation to reject.
func (w *Workflow) UnmarshalJSON(data []byte) error {
	var raw workflowJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	w.QualityGate = raw.QualityGate
	w.CommitStrategy = raw.CommitStrategy

	if raw.ReviewCleanRounds == nil {
		w.ReviewCleanRounds = 2
	} else {
		w.ReviewCleanRounds = *raw.ReviewCleanRounds
	}

	if raw.AuditCleanRounds == nil {
		w.AuditCleanRounds = 2
	} else {
		w.AuditCleanRounds = *raw.AuditCleanRounds
	}

	return nil
}
