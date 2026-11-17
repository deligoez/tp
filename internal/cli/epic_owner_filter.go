package cli

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// epicOwnerFilter keeps the entries whose owner names one of handles, and
// with unowned the ones with no declared owner; with neither it keeps
// everything (spec/backlog/epic-owner.md at edeebda0). An epic naming only handles the
// set does not declare reads as unowned, so some filter always shows it.
type epicOwnerFilter struct {
	handles  []string
	unowned  bool
	declared []string
}

// addEpicOwnerFlags registers --owner and --unowned on cmd.
func addEpicOwnerFlags(cmd *cobra.Command) {
	cmd.Flags().StringArray("owner", nil, "keep the entries whose owner names this declared handle (repeatable; with --unowned, a union)")
	cmd.Flags().Bool("unowned", false, "keep the entries with no owner (with --owner, a union)")
}

// epicOwnerFilterOrExit reads cmd's --owner and --unowned. A handle g's
// owners set does not declare exits 2 naming the declared ones.
func epicOwnerFilterOrExit(cmd *cobra.Command, g *model.EpicGraph) epicOwnerFilter {
	handles, _ := cmd.Flags().GetStringArray("owner")
	unowned, _ := cmd.Flags().GetBool("unowned")
	declared := make([]string, 0, len(g.Owners))
	for _, o := range g.Owners {
		declared = append(declared, o.Handle)
	}
	hint := "the graph declares no owners; tp epic set owners=<json> declares them"
	if len(declared) > 0 {
		hint = "declared: " + strings.Join(declared, ", ") + "; tp epic set owners=<json> declares more"
	}
	for _, h := range handles {
		if !slices.Contains(declared, h) {
			output.Error(ExitUsage, fmt.Sprintf("--owner %q is not a declared handle", h), hint)
			os.Exit(ExitUsage)
		}
	}
	return epicOwnerFilter{handles: handles, unowned: unowned, declared: declared}
}

// on reports whether f filters at all.
func (f epicOwnerFilter) on() bool {
	return len(f.handles) > 0 || f.unowned
}

// keeps reports whether an entry with owner passes f.
func (f epicOwnerFilter) keeps(owner []string) bool {
	if !f.on() {
		return true
	}
	if !slices.ContainsFunc(owner, func(h string) bool { return slices.Contains(f.declared, h) }) {
		return f.unowned
	}
	return slices.ContainsFunc(owner, func(h string) bool { return slices.Contains(f.handles, h) })
}

// epicOwnerKept keeps the items f keeps, in their order.
func epicOwnerKept[T any](f epicOwnerFilter, items []T, ownerOf func(T) []string) []T {
	if !f.on() {
		return items
	}
	return slices.DeleteFunc(slices.Clone(items), func(it T) bool { return !f.keeps(ownerOf(it)) })
}

// filterReady keeps r's ready, claimed and condition entries f keeps. Gates
// and threshold rewrites belong to milestones and are never filtered.
func (f epicOwnerFilter) filterReady(r *epicReadyResult) {
	r.Ready = epicOwnerKept(f, r.Ready, func(e epicReadyItem) []string { return e.Owner })
	r.Claimed = epicOwnerKept(f, r.Claimed, func(e epicClaimedEpic) []string { return e.Owner })
	r.Operator.Conditions = epicOwnerKept(f, r.Operator.Conditions, func(e epicOperatorCondition) []string { return e.Owner })
}

// filterStatus keeps, in each milestone of r, the ready, claimed, blocked
// and condition entries f keeps. Counts, share done, gates and critical
// paths describe the milestone and stay whole.
func (f epicOwnerFilter) filterStatus(r *epicPlanResult) {
	for i := range r.Milestones {
		m := &r.Milestones[i]
		m.Ready = epicOwnerKept(f, m.Ready, func(e epicReadyItem) []string { return e.Owner })
		m.Claimed = epicOwnerKept(f, m.Claimed, func(e epicClaimedEpic) []string { return e.Owner })
		m.Blocked = epicOwnerKept(f, m.Blocked, func(e epicBlockedEpic) []string { return e.Owner })
		m.Operator.Conditions = epicOwnerKept(f, m.Operator.Conditions, func(e epicOperatorCondition) []string { return e.Owner })
	}
}
