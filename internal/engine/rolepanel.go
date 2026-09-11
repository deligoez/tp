// rolepanel.go holds the pure half of role-panel resolution: the domain-filtered
// corpus, the spec's frontmatter overrides and its enabled: false drop, with the
// advisory warnings returned as data and a malformed role file returned as an
// error (1.0.1 §7).
//
// It lives in engine rather than in cli because it already had a consumer here.
// roleUnits resolved the same panel for tp resume's read-only oracle, so putting
// the shared half in cli would have left that copy standing and made tp lint's
// the third — the shape §7 rejects. Instead roleUnits now calls this, and the
// refusing wrapper in internal/cli calls it too, so there is one resolution and
// three consumers.
//
// Nothing here writes to a stream or exits. Refusing is a property of the two
// commands that emit a round, not of resolution: a read-only caller — tp resume's
// oracle, tp lint's review_panel — must be able to describe a panel it would
// refuse to run.

package engine

import (
	"path/filepath"

	"github.com/deligoez/tp/internal/model"
)

// RolePanel is one phase's resolved role panel: the spec's frontmatter, the
// active roles that survived corpus resolution, override layering and §2.3's
// enabled: false drop, that drop set itself, and the advisory warnings the
// resolution produced.
//
// Warnings are carried rather than emitted, and their order is part of the
// contract: the frontmatter's errors and shape warnings first, then the corpus
// warnings ahead of the override warnings, which is the byte sequence the
// wrapper's two output.Notice loops wrote when resolution and refusal were one
// function.
type RolePanel struct {
	// Frontmatter is the parsed spec frontmatter, never nil on a successful
	// resolution — both phases carry it forward past the panel.
	Frontmatter *Frontmatter
	// Roles is the post-drop active panel, in corpus order.
	Roles []model.Role
	// Disabled is §2.3's drop set: the ids this spec deactivated with
	// enabled: false. It survives the drop so a caller can name them — the
	// wrapper's refusals do, and each phase's emission reports them as
	// skipped_roles with reason disabled-by-spec (§2.4).
	Disabled []string
	// Warnings are advisory: a frontmatter block, key or value the parser
	// ignored, an unknown domain, a domain that filtered out every role, a
	// frontmatter override matching no active role. Every one says the panel
	// the spec asked for is not the panel that resolved.
	Warnings []string
}

// ResolveRolePanel resolves a phase's role panel without deciding anything about
// it. The order is §2.6's: domain filtering and the unknown-id check happen
// inside the resolvers, then §2.3's enabled: false drop — applied here, outside
// ResolveActiveCorpus and after its domain filtering, so deactivating every user
// role empties the panel instead of falling back to the embedded default corpus.
//
// A malformed role file comes back as an error with a zero panel. Callers differ
// in what they do with it: tp review and tp audit abort their own phase (§3.6,
// exit 3), while a read-only caller may decline to guess at a panel it cannot
// resolve.
func ResolveRolePanel(specPath, corpusPhase string) (RolePanel, error) {
	fm := ParseFrontmatter(specPath)
	roles, corpusWarnings, err := ResolveActiveCorpus(filepath.Dir(specPath), fm.Domain, corpusPhase)
	if err != nil {
		return RolePanel{}, err
	}
	roles, overrideWarnings, disabled := ResolveOverrideFocus(roles, fm, corpusPhase)

	// The frontmatter's own findings lead: parsing precedes resolution, and a
	// key the parser ignored changes the panel as surely as an override that
	// matched no role. Its structural errors travel here too, as advisories:
	// an unclosed or unparseable block leaves the run on the defaults rather
	// than failing it, and without them that fallback reached tp lint alone.
	warnings := make([]string, 0, len(fm.Errors)+len(fm.Warnings)+len(corpusWarnings)+len(overrideWarnings))
	for _, f := range fm.Errors {
		warnings = append(warnings, f.Message)
	}
	for _, w := range fm.Warnings {
		warnings = append(warnings, w.Message)
	}
	warnings = append(warnings, corpusWarnings...)
	warnings = append(warnings, overrideWarnings...)

	return RolePanel{
		Frontmatter: fm,
		Roles:       DropDisabledRoles(roles, disabled),
		Disabled:    disabled,
		Warnings:    warnings,
	}, nil
}
