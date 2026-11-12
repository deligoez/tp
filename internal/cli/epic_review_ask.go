package cli

import (
	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
)

// epicUnitKinds are the unit kinds a graph role may be asked, in the order a
// prompt names them: how the finding contract locates each, and the noun,
// article form and plural the "No ... unit is asked" sentence uses.
var epicUnitKinds = []struct {
	kind, location, noun, article, plural string
}{
	{model.RoleUnitEpics, `"epic:<id>" for an asked epic`, "epic", "an epic", "epics"},
	{model.RoleUnitGates, `"gate:<milestone>" for the asked gate`, "gate", "a gate", "gates"},
	{model.RoleUnitScope, `"scope:<milestone>" for the asked scope unit, the gate taken whole`, "scope", "a gate's scope", "scope units"},
}

// epicRoleAsk is what one role is asked this round: its kinds, and in slice
// order the units of those kinds the carry asks, plus, for a role with
// purpose: true, those whose milestone's purpose changed
// (engine.EpicCarry.PurposeStale). This per-role set is the only place a
// role's asked units live; the carry itself stays role-blind.
type epicRoleAsk struct {
	kinds map[string]bool
	units []engine.EpicReviewUnit
}

// epicAskOf builds one role's ask over run: the carry's asked units of the
// role's kinds, only those of carried's kinds when carried is not nil, and,
// when purpose is set, the purpose-stale ones of its kinds.
func epicAskOf(run *epicReviewRun, kinds []string, purpose bool, carried map[string]bool) epicRoleAsk {
	ask := epicRoleAsk{kinds: map[string]bool{}}
	for _, k := range kinds {
		ask.kinds[k] = true
	}
	want := map[string]bool{}
	for _, u := range run.slice.Units {
		if carried == nil || carried[engine.EpicUnitKind(u.Key)] {
			want[u.Key] = true
		}
	}
	if purpose {
		for _, u := range run.carry.PurposeStale {
			want[u.Key] = true
		}
	}
	for _, u := range run.all {
		if want[u.Key] && ask.kinds[engine.EpicUnitKind(u.Key)] {
			ask.units = append(ask.units, u)
		}
	}
	return ask
}

// markAsked adds the role's asked unit keys to asked.
func (a *epicRoleAsk) markAsked(asked map[string]bool) {
	for _, u := range a.units {
		asked[u.Key] = true
	}
}

// scopeOnly reports whether scope is the role's only kind.
func (a *epicRoleAsk) scopeOnly() bool {
	return len(a.kinds) == 1 && a.kinds[model.RoleUnitScope]
}

// holds reports whether the role is asked the unit key names.
func (a *epicRoleAsk) holds(key string) bool {
	for _, u := range a.units {
		if u.Key == key {
			return true
		}
	}
	return false
}

// rows returns the rows located on a unit the role is asked, in their order.
func (a *epicRoleAsk) rows(rows []map[string]any) []map[string]any {
	var out []map[string]any
	for _, row := range rows {
		if loc, _ := row["location"].(string); a.holds(loc) {
			out = append(out, row)
		}
	}
	return out
}

// carried returns, in slice order, the units of the role's kinds it is not
// asked this round.
func (a *epicRoleAsk) carried(run *epicReviewRun) []engine.EpicReviewUnit {
	var out []engine.EpicReviewUnit
	for _, u := range run.all {
		if a.kinds[engine.EpicUnitKind(u.Key)] && !a.holds(u.Key) {
			out = append(out, u)
		}
	}
	return out
}
