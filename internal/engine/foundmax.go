package engine

// found_max caps the found tasks one spec's task file may ever hold
// (spec/1.31.0.md §8). 0 is uncapped, as for review_max_rounds, so raising
// it or setting 0 is the operator's decision (EscalateRaiseFoundCap).
const (
	FoundMaxDefault = 5
	FoundMaxMin     = 0
	FoundMaxMax     = 100
)
