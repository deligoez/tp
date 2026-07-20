package model

import "encoding/json"

// Check is a mechanical check: a kebab-case class slug and the command that detects it.
type Check struct {
	Class string `json:"class"`
	Cmd   string `json:"cmd"`
}

// Workflow defines quality gate, commit strategy, and convergence parameters.
type Workflow struct {
	QualityGate        string  `json:"quality_gate,omitempty"`
	CommitStrategy     string  `json:"commit_strategy,omitempty"`
	ReviewCleanRounds  int     `json:"review_clean_rounds"`
	AuditCleanRounds   int     `json:"audit_clean_rounds"`
	GateTimeoutSeconds int     `json:"gate_timeout_seconds"`
	Checks             []Check `json:"checks"`
	ReviewMaxRounds    int     `json:"review_max_rounds"`
	AuditMaxRounds     int     `json:"audit_max_rounds"`
}

// workflowJSON is the raw JSON representation used by UnmarshalJSON
// to distinguish absent fields (nil) from explicit zero (non-nil, value 0).
type workflowJSON struct {
	QualityGate        string  `json:"quality_gate,omitempty"`
	CommitStrategy     string  `json:"commit_strategy,omitempty"`
	ReviewCleanRounds  *int    `json:"review_clean_rounds,omitempty"`
	AuditCleanRounds   *int    `json:"audit_clean_rounds,omitempty"`
	GateTimeoutSeconds *int    `json:"gate_timeout_seconds,omitempty"`
	Checks             []Check `json:"checks,omitempty"`
	ReviewMaxRounds    int     `json:"review_max_rounds"`
	AuditMaxRounds     int     `json:"audit_max_rounds"`
}

// UnmarshalJSON applies defaults for absent fields.
// Absent convergence fields (nil pointer) get default 2; absent
// gate_timeout_seconds gets 600; absent checks gets an empty slice.
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

	if raw.GateTimeoutSeconds == nil {
		w.GateTimeoutSeconds = 600
	} else {
		w.GateTimeoutSeconds = *raw.GateTimeoutSeconds
	}

	if raw.Checks == nil {
		w.Checks = []Check{}
	} else {
		w.Checks = raw.Checks
	}

	w.ReviewMaxRounds = raw.ReviewMaxRounds
	w.AuditMaxRounds = raw.AuditMaxRounds

	return nil
}
