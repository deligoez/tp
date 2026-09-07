// role_panel.go holds the REFUSING half of the role-panel machinery shared by
// tp review and tp audit: the two emission-only refusals (§2.5), the abort on a
// malformed role file, the advisory notices, and the order they are decided in
// (§2.6). It lives in its own file because both command files consume it;
// neither owns it.
//
// The resolution itself is engine.ResolveRolePanel (1.0.1 §7). It moved to
// engine rather than staying here because engine.roleUnits — tp resume's
// read-only oracle — already resolved the same panel, so a shared half in cli
// would have left that copy standing and made tp lint's the third. Everything
// this file adds is what a read-only caller must NOT inherit.

package cli

import (
	"fmt"
	"os"
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

// rolePanel is one phase's resolved role panel as the two emitting commands
// consume it: the spec frontmatter plus the active roles that survived corpus
// resolution, override layering and §2.3's enabled: false drop.
//
// It is engine.RolePanel minus the warnings, which the wrapper has already
// emitted by the time it builds this. Keeping it is what lets tp review and tp
// audit stay byte-for-byte unchanged across the split (1.0.1 §7).
type rolePanel struct {
	fm    *engine.Frontmatter
	roles []model.Role
	// disabled holds §2.3's drop set — the sorted ids this spec deactivated
	// with enabled: false — so each phase's emission can name them in
	// skipped_roles with reason disabled-by-spec (§2.4).
	disabled []string
}

// resolveRolePanel is the refusing wrapper over engine.ResolveRolePanel: it
// resolves a phase's role panel and decides both §2.5 refusals for it. Both
// callers — tp review and tp audit — keep this rather than the resolver,
// because both emit a round and a round that cannot be emitted must refuse.
//
// Callers must invoke it ahead of every write their emission
// path performs (§2.5 item 2): ahead of EnsureReviewState in tp review — which
// creates .tp-review/<spec>/ and state.json before the round snapshot — and
// ahead of the round snapshot in tp audit. A refused run then leaves nothing on
// disk for either command.
//
// The order inside is §2.6's: domain filtering and the unknown-id check happen
// inside the resolvers, then the spec-coverage refusal (auditors only, because
// it names a single entry to remove), then the empty-phase refusal.
func resolveRolePanel(specPath, phase string) rolePanel {
	// The resolution itself is engine.ResolveRolePanel, shared with tp resume's
	// oracle and tp lint (1.0.1 §7). Everything below is the refusing half: the
	// abort, the notice loop and the two §2.5 refusals, in §2.6's order.
	panel, err := engine.ResolveRolePanel(specPath, phase)
	// A malformed role file aborts its own phase (§3.6, exit 3) and never the
	// other one; the phase word doubles as the corpus directory name, so the
	// hint points at the phase that failed.
	if err != nil {
		output.Error(ExitFile, err.Error(), "repair or delete the offending role file under .tp/"+phase+"/")
		os.Exit(ExitFile)
	}
	// Notice, not Info: every one of these says the panel the spec asked for is
	// not the panel that resolved — an unknown domain, a domain that filtered
	// out every role, an id matching no active role, an unknown legacy lens key.
	// Info returns early in JSON mode and JSON mode is on whenever stdout is not
	// a terminal, so on that channel the advisory is invisible in exactly the
	// agent-driven runs where a typo'd role id silently costs a sub-agent round.
	//
	// The loop stays HERE rather than inside the resolver, which is most of what
	// the split is for: tp lint resolves the same panel and must not gain an
	// advisory stderr channel it has never had (§7's third row).
	for _, w := range panel.Warnings {
		output.Notice(w)
	}
	if phase == engine.PhaseAuditors {
		refuseSpecCoverageDeactivated(panel.Disabled)
	}
	if len(panel.Disabled) > 0 && len(panel.Roles) == 0 {
		refuseEmptyPhase(phase, panel.Disabled)
	}
	return rolePanel{fm: panel.Frontmatter, roles: panel.Roles, disabled: panel.Disabled}
}
