package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// guardInFlightEmission is the refusal tp ground, tp review and tp audit make
// before an emission writes anything: overwrites names the files a re-emission
// of the in-flight round would replace with different text (the engine's
// EmissionOverwrites / GroundEmissionOverwrites), and a non-empty list means
// the spec changed after that round was emitted and before it was recorded.
//
// Re-emitting used to replace them silently at exit 0, so the round's units
// were grading a text that no longer existed on disk and --record then scored
// their rows against the new one. Refusing makes the discard a decision:
// record the round first, or pass --force to discard its emission, which is
// said on stderr because the files it overwrote are not in the payload.
//
// It exits 3 — the conflict is about files on disk — and returns only when the
// emission may proceed. Under TP_UNATTENDED a --force that would discard exits
// 2 instead (EscalateDiscardEmission); a --force with nothing to discard is
// accepted there as everywhere. phase is the command name the hint's --record
// names.
func guardInFlightEmission(phase, specPath string, round int, overwrites []string, err error, force bool) {
	if err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot read round %d's in-flight snapshot for %s: %v", round, specPath, err),
			"check the permissions on spec/.tp-review/<base>/: tp will not overwrite a round it cannot read")
		os.Exit(ExitFile)
		return
	}
	if len(overwrites) == 0 {
		return
	}
	files := strings.Join(overwrites, ", ")
	if force && engine.Unattended() {
		// Under a run, sibling role units grade one emission concurrently,
		// so a unit's discard pulls the text out from under the others.
		// Recovering a crashed round is the operator's moment.
		refuseUnattended(fmt.Sprintf("--force discarding round %d's unrecorded emission", round), engine.EscalateDiscardEmission)
		return
	}
	if force {
		output.Notice(fmt.Sprintf("--force: discarding round %d's unrecorded emission; overwriting %s with the spec as it now stands", round, files))
		return
	}
	output.Error(ExitFile,
		fmt.Sprintf("round %d is in flight and unrecorded, and the spec changed since its emission: emitting again would overwrite %s", round, files),
		fmt.Sprintf("record round %d first (tp %s %s --record <file>), or pass --force to discard its emission and emit over the spec as it now stands", round, phase, specPath))
	os.Exit(ExitFile)
}
