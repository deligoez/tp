# refusals-that-name-nothing — measurements

Supplemental material for `refusals-that-name-nothing.md`; the spec stands without it. Every block
below was moved here verbatim from the spec on 2026-09-08, except that two filename citations to
renumbered specs were repointed at slug paths. Every mutant was built and run in an
`rsync -a --exclude .git ./ <copy>/` copy outside the repository.

On the same day the spec was split into six, one per decision that can ship alone. This sidecar was
split with it; **Split 2026-09-08** at the foot of this file maps every section that moved to the
sidecar that took it. The Preamble below is not duplicated into those files — they cite it here.

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
