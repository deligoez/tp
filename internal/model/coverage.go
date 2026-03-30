package model

// Coverage tracks section-to-task mapping completeness.
type Coverage struct {
	TotalSections  int      `json:"total_sections"`
	MappedSections int      `json:"mapped_sections"`
	ContextOnly    []string `json:"context_only"`
	Unmapped       []string `json:"unmapped"`
}
