package model

import "encoding/json"

// Check is a mechanical check: a kebab-case class slug and the command that detects it.
type Check struct {
	Class string `json:"class"`
	Cmd   string `json:"cmd"`
}

// Workflow defines quality gate, commit strategy, convergence parameters, and
// the unattended run's caps and runner.
//
// The run fields carry §7's defaults: run_max_units 100, run_max_wall_clock_seconds
// 28800, both budgets 0 (disabled/flag omitted), run_max_unit_retries 1 and
// runner "claude". Budget fields are decimal dollars. Like every other workflow
// field, the defaults live in the resolver, not in this struct's zero value.
//
// NotifyCmd is the one field that does not resolve through the layered path: it
// is per-operator rather than per-project and is read from .tp/local.json only.
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
	ReviewConvergeOn   string  `json:"review_converge_on,omitempty"`
	AuditConvergeOn    string  `json:"audit_converge_on,omitempty"`

	RunMaxUnits            int             `json:"run_max_units"`
	RunMaxWallClockSeconds int             `json:"run_max_wall_clock_seconds"`
	RunMaxBudgetUSD        float64         `json:"run_max_budget_usd"`
	RunMaxUnitBudgetUSD    float64         `json:"run_max_unit_budget_usd"`
	RunMaxUnitRetries      int             `json:"run_max_unit_retries"`
	Runner                 json.RawMessage `json:"runner,omitempty"`
	NotifyCmd              string          `json:"notify_cmd,omitempty"`
}

// EffectiveGateTimeoutSeconds returns gate_timeout_seconds, falling back to
// 600 when the stored value is outside the valid 30-3600 range.
func (w *Workflow) EffectiveGateTimeoutSeconds() int {
	if w.GateTimeoutSeconds < 30 || w.GateTimeoutSeconds > 3600 {
		return 600
	}
	return w.GateTimeoutSeconds
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
