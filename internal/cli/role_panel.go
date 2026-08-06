// role_panel.go holds the role-panel machinery shared by tp review and tp
// audit: corpus resolution plus override layering (§2.3), the drop set both
// phases report as disabled-by-spec (§2.4), the two emission-only refusals
// (§2.5), and the order they are decided in (§2.6). It lives in its own file
// because both command files consume it; neither owns it.

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// refuseEmptyPhase exits 2 when this spec's enabled: false entries leave a
// phase with no active role. Refusing here is what makes §2.3's placement
// observable: the drop runs outside ResolveActiveCorpus, so an emptied panel
// stays empty instead of silently reverting to the embedded default corpus.
// The phase word is rendered from the PhaseReviewers/PhaseAuditors value and
// the deactivated ids follow it sorted and comma-separated (§2.5). The list is
// §2.3's drop set, so it names only the ids this spec deactivated: a role
// already absent through domains or a missing role file is not active, never
// enters the drop set, and is never named — including when the phase was
// emptied partly by domains and partly by enabled: false.
func refuseEmptyPhase(phase string, disabled []string) {
	ids := make([]string, len(disabled))
	copy(ids, disabled)
	sort.Strings(ids)
	output.Error(ExitUsage,
		fmt.Sprintf("every %s role is deactivated by this spec: %s", phase, strings.Join(ids, ", ")),
		"re-enable at least one role, or remove the enabled: false entries")
	os.Exit(ExitUsage)
}

// refuseSpecCoverageDeactivated exits 2 when §2.3's drop set for the auditor
// phase contains spec-coverage. routeChecklist routes every spec-derived item
// to that id alone, so deactivating it drops the whole spec-derived checklist
// while every other auditor still emits — which is why this is not an emptiness
// check: it fires even when other auditors remain active.
//
// The check keys on the drop set rather than on the frontmatter entry, so a
// corpus with no active spec-coverage role produces no drop and tp.audit_roles
// naming it takes §2.3's "matches no active role" warning path instead.
func refuseSpecCoverageDeactivated(disabled []string) {
	for _, id := range disabled {
		if id == roleSpecCoverage {
			output.Error(ExitUsage,
				roleSpecCoverage+" cannot be deactivated: it carries the entire spec-derived checklist",
				"remove the enabled: false entry for "+roleSpecCoverage)
			os.Exit(ExitUsage)
		}
	}
}

// rolePanel is one phase's resolved role panel: the spec frontmatter plus the
// active roles that survived corpus resolution, override layering and §2.3's
// enabled: false drop.
type rolePanel struct {
	fm    *engine.Frontmatter
	roles []model.Role
	// disabled holds §2.3's drop set — the sorted ids this spec deactivated
	// with enabled: false — so each phase's emission can name them in
	// skipped_roles with reason disabled-by-spec (§2.4).
	disabled []string
}

// resolveRolePanel resolves a phase's role panel and decides both §2.5
// refusals for it. Callers must invoke it ahead of every write their emission
// path performs (§2.5 item 2): ahead of EnsureReviewState in tp review — which
// creates .tp-review/<spec>/ and state.json before the round snapshot — and
// ahead of the round snapshot in tp audit. A refused run then leaves nothing on
// disk for either command.
//
// The order inside is §2.6's: domain filtering and the unknown-id check happen
// inside the resolvers, then the spec-coverage refusal (auditors only, because
// it names a single entry to remove), then the empty-phase refusal.
func resolveRolePanel(specPath, phase string) rolePanel {
	fm := engine.ParseFrontmatter(specPath)
	// A malformed role file aborts its own phase (§3.6, exit 3) and never the
	// other one; the phase word doubles as the corpus directory name, so the
	// hint points at the phase that failed.
	roles, corpusWarnings, corpusErr := engine.ResolveActiveCorpus(filepath.Dir(specPath), fm.Domain, phase)
	if corpusErr != nil {
		output.Error(ExitFile, corpusErr.Error(), "repair or delete the offending role file under .tp/"+phase+"/")
		os.Exit(ExitFile)
	}
	// Notice for the same reason as the override warnings below: an unknown
	// domain or a domain that filtered out every role means the panel the spec
	// asked for is not the panel that ran.
	for _, w := range corpusWarnings {
		output.Notice(w)
	}
	// Layer the spec-frontmatter overrides (tp.review_roles / tp.audit_roles,
	// plus the legacy tp: lens shim) onto each active role's corpus focus.
	roles, overrideWarnings, disabled := engine.ResolveOverrideFocus(roles, fm, phase)
	// Notice, not Info: every one of these says a frontmatter entry was
	// ignored — an id matching no active role, an unknown legacy lens key. Info
	// returns early in JSON mode and JSON mode is on whenever stdout is not a
	// terminal, so on that channel the advisory is invisible in exactly the
	// agent-driven runs where a typo'd role id silently costs a sub-agent round.
	for _, w := range overrideWarnings {
		output.Notice(w)
	}
	if phase == engine.PhaseAuditors {
		refuseSpecCoverageDeactivated(disabled)
	}
	// Apply the enabled: false drop here — outside ResolveActiveCorpus and after
	// its domain filtering — so deactivating every user role empties the panel
	// instead of falling back to the embedded default corpus (§2.3).
	roles = engine.DropDisabledRoles(roles, disabled)
	if len(disabled) > 0 && len(roles) == 0 {
		refuseEmptyPhase(phase, disabled)
	}
	return rolePanel{fm: fm, roles: roles, disabled: disabled}
}
