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

This release makes the emitted prompt self-sufficient, lets a caller ask for one role's prompt
instead of the panel's, and changes the brief `tp run` hands a role unit so that it asks for its own
(§4.2.3) — the release's only change outside the emitter. It does not change the checklist, the role rules, the output schema, or how
convergence is counted.

**Where the benefit lands, stated up front because it is not uniform.** §2 and §3 both address the
**interactive path** — an orchestrator that spawns sub-agents itself, this repository's documented
fallback and the PHP cycle's method. Under `tp run` §2's constraint is already a fence (§2.1) and
§3's salvage is already refused by the driver (§3.1), so on that path the two clauses are
belt-and-braces rather than the mechanism. §4 is the reverse: its measured cost is paid hardest
under `tp run`. The release is worth shipping because the two paths need different halves of it,
not because every half serves both.

### 1.1 What it costs

**467 bytes per role prompt.** The arithmetic is written out because §6.2 property 3
fixes the suffix it derives from, and an earlier draft stated 468 — one more than its own construction
in §2.3 produces:

| | Bytes |
|---|---|
| trailing newline removed from the existing body | **−1** |
| `LF LF` before §2.2's clause | +2 |
| §2.2's clause | +287 |
| `LF LF` before §3.2's clause | +2 |
| §3.2's clause | +177 |
| appended **suffix** | **468** |
| **net delta** | **+467** |

**The two numbers are different and both are used.** The suffix tp appends is 468 bytes; the net
change to the body is 467, because the strip removes one. An earlier draft called the suffix
"467-byte", which is an off-by-one in the opposite direction from the one this table exists to fix.
Two later drafts tried to legislate against that drift — first an index naming which section uses
which figure, then a rule that every later use must write "suffix" or "net delta" beside the number.
The index was wrong at half its entries the round it was written; the rule was violated in the same
round. Both were documents about the document, and both drifted faster than the thing they guarded.
There is no third attempt: this table is the derivation. §6.2 property 3 fixes the **suffix** at 468
bytes, which is the figure an assertion can reach; 467 is this table's arithmetic on it and no
property asserts it separately.

The −1 is not optional and not cosmetic: measured on this spec's own emission, every role prompt
body ends with `\n`, so a construction that appends without stripping produces a blank line and a
different total. Against a round-4 role prompt of 12,376 bytes the net is **3.77%**; against the
whole 131,727-byte payload the same round emits, **0.35%**. The figures above were measured on round 4, and §4 shows why the round matters: the panel and its
size are round-dependent. That is a stamp on these numbers, not a rule for the document — four
authoring rules about this document have now been minted and retired in as many rounds, each one
falsified in the round that wrote it, and a fifth would be the same mistake.

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
<the body as assembled before this release's append, trailing newline removed>
LF LF
<§2.2's clause>
LF LF
<§3.2's clause>
```

and no trailing newline. That is 4 separator bytes, which is the figure §1.1 prices and §6.2
property 3 fixes.

**The anchor is the end of the body, not the `## Unit framing` heading.** An earlier draft pinned
the position relative to that literal heading. It does not survive on this spec: tp embeds changed
spec sections verbatim into the prompt, and this document contains the string `## Unit framing`, so
on the round-3 architect prompt the heading occurs **three** times. Any anchor drawn from the
prompt's own prose can be forged by the spec under review — an end-of-body anchor cannot.

**Ordering is pinned because the clauses are not independent.** Both refer to the output file that
the prompt's last section names, so both must follow it; and §3's "the output file" is the same
antecedent §2's clause has just used. A test asserting only presence would pass on a prompt where
the reference dangles, which is why §6.2 property 1 fixes the whole suffix rather than the
presence of the two strings.

**Emission predicate.** The clauses are appended to every prompt whose `output_path` is non-empty,
and to no other. That is the positive statement §6.2 property 2 states the negative of. The predicate is a property
of the prompt rather than of the role because the same role carries a non-empty `output_path` in one
mode and an empty one in another — §4.2.1's skip table is about roles, and this is not.

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

**Whether a partial file should ever complete a unit has no owner, and this spec does not invent
one.** Two drafts named an owner without checking, which is worth recording because the second did
it in the same paragraph that diagnosed the first. `spec/0.41.0.md` §2 is "Sharding a role's
checklist" and §3 is "Incremental rounds" — neither states a completeness predicate. `spec/0.38.0.md`
was named next; searched, it contains zero occurrences of *partial*, *complete*, *.part* or
*promote*, and its six sections are about output shape, batch input, sub-mode precedence, the
warning channel, resolve safety and evidence counting.

So the honest statement is a question with no owner and an action, not a filename: **a section
defining when a partial role file completes a unit has to be written, and no spec contains it yet**.
Sharding (`spec/0.41.0.md`) is the release that will be unable to proceed without it, because its N
shards are jointly complete and no single one is. Naming a file that does not contain the question is worse
than naming none — it converts an open problem into a closed-looking reference.

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
this release and after it. That hole is the unowned one §3.1 describes.

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

| Round | Prompts | Prompt bytes | Largest entry |
|---|---|---|---|
| 3 (round-2 findings not yet resolved, spec not yet edited) | 4 | 41,015 B | 10,183 – 10,334 |
| 4 (spec rewritten, 36 findings resolved fixed) | **5** | **131,727 B** | **`regression` 82,160 B** |

The round-3 row is the *absence* of the trigger, not a counterexample to it: at that emission the
spec had not been edited since round 2's snapshot and no finding carried a `fixed` disposition, so
neither half of the condition above held and `regression` was not appended.

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

### 4.2 Mechanism — `--role <name>`

- `tp review <spec> --role <name>` and `tp audit <spec> --role <name>` emit the payload with
  `prompts[]` reduced to the single entry whose `role` equals `<name>`.
- **The match set is what this invocation would emit with `--role` removed** — not the corpus, and
  not the active role set for the phase. The qualifier matters: "what this invocation emits" is
  circular, because the invocation being described carries the flag whose effect is under
  definition. Every other argument — the spec, the round, `--perspective`, `--diff-from` — is held
  fixed, and only `--role` is dropped. The two differ: `regression` is emitted and is in no corpus, and
  `--perspective testing` emits `test-planner`, which is in neither. Defining the set as "what this
  invocation emits" is the only definition that holds in all the modes the table below legalises,
  and it makes `--role regression` — the largest entry in the payload — selectable.
#### 4.2.1 A recognised name the round does not emit

- **A name the round does not emit but tp recognises is not an error.** It exits **0** with
  `prompts: []` and the role's own entry present in `skipped_roles`, carrying the reason tp already
  computes. Only a name tp recognises nowhere — absent from the corpus for either phase, from
  `skipped_roles`, and from the built-ins — exits **2**, with a hint listing what the invocation
  would have emitted.

  This split is forced by the driver, not chosen for taste. §4.2.3's brief change makes `--role <id>`
  a *unit's own first command*, and the unit set and the emitted set are computed by different
  filters, so they legitimately diverge. `engine.roleUnits` applies domain filtering and the spec's
  `enabled: false` drop; emission applies those **and** three more skips that
  `internal/engine/skipped.go` enumerates:

  | Skip reason | Drops a unit? | Drops a prompt? |
  |---|---|---|
  | `domain-mismatch` | yes | yes |
  | `disabled-by-spec` | yes | yes |
  | `no-checklist-items` | **no** | yes |
  | `no-spec-change` (under `--diff-from`) | **no** | yes |
  | `no-baseline` (`regression` at round 1) | n/a — never a unit | yes |

  The two rows in bold are the failure: a corpus role with an empty checklist is spawned as a unit
  and emits no prompt, so under an exit-2 rule its own brief would fail before it did any work. An
  earlier draft asserted "the mapping is exact and already exists" without checking, and it is not
  exact. Exit 0 with an empty `prompts[]` lets that unit finish cleanly and record nothing, which is
  the correct outcome for a role that had nothing to review.

  **A unit that receives no prompt still creates its findings file, empty.** Without that sentence
  the exit-0 rule trades one failure for another: `hooks/stop-role-incomplete.sh` fires for every
  `*-role` unit and applies `spec/0.35.0.md` §3.3's predicate to
  `$TP_ROUND_DIR/role-$TP_UNIT_ID.ndjson.part`, so a unit that writes nothing at all is refused its
  first stop. The hook already admits the empty case — its own comment reads "a role with nothing to
  report writes an empty file and passes" — so no hook change is needed, only the instruction. The
  refusal is bounded rather than fatal (the hook stands down on `stop_hook_active`), which is why
  this is a wasted round-trip and a confusing message rather than a hang.

- **The hint enumerates the reason, it does not classify the mistake.** An earlier draft promised
  "three distinct operator mistakes, three distinct hints" and then ruled on one of the five reasons
  in the table above. There is no need to invent a taxonomy: the exit-0 case reports tp's own
  `skipped_roles` reason verbatim, and the exit-2 case has exactly one hint because it has exactly
  one cause — a name tp does not recognise at all.
- `--role` takes exactly one value. It is a string flag, not repeatable; `--role ""` is refused as
  an unknown role rather than treated as absent, so "flag given empty" and "flag absent" are never
  the same command.

#### 4.2.2 Where the flag is legal

- `--role` is legal wherever prompts are emitted and refused with exit **2** where none are. The
  refusal is evaluated **before** the mode's own argument validation, so the hint an operator sees
  names the flag conflict rather than a missing `--findings`. The two commands do not have the same
  flag sets, so the modes are enumerated separately rather than as "and audit's equivalents":

| Command | `--role` legal in | `--role` refused in |
|---|---|---|
| `tp review` | default, `--perspective`, `--diff-from`, `--verify` | `--merge`, `--record`, `--status`, `--report`, `--resolve`, `--resolve-all` |
| `tp audit` | default only | `--merge`, `--record`, `--status`, `--resolve`, `--resolve-all` |

Verified against the shipped binary: `tp audit` registers no `--perspective`, `--diff-from`,
`--verify` or `--report`.

**The split is mechanical and has no exceptions.** A draft refused `--verify` on the reasoning that
it "already emits exactly one prompt, and a flag that cannot change the output should not be
accepted" — measured, `--perspective testing` and `--perspective documentation` each emit exactly
one prompt too, so that criterion condemned two modes the same table calls legal. It is dropped
rather than patched: `--role` is legal wherever prompts are emitted, and in a one-prompt mode it
selects that prompt or exits 0 empty by §4.2.1's rule, which is the same behaviour as everywhere
else.

#### 4.2.3 The brief that passes it

**The driver must pass it, or the saving is unreachable.** This is the half an earlier draft
omitted, and without it `--role` repeats `--out-dir`'s mistake in a new shape: `spec/0.35.0.md`
§3.3.1 fixes the role kinds' brief command as `tp review <spec>` / `tp audit <spec>`, and nothing on
any path appends a flag to it. So this release also changes the brief the role kinds
emit to `tp review <spec> --role <id>` and `tp audit <spec> --role <id>`, where `<id>` is the unit's
own id — the same value that becomes `TP_UNIT_ID` and names `role-<id>.ndjson`.

**The seam is where the id is in scope, which `UnitKind.BriefCommand` is not.** A draft named that
function; it takes a `UnitTarget` and `roleUnits` passes the role id as a separate argument to
`newNextUnit`, so the id is not visible inside it. The change belongs where both are known. Naming
the wrong function is the same class of error as naming a non-owning spec: it reads as settled and
is not buildable.

The flag is passed even though `TP_UNIT_ID` already carries the same value, and that is deliberate:
the brief is the *interactive* path's instruction too, where no `TP_*` variable is set, and a brief
that only worked under the driver would leave the operator back at the split script. One string
serves both, at the cost of one redundant token under `tp run`.

**The id-to-role mapping is not assumed.** An earlier draft called it "exact and already exists";
the skip table above shows it is not, which is why the exit-0 rule exists and why §6.2 property 10
fixes the mapping rather than trusting it.

#### 4.2.3.1 `review_loop.instruction`

**It is addressed to a caller holding the whole panel, and `--role` makes it
false.** The key is emitted on every review payload and currently reads, in part: *"For each prompt,
spawn a sub-agent via the Agent tool… Process the regression prompt first and apply its findings
before or together with the three role prompts."* A unit invoked with `--role` holds exactly one
prompt, is not the orchestrator, and has no regression prompt to process first — so the instruction
tells it to do three things it cannot do. A draft changed what a unit receives and left this key
untouched, which is how a release breaks a shipped contract it never mentions.

**The rule is a property, not a list of sentences to delete.** A draft specified the `--role` form
as "drops the spawn-a-sub-agent sentence and the regression-ordering sentence", which fails twice:
`buildReviewLoopInstruction` assembles the string across nine assignment sites, so there is no
single quoted text to subtract from; and two named sentences are not all of them — the key also
directs the caller to merge, to record the round, and to run an uncounted `--perspective regression`
delta pass, none of which a single-prompt unit does. The rule is therefore:

> Under `--role`, no sentence of `review_loop.instruction` may direct an action the caller's own
> payload cannot support. Without `--role`, the key is unchanged.

Both halves are load-bearing: the first is what makes the key honest, the second is what keeps every
existing caller reading exactly what it read before.

**It does not claim the convergence statement.** A draft said the `--role` form "keeps the round's
convergence statement". Measured: `convergence` is a *sibling key* of `instruction` under
`review_loop`, not a sentence inside it, so that clause named content the edit does not touch.

**It does not reword the unrestricted instruction.** The sentence naming "the three role prompts" is
stale against this repository's four-role corpus, but it is a pre-existing defect: correcting it
would change what every non-`--role` caller reads, and this release's scope is what a `--role`
caller receives. That is a scope decision stated here, not a prohibition read into Non-Goal 7, which
speaks about checklist items, role rules and the output schema and says nothing about this key.

### 4.3 Who it is for

P1 asks whether a change serves the agent. Under `tp run` at round 4 a role unit receives 131,727
bytes to read 12,376 of them; with the brief of §4.2.3 it receives only its own. Across the round that
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
more than this release's size. Non-Goal 6 records where it should land and that nothing there
covers it yet.

What this release does change is that the loss stops being silent. With §4.2.3's brief every unit
receives exactly the roles it was assigned, so a role nobody is assigned is visibly emitted to
nobody rather than buried inside a payload every unit happens to receive. §6.2 property 11 pins that
visibility to an exact set, so it cannot quietly change.

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

`--role` answers the same measurement without any of the four — but only with §4.2.3's brief change,
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
   unchanged. No spec owns the question yet and §3.1 declines to name one.
6. **Giving the `regression` role a unit of its own.** §4.1 measures that its prompt reaches every
   unit and is executed by none; §4.4 explains why the repair is new unit-lifecycle surface. No spec
   contains that section today, and §3.1's rule applies here too: this one names no owner either.
   This release makes the loss visible, not fixed.
7. **Changing what a round asks.** The checklist items, role rules and output schema are untouched.
   The prompt does gain two behavioural clauses — that is §2 and §3, and §1 prices them; the claim
   here is narrower than "nothing changes".

## 6. What must be true

This section states properties, not tests. Nineteen numbered tests stood here across seven rounds
and §6 was the only section that never converged — 71 of the cycle's findings against §2's 16 and
§3's 2 — because a test list is *how to verify*, and every construction it prescribed acquired a
constraint only code could evaluate. The properties below are what an implementation must satisfy;
choosing the fixtures, the harness and the assertions is a task's acceptance, which is where tp's
own workflow already puts them.

### 6.1 Conditions on the suite as a whole

**S1. The suite does not write the repository's live review state.** It must *read* it — property 8
emits from every spec in the repository, and a draft that said "neither reads nor writes" made the
release gate unsatisfiable by its own precondition. What is forbidden is mutation:
`spec/.tp-review/` and `.tp/rounds/` are byte-identical after the suite runs **and** unchanged
throughout it. The second half is load-bearing because properties 7 and 8
compare emissions across separate invocations, and a panel that shifted between them — a round
advancing, `regression` starting or stopping being appended — would make those comparisons fail or
pass for reasons that have nothing to do with `--role`. The tests
invoke `tp review` and `tp audit` many times and both write the round snapshot (§2.2), so a suite
run against the live tree would advance rounds it does not own while reporting green. Two
constraints bound any solution: `--no-state` is not one, because it disables the state the tests
measure — the round number, `skipped_roles`, the consecutive-clean count and whether `regression`
is appended; and relocating the state means relocating the spec, since `engine.ReviewStateDir`
derives the directory from the spec's own path with no override, while the `tp audit` half needs a
working tree with git history because it detects affected files by `git diff`.

**S2. No fixture encodes this repository's review state.** The payload embeds the round number, the
loop budget, the consecutive-clean count and the `output_path`; a stored blob would encode the day
it was written and fail on the next recorded round. Those four inputs are pinned by the test.

### 6.2 Properties

#### 6.2.1 Clause emission

1. Every emitted prompt whose `output_path` is non-empty ends with §2.3's construction exactly —
   trailing newline removed, then `LF LF`, §2.2's clause, `LF LF`, §3.2's clause, no trailing
   newline. Holds for `tp review` and `tp audit`.
2. No prompt whose `output_path` is empty **ends with** property 1's suffix. Positional, like
   property 3, and for the same reason: a containment form ("contains neither clause") is forgeable
   by the document under review, since tp embeds changed spec sections verbatim and this spec quotes
   both clauses. Measured on this round's emission, the `--perspective testing` and
   `--perspective documentation` prompts contain zero occurrences of either — so a reviewer's claim
   that the containment form is false *today* did not survive checking, and the form is replaced for
   the hazard rather than the failure. The modes that produce such a prompt are derived from what
   the code accepts — every `--perspective` value `review.go` validates, plus `--verify` — not from
   a list written here.
3. **The suffix is appended once, not twice.** An emitted body ends with property 1's suffix, and
   that same body with its last 468 bytes removed does **not** also end with it. Decidable on any
   live emission, in one comparison, with no fixture and no second version: it is the assertion that
   catches a double append, which property 1 alone cannot, since `…A B A B` ends with the suffix
   just as `…A B` does.

   Four drafts failed here before this one, and the arc is worth keeping because it is the release's
   own lesson applied to a single property. A cross-version baseline was unbuildable; a build-time
   toggle bought a production seam whose only false caller is a test; a "the remainder contains
   neither clause string" form is forgeable by the document under review, since tp embeds changed
   spec sections verbatim; and the fixture spec commissioned to escape that forgeability was a new
   maintained artifact with no home, bought to fence a hazard measured **latent** rather than live.
   Each repair added a mechanism. This one removes them all and asks a question the emission can
   answer by itself.

#### 6.2.2 `--role`

4. **`<name>` falls in exactly one of three classes**, and properties 4 and 5 cover them without
   overlap or gap: it is in the emitted set (this property); tp recognises it but this invocation
   does not emit it (property 5, exit 0); or tp recognises it nowhere (property 5, exit 2).
   "Recognises" is the union of the emitted set, `skipped_roles`, and the corpus for either phase —
   a draft left the third element unnamed while tp emits four names that qualify.

   **When `<name>` is in the emitted set**, `--role <name>` emits exactly one `prompts[]` entry,
   byte-identical to that role's entry in the otherwise identical invocation without the flag.
   Property 5 governs every other name; a draft opened this one as an unscoped universal and the
   two contradicted each other for a name tp recognises but the round does not emit. Every other top-level key is unchanged, and
   `review_loop` is unchanged **except its `instruction` member**, which property 12 governs — the
   exception is stated at the member's granularity because the rule quantifies over keys and
   `review_loop` is one of them. A draft said "every other top-level key unchanged" with no
   exception at all and contradicted property 12 outright; its successor granted the exception at
   the wrong level and left `review_loop`'s other members unconstrained.
5. A name tp recognises but the round does not emit exits **0** with `prompts: []`, and in the
   default mode that role's reason is echoed from `skipped_roles`. Measured: the `--perspective`
   and `--verify` payloads carry no `skipped_roles` key at all, so there the exit is 0 with an empty
   `prompts[]` and no reason to echo — a draft required the echo everywhere and made the branch
   unsatisfiable in four of the five modes §4.2.2 legalises. `--diff-from` is on the default side of
   that split — it emits `skipped_roles` and is the only mode that can produce `no-spec-change`, so
   a partition that named only the default mode left the release's own required case without a
   rule. A name tp recognises nowhere exits **2**, with a hint naming
   what the invocation would have emitted. §4.2.1's five reasons are a union across both commands,
   not a matrix either one can produce: `no-baseline` is `tp review`'s built-in regression role and
   `no-spec-change` needs `--diff-from`, which `tp audit` does not have. Each reason is exercised on
   the command that can produce it.
6. `--role` is accepted in every mode that emits prompts and refused with exit 2 in every mode that
   emits none, against each command's own flag set (§4.2.2).
7. The per-role emissions, concatenated in the **unrestricted payload's** order, equal that
   payload's `prompts[]` — for both commands. Order comes from the payload rather than from the
   sequence of invocations, which the caller controls.
8. Property 7 holds for every spec in the repository an unrestricted invocation emits prompts for,
   not only a fixture. This is the release gate; §7 says why breadth is what a fixture cannot buy.
9. *Withdrawn.* A draft required that the payload differ from "the same tree's pre-clause emission"
   only by §2.3's construction. That is a claim about this release versus its predecessor, and §6.3
   records why no test in one binary can obtain the predecessor's emission. Property 3 keeps the
   part that is decidable from a single emission; the rest was a promise no assertion could keep,
   and it is better withdrawn here than restated as a fourth unbuildable baseline.

#### 6.2.3 The driver's brief

10. Each role unit's `brief_command` names that unit's own id, and running it exits 0 — including
    where the unit set and the emitted set diverge, which §4.2.1's table shows they do.
11. The emitted role set minus the unit set is exactly `{regression}` at a round where `regression`
    emits, as set equality, so a release that adds a regression unit must update this deliberately.
12. Under `--role`, `review_loop.instruction` is a sentence-subset of the unrestricted one and
    contains none of that key's orchestrator-addressed directives. Without `--role` it is unchanged.
    Which sentences are orchestrator-addressed is a fixture the task owns: it is a reading, not a
    computation, and two drafts that tried to settle it here failed in opposite directions.

### 6.3 Dead ends, recorded once

Four constructions were specified and withdrawn. They are listed here rather than beside the
properties they used to encumber, so that the properties stay readable and the reasons stay found.

| Withdrawn | Why |
|---|---|
| A cross-version baseline (`v0.35.2`'s emitter) | A `go test` binary compiles one tree; the old emitter is not callable from the new one |
| A production toggle to emit "without the clauses" | Buys a seam whose only false caller is a test |
| A `TestMain` bracket for S1 | `TestMain` is per-package and these tests span packages |
| An enumerated list of forbidden directives for property 12 | Measured short by one; its semantic replacement was undecidable |

## 7. Release gate

Every mechanism in this release hides, regroups or narrows what a reviewer sees — `--role` most of
all — which is the shape that cost v0.34.0 §7.1 eight rounds of suppressed findings. So the release
is gated, and the gate is **§6.2 property 8**: the whole-panel equality of property 7, holding for
every spec in the repository rather than a fixture. Failing it blocks the release.

**Breadth is the only thing this adds, and the claim is measured rather than assumed.** A draft
justified it with "the repository has specs with deactivated roles, with domain frontmatter, with
empty checklists" — checked: **zero** specs carry `enabled: false` and **zero** carry a `domain`, so
that sentence was invented, and it was the section's whole argument. What the repository does supply
is narrower and real: **22 version specs, 5 KB to 79 KB, 15 with recorded rounds and 7 with none** —
so the corpus exercises both sides of the one skip reason that is round-dependent (`regression`,
skipped `no-baseline` at round 1 and emitted after), across section structures no fixture author
would think to write. The other four skip reasons do not occur here and must be built as fixtures;
§6.2 property 5 owns them. Breadth buys the shapes, not the skips.

**Why this section is three paragraphs and not a procedure.** Three drafts specified the gate as its
own mechanism — a replay of recorded rounds, then a per-spec comparison with a cost model and a
scope predicate — and each drew more findings than the last: 3, then 8, then 11 across rounds 3 to
5, while §2, the release's actual clause, went 11 → 2 → 1 over the same rounds. The section that was
diverging was the one describing *how to verify* rather than *what must be true*, which is the
distinction §6.1 applies to the tests and this paragraph applies to itself. The gate's content is a
test; the procedure was surface.

Two things earlier drafts required are recorded here as measured dead ends, so they are not
re-proposed. **Replaying recorded rounds does not work**: all 145 recorded review rounds under
`spec/.tp-review/` have a snapshot, but for 35 of them (24%) the snapshot's sha256 does not equal
the `spec_hash` that round recorded — tp refreshes a snapshot when the spec changes, so a quarter of
the corpus is not the text its round reviewed, and a gate reading it as history compares against the
wrong spec and reports clean. **Checking that each finding's class still reaches its role does not
work either**: it needs a `class` → checklist-item mapping that exists nowhere in tp, and 303 of the
corpus's 2,802 recorded rows (10.8%) carry no `role` or `class` at all.
