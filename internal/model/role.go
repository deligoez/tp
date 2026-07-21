package model

// Role is a project-owned reviewer or auditor role definition (v0.25.0).
//
// The role corpus is project data: reviewer roles live in .tp/reviewers/ and
// auditor roles in .tp/auditors/, so the phase (review vs audit) is inferred
// from the directory and is never stored on the role itself. tp owns the finding
// output contract; a Role only customizes the prompt (persona, instructions,
// focus questions).
type Role struct {
	// ID is the role id. It MUST equal the file's stem and match the kebab-case
	// pattern ^[a-z0-9]+(-[a-z0-9]+)*$; the id "regression" is reserved. (The
	// pattern and reserved-id checks are enforced by role-corpus validation, not
	// by this type.)
	ID string `json:"id"`
	// Title is the human label shown in prompts and progress output.
	Title string `json:"title"`
	// Instructions is the role's framing / system-prompt body.
	Instructions string `json:"instructions"`
	// Focus lists focus questions appended to the emitted prompt (review) or the
	// checklist-generation focus (audit). The JSON key is "focus", never "lens" —
	// "lens" is reserved for the retired frontmatter and the informal concept.
	Focus []string `json:"focus,omitempty"`
	// Domains restricts the role to the listed spec domains; an empty or absent
	// list means the role applies to every domain.
	Domains []string `json:"domains,omitempty"`
}
