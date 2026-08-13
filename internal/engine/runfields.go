package engine

import "encoding/json"

// §7's run workflow fields: their built-in defaults and their inclusive ranges.
// Both endpoints are legal values; one step outside is out of range and falls
// back to the default (clampWorkflowRanges), so the bounds are named once here
// rather than repeated at each surface that has to honour them.
const (
	RunMaxUnitsDefault = 100
	RunMaxUnitsMin     = 1
	RunMaxUnitsMax     = 10000

	RunMaxWallClockSecondsDefault = 28800
	RunMaxWallClockSecondsMin     = 60
	RunMaxWallClockSecondsMax     = 604800

	// A budget of 0 is the documented "disabled" value, which is why 0 is the
	// low end of the range rather than below it.
	RunMaxBudgetUSDDefault = 0.0
	RunMaxBudgetUSDMin     = 0.0
	RunMaxBudgetUSDMax     = 10000.0

	// 0 omits the per-unit budget flag entirely rather than passing a literal 0.
	RunMaxUnitBudgetUSDDefault = 0.0
	RunMaxUnitBudgetUSDMin     = 0.0
	RunMaxUnitBudgetUSDMax     = 1000.0

	// The default of 1 gives every unit two attempts.
	RunMaxUnitRetriesDefault = 1
	RunMaxUnitRetriesMin     = 0
	RunMaxUnitRetriesMax     = 5

	// RunnerDefault is the built-in template name the runner field resolves to
	// when no layer sets it. The field's three shapes are told apart by the
	// runner resolver, not by this layer.
	RunnerDefault = "claude"
)

// DefaultRunner returns the resolved runner value for a workflow no layer sets:
// the built-in template name as raw JSON. It is built per call so no caller can
// mutate a shared buffer.
func DefaultRunner() json.RawMessage {
	return json.RawMessage(`"` + RunnerDefault + `"`)
}
