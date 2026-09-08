# what-the-record-does-not-say — measurements

Supplemental material for `what-the-record-does-not-say.md`; the spec stands without it.

Every block below was moved out of `ground-command-friction-measurements.md` on 2026-09-08, where it
sat under the heading of the section it came from, and before that out of the spec body verbatim. The
section numbers in the headings and inside the moved text are the **original** file's: its §9 and
§9.3 are this spec's §2 and §2.1, its §11 is §3, its §12 is §4, its §13 is §5. Its §2 is
`scratch-name-is-unique-per-spec.md` §2, its §3 is `record-diagnoses-every-bad-row.md` §2, its §4 and
§5 are `the-floor-names-what-it-cut.md` §2 and §3, its §10 is
`a-round-can-be-driven-from-the-envelope.md` §2, and its §14 is `emitting-does-not-lose-a-round.md`
§2. Figures are quoted at the ref each block names; re-derive rather than reading one off the page.
Filenames inside `git show <ref>:<path>` and `git log -- <path>` commands are the paths at that ref
and are left as written; every other citation was rewritten to the file's current name.

**§1 and §1.1 below are the split's shared provenance.** Every one of the seven specs
`ground-command-friction.md` was split into cites them from here rather than restating them.

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

(On 2026-09-08 the file was split into seven specs, so the row indexes this paragraph protects did
move — each part's `## Tests` table renumbers from 1, and the stub's table is the map from the old
index to the new one.)

## §7 Open questions inherited when `spec/candidates.md` was split

**One entry arrived here already answered, and is recorded as answered rather than carried.**
`candidates.md` listed *"What `UNVERIFIABLE` costs"* as fog, on the ground that there were **zero
instances across 44 grounded claims**, so the verdict was designed and untested. Re-derived over the
corpus in §1: **13 `UNVERIFIABLE` rows and 11 `QUESTION` rows in 1,459**. Both verdicts are exercised,
and neither is rare enough to call untested. The fog entry is closed by measurement, not by a decision.

(The other item §7 carried, claim enumeration, is an untaken decision and moved to
`spec/undecided.md`.)

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

(The exemption is now an entry in `spec/undecided.md`. The 2026-09-08 decision pass decided it —
no exemption, a `cut` delta instead — and that bullet went to
`the-floor-names-what-it-cut-measurements.md`, under *Decided at the 2026-09-08 decision pass*.)

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

## §15 Tests — the mutant-column prose trimmed from rows 21 and 24 (now rows 5 and 1)

Row 21: append a seventh verdict to `groundVerdictOrder` and leave `SKILL.md` alone — the derived guard fails naming it, the literal-list version passes. The row's earlier mutant, *"write the six as a literal in the test"*, was built and run against both and **passes under both**, so it was replaced by one that discriminates.

Row 24: land `spec/backlog/what-the-carry-can-promise.md` §2.2's multiplicity fence: under it the unit is re-asked instead, the `FAIL` survives, and this row goes red — which is its purpose. It is a characterisation row, and the only one here: it exists so the limitation §9.3 states is retired by a failing test rather than left standing unnoticed.
