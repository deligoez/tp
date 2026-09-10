# refusals-that-name-nothing — measurements

Supplemental material for `refusals-that-name-nothing.md`; the spec stands without it. Every block
below was moved here verbatim from the spec on 2026-09-08, except that two filename citations to
renumbered specs were repointed at slug paths. Every mutant was built and run in an
`rsync -a --exclude .git ./ <copy>/` copy outside the repository.

On the same day the spec was split into six, one per decision that can ship alone. This sidecar was
split with it; **Split 2026-09-08** below maps every section that moved to the sidecar that took it.
The Preamble below is not duplicated into those files — they cite it here.

On 2026-09-11 the spec widened to *a refusal names each bad row, its cause and what it would have
accepted*; the sections from **Absorbed on 2026-09-11** onward record that pass. Their citations
carry line numbers at `18032abe`, as that pass requires for field evidence; the Preamble's
no-line-number rule governs the sections above them.

## Preamble

> **This file is decisions.** What v1.0.0's audit measured and did not repair — the split
> `spec/candidates.md` files them under, and §1 keeps it. Each of v1.0.0's defects was re-run
> against `HEAD` while writing this file rather than
> carried forward from the handover text, because a deferred finding is a claim about a tree that has
> since moved. **§6 is the exception and is not a re-derivation**: `spec/candidates.md` already
> carries that routing correction, in the same words and with the same search, and already re-routes
> the finding to this release. §6 says so.
>
> Every mutant below was built and run in `rsync -a --exclude .git ./ <copy>/`, outside the
> repository. **The check that the repository is unchanged has to have the repository as its
> subject**, and the one this preamble first carried did not:
> `diff -r --brief <copy>/internal ./internal` compares the copy against the tree, so it goes red
> whenever the *copy* still holds a mutant — and it did, on `internal/cli/review.go`, when it was
> re-run. What is asserted instead is `git status --porcelain -- internal/` printing nothing, beside
> a control on a path that *is* edited, which prints ` M <path>`: the empty output is then a reading
> rather than a silence.
>
> **No line number is cited anywhere below.** Round 1 found three line-anchored citations in this
> document pointing at the wrong text — one off by eleven lines, one at a doc comment rather than the
> assertion it names, one naming the wrong test entirely — so every citation here names a symbol or a
> quoted string and the file it lives in.

## §2 — the census of the `kind × tier` pairings

Moved out of the spec body at the 2026-09-08 split. The census was run over the shipped predicate
rather than counted by eye:

```go
// probe: for each kind × tier ask TierAcceptableFor; and which verdicts bind the rule
kinds=7 tiers=6 total=42 acceptable=9 refused_by_tier_rule=33
binds=PASS,PARTIAL,FAIL
```

So **33 of the 42 pairings a unit can write using only §4.1's own values are refused by the message
that names no set — on the three verdicts that bind the rule** — and the nine that pass are the whole
of the rule.

## §2 — the cost of the round trip

**The recovery costs a re-read of the emitted prompt, which is the argument v1.0.0 already accepted
for the other four cells.** The prompt does carry the per-kind sets — `groundPromptEvidence` in
`internal/cli/ground.go` renders them from `TierAcceptableFor` — so the information is reachable and
the cost is the round trip. **How many bytes that round trip is, this file quotes nowhere**, because
the figure is a property of the spec being grounded and not of tp. Derive it in the copy, since
`tp ground` writes a round snapshot beside the spec:
`tp ground spec/1.0.0.md --json | python3 -c 'import sys,json; print(len(json.load(sys.stdin)["prompt"].encode()))'`
— and note the path, because `tp ground 1.0.0.md`, the form this paragraph first carried, exits 3 with
`spec not found` and prints no number at all. The value moved at each of the six most recent commits
touching `spec/1.0.0.md` while `git log <that range> -- internal/` is empty, so the renderer never
changed and the figure tracks the spec's own growth. `internal/engine/groundrow_test.go`'s
enum-refusal doc comment states a **third** figure for the same quantity, measured at a third moment;
`guards-read-what-production-reads.md` edits that comment, and the stale figure is deleted with it
rather than re-measured.

## Implementation notes from the original body

Moved out of the spec body at the 2026-09-08 split: sentences describing how the unbuilt code works,
which the body replaced with the observable requirement.

**One derivation, two sinks.** `engine.AcceptableTiersFor(kind)` returns the kind's tiers filtered out
of `GroundTiers()`, so the order is §4.1's table order; `validateGroundRowTier` and
`groundPromptEvidence` both call it, and the per-kind filter loop currently inlined inside
`groundPromptEvidence` — the one walking `GroundKinds()` × `GroundTiers()` and asking
`engine.TierAcceptableFor` — is replaced by that call. This is a move, not a new abstraction: the sets
already exist, the filter already exists in `cli`, and putting it beside `TierAcceptableFor` is what
makes "the prompt and the refusal agree" a fact rather than a hope.

**The empty-set branch.** `AcceptableTiersFor` on a kind outside the seven returns nothing. The tree's
only direct calls to `validateGroundRowTier` outside `ParseGroundRow` are the two in
`TestAVerdictOutsideTheSixIsHeldToTheTierRule` (`internal/engine/groundtierrule_test.go`), and both
pass `Kind: KindBehaviour`, whose acceptable set is `{run, red-green}`; the out-of-enum value there is
the *verdict*, not the kind. The branch says that no tier is acceptable for that kind.

## Decided at the 2026-09-08 decision pass

From `spec/undecided.md`, *Making `severity` checkable*. It lands on this spec because a refusal that
names the row is its subject.

**Decided: severity stays self-declared.** What makes it trustworthy enough to gate on is the forced
commitment in the brief, not a validator — the mechanism this repository has measured repeatedly, and
the one `spec/backlog/brief-carries-the-forcing-sentences.md` ships.

**Decided: the vocabulary is validated at the record sink, as a warning that names the row** — not a
rejection. Rejection would refuse history: **fifteen of one hundred twelve** audit round files are
off-vocabulary today, under the counting rule carried with the entry in `spec/undecided.md` (a round
file holding at least one row whose `severity` is present and outside its phase's vocabulary). That is
the same shape as this spec's other refusals — say what was refused and against which set — rather
than a silent drop or a hard stop.

The asymmetry this removes: `invalidCategoryRows` in `internal/cli/audit_record.go` validates
**`category`** alone today. The reader that makes severity load-bearing is
`spec/backlog/a-finding-can-leave-an-audit-round.md`, whose test row 1b grades acceptance from the
row's `severity` under `audit_converge_on: blocking`.

## Routed here from the v1.1.1 release (2026-09-08)

Not from an audit round — found while releasing, which is why it has its own heading rather than
being added to a round's list and falsifying that list's own count.

- **A refusal whose remedy names a version that does not fix it.** `hooks/session-start.sh` reads its
  minimum tp version from `.claude-plugin/plugin.json` (:86) and fails at exit 2 when the installed
  binary is below it (:97-99). Between the v1.1.0 tag and the v1.1.1 tag the manifest said `1.1.1`
  while the newest released binary was `v1.1.0`. **Run, with a discriminating control:** manifest
  1.1.1 and PATH `tp` v1.1.0 → **exit 2**, stdout **empty**, stderr
  `tp v1.1.0 at … is older than the tp plugin's minimum 1.1.1.`; the same tree with the manifest
  edited to 1.1.0 → **exit 0** and `tp resume --compact` prints. So the manifest value alone decides
  it. The refusal is correct and its message is accurate; what names nothing actionable is the
  **remedy** it prints — `go install github.com/deligoez/tp/cmd/tp@latest`, where `@latest` resolved
  to `v1.1.0`, the very version being refused. A user following the printed instruction would have
  stayed refused, with no second thing to try.
- **The guard pair permits that state by design, so only a shipped binary closes it.**
  `TestPluginVersionIsNotBehindTheLatestTag` asserts the manifest is **not behind** the newest tag, so
  a manifest ahead of the released binary satisfies it; `TestPluginVersionIsBumpedWhenPluginContentChanges`
  *requires* the bump the moment anything under `.claude-plugin/`, `skills/`, `hooks/` or `agents/`
  differs from the tag. Together they demand the bump and permit the window it opens. Both passed
  throughout. `CLAUDE.md` already states the rule neither enforces — *"the release must ship a binary
  at or above it"* — and v1.1.1 is what closed it, the manifest and the binary shipping together.
- **The window was reachable but never live, and the claim that it was live was wrong.** It was
  reported as already on `origin/main` on the strength of that branch's SHA; `git show
  972b62fc:.claude-plugin/plugin.json` is **1.1.0**, because that commit is the revert. The bump
  commit was local until the v1.1.1 push, so no user was ever offered a manifest ahead of a binary.
  Recorded because the error is this spec's neighbouring class in the reader rather than the code:
  **a commit's SHA is not its content**, and checking the pointer instead of what it points at
  produces a confident answer to a question nobody asked.

## Split 2026-09-08

| was | now |
|---|---|
| Preamble | this file — the five other sidecars cite it here rather than copying it |
| *§2 — the cost of the round trip* | this file |
| *§3 — measured, both directions* | `the-guard-pins-the-whole-listing-measurements.md` |
| *§4 — §7.2's `meaning` column* | `guards-read-what-production-reads-measurements.md` |
| *§5.1 — how far the drop reaches through the shipped command* | `an-unreadable-file-is-named-measurements.md` |
| *§6 — a routing correction, and who actually made it* | `an-unreadable-file-is-named-measurements.md` |
| *§6 of the spec — `checkTaskFileQuality`, measured* | `an-invalid-task-file-is-reported-measurements.md` |
| *§7 of the spec — `Context` truncated by bytes, measured* | `context-is-cut-on-a-rune-boundary-measurements.md` |
| *From the guard spec* and its two subsections | `guards-read-what-production-reads-measurements.md` |
| *Routed here at the 2026-09-08 re-verification* — the mistyped override key, the over-long line | `an-unreadable-file-is-named-measurements.md` |
| *Routed here at the 2026-09-08 re-verification* — the invalid-check sink, the hint guard's blind spots | `the-guard-pins-the-whole-listing-measurements.md` |
| *Routed here at the 2026-09-08 re-verification* — `validate --project`, `--report` reading a spec | `an-invalid-task-file-is-reported-measurements.md` |
| *Routed here from v1.1.0's audit round 3* — `walkDocTree`, and *Not findings* | `an-unreadable-file-is-named-measurements.md` |
| *Routed here from v1.1.0's audit round 3* — `--verify`'s missing zero-parse refusal | `an-invalid-task-file-is-reported-measurements.md` |
| *Routed here from v1.1.0's audit round 3* — `engine.UnitKinds` | `guards-read-what-production-reads-measurements.md` |
| *Decided at the 2026-09-08 decision pass* | this file |
| *Routed here from the v1.1.1 release* | this file |

## Absorbed on 2026-09-11

| from | what | now in the body |
|---|---|---|
| this spec's former §2, §2.1 | the pairing refusal names its set; one derivation; the REFERENCE.md binding | §2, rows 1, 2, 4, 5 |
| `record-diagnoses-every-bad-row.md` §2 | `--record` collects every row's violation, atomic write kept | §3, rows 6 and 8 |
| `record-diagnoses-every-bad-row.md` §3 Non-Goals 1–2 | atomicity; no workflow field, gate or convergence effect | Non-Goal 2; the Class line |
| `record-diagnoses-every-bad-row.md` §4 rows 1, 2 | three rows broken three ways; state byte-identical after a refusal | rows 6 and 8 |
| `the-guard-pins-the-whole-listing.md` §2 | the enum guard asserts the listing's tail, not its members | §8 |
| `the-guard-pins-the-whole-listing.md` §3 Non-Goal 1 | no general validation of message text | Non-Goal 3 |
| `the-guard-pins-the-whole-listing.md` §4 rows 1a, 1b, 1c | truncating, appending, the refused containment repair | rows 18, 17, 19 |
| `guards-read-what-production-reads.md` §2, the comment half | the guard's doc comment says what it derives from | §8, last paragraph |
| `guards-read-what-production-reads-measurements.md`, *`engine.UnitKinds`* | a test-only accessor | §7 — see *`UnitKinds` finds its caller* below |
| this sidecar, *Decided at the 2026-09-08 decision pass* | an off-vocabulary `severity` is a warning naming the row | §3, last paragraph; Non-Goal 5; row 9 |
| field report WB-3155 #8 | the ground prompt's kind/tier table | §5, rows 13–14 |
| field report WB-3155 #23 | `--merge`'s hint on a missing field | §4, rows 10–12 |
| `a-round-can-be-driven-from-the-envelope-measurements.md`, *What the absorbed spec fenced out* | `groundRecordEmptyHint`'s false antecedent | §6, row 15 |

Not carried: `the-guard-pins-the-whole-listing-measurements.md`'s two routed items. The invalid-check
sink is `next-action-and-check-tell-the-truth` §4's subject (a check that could not run neither
passes nor suppresses its class); the hint guard's two blind spots are not taken by any release and
stay in that sidecar as history. `guards-read-what-production-reads.md`'s binding of §7.2's
`partial_kind` cell and its three helper tasks were dropped with that spec; its stub gives the reason.

## Moved out of the body on 2026-09-11

**The former §2's census and round-trip paragraphs**, verbatim:

> **The second refusal is the one a *conforming* unit hits.** The first fires on a value that is in no
> enum — a typo. The second fires on two values §4.1 itself lists, paired wrongly. The census, run over
> the shipped predicate rather than counted by eye, is in the sidecar under *§2 — the census of the
> `kind × tier` pairings*: most of the pairings a unit can write using only §4.1's own values are
> refused by the message that names no set, on the three verdicts that bind the rule, and the ones that
> pass are the whole of the rule. The qualifier belongs in the sentence rather than under it:
> `groundTierRuleBinds` is true for `PASS`, `PARTIAL` and `FAIL` and false for `UNVERIFIABLE`,
> `QUESTION` and `NOT-A-CLAIM`, and `validateGroundRowTier` returns `nil` for the latter three, so a
> `QUESTION` row carrying `{behaviour, read}` is accepted and never sees this message at all. A unit
> that reads the enum listing and picks a legal tier is *more* likely to land here than one that
> mistypes.
>
> The recovery costs a re-read of the emitted prompt, which is the argument v1.0.0 already accepted for
> the other four cells; the prompt does carry the per-kind sets, rendered by `groundPromptEvidence` in
> `internal/cli/ground.go` from `TierAcceptableFor`. How many bytes that round trip is, this file quotes
> nowhere, because the figure is a property of the spec being grounded and not of tp — the sidecar's
> *§2 — the cost of the round trip* gives the derivation. `internal/engine/groundrow_test.go`'s
> enum-refusal doc comment states a figure for that quantity; `guards-read-what-production-reads.md`
> edits that comment, and the stale figure is deleted with it rather than re-measured.

The comment edit that paragraph assigns to `guards-read-what-production-reads.md` is now the body's
§8.

**The empty-set branch, dropped.** The former §2.1 required the refusal to state that no tier is
acceptable for a kind outside the seven, and a test reaching the branch by calling
`validateGroundRowTier` directly. Dropped: `ParseGroundKind` closes the enum before the tier rule
runs, so no payload reaches the branch, and a test-only caller for an unreachable message is surface
without a reader. The former row 3 went with it.

**The limit of the REFERENCE.md binding**, verbatim:

> **The limit of that binding, stated rather than implied.** It establishes that the quote is present
> and current; it cannot establish that no *other* paragraph of REFERENCE.md contradicts it, because a
> `Contains` is a local assertion inside an unbounded text and the complement is free. The replacement
> is not a rewording — it is the whole-artifact shape `the-guard-pins-the-whole-listing.md` uses, and
> REFERENCE.md is not a bounded artifact.

**The body's forwarding table of the 2026-09-08 split**, verbatim. `spec/candidates.md` cites it as
this spec's forwarding table.

| was | subject | now |
|---|---|---|
| §1 bullet 1, §2, §2.1 | the pairing refusal names the tiers it would have accepted | this file |
| §1 bullets 2–3, §3 | the enum-refusal guard pins the whole listing, not its members | `the-guard-pins-the-whole-listing.md` |
| §4, §10 | the guard's doc comment, the §7.2 binding it promised, and the three hand-parsing test helpers | `guards-read-what-production-reads.md` |
| §1 bullet 4, §5, §5.1 | an unreadable file is named on stderr, not dropped in silence | `an-unreadable-file-is-named.md` |
| §1 bullet 5, §6 | an unreadable or unparseable task file is reported by `tp lint`, not swallowed | `an-invalid-task-file-is-reported.md` |
| §1 bullet 6, §7 | a finding's `Context` is cut on a rune boundary | `context-is-cut-on-a-rune-boundary.md` |
| §8 Non-Goals 1–2, 9 | — | this file, §3 |
| §8 Non-Goals 3, 5, 6 | — | `an-unreadable-file-is-named.md` |
| §8 Non-Goals 4, 10 | — | `guards-read-what-production-reads.md` |
| §8 Non-Goal 7 | — | `an-invalid-task-file-is-reported.md` |
| §8 Non-Goal 8 | — | `context-is-cut-on-a-rune-boundary.md` |
| §9 rows 1, 2, 4, 5, 6 | — | this file, §4 rows 1–5 |
| §9 rows 3a, 3b, 3c | — | `the-guard-pins-the-whole-listing.md`, rows 1a–1c |
| §9 rows 7, 7b | — | `guards-read-what-production-reads.md`, rows 1, 1b |
| §9 rows 8, 8b, 8c, 8d | — | `an-unreadable-file-is-named.md`, rows 1, 1b, 1c, 1d |
| §9 rows 9, 9b, 9c | — | `an-invalid-task-file-is-reported.md`, rows 1, 1b, 1c |
| §9 rows 10, 10b | — | `context-is-cut-on-a-rune-boundary.md`, rows 1, 1b |

Where those targets stand on 2026-09-11: `an-unreadable-file-is-named.md` rows 1, 1b, 1c and 1d
still exist and are still the unreadable-ranking rows. `an-invalid-task-file-is-reported.md` and
`context-is-cut-on-a-rune-boundary.md` are forwarding stubs into `an-unreadable-file-is-named`.
`the-guard-pins-the-whole-listing.md` is a forwarding stub into this file (rows 1a–1c → rows 18, 17,
19). `guards-read-what-production-reads.md` is a dropped stub. "This file, §3" and "this file, §4 rows
1–5" name the body as it stood before 2026-09-11: its Non-Goals are now §9 and its rows 1, 2, 4 and 5
are now rows 1, 2, 4 and 5 of §10; row 3 was dropped with the empty-set branch.

## Re-verified 2026-09-11

Every reproduction ran in a fresh directory under the session scratchpad, outside the repository,
with the binary built at `18032abe`. `git diff 18032abe HEAD -- internal/` was empty when it ran.
The fixture is described in words so the reproduction survives the scratch path.

**`tp ground --record` names the first bad row only.** Fixture: a spec of one `## 1. Behaviour`
section holding three sentences — *The tool exits 2 when the flag is missing.*, *The corpus holds 40
rounds.*, *The README lists 3 flags.* — emitted once with `tp ground spec.md`, then a payload of three
rows, one per unit, carrying `{behaviour, read, PASS}`, `{corpus, run, PASS}` and `{document,
squinted, PASS}`. `tp ground spec.md --record bad3.ndjson`:

```
{"error":"line 1: field \"tier\": \"read\" says nothing about a \"behaviour\" claim (§4.1), and a PASS row must be reached at a tier that does","code":1,"hint":"fix the row the message names in the --record NDJSON: every non-blank line is one JSON object carrying one floor unit's disposition"}
```

Exit 1, one line named of three. Source: `parseGroundRows` returns on the first error,
`internal/engine/groundrecord.go:66-67`.

**`tp audit --record` and `tp review --record` abort on the first invalid-JSON line.** Fixture: a
three-line payload — a complete row followed by a trailing comma, a row missing a comma between two
keys, a row missing its closing brace. After one emission in each phase, both `--record` commands
print `line 1: invalid JSON: invalid character ',' after top-level value` at exit 1 and name no other
line. Source: `internal/cli/audit_record.go:279-280`, `internal/cli/review_record.go:303-304`.

**Review's missing-field refusal already names every line.** On a two-line review payload whose rows
lack only `evidence`, `tp review spec.md --record` prints `line 1: missing required field evidence;
line 2: missing required field evidence` at exit 1. Shipped in v1.1.0 by `436a9f5f` and `e9117fe0`,
pinned by `TestReviewRecord_RefusesEveryRowMissingARequiredField`, `TestReviewRecord_RefusesAnEmptyObjectRow`
and `TestReviewRecord_RequiredFieldRefusalIsRaisedLast` in `internal/cli/record_required_fields_test.go`.
The collection is at `internal/cli/review_record.go:324-329`; the aborting rules above it — invalid
JSON, a row pre-resolved `fixed`, a pre-resolved `wontfix` without evidence — still return first.

**The pairing refusal and six sibling messages cite tp's design document.** Read from the source:
the pairing refusal `internal/engine/groundrow.go:392` (`§4.1`), the unknown-key refusal `:413`
(`§7.2's table`), the `unit_id` refusal `:432` (`§2.1`), the `ordinal` refusal
`internal/engine/groundrecord.go:150` (`§8`), the empty-payload hint `internal/cli/ground.go:151`
(`§2.1`), the carry-source hint `:172` (`§8`), and the all-cut notice `:625` (`§2.1`). The anchor
refusal at `internal/engine/groundrow.go:261` (`must be a §n(.n)* section, as §0 is before the first
heading`) describes the user's own numbering and is excluded. No test under `internal/` contains the
pairing refusal's text: a search for `says nothing about a` over `internal/` matches the format string
and one unrelated comment in `groundrow.go`, nothing in a `_test.go` file.

**The ground prompt drops §4.1's reason column.** `groundPromptEvidence` prints, per kind, the kind,
its subject and its acceptable set (`internal/cli/ground.go:989-997`); the tier lines print
`groundTierDid` (`:848-855`), where `query` is *ran a query over the corpus* and `run` *ran the
shipped command*. `spec/1.0.0.md:501-509` carries the fourth column, *and not the others, because*;
its `mechanism` cell refers to *§4's `-type d` pipeline*. `skills/tp/REFERENCE.md:901` repeats the
`query` gloss. No test asserts either gloss.

**The empty-payload hint on a document with no unit.** Fixture: a spec holding `# App` and
`## 1. Behaviour` and nothing else; `tp ground spec.md` emits `floor_size: 0`, `carried: 0`. An empty
payload, `tp ground spec.md --record empty.ndjson`:

```
{"error":"the record holds no rows, so there is nothing to record","code":1,"hint":"record a file holding at least one row: the round carried nothing from a preceding round, so an empty payload would record nothing at all. If the prompt asked for no dispositions because every unit was cut (§2.1), there is no round to record and --status --check reports that floor instead"}
```

`tp ground spec.md --status --check` on the same document reports `emitted: 0`, `cut: 0` and exits 0,
so the hint's second sentence supposes a cut that did not happen. One command reproduces it once the
round is emitted. Source: `groundRecordEmptyHint`, `internal/cli/ground.go:151`.

**The runner map's refusal names no kind.** Fixture: a spec, `tp init`, and `.tp/config.json` holding
`{"workflow":{"runner":{"implemnt":"no-such-template","default":"no-such-template"}}}`. `tp run`
(not `--dry-run`, which does not validate the runner) exits 2 before spawning:

```
{"error":"runner.implemnt: not a unit kind; a per-kind runner map keys on the eight unit kinds plus default","code":2,"hint":"runner takes one of three shapes: a built-in template name (\"claude\" or \"opencode\"), a runner object carrying cmd, or a map from unit kind to either of those with a \"default\" key covering the kinds it does not list"}
```

Source: `internal/engine/runnershape.go:124-128`.

**An off-vocabulary `severity` is recorded in silence.** One review row and one audit row, each
otherwise complete, carrying `severity: "bogus"`; after one emission in each phase, `tp review
spec.md --record rev.ndjson` exits 0 with one notice about the missing `role` field and none about
severity, and `tp audit spec.md --record aud.ndjson` exits 0 with empty stderr.

**Two observations not taken.**

- `tp audit --record` accepts a row missing `status` — a two-row payload lacking it recorded at exit
  0 with `findings: 2` — while `tp audit --merge` skips the same rows as incomplete. The record is
  conservative (a row without `status` counts as not `PASS`), so nothing is lost; the two sinks
  disagree on what a row needs. Not routed.
- `internal/engine/escalation.go:100-101` refuses an escalation record whose `unit_kind` is not a
  unit kind with *"is not one of the documented unit kinds"*, naming none. It is not reachable in
  practice: `tp escalate` with `TP_UNIT_KIND=implemnt` wrote its record carrying that value and
  exited 2, which is its exit on success (`internal/cli/escalate.go:121`). The check runs only when
  the driver reads the record, which treats a failing record as unwritten (the doc comment above
  `Validate`), and `tp run` sets `TP_UNIT_KIND` itself. No reader sees the message. Not taken.

## Field report WB-3155, verified 2026-09-11

**#8 — `corpus` + `run` is refused, and the prompt does not make the distinction clear.** The claim:
a grading unit recorded `git diff` measurements as `kind: corpus, tier: run`, because it ran a command
over the repository; tp refused, since `corpus` accepts only `query`. The report calls the refusal
defensible and its message good, and asks the prompt's table to define `query` as any read-only
interrogation of the corpus, CLI tools included. **Verdict: PARTLY.** The refusal is correct and
unchanged by this spec's §5. What is confirmed is the prompt: it prints the kind, its subject and its
acceptable tiers and drops the reason column `spec/1.0.0.md:501-509` has
(`internal/cli/ground.go:989-997`), and its `query` gloss (`:850`) does not say whether a command run
over a git corpus is a query. Nothing to reproduce beyond reading the prompt block the source renders.

**#23 — `tp review --merge` requires `evidence`, and its error says "format".** The claim: two role
files were refused with the zero-rows error and the trailing-comma hint, though every line was valid
NDJSON and the only difference from the file that passed was a missing `evidence` field; the hint sent
the reporter looking for a format fault. **Verdict: PARTLY.**

- **Confirmed:** the envelope hint is the one format hint whatever the cause —
  `droppedInputHint` (`internal/cli/merge_inputs.go:33`), raised by `finishMerge` (`:169-172`) — on
  both merges.
- **Refuted at `HEAD`:** that no output names the cause. Each skipped line gets a stderr warning that
  names it — `warning: skipping incomplete line (missing evidence) in <file>`
  (`internal/cli/review_merge.go:200`; audit's twin at `internal/cli/audit_merge.go:188` names
  `item_id`/`status`) — though without a line number. The report quotes only the envelope.
- **Intended, not a defect:** requiring `evidence` is `spec/1.1.0.md`'s evidence-at-record rule.

Reproduction: a two-line review role file, each row complete but for `evidence`;
`tp review --merge r-arch.ndjson -o merged.ndjson` prints two stderr warnings naming `evidence`, then
`no line parsed in r-arch.ndjson: every content line was skipped, so that input contributed nothing to
the merge` with the trailing-comma hint, exit 1. The audit twin, two rows lacking `status`, prints the
same envelope. `tp review spec.md --record` on the same rows says `line 1: missing required field
evidence; line 2: missing required field evidence`.

Not taken here: #25's hint half — a batch `tp done` entry carrying `commit` as an array — belongs to
`a-task-file-write-names-its-target`.

## `UnitKinds` finds its caller

The item routed to `guards-read-what-production-reads-measurements.md` from v1.1.0's audit round 3:
`engine.UnitKinds` (`internal/engine/unitkind.go:45-52`) has no production caller, and its doc comment
says it exists *"for callers that need to name the set rather than test one value"*. Re-checked at
`18032abe`: every call is in a `_test.go` file. **Decided: hosted here, not dropped.** The runner map's
refusal (§7) is exactly such a caller — it names the set by count, *"the eight unit kinds"*, where the
listing belongs. The implementing task renders §7's listing from that accessor, which gives it the
production caller its comment promises; nothing else in this spec depends on which accessor it uses.

## The doc-comment correction, taken from `guards-read-what-production-reads`

`TestAnEnumRefusalNamesTheValuesItWouldHaveAccepted`'s doc comment (`internal/engine/groundrow_test.go:77-91`)
carries two statements the body's §8 corrects:

- *"The expectation is derived from those listings rather than restated, so a value added to §7.2's
  table reaches this assertion without anyone editing it"* — the expectation derives from
  `GroundVerdicts()`, `GroundKinds()`, `GroundTiers()` and `GroundPartialKinds()`, Go listings.
  `guards-read-what-production-reads-measurements.md` records the falsifying run: a fourth value added
  to §7.2's `partial_kind` cell of `spec/1.0.0.md` left `internal/engine` and `internal/cli` green.
- *"measured at 21,714 bytes on this repository's own spec"* — the prompt's size, a property of the
  spec being grounded; *§2 — the cost of the round trip* above gives its derivation and why this file
  quotes no figure.

`guards-read-what-production-reads.md` wanted the first corrected **and** the `partial_kind` cell
bound to the code. The binding was dropped with that spec; the correction is one comment edit and
rides with §8's guard change, which rewrites the same test.
