# tp v1.0.1 — tp says what round 1 will read

> **This file is the first instance of the rule it ships.** It carries decisions and, for every
> quantity it reports, the command that derives it; its measurements are in
> `spec/1.0.1-measurements.md`, which `tp ground` does not grade. That shape came out of grading the
> release that is now `spec/1.0.2.md`: counting its graded rows by `kind` put the highest finding rate
> on design claims and a near-worthless one on re-derived figures — correct findings about sentences
> that, unwritten, would have offered nothing to find.

## 1. The decision

tp reports what round 1 of a spec will cost to grade and makes no judgement about it. No threshold, no
warning, no gate, no config field.

**The restraint is measured rather than cautious, and the measurement removed a feature this file
originally carried.** A floor-budget warning was designed and then dropped: over the converged shipped
specs, round-1 floor size does not predict how many rounds a cycle took. Within each class it fails
separately — the two within-class correlations have opposite signs — and within the class most of the
backlog belongs to, line count predicts the outcome far better than floor does. Derive it:

```
for v in $(git tag --list 'v*' | sort -V); do
  git show "v$v:spec/.tp-review/${v#v}/snapshot-round-1.md" > /tmp/s.md 2>/dev/null || continue
  echo "$v $(cd /tmp && tp ground s.md --units | grep -c '^u[0-9]') \
    $(ls spec/.tp-review/${v#v}/{review,audit}-round-*.ndjson 2>/dev/null | wc -l)"
done
```

`spec/1.0.1-measurements.md` §1 carries the stratification and why the pooled coefficient this file
originally quoted was the wrong statistic to quote.

What floor size does measure is one round's grading cost, which is a different quantity: total cost is
rounds times cost-per-round, and the two have different drivers. tp reports the one it can compute and
stays silent about the one it cannot.

## 2. What `tp lint` adds

**Lint defines no new name.** Every count below is ground's, with ground's meaning, from the same
function — `groundFloorSize` over `engine.FloorIndexRows`, which `internal/cli/ground.go` already
calls. A second implementation would be a second splitter, and a second name for the same quantity is
worse than a second implementation because it disagrees silently.

| field | what it is |
|---|---|
| `floor_size` | uncut units, exactly what `tp ground` already reports under this name |
| `cut` | units the arms produced no gradeable text for, ground's term |
| `floor_by_section` | `floor_size` per section, summing to `floor_size` |
| `floor_figure_share` | share of uncut units carrying a digit or a backticked span |
| `spec_bytes` | the spec's size |
| `review_roles` | roles the round-1 review panel emits |
| `audit_roles` | roles the round-1 audit panel emits |

The last three are inputs, not a product. A reader who wants the per-round read multiplies them; tp
supplies the terms because it can resolve them and cannot know what the reader is asking. The role
counts come from the emission's own panel resolver, and they are the **round-1** panel: `regression`
is not emitted in round 1 and is therefore not counted.

**Why the two role counts are separate.** Measured: with one reviewer deactivated for a spec, the
review panel falls while the audit panel does not — the two are resolved from different corpora and
coincide only when a repository's reviewer and auditor sets happen to be the same size.

**Why `cut` is reported at all.** This repository's standing lesson is that the sharpest finding has
repeatedly sat in a unit the arms cut. A rising cut share is a widening blind spot rather than a
cheaper round, and reporting it beside `floor_size` is what stops one being read as the other.

**Lint reports what round 1 will read, and nothing about later rounds.** Lint has no round. What a
given round asks is `tp ground`'s to say, and it already says it — its emitted prompt names how many
of the floor's units the round owes a disposition for, the rest carrying one forward.

**A line that only points at an artifact is not a claim and is not counted** — `see spec/x-measurements.md §2`
or a bare commit SHA carries no assertion to grade. This is a lint-side filter on what `floor_size`
reports, not a change to `floorBlocks`; ground's floor is unchanged.

## 3. The fence and the inline span

`CheckVagueLanguage` is the only rule in `internal/engine/vague.go` that does not skip fenced code
blocks; its four siblings each track the delimiters. It also scans inline code spans. Both make the
rule fire on documentation *of itself* — on a sample review finding whose text names the trigger word,
inside a fence, and on a citation of the trigger word in backticks. Derive the first:

```
tp lint spec/0.12.0-review-rounds.md | python3 -c \
  'import json,sys; [print(x["line"], x["message"]) for x in json.load(sys.stdin)["findings"] if x["rule"]=="vague-language"]'
```

Both exclusions land, matching the siblings exactly: an unterminated fence is open to end of file,
because that is what the siblings do and a third behaviour here would be a new asymmetry rather than a
repair of the existing one. Scope stays backtick fences; `~~~` and indented blocks are out, because
the siblings do not track them either and widening one rule alone is the asymmetry again.
## 4. What ships in `skills/tp/SKILL.md`

The rules are the release, as much as the code is — they reach users through the plugin and the skill
package. Three are new; a fourth already shipped and is named here only as precedent, not as work.

- **A number does not live in a spec. A reference does.** Derivations go in `<base>-measurements.md`,
  which ground does not grade; the spec names the artifact and does not quote the figure, and a
  rationale that cites a figure is a figure.
- **The body is ADR-shaped**: decision, why, consequences, and a link to supplemental material — the
  same split, and the same reason, which is that the decision must stand without the material.
- **Write a test row's mutant before its assertion, and drop the row if the mutant cannot make it
  red.** Measured across two cycles: one row in three named a mutant that survives. EARS shape alone
  did not move that rate — the shape helps a reader, the ordering is what makes the row a test.
- **A check prototype states how it judged its findings** — which it read one by one and which it
  sampled and judged by class. A prototype that reports a hit count without that distinction is not a
  result.
- Already shipped, cited as precedent: a finding leaves a round as a spec change or as a `--resolve`
  disposition, and the prose answer belongs in the disposition's evidence.

## 5. Non-Goals

1. **No threshold and no gate.** §1 gives the measurement that removed them.
2. **No claim that any of this shortens a cycle.** What is checkable is narrower: no tp command
   reports these quantities before a round runs, so an author cannot see them without running one.
   Whether seeing them changes what an author writes is not measured, and this release does not
   assert it.
3. **No new smell rules.** Six candidates were prototyped against this repository's own corpus and all
   six were refuted; `spec/undecided.md`'s `## Refuted` carries each measurement and the single
   condition under which they reopen. §3's fence repair is a defect in the rule that already ships,
   not a new rule.
4. **No class field.** A draft carried one, on the ground that tp cannot infer a spec's class; that
   was refuted by construction, and the field would have been empty on the day it shipped.
   `spec/undecided.md` carries the predicate and the design a later release would take.
5. **No EARS requirement on spec bodies.** Test rows only.
6. **No token figure.** `floor_size` is units, not tokens. One spec's rounds are the only evidence for
   a conversion; a second cycle's measurement would earn it, and it would go in `SKILL.md` rather than
   into tp.
7. **Not the review side's per-section churn signal**, which needs a finding history keyed by section:
   `spec/backlog/12-repair-locality.md`.

## 6. Tests

Each row is `WHEN`/`WHILE`/`IF <trigger>`, `tp SHALL <response>`, with the mutant that must fail it.
A row whose mutant survives its own assertion is not in this table — three were written and dropped
for that reason rather than reworded.

| # | from | requirement | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | WHEN `tp lint <spec>` and `tp ground <spec>` run over the same file, tp SHALL report the same `floor_size` | count the floor index's rows in lint — that is the cut-inclusive population, and on a spec carrying cut units the two numbers differ |
| 2 | §2 | WHEN a spec has sections, tp SHALL report `floor_by_section` values that sum to `floor_size` | count one population per section and another overall, which is the conflation this split exists to prevent |
| 3 | §2 | WHEN a reviewer is deactivated for a spec, tp SHALL report a lower `review_roles` and an unchanged `audit_roles` | resolve one role count for both phases; the two coincide only when the corpora are the same size, which they are here |
| 4 | §2 | WHILE round 1 is the emitted round, tp SHALL exclude `regression` from `review_roles` | count the corpus rather than the panel — `regression` is emitted from round 2 |
| 5 | §3 | WHEN a vague word appears inside a fenced block, tp SHALL report nothing for it | scan every line; the fixture is a sample finding whose own text names the trigger word, which is what the shipped rule reports today |
| 6 | §3 | WHEN a vague word appears inside an inline code span, tp SHALL report nothing for it | blank only fenced blocks, leaving the rule unable to appear in a document that names its own triggers |
| 7 | §3 | WHEN a vague word appears in prose before and after a fenced block, tp SHALL report both | skip everything after the first fence opens, which passes rows 5 and 6 and reports nothing at all |
| 8 | §3 | IF a fence never closes, THEN tp SHALL treat it as open to end of file | invent a third behaviour here; the assertion is that this rule agrees with its four siblings, not that the behaviour is ideal |
