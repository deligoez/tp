package model

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
	LockTimeoutSeconds int     `json:"lock_timeout_seconds"`
	Checks             []Check `json:"checks"`
	ReviewMaxRounds    int     `json:"review_max_rounds"`
	AuditMaxRounds     int     `json:"audit_max_rounds"`
}

// EffectiveGateTimeoutSeconds returns gate_timeout_seconds, falling back to
// 600 when the stored value is outside the valid 30-3600 range.
func (w *Workflow) EffectiveGateTimeoutSeconds() int {
	if w.GateTimeoutSeconds < 30 || w.GateTimeoutSeconds > 3600 {
		return 600
	}
	return w.GateTimeoutSeconds
}

// EffectiveLockTimeoutSeconds returns lock_timeout_seconds, falling back to 5
// when the stored value is outside the valid 1-60 range (§12.1).
func (w *Workflow) EffectiveLockTimeoutSeconds() int {
	if w.LockTimeoutSeconds < 1 || w.LockTimeoutSeconds > 60 {
		return 5
	}
	return w.LockTimeoutSeconds
}

// EffectiveReviewMaxRounds returns review_max_rounds, falling back to 0
// (no cap) when the stored value is outside the valid 0-50 range.
func (w *Workflow) EffectiveReviewMaxRounds() int {
	if w.ReviewMaxRounds < 0 || w.ReviewMaxRounds > 50 {
		return 0
	}
	return w.ReviewMaxRounds
}

// EffectiveAuditMaxRounds returns audit_max_rounds, falling back to 0
// (no cap) when the stored value is outside the valid 0-50 range.
func (w *Workflow) EffectiveAuditMaxRounds() int {
	if w.AuditMaxRounds < 0 || w.AuditMaxRounds > 50 {
		return 0
	}
	return w.AuditMaxRounds
}
