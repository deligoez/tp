package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// groundResult is what `tp ground <spec>` prints: the round it emitted, the two
// files it wrote, and the one prompt (§7.1) with the scratch file that prompt
// tells its unit to write.
//
// Prompt is a string and not a slice. Grounding asks one question of every unit
// of one floor, so there is no panel to fan out over (§7.1, Non-Goal 4), and a
// one-element array would invite a caller to loop over a set that cannot grow.
type groundResult struct {
	Spec       string `json:"spec"`
	Round      int    `json:"round"`
	Snapshot   string `json:"snapshot"`
	Floor      string `json:"floor"`
	OutputPath string `json:"output_path"`
	Prompt     string `json:"prompt"`
}

func newGroundCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ground <spec.md>",
		Short: "Check a spec's claims against the world, before review is told they hold",
		Long: `Emit one prompt asking for a disposition on every unit of the spec's floor.

tp lint checks the document's form, tp validate checks the plan against the spec,
and tp audit checks the code against the spec. None of them checks the spec against
the world — review is explicitly told the spec is complete and authoritative. This
is what makes that true before it is said.

The emission writes the spec's text as this round reads it, plus the index derived
from that text, into spec/.tp-review/<base>/. Editing the spec afterwards does not
change the floor the round is graded against.`,
		Args:              cobra.ArbitraryArgs,
		DisableAutoGenTag: true,
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				output.Error(ExitUsage, "spec path required")
				os.Exit(ExitUsage)
				return nil
			}
			return runGround(args[0])
		},
	}
	return cmd
}

// runGround emits one ground round: it writes the snapshot and the floor index
// derived from it (§7.3), and prints the prompt naming ground-r<N>.ndjson.
func runGround(specPath string) error {
	data, err := os.ReadFile(specPath)
	if err != nil {
		if os.IsNotExist(err) {
			// First contact with the path: a spec-path mistake, so the shared
			// hint rather than the code-3 default's task-file advice.
			output.Error(ExitFile, fmt.Sprintf("spec not found: %s", specPath), specFileMissingHint)
			os.Exit(ExitFile)
			return nil
		}
		output.Error(ExitFile, fmt.Sprintf("cannot read spec: %s", specPath), err.Error())
		os.Exit(ExitFile)
		return nil
	}
	text := string(data)

	round, err := engine.NextGroundRound(specPath)
	if err != nil {
		// An unreadable state directory, which NextGroundRound refuses to
		// answer 1 for: emitting under a guessed number would hand this round
		// an identifier an existing one already holds.
		output.Error(ExitFile, fmt.Sprintf("cannot read the state directory for %s: %v", specPath, err),
			"check the permissions on spec/.tp-review/<base>/")
		os.Exit(ExitFile)
		return nil
	}

	// The index is derived from the bytes written as the snapshot — one read,
	// so the floor and the text it claims to be over cannot disagree.
	index := engine.FormatFloorIndex(groundCommit(specPath),
		engine.FloorIndexRows(text, engine.FloorAnchorOf(text)))

	if err := engine.WriteGroundEmission(specPath, round, data, []byte(index)); err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot write the ground round's artifacts: %v", err),
			"check that spec/.tp-review/<base>/ is writable")
		os.Exit(ExitFile)
		return nil
	}

	snapshotPath := engine.GroundSnapshotPath(specPath, round)
	outputPath := fmt.Sprintf("ground-r%d.ndjson", round)
	return output.JSON(groundResult{
		Spec:       specPath,
		Round:      round,
		Snapshot:   snapshotPath,
		Floor:      engine.GroundFloorPath(specPath, round),
		OutputPath: outputPath,
		Prompt:     buildGroundPrompt(specPath, snapshotPath, index, outputPath, round),
	})
}

// groundCommit is the commit §2.2 puts on the index's first line, discovered
// here and passed to the renderer: a formatter that shelled out to git would
// produce output no test could state. An answer git cannot give is "", which
// FormatFloorIndex renders as `unknown`.
func groundCommit(specPath string) string {
	sha, err := gitHeadShortSHA(filepath.Dir(specPath))
	if err != nil {
		return ""
	}
	return sha
}

// groundKindSubject is §4.1's "the claim is about" column, keyed by kind. It is
// prose for a reader and nothing reads it back; the normative column — which
// tiers are acceptable — is derived from the engine's own sets below rather
// than restated here, so the prompt cannot tell a unit a rule the recorder does
// not enforce.
var groundKindSubject = map[engine.GroundKind]string{
	engine.KindDocument:      "what a document says",
	engine.KindCodeStructure: "how code is structured",
	engine.KindCorpus:        "what the corpus contains",
	engine.KindBehaviour:     "existing behaviour",
	engine.KindMechanism:     "whether a proposed mechanism works",
	engine.KindDefect:        "whether a defect is real",
	engine.KindGuard:         "whether a guard measures anything",
}

// groundTierDid is §4.1's second table: what each tier means the unit did.
var groundTierDid = map[engine.GroundTier]string{
	engine.TierRead:            "read the artifact",
	engine.TierQuery:           "ran a query over the corpus",
	engine.TierRun:             "ran the shipped command",
	engine.TierProbe:           "built a probe and ran it",
	engine.TierRedGreen:        "wrote the test, watched it red, fixed, watched it green",
	engine.TierBreakAndControl: "broke the subject, ran the suite, THEN ran the control",
}

// groundVerdictMeaning is §3's table: what each of the six verdicts says.
var groundVerdictMeaning = map[engine.GroundVerdict]string{
	engine.VerdictPass:         "evidence at a tier acceptable for the claim's kind supports it",
	engine.VerdictPartial:      "true under one reading and not another, or the conclusion holds while the stated reason does not, or it was true when written",
	engine.VerdictFail:         "evidence at an acceptable tier contradicts it",
	engine.VerdictUnverifiable: "no acceptable tier is reachable at all; tier records the deepest one attempted",
	engine.VerdictQuestion:     "the attempt did not settle it, either because the result was unclear or because the tier it reached is not one the kind accepts",
	engine.VerdictNotAClaim:    "the unit is a decision, a prediction, or prose carrying no assertion about the world",
}

// buildGroundPrompt renders the single prompt an emission carries: what a row
// is, which evidence answers which kind of claim, and the floor index every one
// of those rows is owed for.
//
// The index travels whole and the spec's text does not. §2.2 measured inlining
// the floor at 898,131 bytes against the index's 157,654, and the prefix shapes
// that tried to split the difference lost the one thing a prefix cannot carry —
// where a unit ends — which is why the byte length is on every row instead.
//
// The three sections are three functions because the prompt is long enough to
// trip the funlen ratchet as one, and its seams are its own headings.
func buildGroundPrompt(specPath, snapshotPath, index, outputPath string, round int) string {
	return appendClausesGround(
		groundPromptRow(specPath, snapshotPath, round) +
			groundPromptEvidence() +
			groundPromptFloor(index, outputPath))
}

// groundPromptRow opens the prompt and states the row: what the unit is being
// asked, and the six verdicts it may answer with.
func groundPromptRow(specPath, snapshotPath string, round int) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Ground round %d — check this spec's claims against the world\n\n", round)
	fmt.Fprintf(&b, "Spec: %s\nText this round reads: %s (the spec's bytes as of this emission)\n\n", specPath, snapshotPath)
	b.WriteString(`Every claim in this document is a premise the next review round is told to treat
as settled. Your job is to decide, for EVERY unit of the floor below, what it is
worth against the world outside the document. You are not reviewing the spec:
wording, coherence and design are the next phase's subject, not yours.

## The row

One JSON object per line. Copy unit_id, anchor, text_sha and ordinal from the
index row unchanged — they are how a later round joins your disposition to the
sentence you graded. Required on every row: unit_id, anchor, text_sha, ordinal,
verdict. An unknown top-level key is rejected, and one rejected row refuses the
whole round.

verdict — exactly one of:
`)
	for _, v := range engine.GroundVerdicts() {
		fmt.Fprintf(&b, "  %-13s %s\n", v, groundVerdictMeaning[v])
	}
	b.WriteString(`
kind, tier — required unless the verdict is NOT-A-CLAIM, where the pair is
  optional: a unit that states a decision AND a checked fact records both.
evidence — required whenever tier is present: the command you ran, or the
  artifact and line you read. Free text, because a later reader re-runs it.
partial_kind — required on PARTIAL: two-readings, reason-not-conclusion, or
  true-when-written.
held_at — required when partial_kind is true-when-written: the commit, tag or
  date at which the claim held.
causes — required on QUESTION: three to five {"cause": ..., "prediction": ...}
  objects, ranked, each prediction an observation that would confirm or kill its
  cause.
note — free text, optional.
unit_id is null on a claim you add that the floor did not carry; supply the
  text_sha yourself, over the claim's own text.
`)
	return b.String()
}

// groundPromptEvidence renders §4.1: what each tier means, which tiers say
// anything about which kind of claim, and §7.2's per-verdict reading of that
// rule.
//
// The kind–tier column is derived from TierAcceptableFor rather than written
// out, because that predicate is what rejects a row at --record: a prompt
// stating the rule in its own words can drift from the recorder, and the unit
// pays for the drift with a refused round.
func groundPromptEvidence() string {
	var b strings.Builder

	b.WriteString(`
## Which evidence answers which claim

tier records what you ACTUALLY did, never what the kind wanted:

`)
	for _, tier := range engine.GroundTiers() {
		fmt.Fprintf(&b, "  %-18s %s\n", tier, groundTierDid[tier])
	}
	b.WriteString(`
The tiers are NOT ordered. Past ` + "`run`" + ` a "deeper" tier is not more of the same
evidence but evidence about a different subject: probe, red-green and
break-and-control rank rigour on an artifact you built, while read, query and run
examine the real one. So acceptability is a set per kind, not a threshold:

`)
	for _, kind := range engine.GroundKinds() {
		acceptable := make([]string, 0, len(engine.GroundTiers()))
		for _, tier := range engine.GroundTiers() {
			if engine.TierAcceptableFor(kind, tier) {
				acceptable = append(acceptable, string(tier))
			}
		}
		fmt.Fprintf(&b, "  %-15s %-36s %s\n", kind, groundKindSubject[kind], strings.Join(acceptable, ", "))
	}
	b.WriteString(`
On PASS, PARTIAL and FAIL the tier MUST be one of the kind's, or the row is
rejected. On QUESTION either relation is legal — reaching an acceptable tier and
not being settled by the result is one shape of question, not reaching one is the
other. On UNVERIFIABLE the tier names the deepest attempt.

## Where to start, and what to do when a claim resists

Start with the cheap sentences: a number borrowed from another context, a rule
left unstated, a reason that sounds right. That is where the defects were. A claim
whose verdict depends on an unsettled claim belongs to a later pass.

Before concluding on a claim your first attempt did not settle, name three to five
falsifiable causes with their predictions and test them in rank order. Find facts
yourself — two things reach the operator and nothing else: a fact only the author
holds, and a decision. A QUESTION does not stop the round; carry on with every
claim that does not depend on the answer.
`)
	return b.String()
}

// groundPromptFloor carries the emitted index whole, says how to read a row,
// and names the file this round's rows go to.
func groundPromptFloor(index, outputPath string) string {
	var b strings.Builder

	b.WriteString("\n## The floor\n\n")
	b.WriteString(index)
	fmt.Fprintf(&b, `
Each row is `+"`<unit_id> <anchor> <text_sha> #<ordinal> <bytes>B`"+`, and a row ending
in `+"`(cut)`"+` is a unit the arms dropped: it owes no disposition. The index carries
no unit text; the text is in the snapshot named at the top of this prompt. A unit
is one sentence of it, canonicalised — its wrapped lines joined, whitespace
collapsed to single spaces, a list or blockquote marker dropped, and a table row's
cells joined with an em dash; `+"`<bytes>`"+` is that text's length in UTF-8 bytes, which
is how you tell where it ends.

This is a FLOOR, not the set of claims. A claim it missed — including one inside a
cut unit — is recorded with "unit_id": null and is reported apart from coverage.

Write this round's rows to: %s
`, outputPath)

	return b.String()
}
