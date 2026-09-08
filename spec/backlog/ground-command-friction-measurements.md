# ground-command-friction — measurements

Supplemental material for `ground-command-friction.md`; the spec stands without it.

Every block below was moved out of the spec body verbatim, under the heading of the section it came
from. Figures are quoted at the ref each block names; re-derive rather than reading one off the page.
Filenames inside `git show <ref>:<path>` and `git log -- <path>` commands are the paths at that ref
and are left as written; every other citation was rewritten to the file's current name.

## Header: the count of decisions and why the file did not split

> **Of the first four, three are in the
> surfaces around the grounding protocol** — what the emission names, what `--record` says when it
> refuses, and what `--units` will show you. **The fourth is in the floor derivation itself, and
> naming the function in it took two tries.** §5's marker units are emitted by `floorSplitUnits`, on
> text whose list marker `floorCanonicalise` declined to strip because its gate is on the block's
> *first* line; what decides whether that happens at all is the block boundary `floorBlocks` draws,
> which emits blocks and never units. A sentence here named `floorBlocks` alone, and §5's own body
> says the other thing — *"the splitter makes it"*. Two fixtures differing in a single blank line
> separate the cases and both are one command: with no blank line before a `N. **Bold** text` list,
> `tp ground a.md --units` returns two units whose whole text is `1.` and `2.`; with the blank line,
> `tp ground b.md --units` returns none.
>
> What did hold, across the corpus below: the carry, the six verdicts, the kind–tier table and the
> atomicity of `--record`. Every defect here is about naming, reporting or splitting — none is about
> a judgement the protocol got wrong.
>
> This file takes `spec/candidates.md`'s **ground-related** material; the stub itself stays, for the
> references to that filename that survive across the tree. **The count that stood here is deleted
> rather than refreshed.** It read *29 surviving references*, which is exact at `3f80f676` and
> returns **41 occurrences across 15 files** at `9e672387`, under `CLAUDE.md`'s own rule — tracked
> non-`.tp-review` files, the bare filename, discounting `0.35.0-candidates.md` and
> `1.0.0-corrections.md`. `spec/candidates.md` restates the same stale 29 and is deliberately not
> edited from here. Derive it against a ref; do not read it off a sentence. That file held three
> kinds of thing, and `spec/undecided.md` took the larger
> share — its whole refuted record and all its non-ground undecided items; four of its undecided rows
> moved into the pending specs that own their subject; and its closed sections were deleted after each
> was verified present in the release that took it. What came here is about `tp ground`, which is what
> this release is.
>
> **This file has outgrown a patch, and saying so is this paragraph's whole job.** It opened as four
> defects one orchestrator measured against this repository's own specs (§2–§5). It now also carries
> **six more** (§9–§14), from two independent field reports on `tp ground` v1.0.0 run in two other
> repositories, on two Turkish specs, by two sessions that had not seen each other — a provenance this
> tree cannot check, and §1.1 says which claims that covers. **Ten numbered items, more declared
> decisions than that, and stating the counting rule is the point of the sentence — the count itself
> is not stated, for §5.1's reason.** Ten is §1's list plus §1.2's, which is fixed by those lists.
> The decisions are `grep -c '^\*\*The decision' spec/backlog/ground-command-friction.md` plus §9.1's own heading, which the
> anchored pattern cannot see; the anchoring matters, because a bare `grep -c 'The decision'` also
> counts this sentence and every cross-reference to one. **No value is quoted, because a count of
> this file's own text is changed by editing this file, which is the defect §5.1 takes.** What is
> durable is the shape: the two counts differ by exactly one heading, and three items carry more than
> one decision each — §5 two (its splitter rule and §5.1's ban on a figure about this file's own
> floor), §9 four (§9.1's gate, §9.2's key, §9.3's ship-order call and §9.3's characterisation label),
> §10 three (§10.1's `asked`, §10.2's `floor_delta`, §10.3's two halves).
> **Exactly one changes an exit code either way** — §9 gates `--status --check` on a standing `FAIL`,
> which Non-Goal 5 forbade until §9 amended it in §6. That is a minor release's surface, not a
> patch's. The **operator decides** whether this stays `1.0.1` or splits, and **nothing here is
> renumbered**: the roadmap table in `CLAUDE.md` is the only place a release number belongs, and a
> spec that names its own successor is how renumbering leaves stale citations behind — the sweep this
> repository ran after its own found 47 such citations, ~35 of them stale.

(Since moved here, §9.1 and §9.2 left for `spec/backlog/next-action-and-check-tell-the-truth.md`,
so the exit-code change the paragraph counts is no longer this spec's.)

## §1 The evidence base

**The evidence base.** A grounding programme ran `tp ground` across this repository's pending specs —
**not all of them, and the phrase "every pending spec" that stood here was a quantifier over a set
nobody enumerated.** At the commit that adds this file (`9760d053`) a round is recorded for **19 of the 22**:
`spec/backlog/what-the-carry-can-promise.md`, the spec since dropped into `spec/undecided.md` (then `spec/1.58.0.md`) and this file itself have none. Enumerate them rather than
trusting the sentence — `ls spec/.tp-review/*/ground-round-*.ndjson | cut -d/ -f3 | sort -u`. **The
reason first given for the first two is deleted, because the timestamps say the opposite of it.** It
read *"the first two having existed for only the programme's last five and a half hours"*.
`git log --diff-filter=A --date=iso -- spec/1.57.0.md spec/1.58.0.md` puts both at **03:05:20**; the
earliest recorded ground round is `28f6009c` at **03:19:15**, and `9760d053` is at **09:07:31**. So
they existed for **6.04 hours** — *longer* than the programme's own recorded span, and they predate
its first record by fourteen minutes. Why neither was grounded is not recoverable from the tree, and
no reason is asserted in place of the one that was wrong. Counting rule for the figures
below — every row of every `spec/.tp-review/*/ground-round-*.ndjson`:

```
python3 -c 'import json,glob,collections;c=collections.Counter();[c.update([json.loads(l)["verdict"]]) for f in glob.glob("spec/.tp-review/*/ground-round-*.ndjson") for l in open(f) if l.strip()];print(sum(c.values()),c)'
```

At the commit that adds this file: **28 rounds, 1,459 rows** — `PASS` 787 (53.9%), `NOT-A-CLAIM` 323
(22.1%), `PARTIAL` 230 (15.8%), `FAIL` 95 (6.5%), `UNVERIFIABLE` 13 (0.9%), `QUESTION` 11 (0.8%). All
six verdicts and all six tiers were exercised. **Re-derive rather than quoting these**: the corpus
grows, and the shares are the stable part, not the counts.

(The pending specs' round directories have since moved under `spec/backlog/.tp-review/`; a
re-derivation needs both globs, as `CLAUDE.md` records.)

## §1.1 The second evidence base: two independent field reports

Two Claude sessions drove `tp ground`
v1.0.0 through a full loop on real specs in other repositories, neither having seen the other's work:

- **Report A** — a 33 KB Turkish state-machine contract spec. 5 rounds, 226 floor units, converged to
  0 `FAIL` / 0 `PARTIAL`. Ask sizes 191 → 71 → 13 → 4 → 1. The floor was 46% `NOT-A-CLAIM` at
  round 1.
- **Report B** — a 609-line Turkish spec in a Laravel monorepo. 240 floor units, 6 rounds. Ask sizes
  213 → 31 → 13 → 7 → 7 → 3; the `FAIL` curve 3 → 0 → 1 → 0 → 0 → 0. Final: `PASS` 172,
  `NOT-A-CLAIM` 59, `PARTIAL` 5, `UNVERIFIABLE` 3, `QUESTION` 1, `FAIL` 0, and 74 cut.

**Field feedback arrives with the reporter's environment baked in**, and this repository's rule is to
verify each claim against tp's own source before routing it — two earlier field reports did not
survive that check. **What was re-derived here, and what could not be — the split matters and an
earlier draft did not make it.** Every claim below *about tp's own behaviour* was re-derived on a
freshly built binary, against this repository's own specs or against a fixture built outside it,
before it was written down. **That promise is about behaviour and it never covered the corpus
figures, which is a distinction worth making because one of those was false as it was typed.** §5's
`2,275` was already wrong at the commit that wrote it — §5.1 measures it and takes the decision that
follows. So read the sentence as it is scoped: the behavioural claims re-derive on demand, and every
corpus figure carries the ref it was measured at, because that is the only thing that makes one
checkable later. Two did not survive and are recorded with the measurement that refutes
them: §11's escape hatch exists in the mechanism and is closed by the prompt rather than absent, and
§14's fear is simply false. **The reports' own figures are a different class and are not checkable
from here at all** — neither repository, spec nor transcript is reachable, and `git log --all`
carries no trace of either — so every count either report supplies (round counts, ask curves, floor
sizes, token totals) rests on attribution rather than on evidence. They are quoted as the reports'
numbers, never re-derived, and no decision below turns on one. The earlier draft said *"every claim
below was re-derived here"*, a universal over a set half of which is unreachable; it is also not true
of one claim it did cover, §10.3's ranking, which §10.3 now corrects in place.

**Two of the four defects above were confirmed independently by the reports**, which is worth stating
because it is the only external replication any entry here has. §3 by Report A — *"reported only the
FIRST offending row, so the loop is fix-one, re-run, hit-the-next — I ended up telling every agent
about it in its brief, which is duty the prompt should carry"* — the **brief-instruction** workaround
this file records in **§2**, arrived at separately. (An earlier draft called it *"the same workaround
this file already records the orchestrator performing"* and pointed it at §3, which records a
different one: a whole-file validator run before every `--record`. Two workarounds, two sections; the
quote matches §2's.) And §4 by Report B — *"`cut: 74` with no
indication of what was dropped or why… a sentence's justification lived in a *cut* unit, so the graded
unit only held up because of text the arms had removed"* — which is §4's *"the sharpest finding of a
round sat in cut text again and again"*, reached from the other end.

## §1.2 Six further defects, and where they sit

**Three of these moved under this file while it was being written, and none of the three moved the
code.** Three commits to `skills/tp/SKILL.md` — recoverable by subject through
`git log --oneline -- skills/tp/SKILL.md` — closed the documentation half of §9, the whole of §13,
and half of §14 between the reports arriving and these sections being finished. **Each section names
the state it measured at, and they are not the same state**: §13 measures at `v1.0.0`, which is what
both reports read; §9 measures at the commit it was last edited at, *after* all three; §14 measures on
fixtures outside this repository. That §9's later state gives the same answer is a strengthening, not
a slip. **`tp ground spec/backlog/ground-command-friction.md --status --check` still exits 0 with four `FAIL`s standing after
all three**, re-run to check, which is why §9 is the highest-cost item in the file and not a closed
one.

**The numbering is by position and the cost order is this list, and that is a deliberate split.**
§9–§14 are appended after §8 rather than inserted after §5, because every row of §8's table names §2,
§3, §4 or §5 and a renumbering would falsify all nine at once. The precedent is a **sweep**, not a
count: `CLAUDE.md` records one cross-reference sweep after this repository's renumberings that found
47 citations to renumbered specs, ~35 of them stale, **none catchable by checking that the target
file exists** — every one resolved and every one had moved. The number of renumberings is
deliberately not asserted here: `CLAUDE.md` withdrew that ordinal after its own grounding found two
files giving different values, and `git log --diff-filter=R --name-status -- 'spec/*.md'` records one
rename commit, earlier ones predating the tracked names. §15 carries the new rows in §8's shape and
continues its numbering, so no existing row index moves either.

## §2 The scratch file the emission names is not unique across specs

**Assert the equality, never the literal** — the literal is the half that rots. This section was
written when both printed `ground-r1.ndjson`; round 1 was later recorded for the spec since dropped into `spec/undecided.md` (then `spec/1.58.0.md`,
`e309df62`) and for `spec/backlog/what-the-carry-can-promise.md` (then `spec/1.57.0.md`, `ccac3d8c`) and both now print `ground-r2.ndjson`, which is
the collision outliving its own illustration.

**How the workaround measured.** During the grounding programme every brief carried a hand-written
override — *"write to `ground-<spec>-r<N>.ndjson`, NOT the `ground-rN.ndjson` the envelope names"* —
because three units ran concurrently throughout. A default every caller must override is the tell.

## §3 `--record` diagnoses one bad row per invocation

**Why this matters more than its size.** It is a **P2 violation on tp's own terms**: what is easy for
one row is not equally easy for *N*. The tell is what the operator did instead — during the grounding
programme the orchestrator wrote its own whole-file validator against §4.1's kind–tier table and ran
it before every `--record`, four times catching two or three violations at once that tp would have
reported one at a time. **When the caller has to reimplement the tool's validator to use the tool
efficiently, the validator is missing something.**

## §4 A cut unit cannot be addressed

**Measured.** `--units` lists floor units. A reader who finds a defect in text the floor's arms cut
has no id to file against:

```
$ tp ground spec/backlog/ground-command-friction.md --units | cut -f1 | grep -cE '^u(15|16|17|18|19|38|49|50|55)$'
0
```

Those nine ids are exactly the ones that round filed rows against, having inferred each from the
splitter's merge behaviour and computed its own `text_sha` by guessing the canonicalisation. Where
that reconstruction failed the reader gave up and recorded no id at all — **13 rows across the 28
recorded rounds carry `unit_id: null`**:

```
python3 -c 'import json,glob;print(sum(1 for f in glob.glob("spec/.tp-review/*/ground-round-*.ndjson") for l in open(f) if l.strip() and json.loads(l).get("unit_id") is None))'
```

**Why this is the most valuable of the four.** Across the programme the sharpest finding of a round
sat in cut text again and again, and one round diagnosed why: the arms index a section's *reasoning*
and drop the *wording it produces* — a new central clause or a user-facing message string is exactly
the shape that gets cut. So the text least visible to the floor is disproportionately the text worth
grading, and it is the text a reader cannot cite.

## §5 A bare ordered-list marker becomes a floor unit

**Measured.** Counting rule — floor units whose whole text matches `^[0-9]+[.)]$`, read from
`tp ground <spec> --units` (TSV: `unit_id`, `text_sha`, `text`), over an **enumerated** set: the
twenty files then named `spec/1.38.0.md` through `spec/1.58.0.md` (now under `spec/backlog/`), plus `spec/1.0.0.md` — twenty-one in all.
**At `9e672387`: 20 such units across 4 specs, out of 2,192 floor units scanned — 0.91%.** They are
`u84 u87 u89 u91` in `spec/backlog/brief-carries-the-forcing-sentences.md`, `u71 u73 u93 u100` in **`spec/1.0.0.md`** — the document that
defines the floor — `u191 u194 u196 u199 u204 u206 u211` in `spec/backlog/what-the-carry-can-promise.md`, and
`u128 u132 u135 u137 u141` in the spec since dropped into `spec/undecided.md` (then `spec/1.58.0.md`).

**The census moved 2.5× and the instance did not, and keeping those apart is what this paragraph is
for.** The figure first written here was *8 units across 2 specs out of 1,990 — 0.40%*, which
reproduces exactly at `22ed1c4a`, the commit that wrote it. **The eight ids it names still reproduce
at `9e672387`**; the twelve new markers all arrived in `spec/backlog/what-the-carry-can-promise.md` and the spec since dropped into `spec/undecided.md` (then `spec/1.57.0.md` and `spec/1.58.0.md`), added
the same day and edited heavily since. So the defect is neither rarer nor commoner than it was — the
corpus grew — and every share here is quoted with the ref it was measured at, because it will move
again before this is read.

**The set is enumerated because an earlier draft's phrase was not.** That draft said *"every pending
spec plus `spec/1.0.0.md`"* and quoted 1,968, and **no reading of that phrase returns it**: at
`22ed1c4a` the rule gives 1,990 for the twenty-one above and 2,064 adding this file. **A third
denominator stood here — adding this file too — and it is deleted rather than refreshed; §5.1
says why, because it was false when it was written.** The named ids reproduce under every reading;
only the denominator moved, which is the half a stated counting rule exists to pin.

**The marker is not a line in the source — the splitter makes it.** No line in either file matches
`^\s*[0-9]+[.)]\s*$` (`0` hits in both). The items are single lines of the form
`N. **Bold phrase.** text…`, and the split happens *inside* the line.

**Two things are unexplained and are left unexplained rather than guessed.** In `spec/backlog/brief-carries-the-forcing-sentences.md` the
items are numbered 1–5 and only `2.` through `5.` become units. In `spec/1.0.0.md`, 16 lines match
`^\s*[0-9]+[.)]\s+\*\*` and only 4 produce a marker unit. So the shape is necessary and not
sufficient, and what selects the four is not known. An earlier draft of this entry asserted a
mechanism — items whose bold phrase sits on the *following* line — and one command refuted it.

**The extremes a length floor would sit between.** The shortest legitimate
non-marker floor unit across the same corpus is **9 bytes** (`Measured:`, `spec/backlog/red-gate-procedure.md` `u49`, at
`9e672387`; the next shortest are 10) against the markers' 2, so any threshold in 3–8 separates
them.

## §5.1 A figure about this file's own floor does not belong in this file

**Two sentences in this release quoted this file's own floor size, and both were false when
they were written, for the same reason.** §5's triple `1,990 / 2,064 / 2,275` holds at `2c39f718`.
`git show 22ed1c4a -- spec/1.0.1.md` shows the sentence was **written** four commits after that,
where the same rule returns `1,990 / 2,064 / 2,295` — so `2,275` was already false as it was typed.
(At `9e672387` the first two are `2,192` and `2,266`; the third is not stated, for this
subsection's own reason.) It is `2,064 + 211`: this file's floor as it stood four commits earlier,
carried forward unrefreshed. And §10.3's *"it skips this file at 211"* is **that same stale
211**, in a sentence whose entire job is correcting an earlier unreproducible figure; the floor was
**247** at `339368c0`, where that correction was written, and **257** at `9e672387`. Walk it rather
than reading a trace off this page — `tp ground <this file> --units | wc -l`, run in a worktree of
each commit `git log --format=%h -- spec/1.0.1.md` names, returns a different value at nearly every
one of them.

**Stated plainly rather than quietly refreshed: this is the defect the release is about, committed
by the release.** The two places this file corrects a figure nobody could reproduce are the two
places it introduced one — one of them the same number twice, four commits apart.

**The decision: no figure about this file's own floor appears in this file.** Deleted, not annotated
with a ref. This is an exception to the rule every other figure here obeys — *state the command that
derives it and the tree state it was measured at* — and the exception is what the rule cannot cover.
Every other number has a window in which it stands: the corpus grows between commits, the code
changes at release cadence, and a held-at ref names a state a reader can check out and re-run.
**A figure about the containing file's own floor has no such window, because writing the sentence
changes the quantity the sentence measures.** The only honest ref would be the commit being composed
while the sentence is composed, which does not exist yet — so the sentence is falsified by the very
commit that records it, unless it is the last edit the file ever receives. Nothing can promise that,
and neither of these two was. So the sound forms are the derivation with no value, or nothing.

**The scope, said exactly, because §5.1 would otherwise break its own rule three times.** What is
forbidden is a **live** figure — a value this file asserts about its own present floor, or about its
own present text, which the act of asserting it invalidates. (The front matter's decision count is
the same object and is now stated as a rule with no value, for the same reason.) The pinned values
above are a different object: each is tied to an immutable past commit and is offered as *evidence
that the practice fails*, not as a fact about the document a reader is holding. A past commit cannot
be edited by writing about it, which is precisely the window a live figure lacks.

**Neither argument needed the value.** §5's point is that the denominator moves under different
readings of an unstated phrase, which its first two figures make; §10.3's is that a restricted
command returned a narrower set than the rule it was offered under, which does not turn on where
this file ranks inside it. What replaces the number in both places is the command and a ref — a
reader who wants it runs one line, and nothing in either argument waits on the answer.

(The rule itself now lives in `skills/tp/SKILL.md` Step 0.5; this subsection is kept as the
measurement that produced it.)

## §7 Open questions inherited when `spec/candidates.md` was split

**One entry arrived here already answered, and is recorded as answered rather than carried.**
`candidates.md` listed *"What `UNVERIFIABLE` costs"* as fog, on the ground that there were **zero
instances across 44 grounded claims**, so the verdict was designed and untested. Re-derived over the
corpus in §1: **13 `UNVERIFIABLE` rows and 11 `QUESTION` rows in 1,459**. Both verdicts are exercised,
and neither is rare enough to call untested. The fog entry is closed by measurement, not by a decision.

(The other item §7 carried, claim enumeration, is an untaken decision and moved to
`spec/undecided.md`.)

## §8 Tests — the mutant-column prose trimmed from rows 4, 8 and 9

Row 4: The three break *differently* to kill a second mutant: one that collects at most one violation per class. **Not** to defeat a dedup-by-message mutant, which an earlier draft gave as the reason and which cannot be defeated that way — `GroundLineError.Error()` is `"line %d: %v"`, so two *identical* breakages already produce distinct messages (measured: `line 1: …` and `line 5: …`), and a same-way fixture is what would catch a mutant deduplicating on the reason alone.

Row 8: drop every fragment shorter than **10** bytes, which also drops `Measured:` — the 9-byte legitimate unit §5 names. That is the only length floor this row kills, and it is the least tempting one: over §5's own enumerated twenty-one files at `9e672387`, markers are 2 bytes and the shortest non-marker floor unit is `Measured:` at 9 (`spec/backlog/red-gate-procedure.md` `u49`; the next shortest are 10), so every threshold in §5's own blessed 3–8 range survives this row. **An earlier draft wrote *probed over 527 real floor units*, and that number is deleted rather than refreshed: it carries no counting rule and no set returns it** — the twenty-one files hold 2,192 units, 2,172 of them non-marker, `spec/1.0.0.md` alone 401, and units of 60 bytes or fewer 173. The argument never needed a denominator, only the two extremes, and both re-derive. Row 9 is what kills those.

Row 9: implement §5 as a length floor of any kind. Two bytes is the marker's own length, so no threshold that drops the markers can keep this unit under either reading of the comparison. An earlier draft asserted a **3**-byte unit and does not kill them: under *drop when length < T* — the reading §5's 3–8 range makes natural — **T = 3 satisfies rows 8 and 9 together**, dropping every 2-byte marker, keeping `Measured:` and keeping the 3-byte unit. That is a length floor, which is the rule §5 argues hardest against, and it survived the table until this round. **The input this row named was `Go` and it cannot be built as a floor unit** — `printf '# R\n\nThe count is 12 things. Go\n' > c.md; tp ground c.md --units` prints only `u1`, because a bare `Go` fails all three of §2.1's arms; and read as the code span it is 4 bytes, which survives `T = 3` and collapses this row's own argument. It *is* a real 2-byte fragment one level up, at the splitter, pre-`inFloor`, which is where §5's decision operates — so the two readings disagree about it and the row is only buildable under one of them. **`v2` is buildable under both**: 2 bytes, and it reaches the floor on the digit arm. The same fixture with `v2` in place of `Go` prints `u1` and `u2 fb04dcb6970e v2` (built and run at `9e672387`).

## §9 `--check` exits 0 with `FAIL`s standing

**This is the highest-cost item in the file** — higher than §2–§5, which is why §1's list states the
cost order rather than the section numbers doing it. Both field reports hit it independently, neither
having read the other.

**Measured, and still true at the commit this section was last edited.** This spec's recorded
round 1 (then `spec/1.56.0.md`), counting rule — every row of the round file, by `verdict`:

```
python3 -c 'import json,collections;c=collections.Counter(json.loads(l)["verdict"] for l in open("spec/backlog/.tp-review/ground-command-friction/ground-round-1.ndjson") if l.strip());print(dict(c))'
```

**`FAIL` 4**, `PASS` 40, `PARTIAL` 12, `NOT-A-CLAIM` 17. And:

```
$ tp ground spec/backlog/ground-command-friction.md --status --check; echo $?
0
```

Four refuted claims stand in the spec and the gate says go. The same shape reproduces on a two-unit
fixture built outside this repository: a round carrying one `PASS` and one `PARTIAL`, both floor units
dispositioned, exits 0.

**The documentation half was a contradiction inside one section, and it has since been closed — by
three commits landing while this section was being written.** As v1.0.0 shipped it and as both reports
read it, `skills/tp/SKILL.md`'s ground-loop section opened *"Repeat until `tp ground <spec> --status
--check` exits 0"* and, four steps later, told the same reader to *"Repair the spec against the
`FAIL` and `PARTIAL` rows, then run the next round"* — a repair loop whose stop condition cannot see
the rows it is a loop over — while the command table further down stated the gate correctly and in
the opposite direction: *"It gates on nothing else — a round of nothing but `FAIL`s is fully covered
and exits 0."* Report B: *"An agent following the loop literally stops after round 1 with three false
claims in the spec. I only continued because I read `by_verdict` by habit."*

`skills/tp/SKILL.md` now opens the same loop with *"Repeat until **both** hold"* and spends four lines
saying that a driver stopping on the exit code alone stops with false claims standing. Cite it by that
text, never by a line: a `path:NNN` citation in this repository rots within a handful of commits,
measured three times in one week. The three commits are recoverable by subject —
`git log --oneline -- skills/tp/SKILL.md` names *"the ground loop condition names the breakdown, not
just the exit code"*, *"name all six verdicts where the loop is driven, `UNVERIFIABLE` included"* and
*"re-emitting an unrecorded round is idempotent, and a round file holds the carry"*.

**That closure is the argument for §9.1, not against it, and the file would be dishonest to present
it as agreement.** What was repaired is the sentence; the gate is unchanged, and the measurement above
was re-run after the three commits with the same result. The document now has to spend four lines
telling a human driver that the tool's own exit code is not the answer to the loop's own question —
which is a workaround with a reader in it. **`tp run` has no reader.** A prose caveat is not
available to a driver that branches on an exit code, and grounding is already the phase `tp resume`
schedules nothing for, so nothing else in the system will catch it.

## §9.1 The decision: `--check` gains a third condition (the decision itself is now `spec/backlog/next-action-and-check-tell-the-truth.md`'s)

**The benefit is prospective, and §9 leans on it hard enough that the scope belongs beside the
claim rather than a paragraph earlier.** `tp run` schedules no grounding unit today:
`internal/engine/unitkind.go` exports eight kinds — `implement`, `review-role`, `review-record`,
`review-resolve`, `decompose`, `audit-role`, `audit-record`, `audit-fix` — and none is grounding,
while a case-insensitive search for `ground` in `internal/engine/nextunits.go` returns three hits
that are all the word *round*. So no run reads this exit code at present, and none can until
grounding becomes a unit kind. What the gate is for is that driver, and any script branching on
`$?` in the meantime; what it is **not** is a fix for a reader `tp run` has today.

**No deadlock, one unconditional exit, one hole — all three measured on fixtures.** The audit phase's
equivalent, a `FAIL` accepted with evidence still recording `clean: false`, is the defect the
accepted-finding release takes, and grounding does not have it. **The unconditional exit is repairing
the unit's text**: any edit that moves the canonicalised text moves the hash, so the unit leaves the
carry and is re-asked — measured on §11's fixture at round 3, where `u2` gained two words and its
hash went `0a4d14737541` → `a4bec125b33d`. **Re-deciding it in a later round** works too (§11) but is
*not* unconditional: §11 permits that only *when the ground beneath it moved* and forbids re-deciding
to repeat a verdict, so a `FAIL` filed in error against a true claim has only the first exit — which
means editing a correct sentence. Not a deadlock, and not the clean two-exit picture an earlier draft
drew either.

**§9 still depends on §11**, for §11's reason rather than the one an earlier draft gave: the second
exit is legal in the mechanism and forbidden by the prompt, and a gate whose escape the protocol
forbids has one exit, not two.

## §9.3 The limitation §9's gate ships with, and why it ships anyway

**This subsection exists because the finding that produced it arrived in a *cut* unit**, which is §4's
whole subject, and the file's own rule is to move a load-bearing finding into prose the arms can see.

**The gate has a hole that runs the other way from a deadlock, and §9.1's earlier enumeration was
short by exactly it.** §9.1 argued only about a gate that cannot be *satisfied*; this path **clears
the gate with no repair at all**, which is the failure a gate exists to prevent. Constructed and run:
a fixture whose §1 and §2 hold **byte-identical** sentences, one hash at ordinals #1 and #2; round 1
records `u1` `PASS` and `u2` `FAIL`; **§1 alone is edited and §2's failing sentence is untouched**;
the ordinal shift re-points the `(text_sha, ordinal)` join at the `PASS` row, so round 2 records
`u2 … PASS carried_from: 1` carrying evidence written about **§1**, `by_verdict` reports `FAIL: 0`,
and `--status --check` exits 0. `spec/backlog/what-the-carry-can-promise.md` §2.2 takes this defect and prototypes a multiplicity
fence over it; §9.1's refuted premise — §8's promise that an unrepaired `FAIL` is permanent while its
text stands — is the same sentence that spec is written to narrow.

**The decision: §9's gate ships before that fence, with this limitation stated and guarded by §15
row 24.** Counting rule for the exposure — duplicated `text_sha` values inside one spec's floor, over
`spec/*.md` with a freshly built `tp`:

```
for f in spec/*.md; do tp ground "$f" --units | cut -f2 | sort | uniq -c |
  awk -v F="$f" '$1>1{print $1, F, $2}'; done | sort -rn
```

Three things decide it. **4 files of 62 hold any duplicated hash; six hashes cover 15 units** — and
all four (`spec/0.1.0.md`, `spec/0.13.0-review-perspectives.md`, `spec/0.17.0-ax-improvements.md`,
`spec/0.23.0.md`) are shipped specs, so **no pending spec — the only kind grounding is run against —
holds one.** The share is quoted **held-at** rather than restated, because its denominator moves with
every edit to any spec: **0.22%** at `77f61373`, the commit that wrote it (15/6,828), and **0.21%**
at `9e672387` (15/7,051). Run the command; do not read the percentage.

**And there is a stronger measurement of the same exposure, which the static scan structurally
cannot make.** The scan above reads specs *as they now stand*, so it cannot see a duplicate that
existed at the round a `FAIL` was recorded in — which is the only state the hole needs. The realized
form reads the emitted artifact instead, and answers the sharper question: has any round this
repository has ever emitted been graded against a floor holding a duplicated `text_sha`?

```
python3 -c '
import glob,re,collections
fs=sorted(glob.glob("spec/.tp-review/*/floor-ground-round-*.txt")); n=d=0
for f in fs:
    h=[m.group(1) for l in open(f) for m in [re.match(r"^u\d+ \S+ ([0-9a-f]{12}) ", l)] if m]
    n += len(h)
    d += any(v > 1 for v in collections.Counter(h).values())
print(len(fs), n, d)'
```

**Zero.** At `9e672387` that prints `36 2339 0` — thirty-six emitted floors, 2,339 floor units
between them, and not one floor holding a duplicated hash. It points the same way as the static
figure and answers the objection the static figure cannot. Second, the gate is nowhere worse than
what ships: the hole is there today and silent today, while what ships additionally exits 0 on
this spec's four standing `FAIL`s, in a spec with no duplicated hash at all. Third,
`spec/backlog/what-the-carry-can-promise.md` is a pending, unreviewed `loop`-class release, and blocking a measured exit-code
fix behind it buys nothing the limitation and its guard do not. §15 row 24 asserts the hole **as
shipped behaviour**, so it goes red the day the fence lands and takes this subsection with it.

**The characterisation label goes into the test, not only into prose — and this is a decision, taken
because the answer is that prose-only is not acceptable.** At `9e672387` the word appears in exactly
two places, both in this file, §15's preamble and row 24 itself
(`grep -rn characterisation spec/ skills/ internal/ CLAUDE.md README.md`, discounting
`spec/.tp-review/`; `spec/backlog/what-the-carry-can-promise.md`'s two occurrences are about a different subject). No test name,
no doc comment and no `--check` output carries it. So a later reader meets a **green** test asserting
that a `FAIL` is cleared by editing an unrelated section, with nothing at the test saying the green
is deliberate — which is the shape most likely to be tidied away by someone who reads it as a bug in
the test.

## §10.1 There is no machine-readable ask set

Report A calls this *"the single biggest papercut"*. **All three of its claims hold.** Measured on
this spec's round 2 (then `spec/1.56.0.md`) in a copy outside this repository — floor 61 units, all 61 carried:

- **`carried` is a count.** The envelope's keys are `carried, floor, floor_size, output_path, prompt,
  round, snapshot, spec`, and `carried` is the integer 61. No list of ids appears anywhere in it.
- **The `(carried)` marking is prose only.** `grep -c '(carried)' ` over the emitted `prompt` returns
  **62** — 61 index rows plus the ask sentence. It is inside a string a driver would have to parse.
- **The floor file on disk is unmarked.** `grep -c '(carried)' spec/backlog/.tp-review/ground-command-friction/floor-ground-round-2.txt`
  returns **0**. That is deliberate and stays: `runGround`'s own comment gives the reason — *"a copy in
  the floor file would be a second statement of the same fact with nothing comparing the two"* — and
  §2's floor is the artifact the round is graded against.

So a driver that wants the ask set reconstructs it. Report A's reconstruction — join
`floor-ground-round-N.txt` against `ground-round-(N-1).ndjson` on `(text_sha, ordinal)` — is
**exactly `GroundCarriedRows`**, which joins on the same pair for the same reason. *"A 20-line script
every driver will rewrite"* is precise: the script is a reimplementation of shipped tp code, and this
is the same P2 tell §3 records, in a different surface.

## §10.2 Floor growth conflates two different things

Report A's rounds ran 191 → 225 → 224 → 226 floor units, because repairing a spec adds sentences. A
unit is in the ask set either because its text changed or because it never existed, and the report
asks whether anything separates them.

**Measured: nothing does.** On the §11 fixture, round 3 emits with `u2` edited (previously `PASS`ed,
hash moved) and `u5` written fresh in a new section. Both are un-marked in the index, both are in the
ask set, and the ask reads *"This round owes a disposition for 2 of the 3 floor units above"*.
`floor_size` 3, `carried` 1. No key, marker or sentence in the envelope, the prompt, the floor file or
`--status` separates the re-ask from the first ask.

**Half of the report's framing does not survive, and the half that does is the useful one.**
New-versus-edited is **not recoverable from hashes at all** — identity *is* the hash, so an edited
sentence and a deleted-plus-added pair are the same event to every artifact tp writes, and no
decision here pretends otherwise. What *is* exact and cheap is the comparison the report actually
wants for its growth question: **the previous round's floor as a set of hashes against this one's.**

## §10.3 A large floor has no sharding path

Both reports. Report A split 191 units into 5 slices by section anchor, each agent spending 100–250k
tokens on 27–49 units, and writes: *"'Spawn ONE sub-agent on that prompt' does not survive a 191-unit
floor. The instruction reads as a correctness constraint (no panel, no roles) but lands as a capacity
claim."* Report B: **213 asked units out of a 240-unit floor** — §1.1 and this section quote different
quantities and the file conflated them until now (240 is that spec's floor, 213 its round-1 ask) —
306k tokens, 83 tool calls: *"it held, but a spec 2× this size would blow the context."*

**The emitted prompt does not say it.** Measured over the emission for this spec (then `spec/1.56.0.md`), at round 1
and round 2 alike: `grep -iE 'sub-agent|panel|alone|isolation|single'` over the `prompt` string
returns **one** line — *"joined, whitespace collapsed to single spaces, a list or blockquote marker
dropped,"* — about canonicalisation, and **nothing about how many readers the prompt has.** An earlier
draft said four lines; re-running returns one, and the error ran against this section's own case,
because at one match the claim is stronger. The sentence Report A quotes is `skills/tp/SKILL.md`'s
ground-loop step
*"Spawn **one** sub-agent on that prompt"*, and the reason the same section gives for the singular is
in the step above it: *"grounding asks one question of every unit, so there is no panel and no
role"*. **That is a claim about the panel, not about capacity** — Report A's diagnosis is correct,
and it is correct about SKILL.md rather than about the emission.

**Slicing already works, and that is measured, not inferred.** On the fixture, a payload holding one
row for one of three owed units records at **exit 0** with `rows: 1`, `--status` reports
`dispositioned: 2 of 3`, and `--status --check` exits **1**. So a slice is a well-formed round
contribution, the shortfall is reported, and the gate holds until the slices are all in. NDJSON
concatenates, and grounding has no `--merge` to need.

(The paragraphs ranking this repository's largest floors were deleted from the spec rather than
moved: the ranking they asserted does not hold at the current tree.)

## §11 A carried disposition has no escape hatch the protocol permits

**Constructed and confirmed, in the reporter's own shape.** A three-section fixture built outside this
repository: §1 *"The close window ends at 23:59 on the last day of the month."* (`u1`), §2 *"The test
asserts that a submission at 23:59:59 is inside the window."* (`u2`). Round 1 records `u1` `PASS` and
`u2` `PARTIAL/two-readings`, evidence naming §1 as the cause. §1 is then repaired to `23:59:59` —
`u2`'s bytes untouched. Round 2 emits:

```
u1 §1 38fa0f153fb5 #1 63B
u2 §2 0a4d14737541 #1 68B (carried)
```

`carried: 1`, and the ask reads *"This round owes a disposition for 1 of the 2 floor units above"*.
The repaired unit is asked about; **the unit the repair was for carries its stale `PARTIAL`**, and the
prompt tells the reader *"do not decide those units again, and write no row for them."* The
disposition survives every later round for as long as `u2`'s bytes stand.

**The hatch, measured on the same fixture** — round 2 recorded with a fresh `PASS` for `u2` beside `u1`'s row;
`--record` returned `rows: 2, carried: 0`, the round file holds the fresh row and not the inherited
one, and `--status` reports `PARTIAL: 0, PASS: 2`. `groundCarryForward` takes the round's own payload
as `decided` and does not carry what the round decides, which is the documented behaviour: *"A unit it
decides is not also carried."*

**Produced in the field again, by exactly the route this section predicts.** `spec/1.1.0.md`'s ground
round 3 carried a `PASS` whose note described a disposition the spec had since cut. The grading unit was
briefed to re-decide it, wrote a row for the carried unit, and `--record` took it at exit 0 with `carried`
falling from 11 to 10 — the override, reached the way §11 says it is reached, by a reader who did not
follow the prompt. Two things follow. The instruction to use the hatch must not be written into
`skills/tp/SKILL.md` while the prompt still forbids it: a skill saying *write the row* beside a prompt
saying *write no row for them* turns an undocumented gap into a documented contradiction, and §11's own
decision is what closes it. And the round that used the hatch reported the mechanism as unmeasured, a
reading that was relayed onward before this section was checked — it is measured here, and has been since
this file was written.

## §11.1 Two properties of the carry that the field instance exposed

**A plan expressed in unit ids cannot survive an emission.** Ids are re-assigned every time the floor is
emitted, while the carry is keyed on `(text_sha, ordinal)` — which §11 already notes it does not touch. An
id is therefore a coordinate in one round's index and nothing more. Measured on `spec/1.1.0.md`: the unit
holding the stale note was `u35` in round 2's index; in round 3 `u35` names a different sentence and is
`(cut)`, while the stale note's own text is `u46`. A sidecar had recorded the repair plan as *left for
round 3 to re-ask*, naming the id — and round 3 re-asks it under neither id, because the text was
unchanged and so it carried, as it would have into every round after. **Write a plan against the
`text_sha`, or against the sentence; never against the id.**

**A repair can remove the sentence it repairs from every future floor.** Round 2 filed a finding against a
uniqueness quantifier in `spec/1.1.0.md`'s only appeal to a source outside this repository. The repair
removed the quantifier; the shortened sentence fell below the arms' cut threshold; the sentence now sits
in no round's ask set. The finding was answered and the claim left the floor in the same edit, and nothing
in the round reports that. **Undecided, and not proposed as a decision here:** a sentence rewritten in
response to a finding could be exempt from the cut for one round, so that the repair is graded once before
the arms drop it. Neither the cost of that exemption nor whether it is expressible in the floor's own
terms has been measured.

**A third instance, and it is the one that shows the cost is not theoretical.** `spec/1.1.0.md`'s
final repair pass wrote a new requirement into §3 as a short standalone sentence; `cut` went **23 → 25**
and the requirement was not in the floor at all — the release's own new obligation, ungraded from the
moment it was written. The unit noticed because the brief obliged it to report `cut` before and after,
folded the sentence into the surrounding paragraph, deleted the closer, and `cut` returned to 23. So
across one document the pattern has now cost: a repair removing the claim it repaired (§2's uniqueness
quantifier), a narrowing removing the sentence the decision was about (§2's shared-rule sentence), and
a new requirement never entering the floor at all. **All three were invisible without a before/after
`cut` reading**, which is the cheapest form the exemption proposed above could take: not an exemption
at all, but a reported delta, since two of the three were repaired by the author the moment they saw
the number.

(The exemption is now an entry in `spec/undecided.md`.)

## §12 The coverage ratio's denominator counts non-claims

**Derived over this repository's own corpus.** Counting rule — every recorded ground round, rows by
`verdict`, `NOT-A-CLAIM` over the total, plus the same per round-1 file so the figures compare with
the reports' single-cycle numbers:

```
python3 -c 'import json,glob,collections;c=collections.Counter();[c.update([json.loads(l)["verdict"]]) for f in glob.glob("spec/.tp-review/*/ground-round-*.ndjson") for l in open(f) if l.strip()];print(c["NOT-A-CLAIM"],sum(c.values()))'
```

**323 of 1,459 rows, 22.1%**, across 28 rounds — §1's corpus, at the commit that adds this file;
re-derive rather than quoting it. Per round-1 file — 19 of them, 931 rows, pooled 23.3% — the spread
is what matters and the mean is nobody's experience: **4.4% (`mutation-run-check`) to 52.2% (`reconcile`), median 25.0%.**

**So the two field figures do not differ enough to matter, and that strengthens the ask rather than
dismissing it.** Report A's 37.2% sits between `two-advisories`'s 37.0% and `round-knows-its-panel`'s 38.5% — exact. Report
B's 24.6% is placed on the **range, not on an identity**: an earlier draft called it *"this corpus's
median to one decimal place"*, which was **false** when written (round-1 median 25.0%, all-rounds
23.9%) and became true afterwards only because one further recorded round moved the median onto it —
a figure a single round can flip is evidence about neither corpus. It is inside the 4.4–52.2% spread
and within half a point of the round-1 median, which is what holds under every reading. The useful
finding is the **12× spread inside one repository**: a share ranging from 4% to 52% document by
document is not a constant a reader can mentally correct for, which is why it has to be reported.

**And the payload cannot be corrected by hand, which is the measured half.** `emitted` and
`dispositioned` count **units**; `by_verdict` counts **rows** — `GroundStatus`'s own doc says so
(*"the breakdown's total is the round's row count and need not equal `Dispositioned`"*) — so
`emitted − by_verdict["NOT-A-CLAIM"]` subtracts a row count from a unit count. Constructed on the
fixture, a round recording one floor `NOT-A-CLAIM`, one reader-added `NOT-A-CLAIM` and one off-floor
`NOT-A-CLAIM` on a cut unit:

```
"emitted": 3, "dispositioned": 3, "reader_added": 1, "off_floor": 1,
"by_verdict": { "NOT-A-CLAIM": 3, "PASS": 2, ... }
```

The subtraction gives a claims-only denominator of **0** where the true answer is **2**. It is wrong
by exactly the rows that move neither side of the ratio, and nothing in the payload labels it as
unavailable — the two counts sit adjacent and read as commensurable.

## §13 `SKILL.md` did not name `UNVERIFIABLE`

**Measured, at the state both reports read.** Counting rule — occurrences, not lines, and a plain
substring match, so `FAIL` also counts `FAILED`; only the zero was load-bearing and a substring match
cannot manufacture one:

```
for v in PASS FAIL PARTIAL QUESTION NOT-A-CLAIM UNVERIFIABLE; do printf '%-13s %s\n' "$v" "$(git show v1.0.0:skills/tp/SKILL.md | grep -o "$v" | wc -l)"; done
```

`PASS` 15, `FAIL` 7, `PARTIAL` 2, `QUESTION` 1, `NOT-A-CLAIM` 1, **`UNVERIFIABLE` 0** — while
`skills/tp/REFERENCE.md` named it 3 times and the corpus in §1 holds 13 recorded `UNVERIFIABLE` rows.
Five of the six verdicts reached the operator-facing document; the sixth reached only the emitted
prompt and the reference. Report B: *"the operator driving the loop reads `SKILL.md`, not the emitted
prompt, so the full verdict vocabulary isn't visible where the decisions get made."*

**The commit *"name all six verdicts where the loop is driven, `UNVERIFIABLE` included"* closed it**
while §9–§14 were being written; the same run of the command above over `HEAD` returns
`UNVERIFIABLE` 1. Re-derive rather than reading that figure — it is a count over prose and will move.

**The guard is one-directional, and the direction it misses was built and run rather
than reasoned about.** It fails on the two ways the map goes stale — a seventh verdict appended to
`groundVerdictOrder` and left unnamed in `SKILL.md`, or a name dropped from `SKILL.md` — and it does
**not** fail when a verdict is deleted from `groundVerdictOrder`: the assertion quantifies over the
code's set and interrogates the document, so shrinking the set shrinks the quantifier and the
document is never asked what is missing. Measured in a copy outside this repository, all three arms
run in one package: control **PASS**, seventh verdict appended **FAIL** naming it, `VerdictUnverifiable`
removed **PASS**. Closing that third direction needs a second, opposite assertion — that `SKILL.md`
names no verdict outside the exported set, or a count — and this release does not take it. §7's
closed fog entry is why a guard is worth more here than a one-time fix: the corpus says
`UNVERIFIABLE` is exercised, so its absence was a gap in the map and not in the territory, and
nothing but a derived assertion notices a map going stale again.

## §14 A fear the tree refutes

**The fear is false.** Measured in a copy outside this repository: two consecutive bare emissions on an
unrecorded round return **the same round number, the same floor size and a byte-identical floor file**
(sha256 `40302ef74b9cbfdb…` before and after). Reproduced independently on the §11 fixture at round 3:
round unchanged at 3, `floor_size` 3, and
`shasum -a 256 spec/.tp-review/demo/floor-ground-round-3.txt` identical across the two runs.
**Re-emitting an unrecorded round is idempotent** — `NextGroundRound` answers *recorded rounds + 1*,
and the emission rewrites the same two files.

**The ask survives the correction, and that is why both halves are recorded.** Nothing in the
**output** says it is idempotent — an agent has to know, and Report B reasonably did not — and once a
round is recorded a bare emit legitimately opens round N+1, so the same call is safe in one state and
round-advancing in the next, **with no way to ask which state you are in from the emission's own
output**. That qualifier is load-bearing and an earlier draft omitted it: a second call does answer,
and it was measured — with round 4 emitted and unrecorded, `tp ground <spec> --status` returns
`"round": 4, "dispositioned": 0` at exit 0, and after a `--record` the same call reports a positive
`dispositioned`; the state directory answers it too, by whether `ground-round-4.ndjson` exists. Both
answers cost a call the reader that matters cannot make — the sub-agent holding only its prompt, which
is the same reader the next paragraph names. `tp ground` has four flags today (`--record`, `--status`,
`--check`, `--units`), and none of them re-prints a prompt.

**Half of this was closed while the section was being written, in the document and not in the
output.** The commit *"re-emitting an unrecorded round is idempotent, and a round file holds the
carry"* added the fact to `skills/tp/SKILL.md`'s ground loop, independently measured and agreeing
with the paragraph above. That closes it for a human reading the skill and changes nothing for a
sub-agent holding only its prompt, which is the reader Report B's six briefs were written for.

## §15 Tests — the mutant-column prose trimmed from rows 21 and 24

Row 21: append a seventh verdict to `groundVerdictOrder` and leave `SKILL.md` alone — the derived guard fails naming it, the literal-list version passes. The row's earlier mutant, *"write the six as a literal in the test"*, was built and run against both and **passes under both**, so it was replaced by one that discriminates.

Row 24: land `spec/backlog/what-the-carry-can-promise.md` §2.2's multiplicity fence: under it the unit is re-asked instead, the `FAIL` survives, and this row goes red — which is its purpose. It is a characterisation row, and the only one here: it exists so the limitation §9.3 states is retired by a failing test rather than left standing unnoticed.

## The silent overwrite: emitting over an unrecorded round

**Measured on this repository's own hotfix cycle.** A round was graded, the spec repaired before the
round was recorded, and `tp ground <spec>` re-emitted round 1 against the repaired text. The emission
**overwrote the unrecorded round's floor file without a word**, and `--status` then reported a round
with 0 dispositions — indistinguishable, from the record alone, from a round nobody has graded yet.

Recovery was possible only by accident: an `rsync` copy taken for unrelated work held the pre-repair
floor and most of the grader's rows. Recording those made the next round ask 38 of 44 units instead of
44. The cost of the missing guard is therefore measurable: one full round of grading.

## `FloorAnchorOf` bills six kinds of unit to an anchor no reader would predict

**Measured in this spec's third grounding round (the file was then `spec/1.0.1.md`)**, when a lint field grouped by anchor was
being specified and the specification kept describing behaviour the function does not have. The
field was dropped; the six defects are the function's and belong here. Each is a fixture built and
run against `engine.FloorAnchorOf` at the commit that round graded.

## The two zeros (from the absorbed spec)

Moved from the absorbed two-zeros spec (deleted after absorption), whose three decisions are now §16 of
the spec. Every figure below was re-measured against `dbf7fdeb` in an
`rsync -a --exclude .git` copy outside the repository rather than relayed from the handover that
routed these findings here — and the third claim came back **different** from the way it was
handed over.

### The two emissions, measured

Measured on two documents built for this file — A is a heading plus two fenced blocks, B is four
prose sentences no arm keeps. Their floor indexes read `# 0 in floor, 0 cut` and `# 0 in floor,
4 cut`. Both asks, verbatim, **byte for byte identical**:

```
This round owes no dispositions: every unit in this document was cut (§2.1), so
there is no floor to ground.
```

Both envelopes, prompt removed, byte for byte identical (wrapped here, one line on the wire):

```json
{"spec":"spec.md","round":1,"snapshot":".tp-review/spec/snapshot-ground-round-1.md",
 "floor":".tp-review/spec/floor-ground-round-1.txt","output_path":"ground-r1.ndjson",
 "floor_size":0,"carried":0}
```

A unified diff over the two whole prompts returns the index block and nothing else.

**The derivation.** Build the two documents in a copy outside the repository, emit in each, and
compare:

```
python3 -c 'import json;a=json.load(open("A/emit.json"));b=json.load(open("B/emit.json"));
pa=a.pop("prompt");pb=b.pop("prompt");k="This round owes";
print("envelope identical:",a==b);print("ask identical:",pa[pa.index(k):]==pb[pb.index(k):])'
```

**Three shipped places already separate the two.** §11 row 23's fixture pair, `GroundStatus.Cut`'s
doc (`internal/engine/groundstatus.go:34-41`, *"only the first is a document nobody checked"*), and
`runGroundStatus`'s second `--check` condition — exit 0 on one, exit 1 on the other. Two do not, and
they are the two an operator meets **first**, before any round exists for `--status` to report:
`groundPromptAsk` (`internal/cli/ground.go:1129`) returns one literal for both, asserting *"every
unit in this document was cut"* — false on the first document, and contradicted by the index block
four lines above it **in the same prompt**; and `groundResult` (`internal/cli/ground.go:41-50`)
carries `floor_size: 0` and no `cut`, so nothing a caller can read without parsing the prompt string
tells the two apart.

### The guard on `groundPromptAsk`, and a correction

`TestTheAskAgreesWithTheCountsItStates` states its input set as `(floorSize ∈ {0, 1, many}) ×
(carried ∈ {0, 1, all})`, and `groundPromptAsk`'s doc comment ends *"walks the whole set"*.

**Measured: the sentence is false, and the handover's reason for it was not quite right.** The
handover says the default arm — `2 ≤ carried < floorSize`, the ordinary shape of a settling round —
is *omitted*. It is not omitted from execution. Tracing every call of `groundPromptAsk` during
`go test ./internal/cli` returns **96 calls over 11 distinct `(floorSize, carried)` pairs**, and the
default arm is reached **twice**, at `(4, 3)`, through
`TestTheRoundTwoPromptAsksOnlyForTheDispositionsItOwes`. What is missing is the **assertion**: that
test's `Contains` stops at `"for 1 of the 4 floor units above"`, before the colon, so the clause the
arm actually renders is never compared to anything.

The consequence is the same and it is measured: mutating `internal/cli/ground.go:1158` from
`"the other %d already carry"` to `"the other %d already carries"` leaves **`go test ./...` green**
on every package, in a clean copy at `dbf7fdeb`.

`grep -rn 'already carry' --include='*_test.go' .` returns four lines. Two are prose comments; of the
two assertion sites, one is the **all-carried** arm's clause and the other is
`ground_ask_test.go:220`, an `assert.NotContains` on `"the other 2 already carry"`. So the default
arm's clause is asserted **absent** in one place and **present** in none — and a mutant that changes
the string leaves the `NotContains` satisfied.

### What the absorbed spec fenced out

- No third sentence in `groundPromptAsk` — three consecutive audit rounds of findings on this
  function were each produced by a repair that added text about the state its author had just built
  a fixture for, and each missed the sibling state that fixture excluded.
- No gate change, no exit-code change, no refusal. Measured on both documents: `--units` prints
  zero bytes and exits 0, and `--record` of an empty payload is refused with the same code and the
  same message. The unit's action is identical under both readings today.
- `groundRecordEmptyHint` (`internal/cli/ground.go:151`) is not repaired: it is byte-identical on
  both documents and carries *"If the prompt asked for no dispositions because every unit was cut
  (§2.1)…"* — a conditional with a false antecedent on the 0-cut document, unhelpful rather than
  false. It goes to whichever release next opens `--record`'s hints.
- Only the two sinks the finding names were measured; no claim is made about a third.

### The absorbed spec's tests

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | the ask | on the two zero-floor documents emitted through the command, the asks **differ**, and the 0-cut ask makes no claim that anything was cut | the shipped single literal, byte-identical on both — so the row is observably red against `dbf7fdeb` before the change, and is written first |
| 2 | the ask *past the opening* | the assertion is on the clause **after** `This round owes no dispositions:`, which both literals keep | assert the opening alone — `ground_ask_test.go:254`'s shipped shape, which passes under the collapsed literal and under every possible repair |
| 3 | the ask *no count* | on a document with an empty floor and **exactly one** cut unit, the emitted ask is byte-equal to the `cut > 0` literal | print the count in the sentence, which renders `1 units` on this exact fixture and forces the declension case the non-goal forbids; a fixture with four cut units passes either way |
| 4 | the ask *the other arms* | the five non-empty arms render identically before and after the parameter is added, asserted on the same fixtures | let `cut` reach a non-empty branch, changing what an ordinary settling round reads |
| 5 | the envelope | across the two documents every envelope key is equal except `cut`, which is 0 and N, and N equals the count read back off the emitted floor index | omit the key — the shipped state, measured to leave the two payloads byte-identical |
| 6 | the envelope *a count, not a flag* | on a **partly**-cut floor `cut` is the index's cut count beside a non-zero `floor_size` | set `cut` only when `floor_size == 0`, which reports 0 for every document that has a floor and makes the key unreadable |
| 7 | the generator | the subtest table is generated from a list of `(floorSize, carried)` pairs, and a `require` asserts that list holds a pair with `2 ≤ carried < floorSize` | a loop that generates only the five pairs the hand-written subtests named — the same test with a `for` around it, which is what an implementer will write |
| 8 | the whole clause | each generated pair asserts the sentence from `This round owes` through the end of the clause, not a prefix of it | `"the other %d already carries"`, measured green on `go test ./...` at `dbf7fdeb` |
| 9 | action unchanged | on both documents `--units` prints zero bytes at exit 0, and `--record` of an empty payload exits 1 with the same message | branch `--record` or the emission on `cut`, turning a reporting fix into a gate change |
| 10 | one source | `--status --check` still exits 0 on the 0-cut document and 1 on the N-cut one, and its `cut` still comes from `GroundStatus` | route the gate through `groundResult.Cut`, giving §11 row 23's condition a second source that can drift from the first |

**Row 7 is the one an implementer will get backwards.** The defect measured above is not that the
default arm is unreached — it is reached, twice, and traced. It is that nothing compares what the arm
renders. A table built from the five pairs the old subtests already covered reproduces exactly that:
every pair passes, the arm runs, and the mutant in row 8 still ships green. The `require` on the
generated list is what makes the loop a fix rather than a refactor.

**Rows 1, 3 and 5 are red against `dbf7fdeb` and are written and watched red first** — they are the
three the defect itself fails. Row 8 is **green** against `dbf7fdeb` and red only against its named
mutant, because the production string is correct today; that is the whole point of it, and stating
the difference is what keeps a row that pins working text from being read as a row that repairs
broken text.
