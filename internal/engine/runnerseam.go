package engine

import (
	"os"
	"slices"
	"strings"
)

// EnvRunnerSeam is §3.2.1's test seam: when set, its value is the cmd of the
// runner every unit spawns, whatever the repo's configuration says.
//
// The name is deliberately not TP_RUNNER. That would be the fenced env layer of
// the runner workflow field (§5.1), an ordinary layer of §7's precedence; this
// is a test-only override that sits above all of them. Two different things
// deserve two different names, and the one that outranks a CLI flag is the one
// that must never be reachable by mistake.
const EnvRunnerSeam = "TP_RUNNER_SEAM"

// seamArgs is the seam runner's argument template (§3.2.1): the four values a
// fake runner needs to identify the unit it was spawned for, positionally, so
// the fake needs no flag parsing at all.
var seamArgs = []string{
	"{" + placeholderUnitKind + "}",
	"{" + placeholderUnitID + "}",
	"{" + placeholderLogPath + "}",
	"{" + placeholderMaxBudgetUSD + "}",
}

// SeamRunner returns the runner TP_RUNNER_SEAM pins, and whether the seam is
// set at all.
//
// It is read from tp's own environment — the driver's, never a child's — so a
// runner.env entry cannot introduce one and a child cannot pin its own
// grandchildren by accident. A value that is absent or blank is no seam: a
// runner with nothing to spawn is not a runner, and an exported-but-empty
// variable is the ordinary way a shell produces one.
//
// The spend_key is the same key the claude template declares, which is what
// makes the spend (§3.2.2) and budget-cap paths testable without an agent: the
// fake runner writes a final log line carrying it.
func SeamRunner() (*Runner, bool) {
	cmd := strings.TrimSpace(os.Getenv(EnvRunnerSeam))
	if cmd == "" {
		return nil, false
	}
	// The args are cloned, so a caller that appends to the returned runner
	// cannot edit the package-level template the next unit will read.
	return &Runner{Cmd: cmd, Args: slices.Clone(seamArgs), SpendKey: claudeSpendKey}, true
}
