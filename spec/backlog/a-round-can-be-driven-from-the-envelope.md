# tp — A round can be driven from the envelope

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `a-round-can-be-driven-from-the-envelope-measurements.md` beside it, and this
file stands without them. It took the two zeros, the `cut` key and the cut delta from
`the-floor-names-what-it-cut` on 2026-09-11, so the `tp ground` envelope keys it adds are decided
together.

Class: **tool** — it adds keys to an envelope and a report, and changes nothing a round is graded
against or converges on.

## 1. The decision

**Context.** A process driving a ground round — a script that shards a large floor across readers,
or an operator telling a repair's re-asks from a spec's first asks — reads the emission envelope,
and the envelope gives it two counts: `floor_size` and `carried`. It does not say which units the
round owes (§2.1), what changed since the last round (§2.2), or why a floor is empty — a document
§2.1 of the ground spec produced no unit from and a document whose every unit the arms cut emit the
same envelope and the same ask (§2.4). `--status` does not say whether the arms cut a sentence a
repair just wrote (§2.5). And the one instruction a driver reads as forbidding a shard is a skill
sentence that means something else (§2.3).

**Decision.** The emission envelope carries `asked`, `floor_delta` and `cut`; the zero-floor ask
says which zero it is; `--status` reports a cut delta; and `skills/tp/SKILL.md`'s ground step says
what its singular means.

**Consequences.** Nothing new is written to the floor file, and nothing a round records or gates on
changes. The envelope grows by one id per owed unit. §2.5 reads cut units' hashes under the
canonicalisation `the-floor-names-what-it-cut` §2 has tp compute, so it ships after that spec.

## 2. What the emission hands a driver

### 2.1 The ask set, as a list

`carried` is a count, the `(carried)` marking lives only in the prose of `prompt`, and the floor file
on disk is deliberately unmarked, so a driver that wants the owed set rebuilds it by joining the
floor file against the previous round's rows on `(text_sha, ordinal)` — the join tp already performs
to compute `carried` (sidecar, *§10.1*).

**The decision: the envelope carries `asked`, the list of `unit_id`s this round owes.** A list
beside the count, not instead of it: `carried` and `floor_size` stay as they are, and `asked` holds
`floor_size − carried` ids, the set the prompt's ask sentence already names in English. It goes in
the envelope and never in the floor file (Non-Goal 1).

### 2.2 What changed since the last round

A unit is in the ask set because its text changed or because it never existed, and nothing tp emits
separates the two (sidecar, *§10.2*). New-versus-edited is not recoverable from hashes at all —
identity is the hash, so an edited sentence and a deleted-plus-added pair are the same event to every
artifact tp writes. What is exact and cheap is comparing the previous round's floor, as a set of
hashes, against this one's.

**The decision: the envelope reports `floor_delta` against the preceding round's floor —
`{added, removed, unchanged}`, counts over `text_sha`.** Its limit is stated because it is the
reason nothing more is promised: a rewritten sentence is one `added` and one `removed`,
indistinguishable from an unrelated insertion and an unrelated deletion. The counts bound the churn;
they do not identify it.

### 2.3 One panel, any number of readers — a doc task

Readers of a large floor have sharded it across several sub-agents and read `skills/tp/SKILL.md`'s
ground step, *"Spawn **one** sub-agent on that prompt"*, as a capacity limit. The emitted prompt says
nothing about how many readers it has; the singular is the skill's, and the step above it gives the
reason — grounding asks one question of every unit, so there is no panel and no role. Slicing
already records and gates correctly (row 4).

**The decision is a one-sentence edit to that step and no code:** it says one panel, any number of
readers, because the floor is a partition and a slice of it is a well-formed ask. The emitted prompt
stays silent on the question — it is addressed to whoever reads the units, and a sentence about
process there would be tp telling an orchestrator how to spend its context. `asked` (§2.1) is what
makes a slice expressible without a join.

### 2.4 The two zeros

`tp ground` emits a floor of zero for two different reasons and says the same thing about both: a
document of headings and fenced blocks produces no unit at all, and a document whose every sentence
the arms dropped produces units and keeps none. `--status` and `--status --check` already separate
the two; the ask and the envelope — the two surfaces an operator meets first, before any round
exists — do not (sidecar, *The two zeros*).

1. **The ask says which zero it is, and states no count.** The zero-floor ask becomes two literals
   that keep the opening clause `This round owes no dispositions:` — one saying §2.1 produced no
   unit, one saying it produced units and the arms cut every one. Neither states the cut count: the
   index block above it already does, and a count would force a singular form into a sentence that
   must not gain a third.
2. **The envelope carries `cut`**, an integer present at zero rather than omitted, read from the
   emitted index by the rule `floor_size` is read by. On a partly-cut floor it is the index's cut
   count beside a non-zero `floor_size`. `--status --check`'s own `cut` keeps its own source.
3. **The ask's tests become a generated `(floorSize, carried)` table**, required to hold a pair with
   `2 ≤ carried < floorSize`, each pair asserting its sentence from `This round owes` through the end
   of the clause rather than a prefix of it.

### 2.5 The cut delta on `--status`

Decided on 2026-09-08 in place of exempting a rewritten sentence from the cut for one round
(sidecar, *Decided at the 2026-09-08 decision pass*): **`tp ground --status` reports, per round,
how many of that round's cut units have text that appears in no unit of the preceding round.** A
repair whose new sentence the arms cut is then visible once, without the cut being relaxed and
without the sentence being graded twice. The exemption was the expensive form of the same want and
would have lived in the arms; the delta touches only the report.

## 3. Non-Goals

1. **Nothing is added to the floor file, and the arms are unchanged.** The floor file is what a
   round is graded against; a copy of the carry or the cut beside it would be a second statement of
   the same fact with nothing comparing the two. §2.3 splits the reading of a floor, never the floor.
2. **No new workflow field, no gate, no convergence effect, no exit code.** Nothing here adds a knob
   to `.tp/config.json` or a task file's `workflow` block, and nothing changes `clean`, a streak,
   coverage or `--check` — the `--check` gate is `next-action-and-check-tell-the-truth`'s.
3. **No `--merge` for ground, and no orchestration.** NDJSON concatenates; nothing here spawns,
   schedules or counts readers.
4. **§2.4 takes three tasks and no more.** No third sentence in the ask, no refusal, and the hint
   `--record` prints for an empty payload is not repaired here, though its antecedent is false on the
   no-unit document (sidecar); `refusals-that-name-nothing` takes it.

## 4. Tests

Every row derives from a numbered decision and names the mutant that must fail it. Rows 4, 5 and 8
quote their `HEAD` behaviour, which was measured; every other row's subject does not exist at
`HEAD`, so its two counts belong to the implementing task's acceptance.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2.1 | `asked` holds exactly the ids the ask sentence counts — `len(asked) == floor_size − carried` — on a fixture with at least one carried and one owed unit | emit every floor unit as `asked`, which is right in round 1 and wrong from round 2 on |
| 2 | §2.1 *the floor file* | the floor file on disk carries zero `(carried)` markers after §2.1 ships | write the marks into the floor file, which Non-Goal 1 refuses |
| 3 | §2.2 | on a round where one unit was edited and one written fresh, `floor_delta` reports `added: 2, removed: 1` against the preceding floor | compare against the current spec instead of the preceding round's floor, which reports no change for a spec nobody edited since the emission |
| 4 | §2.3 | on a three-unit floor, a payload holding one row records at exit 0, `--status` reports one of three dispositioned, and `--status --check` exits 1 — as at `HEAD` | refuse a partial payload, which forbids the split a large floor needs |
| 5 | §2.4 *the ask* | on the two zero-floor documents, the asks differ past `This round owes no dispositions:`, and the no-unit ask claims nothing was cut. At `HEAD` both asks are byte-identical and both say every unit was cut | `HEAD`'s single literal |
| 6 | §2.4 *no count* | on an empty floor with exactly one cut unit, the ask is byte-equal to the cut literal | print the count, which renders `1 units` on exactly this fixture; a fixture with four cut units passes either way |
| 7 | §2.4 *the other arms* | every non-empty ask renders identically before and after, on the same fixtures | let `cut` reach a non-empty branch, changing what an ordinary settling round reads |
| 8 | §2.4 *the envelope* | across the two zero-floor documents every envelope key is equal except `cut`, which is 0 and N, N being the cut count read off the emitted index; on a partly-cut floor `cut` is the index's count beside a non-zero `floor_size`. At `HEAD` the two envelopes are byte-identical and carry no `cut` | omit the key (`HEAD`); or set it only when `floor_size` is 0 |
| 9 | §2.4 *the generator* | the ask's subtests are generated from a pair list holding a pair with `2 ≤ carried < floorSize`, each asserting through the end of its clause | a loop over only the hand-written pairs, under which changing `already carry` to `already carries` stays green |
| 10 | §2.4 *action unchanged* | on both zero-floor documents `--record` of an empty payload exits 1 with the same message, and `--status --check` still exits 0 on the no-unit one and 1 on the all-cut one | branch `--record` on `cut`; or take `--check`'s `cut` from the envelope, giving one condition two sources |
| 11 | §2.5 | after a repair that rewrites one floor sentence into a form the arms cut, the next round's cut delta is 1; after a round with no edits it is 0 | report the round's total cut count, which is non-zero on the unedited round |
