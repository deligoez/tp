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
	// Context scopes the context blocks tp epic review shows a graph role:
	// RoleContextNeighbours (the default when empty),
	// RoleContextNeighboursBrief or RoleContextDependencies. Spec review and
	// audit do not read it.
	Context string `json:"context,omitempty"`
	// Units lists the unit kinds tp epic review asks a graph role:
	// RoleUnitEpics, RoleUnitGates and RoleUnitScope. Absent means epics and
	// gates (UnitKinds), so a corpus never receives a kind it was not written
	// for. Spec review and audit do not read it.
	Units []string `json:"units,omitempty"`
	// Purpose says the graph role is shown the purpose of each milestone its
	// asked units belong to, and is re-asked a milestone's units when that
	// purpose changes. Spec review and audit do not read it.
	Purpose bool `json:"purpose,omitempty"`
}

// The unit kinds a graph role's Units may list, in the order a unit key's
// prefix names them: epic:<id>, gate:<m> and scope:<m>.
const (
	RoleUnitEpics = "epics"
	RoleUnitGates = "gates"
	RoleUnitScope = "scope"
)

// UnitKinds returns the unit kinds the role is asked: its Units, or epics
// and gates when it lists none.
func (r *Role) UnitKinds() []string {
	if len(r.Units) == 0 {
		return []string{RoleUnitEpics, RoleUnitGates}
	}
	return r.Units
}

// The values a graph role's Context may take. RoleContextNeighboursBrief
// shows each dependency's full block and each dependent by its id, title,
// milestone and its edges into the asked units (spec/1.7.0.md §4).
const (
	RoleContextNeighbours      = "neighbours"
	RoleContextNeighboursBrief = "neighbours-brief"
	RoleContextDependencies    = "dependencies"
)
