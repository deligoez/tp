# tp v1.0.1 — The ground command's own friction

> **This file is decisions.** Four defects in `tp ground` itself, every one measured while running the
> command against this repository's own specs rather than reasoned about. Each states the command that
> derives its figures; no figure appears here without one. **Three of the four are in the surfaces
> around the grounding protocol** — what the emission names, what `--record` says when it refuses, and
> what `--units` will show you. **The fourth is in the protocol itself**: §5's marker units come out of
> `floorBlocks`, which is the floor, and an earlier draft of this paragraph said all four were
> surface-level. That was wrong on one command and is recorded rather than quietly corrected.
>
> What did hold, across the corpus below: the carry, the six verdicts, the kind–tier table and the
> atomicity of `--record`. Every defect here is about naming, reporting or splitting — none is about
> a judgement the protocol got wrong.
>
> This file replaces `spec/candidates.md`. That file held three kinds of thing; its refuted record and
> its non-ground undecided items moved to `spec/undecided.md`, four of its undecided rows moved into
> the pending specs that own their subject, and its closed sections were deleted after each was
> verified present in the release that took it. What is left is about `tp ground`, which is what this
> release is.
>
> **This file has outgrown a patch, and saying so is this paragraph's whole job.** It opened as four
> defects one orchestrator measured against this repository's own specs (§2–§5). It now also carries
> **six more** (§9–§14), from two independent field reports on `tp ground` v1.0.0 run in two other
> repositories, on two Turkish specs, by two sessions that had not seen each other. **Ten decisions,
> one of which changes an exit code** — §9 gates `--status --check` on a standing `FAIL`, which
> Non-Goal 5 forbade until §9 amended it in §6. That is a minor release's surface, not a patch's. The
> **operator decides** whether this stays `1.0.1` or splits, and **nothing here is renumbered**: the
> roadmap table in `CLAUDE.md` is the only place a release number belongs, and a spec that names its
> own successor is how three renumberings left stale citations behind.

## 1. Overview

**The evidence base.** A grounding programme ran `tp ground` over every pending spec in this
repository. Counting rule — every row of every `spec/.tp-review/*/ground-round-*.ndjson`:

```
python3 -c 'import json,glob,collections;c=collections.Counter();[c.update([json.loads(l)["verdict"]]) for f in glob.glob("spec/.tp-review/*/ground-round-*.ndjson") for l in open(f) if l.strip()];print(sum(c.values()),c)'
```

At the commit that adds this file: **28 rounds, 1,459 rows** — `PASS` 787 (53.9%), `NOT-A-CLAIM` 323
(22.1%), `PARTIAL` 230 (15.8%), `FAIL` 95 (6.5%), `UNVERIFIABLE` 13 (0.9%), `QUESTION` 11 (0.8%). All
six verdicts and all six tiers were exercised. **Re-derive rather than quoting these**: the corpus
grows, and the shares are the stable part, not the counts.

The four defects, in the order they cost the most:

1. **The scratch file the emission names is not unique across specs** (§2). Two units grounding two
   different specs are told to write the same filename, and the second overwrites the first at exit 0.
2. **`--record` diagnoses one bad row per invocation** (§3), so a payload with *N* schema violations
   costs *N* round trips.
3. **A cut unit cannot be addressed** (§4). `--units` lists floor units only, so a reader who finds a
   defect in text the arms cut has no id to file against and must reconstruct the hash.
4. **A bare ordered-list marker becomes a floor unit** (§5) — two bytes, no claim, and a disposition
   the round is obliged to spend.

### 1.1 The second evidence base: two independent field reports

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
survive that check. Every claim below was re-derived here on a freshly built binary, against this
repository's own specs or against a fixture built outside it, before it was written down. Two did not
survive: §11's escape hatch exists in the mechanism and is closed by the prompt rather than absent,
and §14's fear is simply false. Both are recorded with the measurement that refutes them.

**Two of the four defects above were confirmed independently by the reports**, which is worth stating
because it is the only external replication any entry here has. §3 by Report A — *"reported only the
FIRST offending row, so the loop is fix-one, re-run, hit-the-next — I ended up telling every agent
about it in its brief, which is duty the prompt should carry"* — the same workaround this file already
records the orchestrator performing, arrived at separately. And §4 by Report B — *"`cut: 74` with no
indication of what was dropped or why… a sentence's justification lived in a *cut* unit, so the graded
unit only held up because of text the arms had removed"* — which is §4's *"the sharpest finding of a
round sat in cut text again and again"*, reached from the other end.

### 1.2 Six further defects, and where they sit

**One of them costs more than any of the four above.** In the order they cost the most:

5. **`--check` exits 0 with `FAIL`s standing** (§9). Both reports hit it independently. Its pair,
   §9.2: grounding's `--status` carries no `next_action` where review's and audit's do.
6. **A round cannot be driven from the envelope** (§10): the ask set is a count and a prose marker
   (§10.1), a growing floor cannot say what grew (§10.2), and a 401-unit floor has no sharding path
   while the sentence that seems to forbid one is `SKILL.md`'s and means something else (§10.3).
7. **A carried disposition has no escape hatch the protocol permits** (§11) — the hatch exists, the
   prompt closes it, and the sink does not report an override.
8. **The coverage denominator counts non-claims** (§12), and the payload cannot be corrected by
   subtraction because it mixes a unit count with a row count.
9. **`SKILL.md` did not name `UNVERIFIABLE`** (§13) — five verdicts of six reached the document the
   operator drives the loop from. **Closed by commit** while §9–§14 were being written; §13 keeps the
   measurement and routes the durable form to a derived guard.
10. **A fear the tree refutes** (§14): re-emitting an unrecorded round is idempotent, measured twice.
    The ask survives the refutation, because nothing in the **output** says so and the same call
    advances a recorded round.

**Three of these moved under this file while it was being written, and none of the three moved the
code.** Three commits to `skills/tp/SKILL.md` — recoverable by subject through
`git log --oneline -- skills/tp/SKILL.md` — closed the documentation half of §9, the whole of §13,
and half of §14 between the reports arriving and these sections being finished. Each section says so
where it says what it measured, with the state both reports read named as the state it was measured
at. **`tp ground spec/1.56.0.md --status --check` still exits 0 with four `FAIL`s standing after all
three**, re-run to check, which is why §9 is the highest-cost item in the file and not a closed one.

**The numbering is by position and the cost order is this list, and that is a deliberate split.**
§9–§14 are appended after §8 rather than inserted after §5, because every row of §8's table names §2,
§3, §4 or §5 and a renumbering would falsify all nine at once — this repository has paid for three
renumberings and one cross-reference sweep that found 47 citations, ~35 of them stale, none catchable
by checking that the target file exists. §15 carries the new rows in §8's shape and continues its
numbering, so no existing row index moves either.

## 2. The scratch file the emission names is not unique across specs

**Measured.** `internal/cli/ground.go`, in the emission path:

```go
outputPath := fmt.Sprintf("ground-r%d.ndjson", round)
```

The round is in the name; the spec is not. So:

```
$ tp ground spec/1.57.0.md | python3 -c 'import sys,json;print(json.load(sys.stdin)["output_path"])'
ground-r1.ndjson
$ tp ground spec/1.58.0.md | python3 -c 'import sys,json;print(json.load(sys.stdin)["output_path"])'
ground-r1.ndjson
```

Two units grounding two specs concurrently are told to write the same file. The second finishes and
**overwrites the first at exit 0** — no lock, no warning, and nothing downstream can tell that a
round's dispositions were replaced by another spec's.

**This is not the recorded round's name, and the difference is deliberate.**
`engine.groundRoundFileName` carries the reason in its own doc comment: `ground-rN.ndjson` is *"the
scratch file a unit writes and an operator collects"*, while `--record` writes
`ground-round-<N>.ndjson` beside the snapshot and the floor. **That decision is not what this section
changes.** The defect is narrower: a scratch name must still be unique among the things that may be
scratched at the same time, and the spec base is the only thing that separates them.

**The decision: put the base in the scratch name** — `ground-<base>-r<N>.ndjson`, where `<base>` is
the same base the state directory already uses (`spec/.tp-review/<base>/`), so nothing new has to be
derived and no two specs can collide.

**The sibling case is named and NOT taken here.** `roleOutputPath` gives review and audit
`review-r<N>-<role>.ndjson` and `audit-r<N>-<role>.ndjson`; those qualify by round and role and are
equally un-qualified by spec, so two concurrently-reviewed specs collide the same way. It is left out
because this release is `tp ground`'s friction and because the review/audit collision needs its own
acceptance over per-role emission, not because it is not real. `spec/undecided.md` is not the right
home for it either — it has an obvious design and no undecided part; it wants a release.

**How the workaround measured.** During the grounding programme every brief carried a hand-written
override — *"write to `ground-<spec>-r<N>.ndjson`, NOT the `ground-rN.ndjson` the envelope names"* —
because three units ran concurrently throughout. A default that every caller must override is the
tell that the default is wrong.

## 3. `--record` diagnoses one bad row per invocation

**Two properties, and only one of them is a defect.**

**Atomicity is correct and stays.** `engine.RecordGroundRound`'s doc comment states it and gives the
reason from §7.2 of the ground spec: *"a partially valid round would make coverage a lie"* — a round
recorded with its bad rows dropped is counted as a round in which those units were decided, and
nothing in the record says otherwise. Validation completes before anything is opened, created or
truncated. **Nothing here changes that.**

**Diagnosis is first-error-only, and that is the defect.** `parseGroundRows` returns on the first
invalid row, so the caller reports one. Measured on a 73-row payload with three rows broken the same
way (a `document` claim given `tier: run`, which §4.1 refuses):

```
{"error":"line 2: field \"tier\": \"run\" says nothing about a \"document\" claim (§4.1), and a PASS row must be reached at a tier that does", ...}
```

One line named, three broken. Each fix is a full round trip, and in the reset-native model a round
trip is a subagent re-invocation.

**Why this matters more than its size.** It is a **P2 violation on tp's own terms**: what is easy for
one row is not equally easy for *N*. The tell is what the operator did instead — during the grounding
programme the orchestrator wrote its own whole-file validator against §4.1's kind–tier table and ran
it before every `--record`, four times catching two or three violations at once that tp would have
reported one at a time. **When the caller has to reimplement the tool's validator to use the tool
efficiently, the validator is missing something.**

**The decision: collect every row's violations and report them together**, keeping the write atomic
exactly as it is. The refusal stays one refusal — it gains a list.

## 4. A cut unit cannot be addressed

**Measured.** `--units` lists floor units. A reader who finds a defect in text the floor's arms cut
has no id to file against:

```
$ tp ground spec/1.56.0.md --units | cut -f1 | grep -cE '^u(15|16|17|18|19|38|49|50|55)$'
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

**The decision: `--units` lists cut spans too**, each with an id, its `text_sha` under the same
canonicalisation the floor uses, and a marker saying it is cut. A reader then files against a real id
and a hash tp computed, and a wrong join becomes impossible rather than repairable.

**What this does NOT do, so it is not read as more than it is.** It does not put cut text in the
floor, does not make a cut unit owed a disposition, and does not change coverage or `--check`. The
floor's arms decide what a round *must* answer; this decides what a reader can *name*. Whether the
arms cut the right things is a different question and is not taken here.

## 5. A bare ordered-list marker becomes a floor unit

**Measured.** Counting rule — floor units whose whole text matches `^[0-9]+[.)]$`, over every pending
spec plus `spec/1.0.0.md`, read from `tp ground <spec> --units` (TSV: `unit_id`, `text_sha`, `text`):
**8 such units across 2 specs, out of 1,968 floor units scanned — 0.41%.** They are `u84 u87 u89 u91`
in `spec/1.41.0.md` and `u71 u73 u93 u100` in **`spec/1.0.0.md`**, the document that defines the floor.

**The marker is not a line in the source — the splitter makes it.** No line in either file matches
`^\s*[0-9]+[.)]\s*$` (`0` hits in both). The items are single lines of the form
`N. **Bold phrase.** text…`, and the split happens *inside* the line.

**Two things are unexplained and are left unexplained rather than guessed.** In `spec/1.41.0.md` the
items are numbered 1–5 and only `2.` through `5.` become units. In `spec/1.0.0.md`, 16 lines match
`^\s*[0-9]+[.)]\s+\*\*` and only 4 produce a marker unit. So the shape is necessary and not
sufficient, and what selects the four is not known. An earlier draft of this entry asserted a
mechanism — items whose bold phrase sits on the *following* line — and one command refuted it.

**The decision: the splitter drops a fragment whose whole text is an ordered-list marker.** The test
is the marker shape, not a length floor. A length floor would work *here* — the shortest legitimate
non-marker floor unit across the same corpus is **9 bytes** (`Measured:`, `spec/1.48.0.md` `u49`)
against the markers' 2, so any threshold in 3–8 separates them — and it is still the wrong rule,
because it says nothing about what it means and would silently start dropping real units the day
someone writes a shorter one.

## 6. Non-Goals

**Two of these were falsified by §9–§14 and are amended in place rather than deleted.** Both were
true of the four-defect release this file opened as, and neither is true of what it now holds; a
non-goal quietly dropped is the shape §5 and §14 exist to refuse.

1. **No change to the floor's arms.** Which sentences reach the floor is the grounding protocol's own
   question and §4 above deliberately stops short of it. **Unchanged** — §10.3 splits the reading of
   a floor and never the floor.
2. **No change to `--record`'s atomicity or to §7.2's table.** The corpus exercised both and they
   held; §3 changes what a refusal *says*, never what it refuses. **Amended:** this read "or to §8's
   carry" until §11, which leaves the `(text_sha, ordinal)` join and the inheritance untouched but
   changes what the prompt permits and what `--record`'s envelope reports about an override.
3. **No change to the recorded round's filename.** `ground-round-<N>.ndjson` is deliberate and §2
   says why it stays.
4. **The review/audit scratch-name collision is named in §2 and not fixed here.**
5. **No new workflow field.** Nothing here adds a knob to `.tp/config.json` or to a task file's
   `workflow` block, and nothing reads one. **Amended:** this read "no gate, no convergence effect —
   nothing in this release changes `clean`, a streak, coverage or an exit code" until §9, which
   changes `tp ground --status --check`'s exit code on a round holding a `FAIL`. It still touches no
   `clean` flag, no role streak and no review or audit convergence: grounding records by filename and
   reads no `state.json` key, which is what keeps a gate change here local to grounding.
6. **No `--merge` for ground, and no orchestration.** §10.3 establishes that a slice records and
   gates correctly and that NDJSON concatenates; it adds no command to do the concatenating and
   nothing that spawns, schedules or counts readers.

## 7. Open questions inherited when `spec/candidates.md` was split

These are grounding's own undecided items. They have no design and are **not** decisions this release
takes; they are here because this is now the file that owns `tp ground`.

- **Claim enumeration** — the weakest step of the grounding protocol. Intuition counted 11 where a
  spec carried 17, and 10 where another carried 17 again after a second read. Whether that is a
  parsing problem, a definition problem, or irreducibly a reading problem is not yet clear. §5 above
  is one small, measured piece of it; the rest is not.

**One entry arrived here already answered, and is recorded as answered rather than carried.**
`candidates.md` listed *"What `UNVERIFIABLE` costs"* as fog, on the ground that there were **zero
instances across 44 grounded claims**, so the verdict was designed and untested. Re-derived over the
corpus in §1: **13 `UNVERIFIABLE` rows and 11 `QUESTION` rows in 1,459**. Both verdicts are exercised,
and neither is rare enough to call untested. The fog entry is closed by measurement, not by a decision.

## 8. Tests

Every row derives from a numbered decision and names an input that must fail it. Where a row's mutant
is a change to the test rather than to the product, the row says so.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | two specs emitted at the same round produce **different** `output_path` values, asserted on the pair rather than on either alone | keep the round-only name, under which the two are equal and a concurrent run silently overwrites |
| 2 | §2 | the emitted `output_path` contains the same base the state directory uses, asserted by deriving the base rather than by matching a literal | hardcode a base in the test, which passes for the fixture spec and for no other |
| 3 | §2 *unchanged* | the **recorded** file is still `ground-round-<N>.ndjson` | rename the recorded file to match the scratch name, which is the decision §2 explicitly does not take |
| 4 | §3 | a payload with **three** rows broken three *different* ways reports three violations naming three distinct line numbers | report the first and stop — the shipped behaviour; a fixture breaking three rows the *same* way would pass a mutant that deduplicates by message, which is why the three differ |
| 5 | §3 *atomicity* | after a refused `--record`, the state directory is byte-identical to what the preceding emission left | validate row by row as each is appended, which satisfies §3's wording for a payload whose first row is bad and breaks it for every other |
| 6 | §4 | `--units` lists an id for a span the floor cut, and the `text_sha` on that line equals the hash of the text printed beside it | list the cut span with no hash, which puts the reader back to guessing the canonicalisation — the exact failure 13 recorded rows show |
| 7 | §4 *bounded* | a cut unit is **not** owed a disposition: coverage and `--check` answer identically before and after §4, on one spec with at least one cut span | count cut spans toward coverage, which makes every document permanently uncovered |
| 8 | §5 | a document with a `N. **Bold.** text` list produces no floor unit whose whole text matches `^[0-9]+[.)]$`, **and** produces the same number of other units as before | drop any short fragment, which also drops `Measured:` — the 9-byte legitimate unit §5 names |
| 9 | §5 *not a length rule* | a legitimate unit of 3 bytes survives | implement §5 as a length floor, which is sufficient on today's corpus and fails this row |

## 9. `--check` exits 0 with `FAIL`s standing

**This is the highest-cost item in the file** — higher than §2–§5, which is why §1's list states the
cost order rather than the section numbers doing it. Both field reports hit it independently, neither
having read the other.

**Measured, and still true at the commit this section was last edited.** `spec/1.56.0.md`'s recorded
round 1, counting rule — every row of the round file, by `verdict`:

```
python3 -c 'import json,collections;c=collections.Counter(json.loads(l)["verdict"] for l in open("spec/.tp-review/1.56.0/ground-round-1.ndjson") if l.strip());print(dict(c))'
```

**`FAIL` 4**, `PASS` 40, `PARTIAL` 12, `NOT-A-CLAIM` 17. And:

```
$ tp ground spec/1.56.0.md --status --check; echo $?
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

### 9.1 The decision: `--check` gains a third condition

**`--check` exits 1 when the latest recorded round carries a row whose `verdict` is `FAIL`.** The two
conditions it already has are unchanged, and the command-table sentence quoted above goes with the
behaviour it describes.

**`FAIL` and nothing else, and the reason is the protocol's own.** §8 makes an unrepaired `FAIL`
permanent while its text stands — it is the one verdict the carry already treats as an obligation
rather than as an answer. A `PARTIAL` carrying `partial_kind: true-when-written` is a settled
disposition with a `held_at` beside it, and `QUESTION` is explicitly non-blocking in the same
SKILL.md step that asks for the repairs. Neither becomes a gate here. Both stay visible: `by_verdict`
already reports them and this change removes nothing from it.

**This gate cannot deadlock, and that is measured rather than asserted.** The audit phase's
equivalent — a `FAIL` accepted with evidence still recording `clean: false`, so the only way out is
destroying the record — is the defect the accepted-finding release takes. Grounding does not have it,
because a ground `FAIL` has two exits and the fixture ran both. **Repairing the unit's text** drops
its carry: an edited unit is re-asked, its hash having moved — measured on §11's fixture at its
round 3, where `u2`'s text gained two words, its hash went `0a4d14737541` → `a4bec125b33d`, and it
left the `(carried)` set in that same round. **Deciding it
again in a later round** also works: a fresh row for a carried unit overrides the carry and records
at exit 0 (§11). The second exit is legal in the mechanism and forbidden by the prompt, which is why
**§9 depends on §11** and neither should ship without the other.

### 9.2 `--status` carries no `next_action`, unlike review's and audit's

**Measured.** The payload's keys, in full:

```
tp ground <spec> --status | python3 -c 'import sys,json;print(sorted(json.load(sys.stdin)))'
```

`by_verdict, cut, dispositioned, emitted, off_floor, reader_added, round, spec`. Review's and audit's
`--status` both carry `next_action` — *"the single next step"*, in SKILL.md's words for each —
and grounding's does not. Report A: *"nothing in the payload tells a driver 'you have unrepaired
`FAIL`s'."* Both reports asked for it.

**It is a pair with the gate above, not a separate want.** The exit code says *not yet*; the key says
*what to do*, which is how a driver branches without parsing prose. A gate added without it leaves
every caller re-deriving the branch from `by_verdict`, which is exactly the habit Report B says was
the only thing that saved its round 1.

**The decision: `--status` carries `next_action`, under that name and in that role.** No new
vocabulary: it names the same step SKILL.md's loop names, and it is reporting, so a driver that
ignores it is exactly as correct as one that reads it.

## 10. Driving a round: three things the emission cannot hand a driver

Three findings that look separate in the reports and are one subject: what a process other than the
reader itself can do with a round. Each is measured on the shipped binary.

### 10.1 There is no machine-readable ask set

Report A calls this *"the single biggest papercut"*. **All three of its claims hold.** Measured on
`spec/1.56.0.md`'s round 2 in a copy outside this repository — floor 61 units, all 61 carried:

- **`carried` is a count.** The envelope's keys are `carried, floor, floor_size, output_path, prompt,
  round, snapshot, spec`, and `carried` is the integer 61. No list of ids appears anywhere in it.
- **The `(carried)` marking is prose only.** `grep -c '(carried)' ` over the emitted `prompt` returns
  **62** — 61 index rows plus the ask sentence. It is inside a string a driver would have to parse.
- **The floor file on disk is unmarked.** `grep -c '(carried)' spec/.tp-review/1.56.0/floor-ground-round-2.txt`
  returns **0**. That is deliberate and stays: `runGround`'s own comment gives the reason — *"a copy in
  the floor file would be a second statement of the same fact with nothing comparing the two"* — and
  §2's floor is the artifact the round is graded against.

So a driver that wants the ask set reconstructs it. Report A's reconstruction — join
`floor-ground-round-N.txt` against `ground-round-(N-1).ndjson` on `(text_sha, ordinal)` — is
**exactly `GroundCarriedRows`**, which joins on the same pair for the same reason. *"A 20-line script
every driver will rewrite"* is precise: the script is a reimplementation of shipped tp code, and this
is the same P2 tell §3 records, in a different surface.

**The decision: the emission's envelope carries `asked`, the list of `unit_id`s this round owes.** A
list beside the count, not instead of it — `carried` and `floor_size` are what an operator branches
on and stay exactly as they are. It is added to the **envelope**, never to the floor file, for the
reason quoted above. And it is the shipped fact rather than a new one: `asked` is
`floor_size − carried` ids, the same set the prompt's ask sentence already names in English.

### 10.2 Floor growth conflates two different things

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

**The decision: the envelope reports `floor_delta` against the preceding round's floor —
`{added, removed, unchanged}`, counts over `text_sha`.** On 191 → 225 that separates 34 net new
sentences from whatever churn replaced existing ones, which is the question the growth curve raises.
Stated with its limit, because the limit is the reason the other cut was refused: **a rewritten
sentence is one `added` and one `removed`, indistinguishable from an unrelated insertion and an
unrelated deletion.** The counts bound the edit churn; they do not identify it.

### 10.3 A large floor has no sharding path, and the instruction that seems to forbid one is not the emission's

Both reports. Report A split 191 units into 5 slices by section anchor, each agent spending 100–250k
tokens on 27–49 units, and writes: *"'Spawn ONE sub-agent on that prompt' does not survive a 191-unit
floor. The instruction reads as a correctness constraint (no panel, no roles) but lands as a capacity
claim."* Report B: 213 units, 306k tokens, 83 tool calls — *"it held, but a spec 2× this size would
blow the context."*

**The emitted prompt does not say it.** Measured over the emission for `spec/1.56.0.md`:
`grep -iE 'sub-agent|panel|alone|isolation|single' ` over the `prompt` string returns four lines, all
of them about `text_sha`, whitespace collapsing, and what the unit reports — **none about how many
readers the prompt has.** The sentence Report A quotes is `skills/tp/SKILL.md`'s ground-loop step
*"Spawn **one** sub-agent on that prompt"*, and the reason the same section gives for the singular is
in the step above it: *"grounding asks one question of every unit, so there is no panel and no
role"*. **That is a claim about the panel, not about capacity** — Report A's diagnosis is correct,
and it is correct about SKILL.md rather than about the emission.

**Slicing already works, and that is measured, not inferred.** On the fixture, a payload holding one
row for one of three owed units records at **exit 0** with `rows: 1`, `--status` reports
`dispositioned: 2 of 3`, and `--status --check` exits **1**. So a slice is a well-formed round
contribution, the shortfall is reported, and the gate holds until the slices are all in. NDJSON
concatenates, and grounding has no `--merge` to need.

**This repository's own largest floor is bigger than either report's.** Counting rule — lines of
`tp ground <spec> --units`, which prints one per floor unit, run over every spec in a copy:

```
for f in spec/*.md; do echo "$(tp ground "$f" --units 2>/dev/null | wc -l) $f"; done | sort -rn | head -3
```

**401 `spec/1.0.0.md`**, 136 `spec/1.55.0.md`, 127 `spec/1.50.0.md`. Against the reports' 226 and 240.
So this is not a foreign-environment artifact: the document that *defines* the floor has a floor 1.7×
the larger field case.

**The decision has two halves and neither is a code change to the emission.** SKILL.md's step says
what it means — **one panel, any number of readers**, because the floor is a partition and a slice of
it is a well-formed ask, with the recorded evidence above that a slice records and gates correctly.
And the emitted prompt **stays silent** on the question, deliberately: it is addressed to whoever is
reading the units, and a sentence about process there would be the emission telling an orchestrator
how to spend its context, which is not something tp knows. `asked` (§10.1) is what makes a slice
expressible without a join, which is why §10 is one section and not three.

## 11. A carried disposition has no escape hatch the protocol permits

Report B. A `PARTIAL` on unit X whose *cause* lives in section Y: fix Y, X's own text is unchanged, so
X carries its stale `PARTIAL` forward indefinitely. Their concrete case is a test-case unit flagged
because §4's window bound said `23:59` while the test said `23:59:59`; they fixed §4 and the unit
still carried the old `PARTIAL`.

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

**The report's claim is right in effect and wrong in mechanism, and the difference is the whole
decision.** A hatch exists: a row naming a carried unit **overrides the carry** and records at exit 0.
Measured on the same fixture — round 2 recorded with a fresh `PASS` for `u2` beside `u1`'s row;
`--record` returned `rows: 2, carried: 0`, the round file holds the fresh row and not the inherited
one, and `--status` reports `PARTIAL: 0, PASS: 2`. `groundCarryForward` takes the round's own payload
as `decided` and does not carry what the round decides, which is the documented behaviour: *"A unit it
decides is not also carried."*

So the mechanism has the hatch and **the protocol closes it**. Two sentences do: the ask states the
unit is not owed, and the prompt says to write no row for it. A reader following the prompt cannot
reach the override; a reader who ignores the prompt gets it silently, and the `--record` envelope
reports `carried: 0` with nothing saying an inherited disposition was displaced.

**The decision, in two parts.** The prompt **names the override** — a carried disposition may be
re-decided by writing a row for that unit, and the round that does so says why in `note` — replacing
the unconditional *"write no row for them"* with the condition it means: do not re-decide a carried
unit **to repeat its verdict**; re-decide it when the ground beneath it moved. And `--record`'s
envelope **reports the displacement**: a count of carried dispositions the payload overrode, beside
`rows` and `carried`, so an override is never silent at the sink. Both are needed for §9: the gate on
a standing `FAIL` needs an exit that the protocol permits and the record shows.

**What this does not do.** It does not make the carry re-derive a disposition, does not invalidate a
carry when another unit changes — tp cannot know that §1 is *why* §2 was `PARTIAL`, and the fixture's
`note` is the only place that lives — and does not touch the `(text_sha, ordinal)` join. It makes the
override sayable and visible; deciding when it is right stays the reader's.

## 12. The coverage ratio's denominator counts non-claims, and the payload cannot be corrected by subtraction

Report A: 84 of 226 units were `NOT-A-CLAIM`; Report B: 59 of 240. Both want a claims-only denominator
beside the raw one, on the ground that a ratio over a floor a third of which asserts nothing is not
the number it looks like.

**Derived over this repository's own corpus.** Counting rule — every recorded ground round, rows by
`verdict`, `NOT-A-CLAIM` over the total, plus the same per round-1 file so the figures compare with
the reports' single-cycle numbers:

```
python3 -c 'import json,glob,collections;c=collections.Counter();[c.update([json.loads(l)["verdict"]]) for f in glob.glob("spec/.tp-review/*/ground-round-*.ndjson") for l in open(f) if l.strip()];print(c["NOT-A-CLAIM"],sum(c.values()))'
```

**323 of 1,459 rows, 22.1%**, across 28 rounds. Per round-1 file — 19 of them, 931 rows, pooled
23.3% — the spread is what matters and the mean is nobody's experience: **4.4% (`1.46.0`) to 52.2%
(`1.51.0`), median 25.0%.**

**So the two field figures do not differ enough to matter, and that strengthens the ask rather than
dismissing it.** Report B's 24.6% is this corpus's median to one decimal place; Report A's 37.2% sits
between `1.45.0`'s 37.0% and `1.39.0`'s 38.5%. Neither is an outlier and neither is foreign. The
useful finding is the **12× spread inside one repository**: a share that ranges from 4% to 52%
document by document is not a constant a reader can mentally correct for, which is precisely why the
number has to be reported rather than known.

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

**The decision: `--status` reports the claims-only denominator itself** — the count of **emitted floor
units** whose disposition is `NOT-A-CLAIM` — beside `emitted` and `dispositioned`, under a name that
cannot be confused with `by_verdict`'s row count. The raw ratio stays exactly as it is: §8's *did
anyone look* is a question about the floor, and narrowing its denominator would change what coverage
means. This adds the second reading; it replaces nothing.

## 13. `SKILL.md` did not name `UNVERIFIABLE` — closed by commit, and here is what keeps it closed

**Recorded as closed rather than dropped**, on the rule §5 and §14 follow: an entry deleted once
someone fixes it leaves nothing that would notice the fix being undone.

**Measured, at the state both reports read.** Counting rule — occurrences, not lines, and a plain
substring match, so `FAIL` also counts `FAILED`; only the zero was load-bearing and a substring match
cannot manufacture one:

```
for v in PASS FAIL PARTIAL QUESTION NOT-A-CLAIM UNVERIFIABLE; do printf '%-13s %s\n' "$v" "$(git show <ref>:skills/tp/SKILL.md | grep -o "$v" | wc -l)"; done
```

`PASS` 15, `FAIL` 7, `PARTIAL` 2, `QUESTION` 1, `NOT-A-CLAIM` 1, **`UNVERIFIABLE` 0** — while
`skills/tp/REFERENCE.md` named it 3 times and the corpus in §1 holds 13 recorded `UNVERIFIABLE` rows.
Five of the six verdicts reached the operator-facing document; the sixth reached only the emitted
prompt and the reference. Report B: *"the operator driving the loop reads `SKILL.md`, not the emitted
prompt, so the full verdict vocabulary isn't visible where the decisions get made."*

**The commit *"name all six verdicts where the loop is driven, `UNVERIFIABLE` included"* closed it**
while §9–§14 were being written; the same run of the command above over `HEAD` returns
`UNVERIFIABLE` 1. Re-derive rather than reading that figure — it is a count over prose and will move.

**The decision is therefore not the paragraph but the guard**: §15 row 21 asserts that every verdict
in the set the code exports appears in `SKILL.md`, derived from `GroundVerdicts()` rather than from a
literal list, so a seventh verdict fails the row on the day it is added and a deleted sixth fails it
on the day it is deleted. §7's closed fog entry is why this is worth a guard rather than a fix: the
corpus says `UNVERIFIABLE` is exercised, so its absence was a gap in the map and not in the
territory, and nothing but a derived assertion notices a map going stale again.

## 14. A fear the tree refutes, and the ask that survives the refutation

Report B feared that a sub-agent running bare `tp ground <spec>` to fetch its own prompt would open a
new round and orphan the one in flight. It worked around this with saved envelopes and explicit
warnings in six briefs.

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
round-advancing in the next, with no way to ask which state you are in. `tp ground` has four flags
today (`--record`, `--status`, `--check`, `--units`), and none of them re-prints a prompt.

**Half of this was closed while the section was being written, in the document and not in the
output.** The commit *"re-emitting an unrecorded round is idempotent, and a round file holds the
carry"* added the fact to `skills/tp/SKILL.md`'s ground loop, independently measured and agreeing
with the paragraph above. That closes it for a human reading the skill and changes nothing for a
sub-agent holding only its prompt, which is the reader Report B's six briefs were written for.

**The decision: a read-only way to re-print the current round's prompt**, which emits nothing, writes
nothing, advances no round, and refuses rather than emitting when there is no round in flight. Named
here as a decision and not as a design: whether it is a flag on `ground` or a mode of `--status` is
the implementing task's call, and the property that matters is that it cannot be the same call as the
emission — a mode that is safe or destructive depending on state is the shape the report was right to
be afraid of even though its specific fear was wrong.

This is the second report claim the tree contradicts. The first is §11's: the escape hatch exists in
the mechanism and is closed by the prompt. Both are recorded with the measurement rather than quietly
corrected, on the same rule §5 and the header follow.

## 15. Tests for §9–§14

The same shape as §8 and continuing its numbering, so no row index moves. Each row names an input that
must fail it; where the mutant is a change to the test rather than to the product, the row says so.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 10 | §9 | a round whose rows are fully covered and hold at least one `FAIL` exits **1** from `--status --check` | keep the two shipped conditions, under which `spec/1.56.0.md` round 1 — 4 `FAIL`s, 100% coverage — exits 0 |
| 11 | §9 *bounded* | the same fixture with the `FAIL` replaced by a `PARTIAL` and by a `QUESTION`, each in turn, exits **0** | gate on any non-`PASS` verdict, which passes row 10 and makes 230 recorded `PARTIAL` rows blocking |
| 12 | §9 *no deadlock* | a round-2 payload that decides a `FAIL`-carrying unit afresh records at exit 0 and `--check` then exits 0, on a fixture whose spec text did not change | carry the `FAIL` unconditionally, which is the accepted-finding release's audit defect reproduced here |
| 13 | §9.2 | `--status`'s payload carries `next_action`, asserted by naming the key rather than by matching its text | assert on the sentence, which pins prose a rewording breaks while the key survives — a test-side mutant |
| 14 | §10.1 | `asked` holds exactly the ids the prompt's ask sentence counts: `len(asked) == floor_size - carried`, on a fixture with at least one carried and one asked unit | emit `asked` as every floor unit, which is right in round 1 and wrong from round 2 on |
| 15 | §10.1 *unchanged* | the floor file on disk still carries **zero** `(carried)` markers after §10.1 ships | write the marks into the floor, which is the decision `runGround`'s own comment refuses |
| 16 | §10.2 | on a round where one unit was edited and one written fresh, `floor_delta` reports `added: 2, removed: 1` against the preceding floor | compute the delta against the current spec rather than the preceding round's floor, which reports `added: 0` for a spec nobody edited since the emission |
| 17 | §10.3 | a payload holding a strict subset of the owed units records at exit 0, `--status` reports the shortfall, and `--check` exits 1 | refuse a partial payload, which forbids the split both reports had to perform |
| 18 | §11 | a row naming a carried unit displaces the inherited disposition, and `--record`'s envelope reports the displacement count as 1 | report only `rows` and `carried`, under which the override records silently — the shipped behaviour |
| 19 | §11 *the stale carry* | on the §11 fixture, after §1's repair, `u2` is still marked `(carried)` and is still not in the ask | assert that repairing §1 clears `u2`'s disposition, which no join in tp can do and which this section explicitly does not decide — a test-side mutant, and the row that keeps §11 from being read as more than it is |
| 20 | §12 | the claims-only count equals the number of **emitted floor units** disposed `NOT-A-CLAIM`, on a round carrying a reader-added and an off-floor `NOT-A-CLAIM` row as well | derive it as `emitted - by_verdict["NOT-A-CLAIM"]`, which returns 0 where the answer is 2 on exactly that fixture |
| 21 | §13 | all six of §3's verdicts appear in `SKILL.md`, derived from the verdict set the code exports rather than from a literal list | write the six as a literal in the test, which passes today and says nothing the day a seventh is added |
| 22 | §14 | two bare emissions on an unrecorded round leave the round number and the floor file's bytes identical, asserted on the pair | assert only that the second call exits 0, which is true of a call that opened a new round |
| 23 | §14 | the read-only re-print emits no round: the state directory is byte-identical before and after, and it refuses when no round is in flight | implement it as a bare emit with the write skipped, which is idempotent on an unrecorded round and advances the round on a recorded one |
