# tp v0.36.0 — The emitted round

## 1. Overview

tp emits one prompt per role per round. That prompt carries the checklist, the role rules, the
affected files and the output schema — everything about *what to judge*. It carries nothing about
*how the unit must behave while judging*, and every invocation emits the whole panel to whoever
asked, however many roles that caller actually needs.

The consequence is that every round costs the orchestrator the same manual additions, and a round
that does not get them loses work or produces false findings. Two independent cycles measured the
same shortfalls, on different projects, in different languages:

- tp's own v0.35.0 audit — nine rounds, four roles, this repository.
- A field cycle on a PHP package, nine rounds, five roles, driven by a different operator.

This release makes the emitted prompt self-sufficient and lets a caller ask for one role's prompt
instead of the panel's. It does not change the checklist, the role rules, the output schema, or how
convergence is counted.

**Where the benefit lands, stated up front because it is not uniform.** §2 and §3 both address the
**interactive path** — an orchestrator that spawns sub-agents itself, this repository's documented
fallback and the PHP cycle's method. Under `tp run` §2's constraint is already a fence (§2.1) and
§3's salvage is already refused by the driver (§3.1), so on that path the two clauses are
belt-and-braces rather than the mechanism. §4 is the reverse: its measured cost is paid hardest
under `tp run`. The release is worth shipping because the two paths need different halves of it,
not because every half serves both.

**What it costs.** Two clauses go into every emitted role prompt: **464 bytes** of clause text
(287 + 177) plus the **4 separator bytes** §2.3 fixes, for **468 bytes, 87 words**. Measured against
this spec's own round-4 role prompt of 12,376 bytes that is **3.78%**; against the whole 131,727-byte
payload the same round emits, **0.36%**. Every figure in this spec is stated with the round it was
measured on, because §4 establishes that the panel and its size are round-dependent.

The clauses are emitted on every path, including the one where §2's constraint is already enforced
by a fence — tp emits the prompt before anything knows which runner will consume it, so a
conditional emission is not computable at the point of emission. §4's flag adds no prompt bytes and
removes them from what a unit receives.

## 2. The prompt does not carry its own isolation constraint

**The field instance is a false finding, not an inconvenience.** In round 2 of the PHP cycle an
auditor mutated a file in the repository to test whether a test was tautological — a legitimate
experiment, and the one this project's own evidence rule asks for. A *different* auditor, running
concurrently, read the mutated file and reported the static analyser's failure as a real defect. The
finding was caused entirely by the absence of isolation guidance in the prompt both agents received.
The operator then added "DO NOT modify the working tree" to all five role prompts, by hand, every
round.

tp's own cycle paid the same cost without the failure: the orchestrator injected "Do NOT modify any
repo source file" into all four role prompts for nine consecutive rounds.

### 2.1 Which fence exists, and on which path

tp ships two `PreToolUse` hooks and they are not interchangeable.

| Hook | Registered in | Denies a source write? |
|---|---|---|
| `pre-tool-use-write-deny.sh` (`spec/0.35.0.md` §6.2) | `hooks.json` | **No** — denies four tp state paths, allows every source file |
| `pre-tool-use-role-write-allow.sh` (`spec/0.35.0.md` §6.3) | frontmatter of `agents/tp-reviewer.md`, `agents/tp-auditor.md` | **Yes** — a role unit may write exactly one path |

Both were verified against the shipped hooks rather than read: `internal/cli/audit.go` is allowed by
the §6.2 hook and denied by the §6.3 hook.

The split is drawn on §6.3, and **the clause is for the path §6.3 does not reach**. A runner that
loads the agent definitions gets the fence; an orchestrator that spawns sub-agents itself loads
nothing and gets no fence. Under `tp run` the clause therefore restates a rule the fence already
enforces — which is why §1 prices it rather than claiming it. `spec/0.35.0.md` §6.3 itself calls the
restriction "defence in depth", which is a statement that it is not meant to be the only thing that
is true.

### 2.2 The clause

Emitted verbatim as this exact string, one line, no embedded newline:

```text
Do not edit any file in the working tree. Read anything, and run tp itself freely — its own state writes are expected. Write no other file except the output file this prompt names. If proving a defect would require changing code, report it with its evidence instead of making the edit.
```

**It is one line, not a wrapped paragraph.** Tests assert byte equality against this string, so the
spec gives it unwrapped: a hard-wrapped blockquote would leave an implementer three transcription
decisions (join the lines with a space, or with `\n`; keep the `> ` prefixes or strip them) that
each produce a different artifact and each pass a loose reading of "verbatim".

**Running tp is carved out because the earlier wording forbade the unit's own verification.** A
draft said "write only the output file this prompt names". A role unit is told to run
`tp review <spec>`, and that command writes repository state as a side effect — it takes the round's
snapshot under `spec/.tp-review/<base>/`. Observed twice while this spec was being written: once by
a round-3 reviewer, and once by the orchestrator, whose read-only measurement invocation rewrote
`snapshot-round-3.md`. Under the draft wording a unit that obeyed the clause could not run the
command its brief names, and a unit that ran it violated the clause. The carve-out is narrow on
purpose: tp's own state writes, not a general licence.

**This is also a channel neither hook in §2.1 covers.** Both hooks match tool calls; a state write
performed inside a `tp` subprocess is not one. So on the `tp run` path the clause is not purely
belt-and-braces after all — it is the only statement about a write channel the fence cannot see.

**"the output file" is the antecedent the prompt already supplies.** The emitted prompt's last
section names the file. Its exact text is environment-dependent — interactive emission writes a
round-relative name, and under `tp run` the same line carries an absolute path because `TP_ROUND_DIR`
is set — so the clause names the *role* of the file rather than quoting either form. §2.3 pins the
clause's position so the antecedent always precedes it.

**Naming the output file also makes the clause work for both commands.** `tp audit` produces result
rows, not findings; a clause that said "the findings file" would be wrong in half the prompts it is
emitted into. `output_path` is a key both commands emit, so one string serves both. The framing line
the prompt already carries is itself in findings vocabulary; this release does not reword it, and
the clause is written so that it does not have to.

### 2.3 Placement, to the byte

Both clauses are appended **at the very end of the prompt body**, §2's first and §3's second. The
emitted body ends with exactly:

```text
<the body as v0.35.2 emits it, with any trailing newline removed>
LF LF
<§2.2's clause>
LF LF
<§3.2's clause>
```

and no trailing newline. That is 4 separator bytes, which is the figure §1 prices and test 5
asserts.

**The anchor is the end of the body, not the `## Unit framing` heading.** An earlier draft pinned
the position relative to that literal heading. It does not survive on this spec: tp embeds changed
spec sections verbatim into the prompt, and this document contains the string `## Unit framing`, so
on the round-3 architect prompt the heading occurs **three** times. Any anchor drawn from the
prompt's own prose can be forged by the spec under review — an end-of-body anchor cannot.

**Ordering is pinned because the clauses are not independent.** Both refer to the output file that
the prompt's last section names, so both must follow it; and §3's "the output file" is the same
antecedent §2's clause has just used. A test asserting only presence would pass on a prompt where
the reference dangles, which is why test 3 asserts on offsets.

**Emission predicate.** The clauses are appended to every prompt whose `output_path` is non-empty,
and to no other. That is the positive statement §6 test 4 tests the negative of; §4.4 records why
the predicate is a property of the prompt rather than of the role.

### 2.4 What is deliberately not provisioned

**No perturbation recipe is offered.** An earlier draft told the unit to copy the subject out of the
repository and mutate the copy. Three things defeat it: the copy is a second write, which
`spec/0.35.0.md` §6.3 forbids outright; a source file moved out of its package does not build, so
the experiment the motivating story describes cannot be run on it; and `spec/0.42.0.md` §9 has
already assessed exactly this recipe and found against it, noting that N auditors each building a
scratch copy would dissolve the concurrency argument `spec/0.35.0.md` §3.3 rests on. Giving an
auditor a way to perturb is that file's open question. This release constrains; it does not
provision.

**The clause creates a finding class this release does not close.** Telling a unit to report what it
would have had to edit to prove converts one class of row — a defect whose proof needs a mutation,
a broken precondition or an injected failure — from run evidence into asserted evidence, and tp has
no verdict for "asserted, not run". That gap is `spec/0.42.0.md`'s subject and Non-Goal 2 names it
rather than pretending the clause is free.

**Why the prompt rather than the agent definitions.** The definitions are the cheaper channel — they
cost nothing per round — and they are the right home for a standing rule under `tp run`. They are
also loaded by nothing on the interactive path, which is the only path where this constraint is not
already enforced. A future release that makes the interactive path load a definition should move the
clause there and delete it here.

## 3. A unit that buffers its findings loses all of them

Both cycles lost whole rounds to the same shape. An agent accumulates its rows in context and writes
the NDJSON once, at the end; the process dies before that write; nothing survives.

The field cycle lost **six** subagents this way over one session. The single run that had been
instructed to append each row as it was decided preserved **72 rows** despite dying near the end.
tp's own cycle lost four, and the one told to write early kept 9 of its 10 rows — so the relaunch
completed the round instead of redoing it, and the recovered rows were the expensive ones.

The asymmetry is the argument: buffering saves nothing and risks everything, because a findings file
is append-only NDJSON whose partial state is already valid input to `--merge`. Verified on the
shipped binary: a file whose trailing line is truncated mid-object merges with
`warning: skipping malformed line (invalid JSON)`, keeps every complete row, and exits **0**.

### 3.1 Where the salvage is real, and where it is not

The measured recoveries above are **interactive-path** results, and they do not carry to `tp run`
unchanged. `spec/0.35.0.md` §3.3 defines a role unit's durable write as
`$TP_ROUND_DIR/role-$TP_UNIT_ID.ndjson` existing with **every line parsing**, and §3.3.1 has the
unit write `role-<id>.ndjson.part` while the driver renames it to the final name **only on exit 0**.
A unit that dies therefore leaves rows that are never promoted, and a truncated trailing line would
fail the predicate even if they were.

Under `tp run` the clause's benefit is consequently smaller and different: the `.part` file survives
on disk with its complete rows intact, so a human — or a later release that decides to promote a
partial file — can recover the round's expensive work instead of finding an empty context. It does
not salvage the round automatically, and this spec does not change the driver to make it do so.

**Whether a partial file should ever complete a unit has no owner today.** Earlier drafts routed it
to `spec/0.41.0.md` §2; checked against that file, §2 is "Sharding a role's checklist" and §3 is
"Incremental rounds", and neither states the completeness predicate. Sharding will *raise* the
question — N shards jointly complete, no single one complete — but it does not currently answer it.
It belongs in `spec/0.38.0.md`, the backlog release, whose job is exactly the defects earlier cycles
deferred with reasons. Naming an owner that does not own it is worse than naming none.

### 3.2 The clause

Emitted verbatim as this exact string, on the terms §2.2 and §2.3 fix:

```text
Write each row to the output file as you decide it, not once at the end. A run that dies with its rows unwritten loses the whole round; a partially written file is still usable.
```

### 3.3 What this does not change

A findings file that exists and parses does not mean the unit finished. It never did: completion is
`exit 0` **plus** the driver's rename (`promoteRoleFindings`, `spec/0.35.0.md` §3.3.1), and
`spec/0.35.0.md` §6.2's Stop hook applies an "exists and parses" predicate that is a liveness check
— it catches a unit stopping with nothing — not a completeness check. Incremental append makes
partial files the normal intermediate state, so it makes that misreading easier; it does not create
it. A unit that stops cleanly having judged three of forty items merges as complete today, before
this release and after it. That hole is the unowned one §3.1 routes to `spec/0.38.0.md`.

## 4. Every caller gets the whole panel

`tp review <spec>` and `tp audit <spec>` emit one `prompts[]` entry per emitted role, always, to
whoever ran the command. Under `tp run` each role unit's brief command is exactly that invocation
(`spec/0.35.0.md` §3.3.1), so **every unit receives every role's prompt and reads one**.

### 4.1 The measurement, and why it has to name its round

The panel is **round-dependent**, which an earlier draft of this section got wrong by quoting one
round's figures as general. From round 2 onward, when the spec has changed or findings were resolved
fixed, `buildReviewPrompts` auto-appends the built-in `regression` role; at round 1 it is skipped
with reason `no-baseline`. So the same command emits a different panel on different rounds.

Measured on this spec, current binary, `TP_*` cleared from the environment:

| Round | Prompts | Payload | Largest entry |
|---|---|---|---|
| 3 (spec unchanged at emission, nothing resolved) | 4 | 41,015 B | tester 10,183 – ax-economist 10,334 |
| 4 (spec rewritten, 36 findings resolved fixed) | **5** | **131,727 B** | **`regression` 82,160 B** |

At round 4 the four corpus roles are 12,321 – 12,472 bytes each, 49,567 together. `regression` alone
is **62% of the payload**.

**The driver spawns four units, not five, and it is the corpus that decides.**
`engine.BuildNextUnits` derives the panel through `roleUnits`, which calls `ResolveActiveCorpus` —
corpus roles only. `regression` is a reserved built-in that `engine.ParseRoleBytes` refuses as a
corpus file (`role id %q is reserved for the built-in regression role`). So no `regression` unit
exists to be spawned.

Per round 4, under `tp run`:

| | Bytes |
|---|---|
| Emitted — 4 units × 131,727 | 526,908 |
| Read — each unit's own prompt | 49,567 |
| **Never read** | **477,341 (90.6%)** |

**And the regression prompt is emitted to everyone and executed by nobody.** Verified on round 3's
own merged findings: the role histogram is `tester 13, implementer 11, architect 9, ax-economist 8`
— **zero regression rows**. Its 82,160 bytes reach four units, are assigned to none, and produce no
output file. That is a defect in the driver's panel derivation, not in emission; §4.4 says what this
release does and does not do about it.

A human orchestrator pays the same shape differently: both cycles piped the JSON into a script and
split it by role, every round. That script re-derives `prompts[].role`, which tp already computed,
and it is the step most likely to be skipped or mis-written under time pressure.

### 4.2 Mechanism — `--role <name>`, and the brief that passes it

- `tp review <spec> --role <name>` and `tp audit <spec> --role <name>` emit the payload with
  `prompts[]` reduced to the single entry whose `role` equals `<name>`.
- **The match set is the set this same invocation would emit** — not the corpus, and not the active
  role set for the phase. The two differ: `regression` is emitted and is in no corpus, and
  `--perspective testing` emits `test-planner`, which is in neither. Defining the set as "what this
  invocation emits" is the only definition that holds in all the modes the table below legalises,
  and it makes `--role regression` — the largest entry in the payload — selectable.
- A name outside that set exits **2**, with a hint listing the names the invocation would have
  emitted.
- A name in the corpus but in `skipped_roles` for this spec (the per-spec deactivation of v0.32.0)
  exits **2** with a hint that says it is *skipped*, not unknown; the same for a name active in the
  other phase's corpus, whose hint says which phase it belongs to. Three distinct operator mistakes,
  three distinct hints.
- `--role` takes exactly one value. It is a string flag, not repeatable; `--role ""` is refused as
  an unknown role rather than treated as absent, so "flag given empty" and "flag absent" are never
  the same command.
- `--role` is legal wherever prompts are emitted and refused with exit **2** where none are. The
  refusal is evaluated **before** the mode's own argument validation, so the hint an operator sees
  names the flag conflict rather than a missing `--findings`. The two commands do not have the same
  flag sets, so the modes are enumerated separately rather than as "and audit's equivalents":

| Command | `--role` legal in | `--role` refused in |
|---|---|---|
| `tp review` | default, `--perspective`, `--diff-from` | `--verify`, `--merge`, `--record`, `--status`, `--report`, `--resolve`, `--resolve-all` |
| `tp audit` | default only | `--merge`, `--record`, `--status`, `--resolve`, `--resolve-all` |

Verified against the shipped binary: `tp audit` registers no `--perspective`, `--diff-from`,
`--verify` or `--report`. `--verify` moves to the refused column because it already emits exactly
one prompt (role `verifier`); `--role` there could only be a no-op or a refusal, and a flag that
cannot change the output should not be accepted.

**The driver must pass it, or the saving is unreachable.** This is the half an earlier draft
omitted, and without it `--role` repeats `--out-dir`'s mistake in a new shape: `spec/0.35.0.md`
§3.3.1 fixes the role kinds' brief command as `tp review <spec>` / `tp audit <spec>`, and nothing on
any path appends a flag to it. So this release also changes `UnitKind.BriefCommand` for
`review-role` and `audit-role` to emit `tp review <spec> --role <id>` and
`tp audit <spec> --role <id>`, where `<id>` is the unit's own id — the same value that becomes
`TP_UNIT_ID` and names `role-<id>.ndjson`. The mapping is exact and already exists; only the brief
string changes.

### 4.3 Who it is for

P1 asks whether a change serves the agent. Under `tp run` at round 4 a role unit receives 131,727
bytes to read 12,376 of them; with the brief of §4.2 it receives only its own. Across the round that
is 526,908 emitted bytes down to 49,567 — a **90.6%** reduction in bytes handed to units.

P2 asks whether N is as easy as 1. It is: the driver already spawns one process per role and now
gives each its own brief, so the N case needs no new surface. For a human orchestrator the N case
becomes one invocation per role instead of one invocation plus a split script — more invocations,
but no script to write, skip or mis-write.

**`--perspective` is the adjacent knob and is not the same axis.** `--perspective regression` also
emits exactly one prompt, so the two look alike; they are not. `--perspective` selects a *kind* of
review with its own prompt builder and its own preconditions, and its four values are fixed in
`review.go`. `--role` selects one member of whatever panel the current invocation produces. A
release that added `--perspective <role-name>` would have to make every corpus role a perspective,
which is the coupling the corpus exists to avoid.

### 4.4 What this release does not fix about the panel

The regression prompt reaching four units and no unit is a **driver** defect: the emitter is right
and `BuildNextUnits` is short one unit kind. Fixing it means either a `regression` role unit or a
rule that folds regression into one of the corpus units, and both are new unit-lifecycle surface —
more than this release's size and squarely `spec/0.38.0.md`'s kind of item.

What this release does change is that the loss stops being silent. With §4.2's brief every unit
receives exactly the roles it was assigned, so a role nobody is assigned is visibly emitted to
nobody rather than buried inside a payload every unit happens to receive. §6 test 13 pins that
visibility.

### 4.5 Why not `--out-dir`

An earlier draft wrote one `<role>.prompt.md` per prompt into a directory and added a `prompt_path`
key. It was cut for reasons that were measured, not argued:

1. Its only P1 claim — that under `tp run` "the spawner reads `prompt_path` and passes a file rather
   than carrying a 75 KB body through its own context" — is false. Verified in
   `internal/engine/childenv.go`: the child environment carries `TP_RUN_ID`, `TP_ROUND_DIR`,
   `TP_UNIT_ID` and five others, and **no prompt at all** — neither a body nor a path. The unit
   calls `tp review` itself. The driver never pays the cost the flag claimed to remove.
2. It multiplies the waste it was meant to fix. Each of N units still emits the whole panel, so a
   round with five prompts and four units writes twenty files, sixteen of them redundant.
3. `prompt_path` carries no information its consumer lacks: the caller passed the directory and
   `prompts[].role` is on the same object.
4. `<role>.prompt.md` has no phase discriminator, so a review role and an audit role of the same
   name collide in one directory.

`--role` answers the same measurement without any of the four — but only with §4.2's brief change,
which is the lesson item 1 should have taught the first time.

## 5. Non-Goals

1. **Sharding a role's checklist.** `spec/0.41.0.md`. This release does not claim to be a
   precondition for it: sharding is more entries in a `prompts[]` array that already carries N of
   them, and that file names `spec/0.40.0.md`'s measurement as its real precondition.
2. **Provisioning an auditor with a way to perturb, or grading the evidence it returns.** §2's
   clause converts the one class §2.4 names — a defect whose proof needs a code change — into an
   asserted finding, and `spec/0.42.0.md` owns both the recipe (§9) and the verdict for un-run
   evidence. §2.4 states the gap rather than closing it.
3. **Convergence arithmetic.** `spec/0.37.0.md` owns the rule that turns a row set into a verdict.
4. **Enforcing the isolation constraint on the interactive path.** It gets the instruction, not a
   fence; a fence there needs a hook that path does not install.
5. **Admitting a partial file as a complete unit.** §3.1 and §3.3 explain why the predicate is
   unchanged; the question is unowned today and §3.1 routes it to `spec/0.38.0.md`.
6. **Giving the `regression` role a unit of its own.** §4.1 measures that its prompt reaches every
   unit and is executed by none; §4.4 explains why the repair is new unit-lifecycle surface and
   belongs to `spec/0.38.0.md`. This release makes the loss visible, not fixed.
7. **Changing what a round asks.** The checklist items, role rules and output schema are untouched.
   The prompt does gain two behavioural clauses — that is §2 and §3, and §1 prices them; the claim
   here is narrower than "nothing changes".

## 6. Tests

### 6.1 The baseline both regression tests need

Tests 5 and 12 compare against "the v0.35.2 payload".
It is obtained by running the v0.35.2 emitter in-process against the same fixture — the release tag
is in this repository, so the test builds the baseline rather than storing it. Nothing is committed:
the payload embeds the round number, the loop budget, the consecutive-clean count and the
`output_path`, so a stored blob would encode this repository's review state on the day it was
written and fail on the next recorded round. Each test fixes those inputs explicitly.

### 6.2 Clause emission

1. The emitted role prompt for `tp review` contains §2.2's clause as an exact byte-equal line, and
   the same for `tp audit`.
2. The same, for §3.2's clause.
3. The prompt body **ends with** exactly `LF LF` + §2.2's clause + `LF LF` + §3.2's clause, and no
   trailing newline — asserted as a suffix comparison on the whole body, which pins order, position
   and every separator byte in one assertion and cannot be satisfied by a prompt that merely
   contains both strings.
4. Neither clause appears in any prompt whose `output_path` is empty. Asserted over every mode that
   emits such a prompt: `--perspective testing` (role `test-planner`), `--perspective
   documentation`, `--perspective code-audit`, and `--verify` (role `verifier`). The discriminator
   is the empty `output_path` and not the role name, because the same role can carry a non-empty
   `output_path` in another mode.
5. Byte cost: for the same spec and corpus, a v0.36.0 role prompt is exactly 468 bytes longer than
   the v0.35.2 one — the two clause strings plus 4 separator bytes. Asserted as an exact integer, so
   rewording either clause fails it.

### 6.3 `--role`

6. `tp review <spec> --role <name>` emits exactly one `prompts[]` entry, whose `role` is `<name>`
   and whose `prompt` is byte-identical to that role's entry in the unrestricted invocation of the
   same command. `tp audit` likewise.
7. `--role regression` succeeds at a round where regression is emitted, proving the match set is the
   emitted set and not the corpus. At round 1, where regression is skipped `no-baseline`, the same
   command exits 2.
8. An unknown role exits 2 with a hint listing the names the invocation would have emitted.
9. Three refusals produce three different hints, each pinned by a required substring: an unknown
   name, a name in `skipped_roles` (hint contains `skipped`), and a name active only in the other
   phase's corpus (hint names that phase). Asserting only that the hints differ would pass on any
   byte difference, so each is matched on a token.
10. `--role` with each refusing mode in §4.2's table exits 2 with a hint naming both flags — run per
    command against that command's own list. Each case is run with the refusing mode's *own*
    arguments absent, which is what makes the test discriminating: it fails if the flag conflict is
    checked after argument validation, because the operator would then see the wrong error.
11. `--role` with `--perspective` and with `--diff-from` exits 0 and emits one prompt. `tp review`
    only; `tp audit` has none of those flags. `--verify` is not in this list — it is in test 10's,
    per §4.2.
12. Without `--role`, the payload for a fixed spec and corpus differs from the v0.35.2 baseline only
    by the two clauses inside each `prompts[].prompt`.

### 6.4 The driver's brief

13. `tp resume`'s `next_units[].brief_command` for a `review-role` unit is
    `tp review <spec> --role <id>` with `<id>` equal to the unit's own id, and the `audit-role`
    equivalent. Asserted against `engine.BuildNextUnits` directly, and end-to-end through
    `tp run --dry-run`.
14. Every unit the driver spawns for a round has a brief naming a role, and every role the round
    emits either has a unit or is absent from the units list. The test asserts the two sets and
    their difference, so `regression`'s absence from the unit set is recorded as a fact rather than
    discovered later — this is §4.4's visibility, pinned.

## 7. Release gate

Every mechanism in this release hides, regroups or narrows what a reviewer sees — `--role` most of
all — which is the shape that cost v0.34.0 §7.1 eight rounds of suppressed findings. So the gate is
a replay, and it is defined here to the point of being runnable rather than left as an intention.

**What is replayed.** For each recorded round R of each spec under `spec/.tp-review/` that has a
`snapshot-round-R.md` and a `review-round-R.ndjson`: re-emit the round's prompts from that snapshot
with the v0.36.0 binary, once unrestricted and once per role with `--role`, and compare against the
same emission from the v0.35.2 binary.

**Pass criterion, in three parts.**

1. **No prompt content is lost.** For every role R emits, the v0.36.0 `--role <name>` prompt equals
   the v0.35.2 prompt for that role, byte for byte, except for the 468 bytes §2.3 appends. A
   difference anywhere else fails the gate.
2. **No role is lost.** The set of roles emitted by v0.36.0 equals the set emitted by v0.35.2, for
   every replayed round. `--role` narrows what one *invocation* returns and must never narrow what
   the panel *contains*.
3. **No finding class becomes unreachable.** For each recorded finding in `review-round-R.ndjson`,
   the role that produced it still receives a prompt containing the checklist item its `class` maps
   to. This is the part that would actually catch a suppression, and it is checked by class rather
   than by re-running the round, because re-running is not reproducible.

**Failing the gate blocks the release**, and the replay's output — the per-round, per-role byte
diffs — is the artifact that records it passed. A round whose snapshot or findings file is missing
is reported as un-replayable and named, never silently skipped.
