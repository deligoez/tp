package engine

import (
	"fmt"
	"os/exec"
)

// Commit-strategy names. commit_strategy resolves to one of these; the effective
// committing behavior after auto-detection is only ever builtin or hc (§5.1).
const (
	CommitStrategyBuiltin = "builtin"
	CommitStrategyAuto    = "auto"
	CommitStrategyHC      = "hc"
)

// ResolveCommitStrategy resolves the commit_strategy NAME (builtin, auto, or hc)
// from the layered overrides by task-override > project > built-in precedence,
// with auto as the built-in default. A value PRESENT at the highest-precedence
// layer but outside the three names — including a pre-0.28.0 free-form
// placeholder such as "squash" — resolves to builtin, distinct from an absent
// value (every layer nil), which resolves to the auto default. When a present
// value is unrecognized the returned warning is non-empty so the caller can
// print a one-line stderr notice; the command still exits 0 (§5.1, §5.2).
func ResolveCommitStrategy(taskOverride, project *string) (name, warning string) {
	picked := pickString([]*string{taskOverride, project}, CommitStrategyAuto)
	switch picked {
	case CommitStrategyBuiltin, CommitStrategyAuto, CommitStrategyHC:
		return picked, ""
	default:
		return CommitStrategyBuiltin, fmt.Sprintf("warning: unrecognized commit_strategy %q; using builtin", picked)
	}
}

// EffectiveCommitStrategy resolves the concrete committing behavior — builtin or
// hc — for a resolved strategy name, given whether hc is available on PATH. An
// auto name resolves to hc when hc is present, otherwise to builtin. An explicit
// hc name stays hc regardless of presence: tp never runs hc (§5.3), so hc's
// absence changes no behavior, only the courtesy warning. A builtin name (and
// any value already mapped to builtin) stays builtin. hcMissing is true only
// when the effective strategy is hc while hc is absent, so the caller can print
// a one-line courtesy warning that the agent — not tp — will fail to run hc.
func EffectiveCommitStrategy(name string, hcAvailable bool) (effective string, hcMissing bool) {
	switch name {
	case CommitStrategyHC:
		return CommitStrategyHC, !hcAvailable
	case CommitStrategyAuto:
		if hcAvailable {
			return CommitStrategyHC, false
		}
		return CommitStrategyBuiltin, false
	default:
		return CommitStrategyBuiltin, false
	}
}

// HasHC reports whether the hc (hunk-commit) binary is on PATH. It is the probe
// the auto strategy uses to decide its effective behavior; tp itself never
// invokes hc (§5.3).
func HasHC() bool {
	_, err := exec.LookPath("hc")
	return err == nil
}
