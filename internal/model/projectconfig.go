package model

// ProjectConfig models the committed .tp/config.json file, which holds the
// single top-level "workflow" key of project-wide workflow-field defaults.
// It MUST be committed to version control; import convergence enforcement and
// CI depend on it traveling with the repository.
//
// An absent .tp/config.json is equivalent to an empty object: every field is
// nil (unset), so the file contributes no overrides to resolution.
type ProjectConfig struct {
	Workflow WorkflowOverride `json:"workflow"`
}

// LocalConfig models the git-ignored .tp/local.json file, which holds
// per-developer, per-session state: the active task-file pointer and the
// global flag defaults. It MUST NOT be committed.
//
// LocalConfig and ProjectConfig are optional and independent: either may exist
// without the other, and an absent file is equivalent to an empty object.
type LocalConfig struct {
	// Active is the project-root-relative path to the active task file, or nil
	// when unset. A pointer distinguishes an unset pointer from an empty string.
	Active *string `json:"active,omitempty"`
	// Defaults holds boolean defaults for the global flags compact, quiet, and
	// no_color, applied when the corresponding flag is absent from the CLI.
	Defaults map[string]bool `json:"defaults,omitempty"`
}

// WorkflowOverride is a sparse, presence-tracking view of the workflow fields.
// A nil pointer means the field is absent (inherit from the next layer); a
// non-nil pointer is an explicit override, even when its value equals a default
// — presence, not value, defines an override. This is why review_max_rounds: 0
// (a legitimate "no cap" value) stays distinguishable from an absent key.
//
// Checks uses a pointer to a slice so that an explicit empty array (present,
// replace with nothing) is distinct from an absent key (inherit).
type WorkflowOverride struct {
	QualityGate        *string  `json:"quality_gate,omitempty"`
	CommitStrategy     *string  `json:"commit_strategy,omitempty"`
	GateTimeoutSeconds *int     `json:"gate_timeout_seconds,omitempty"`
	ReviewCleanRounds  *int     `json:"review_clean_rounds,omitempty"`
	AuditCleanRounds   *int     `json:"audit_clean_rounds,omitempty"`
	ReviewMaxRounds    *int     `json:"review_max_rounds,omitempty"`
	AuditMaxRounds     *int     `json:"audit_max_rounds,omitempty"`
	Checks             *[]Check `json:"checks,omitempty"`
}

// IsEmpty reports whether the override sets no fields at all, which is the case
// for an absent file (equivalent to an empty object).
func (o WorkflowOverride) IsEmpty() bool {
	return o.QualityGate == nil &&
		o.GateTimeoutSeconds == nil &&
		o.ReviewCleanRounds == nil &&
		o.AuditCleanRounds == nil &&
		o.ReviewMaxRounds == nil &&
		o.AuditMaxRounds == nil &&
		o.Checks == nil
}
