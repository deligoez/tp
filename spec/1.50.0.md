# tp v1.50.0 — What a round's rows actually say

> **This file is decisions.** Its three parts move in **two directions** — two stop gating, one starts
> being reported — and an earlier statement of this release described all three as "stops gating",
> which is wrong for the second. The correction is recorded because the symmetry is what made it
> plausible.
>
> **Every figure names the command that derives it.** Ground round 1 falsified four claims here; each
> is replaced by what was measured, not by a softer version of itself.

## 1. Overview

A round's convergence is computed from its rows. Three times, the row's own field is not what the row
means:

| | today | this release |
|---|---|---|
| a finding recorded `wontfix` or `duplicate` — **accepted, with evidence** | gates exactly like an undispositioned row: `resolved` is never read (§2, measured) | stops gating |
| a `PASS` row whose note names a defect | invisible to every counter `--status` emits | **reported**, still not gating (§3) |
| a row whose role does not decide the question | every row gates, whatever its role — including a row with no `role` key (§4, measured) | stops gating when the role is listed out, behind a fence (§4) |

**This release needs the recorded panel.** §4 reads `expected_roles`, which the release that records
the panel a round expected introduces. It does not exist yet: a search for `expected_roles` or
`ExpectedRoles` across `internal/` returns zero matches.

## 2. An accepted finding stops blocking

Measured on a built fixture outside the repository — a two-section `spec.md`, `tp init spec.md`, and a
one-row results file:

```json
{"item_id":"i1","status":"FAIL","severity":"warning","role":"go-safety",
 "resolved":{"status":"wontfix","evidence":"accepted, named next version",
             "resolved_at":"2026-09-02T00:00:00Z"}}
```

`tp audit spec.md --record results.ndjson` prints:

```
round: 1   clean: false   consecutive_clean: 0   converged: false   findings: 1
```

Re-recording the identical row as rounds 2, 3 and 4 leaves `consecutive_clean` at 0 every time. The
mechanism is `internal/engine/auditclean.go`'s `AuditRowsClean`, which consults `AuditRowIsPass` and
the row's severity and **never reads the `resolved` key at all**. Substituting `duplicate` for
`wontfix` gives the same result, for the same reason.

**An accepted finding gates permanently under the default `audit_converge_on: all`.** Nothing a
recorder can write into the row changes that, because the field that records the acceptance is not
read.

### 2.1 What the shipped severity policy already reaches, and what it does not

**One escape exists today and destroys nothing** — and it is the fixture above that shows it. Its
severity is `warning`, which `auditclean.go`'s `advisoryAuditSeverities` classes advisory, so under
the field v0.37.0 shipped:

```
tp set --workflow audit_converge_on=blocking
tp audit spec.md --record results.ndjson    →  round: 1  clean: true  consecutive_clean: 1  findings: 1
```

The row is still recorded, still counted as a finding, and the round is clean.

**It does not reach the case this release is about.** The same fixture with `severity: "error"`, under
`audit_converge_on=blocking`, records `clean: false`, `consecutive_clean: 0`. So an accepted **`error`**
row — and any row whose severity tp cannot grade, which `auditclean.go` blocks by design — has no exit
that leaves the record intact.

No exhaustiveness claim is made about the ways out. An earlier draft said *"the only ways out are to
stop recording it or to record it as `PASS`"*, which the fixture above falsifies — `blocking` is a
third, and it destroys nothing.

### 2.2 This is a documented decision, and reversing it is the release

**This is not a bug report.** `skills/tp/SKILL.md` states the behaviour and its reason, in the
Workflow D step that describes `--resolve` — cited by its anchor phrase, because the line number
moved twice on 2026-09-03 and both destinations are still normative prose in the same file:

> **A disposition is not an escape hatch from the gate:** `tp audit --record` counts every row whose
> `status` is not exactly `PASS` and reads no disposition at all, so parking a finding leaves the
> round's finding count, the role streak and `--status --check` exactly where they were.

A second passage, in the `divergence` paragraph, carries the policy half rather than repeating the
mechanism — *accepting findings outside spec conformance is a user-approved decision, never the
agent's.* **The fear is correct**: a disposition an agent can write is a way for an agent to make its
own findings stop counting.

**What the promise did not anticipate is that the project's own release rule requires shipping over an
accepted finding.** `CLAUDE.md`'s rule is to record out-of-surface findings with justification, name
the version that takes them, and ship. Both hold at once only because the project does not use
`--check` as its ship signal — which is the divergence two releases have now shipped through by hand.

### 2.3 The fence this reversal needs does not exist yet, and building it is part of the release

An earlier draft argued that the agent-safety property survives because *"recording a disposition is a
user-approved decision under `TP_UNATTENDED=1`"*. **That fence does not exist.** Measured at HEAD:

- a search for `Unattended` in `internal/cli/audit_resolve.go` returns **0** matches; the call sites
  of `engine.Unattended()` in `internal/cli` are `unattended.go`, `config_extract.go`, `set.go`,
  `set_project.go`, `set_local.go`, `importcmd.go`, `done.go` and `close.go` — `audit_resolve.go` is
  not among them;
- `TP_UNATTENDED=1 tp audit raw.ndjson --resolve 0 wontfix "accepted for now"` exits **0** and writes
  `resolved.status: "wontfix"` into the file;
- `TP_UNATTENDED=1 tp audit raw.ndjson --resolve 0 wontfix ""` also exits **0**, writing
  `resolved.evidence: ""`.

So an unattended agent can today write the exact row §2 would make non-gating, with an empty reason.
Shipping §2 without closing this turns that write into precisely the escape hatch the quoted passage
exists to deny, while removing the only thing that currently stops it.

**Therefore the release carries the fence rather than assuming it.** Two sinks, both new work:

1. `tp audit --resolve` / `--resolve-all` become user-approved decisions under `TP_UNATTENDED=1`,
   refused at exit 2 with an escalation hint, like every other decision `CLAUDE.md` reserves for the
   operator.
2. **`resolved.evidence` must be non-empty** — both at the write, which accepts `""` today, and at
   record time, where §2's new rule reads it. An acceptance without a stated reason is
   indistinguishable from a deletion. tp does not judge the reason; it requires one.

**So the reversal is narrow and keeps the fear intact.** A disposition stops gating **only** when it
carries non-empty `evidence` and only for `wontfix`/`duplicate`. **A rule enforced by making a
recorded decision meaningless is enforced in the wrong place** — but it has to be enforced somewhere,
and this release is where.

The two `SKILL.md` passages above are rewritten by this release, and the release is not complete until
both documents say the new thing. **Identify them by their anchor phrases, not by line number** — they
sat at 145 and 149 when this file was drafted, moved twice the next day, and what sits at those lines
now is unrelated normative prose a unit could plausibly rewrite without noticing.

**`wontfix` and `duplicate` stop gating; nothing else does.** `fixed` still voids the round, correctly
— a fix means the spec or code changed and the round's findings were read against older text.

**The row stays in the round and stays visible.** It appears in `role_streaks`' open count and in
`tp audit --merge`, both confirmed at HEAD: recording the fixture gives
`role_streaks: [{role: go-safety, consecutive_clean: 0, open: 1}]`, and `tp audit --merge` emits the
row with its `resolved` block intact. It stops gating convergence; it does not stop existing.
(`--report` is a `tp review` flag only — `tp audit --help` does not list it — so this release does not
claim it as a place the row stays visible.) `CLAUDE.md`'s own shipping rule already works this way in
prose — *record the out-of-surface findings with justification, name the version that takes them, and
ship* — and this makes the tool agree with it.

## 3. A PASS row carrying a note is reported

`CLAUDE.md` records a peer cycle where a role scored **55/55 PASS in three consecutive rounds while
its prose named a real spec defect each time** — all three fixed by commit. This repository has a
weaker instance of its own: `spec/.tp-review/0.36.0/audit-round-13.ndjson` holds a `spec-coverage`
PASS row (`item_id: task-role-mode-test`) whose notes say a doc comment *"claims more than its body
delivers"*, inside a round counted toward the `spec_coverage_clean_rounds` v0.36.0 shipped on.

**`--status` reports how many `PASS` rows carry a non-empty `notes` string, per role.** A clean streak
beside *"14 PASS rows carry notes"* is a different object from a clean streak beside zero. The field
is `notes`, plural — `internal/cli/audit_schema.go` asks for it as *"always; short string, max 500
chars; `\"\"` if no notes"*.

**It does not gate, and the reason is a limit rather than caution.** tp cannot distinguish a note
naming a defect from a note recording what was read, and the second kind is almost all of them:

```
python3 -c 'import json,glob,collections;R=[json.loads(l) for f in glob.glob("spec/.tp-review/*/audit-round-*.ndjson") for l in open(f) if l.strip()];P=[r for r in R if r.get("status")=="PASS"];N=lambda r:bool((r.get("notes") or "").strip());print(len(P),sum(map(N,P)),collections.Counter((r.get("role"),N(r)) for r in P))'
```

**12,113 of 12,447 PASS rows carry a note (97.3%) — and per role it is 100.0% on every one**:
`spec-coverage` 8,304, `maintainability-conventions` 818, `go-safety` 777, `ax-contract` 769, each
`True` in every row. The whole 334-row shortfall keys on `role: None` — pre-role rounds, which a
per-role counter cannot bucket in the first place. A gate here would be a gate on prose length.

**That is also why the count is worth emitting at all**, and the per-role figure sharpens the
argument rather than weakening it: on the population this field would actually count, the number is
not merely near-constant but identically equal to the role's PASS count in every round tp has
recorded. What a reader needs beside a clean streak is that number, so that *"55/55 PASS, 55 carrying
notes"* stops reading as *"55/55 PASS"*. The field asserts that prose exists to be read, not that it
says anything.

**This is the counter's half; the prompt's half ships separately.** The release that returns a
`PASS`-with-note row to its own author next round closes the same gap from the other end. Neither
needs the other, and both are cheap.

## 4. A role that does not decide the question stops gating

`audit_converge_roles` — a list of role ids whose rows gate convergence.

**Default: unset, and unset means every row gates, exactly as today.** The default is stated over
*rows* rather than over roles because that is what was measured, and the difference is a relaxation
hiding inside a word. At HEAD, in two fresh fixtures:

- a round holding one `FAIL` row with `role: "not-a-real-role"` records `clean: false`, `findings: 1`,
  `role_streaks: [{role: not-a-real-role, consecutive_clean: 0, open: 1}]`;
- a round holding one `FAIL` row with **no `role` key** records `clean: false`, `findings: 1`,
  `role_streaks: []` — it gates while appearing in no per-role signal, announced only by a stderr
  line (*"1 row(s) … are missing the role field"*) that `--quiet` erases.

A default phrased as *"every expected role"* would therefore have shipped two silent relaxations
before anyone narrowed anything: a row whose role is not in `expected_roles` would stop gating, and a
row with no role at all would stop gating with nothing left to notice it. The second is the reverse of
the fail-closed rule `auditclean.go` states for the sibling field — *a row tp cannot grade is a row tp
must not stop counting* — in a release whose whole subject is that narrowing must be fenced. So:

**When the list is set, a row gates only if its `role` is in the list.** A row with no `role` key
gates under every setting, including a set one; it is not a role and cannot be listed out.

### 4.1 What the two shipped releases actually measure

**This is the *too strict* direction.** Two releases shipped with `--check` at exit 1 while
`spec-coverage` was clean: v0.36.0's rounds 10–13 and v0.37.0's rounds 6–7 each hold **zero**
non-PASS `spec-coverage` rows, while the round-level `clean` bit is false in all of them.

**But severity parity closes it for one of the two, and the earlier compound claim — *"while the
conformance role was clean and no role held a FAIL"* — is satisfied by neither release.** Replaying
each release's own last two recorded rounds into a throwaway project (`tp init`, optionally
`tp set --workflow audit_converge_on=blocking`, `tp audit spec.md --record <round>.ndjson` twice, then
`tp audit spec.md --status --check`):

| | rows the round holds | `all` | `blocking` |
|---|---|---|---|
| v0.36.0 r12–13 | r12: 2 `PARTIAL`/`error`; r13: 2 **`FAIL`**/`error` + 1 `PARTIAL`/`warning` — all `maintainability-conventions` | exit 1 | exit **1** |
| v0.37.0 r6–7 | r6: 4 `PARTIAL`/`warning`; r7: 3 `PARTIAL`/`warning` — `ax-contract` and `maintainability-conventions`, **zero `error` rows** | exit 1 | exit **0** |

So: for v0.37.0 *"no role held a FAIL"* is true and a severity-scoped `--check` **does** close the gap.
For v0.36.0 severity parity does not close it — but its shipping round holds two `FAIL` rows, so the
antecedent fails. **The general claim that severity parity does not close this is proved on one
release and refuted on the other, and is therefore not made here.** What both releases do share is the
role axis: in all four rounds the masking rows belong to a non-conformance role and `spec-coverage`
holds none. **The missing axis is role** — on the evidence of these two replays and no more.

### 4.2 The fence — and the one part of it this file leaves open

**`audit_converge_roles` is fenced under `TP_UNATTENDED=1` like `audit_converge_on`**, because a set
list relaxes a gate. The shipped analogue, measured at all four sinks in a fresh project:

- `TP_UNATTENDED=1 tp set --workflow --project audit_converge_on=blocking` refuses at exit 2 **on the
  value alone**, both before and after `blocking` already resolves — `unattended.go` compares
  `value == engine.AuditConvergeOnBlocking`;
- the task layer, `tp import` and `tp config --extract` refuse only a write that **changes what
  resolves** — all three call `engine.AuditConvergeOnRelaxes(before, after)`. `tp import` carrying an
  already-resolved `blocking` block forward imports at exit 0, which is exactly why a value rule there
  would deadlock it.

Note what that predicate is, because an earlier draft got the reason wrong: the shipped fence keys on
**relaxation**, not on non-defaultness. A write of `all` over a resolved `blocking` is a tightening
and passes. The `--project` literal comparison is a *shortcut* that happens to be exact for this
field, because `audit_converge_on` has exactly one non-default literal and it is the relaxing one.

**Open decision, stated rather than asserted: what does the `--project` sink compare a *list* against?**
There is no relaxing literal for a list. Whether a given list narrows is decidable only against
`expected_roles`, which is per-base and per-round — and `unattended.go` says in terms that the
`--project` sink *"has no single such value: the write lands under every base at once, and the bases
it moves include ones tp cannot enumerate."* Two candidates, neither taken here:

1. **Refuse every non-empty list at `--project`.** Sound and blunt: it also refuses a list that
   narrows nothing under any base, and there is no way to express the policy repo-wide.
2. **Refuse at `--project` only when the list is not a superset of every base's `expected_roles`.**
   Precise, and it requires the sink to enumerate bases it cannot see.

The analogy to `audit_converge_on` is exact at the other three sinks and under-determined at this one.
**The fenced release's own history is that an under-determined `--project` rule cost it four audit
rounds**, so this is named as an open decision for the review round rather than settled by assertion
here.

**A narrowed panel does not silence anyone.** Non-gating roles still emit, still record, still appear
in `role_streaks` and `--merge`. They stop deciding whether the loop continues.

### 4.3 The separation is the point, and merging is what this release stops short of

A sibling discipline outside this repository runs a two-axis review — *does the code follow the
standards* and *does it match the spec* — in parallel agents whose contexts never touch, then reports
them **side by side with an explicit rule never to merge or rerank them**, and never to name a single
worst finding across axes, because *"reporting them separately stops one axis from masking the
other."*

**That is precisely the failure this release exists to fix, arrived at independently.** tp's
`--check` collapses every role into one `clean` bit — `AuditRowsClean` iterates the round's rows with
no role dimension at all — and the measured consequence is one role's rows masking a conformance role
that was clean four rounds running (§4.1).

**tp merges in a second place, and it is not the one an earlier draft named.** The `(location, class)`
clustering is `tp review --merge`, in `clusterMergeFindings`. `tp audit --merge` does something
different: it dedups on **`(role, item_id)`** — role *inside* the key, so it cannot cluster across
roles at all — and its `overlap_report` clusters by `(item_id, category)`. Since §5's Non-Goal 4 puts
the review phase outside this release, the audit-side merge is what is in scope, and it does not
collapse the role axis in the first place. **Nothing in the merge surface is changed here**, and the
sibling's rule is about the *verdict* rather than the display in any case.

**A role listed but not expected is an error, not a silent no-op.** `audit_converge_roles: ["typo"]`
would otherwise converge on an empty gating set — the failure mode of every allow-list that is not
checked against its universe, and nothing catches it today: a round holding one `FAIL` row with role
`not-a-real-role` records at HEAD with no complaint (§4).

## 5. Non-Goals

1. **No new disposition value.** `fixed`, `wontfix` and `duplicate` are the three `tp audit --resolve`
   accepts, and this release adds none. Two of the three appear in the recorded corpus —
   `python3 -c 'import json,glob,collections;c=collections.Counter(json.loads(l)["resolved"]["status"] for f in glob.glob("spec/.tp-review/*/*.ndjson") for l in open(f) if l.strip() and isinstance(json.loads(l).get("resolved"),dict));print(c)'`
   gives `{'fixed': 1467, 'wontfix': 99}`, with `duplicate` at zero. **The glob is part of the claim**:
   a tree-wide walk over every `*.ndjson` returns a larger `fixed` count, because it picks up untracked
   working files, and the same zero.
2. **No judgement of `resolved.evidence`'s content.** §2.3 requires it to be non-empty at both sinks
   and reads no further.
3. **No gate on §3's count.** The limit is stated in §3 and is not a stance to be revisited by
   tightening a keyword list.
4. **The review phase keeps its own convergence policy.** `review_converge_on` and
   `ReviewConsecutiveClean` are untouched, and `tp review --merge`'s clustering is out of scope; every
   measurement here is from the audit side. (Corpus check on that claim: counting `status == "PASS"`
   rows over `spec/.tp-review/*/review-round-*.ndjson` returns **0** — PASS rows exist only in audit
   files, so §3's counter can only be an audit-side counter.)
5. **No retroactive re-grading.** `clean` is stamped at record time and stays stamped — measured: four
   rounds recorded under `all` still read `clean: false` after `tp set --workflow
   audit_converge_on=blocking`, although the same rows recorded fresh under `blocking` grade clean.
   These rules apply to rounds recorded after they ship.

## 5a. Open questions carried in from `spec/candidates.md`

**This section takes no decision.** Three *Undecided* entries are moved out of `spec/candidates.md` —
where every row names the decision nobody has taken — into the release that owns their subject, which
is what a round's recorded rows are allowed to say. They are recorded here as questions. §6 does not
depend on any of them and no row of §6 tests one. A spec that presented one of these as settled would
be worse than not moving it: their *design* has no answer.

### 5a.1 A durable home for an accepted finding

**The decision nobody has taken is the target shape.** §2 is the counting half — an accepted finding
stops blocking — and this is the durability half: an audit finding has three ends (fix it, reject it,
accept it as backlog), tp records all three the same way in `.tp-review/<spec>/`, and that directory
is archived at release along with the spec. There is no supported path from *accepted* to something a
maintainer trips over later. A deferral whose stated reason is self-renewing can be re-derived every
round forever, and its only record is scheduled for archival on the very release it is deferred past.
This repository has been working around it by hand for four cycles: the candidates files are that
durable target, maintained by an operator.

**Three options were named and none chosen.** The candidates row records only their number; the three
themselves survive in git, in the pre-2026-09-02 spec that carried this subject —
`git show 3a83be30:spec/0.41.0.md`, whose §2 is *Mechanism*. **That filename is not today's
`spec/1.41.0.md`**; the 2026-09-02 renumbering moved every number, which is why the commit is cited
rather than a path at `HEAD`. The three:

1. a checklist file at a stable path, appended to rather than rewritten;
2. an issue template written to disk for the operator to file;
3. a `TODO` entry with an owner and the finding's `item_id`.

**What decides between them is a property rather than a preference**, and it is the part already
agreed: *the target must be readable by the next cycle's decomposition without a human remembering it
exists.* An option that only works when someone happens to open it is not better than the round
directory it replaces.

### 5a.2 Making `severity` checkable

**The decision nobody has taken is what could check it that is not the row's author.** A non-`PASS`
row's severity is self-declared: the prompt renders the requirement and nothing validates what comes
back. `internal/cli/audit_record.go` validates **`category`** alone, through `invalidCategoryRows` —
there is no severity equivalent.

**One clause of the entry is stale and is corrected here rather than carried.** It said *"nothing on
the audit path reads the field"*. At `HEAD` two paths read it: `internal/engine/auditclean.go`'s
`AuditSeverityBucket` and `advisoryAuditSeverities` grade on it whenever `audit_converge_on` is
`blocking` (v0.37.0), and `tp audit --merge` buckets `by_severity` through the same classifier. What
survives is narrower and still the point: **nothing validates it**, and under the resolved default —
`tp config --resolved` reports `audit_converge_on` at `all`, source `default` — no grading path reads
it at all.

**Two mechanisms were drafted and both withdrawn within a round.**

1. **Rejecting an out-of-enum severity at `--record` inverts its own precedent.** The category sink
   returns early on an *empty* category — `if category == "" || engine.IsValidCategory(category)` in
   `invalidCategoryRows.observe` — and that early return is pinned deliberately by
   `TestParseAuditRows_AcceptsTheEnumAndAbsentCategory` in `internal/cli/audit_category_sink_test.go`,
   whose comment gives the reason: *"treating that as invalid would reject every clean round."*
   (Cite the symbol and the test name, not a line: the row cited `audit_record.go:282` and the check
   sits elsewhere in the file at `HEAD`.)
2. **It would refuse fifteen of this repository's own recorded round files.** Reproduced exactly at
   `HEAD` — **15 of 108**. The counting rule: an audit round file
   `spec/.tp-review/*/audit-round-*.ndjson` holding at least one row whose `severity` is present and
   outside the audit vocabulary `{error, warning, info}` that `auditclean.go` defines. The offending
   values are the *review* vocabulary leaking into audit files — 16 `high`, 15 `medium`, 36 `low`
   rows — and the fifteen are 0.29.0 rounds 1–2, 0.30.0 round 1, 0.31.0 round 1, 0.31.2 rounds 1–5
   and 0.32.0 rounds 1–6. The review side is clean under its own vocabulary: **0 of 172** review
   round files carry a severity outside `{critical, high, medium, low}`. Derive both with a walk over
   those globs counting files whose `severity` values fall outside the phase's set.

**And validation cannot deliver what it was introduced for.** It makes a label well-formed, never
truthful, and it cannot make an unrun round run.

### 5a.3 An audit-side `nonblocking_open`

**The decision nobody has taken is whether to invert a guard that pins the key's absence.** Under
`audit_converge_on=blocking` a clean round can carry `warning` and `info` rows — §2.1's fixture is
exactly that, `clean: true` with `findings: 1` — so the audit phase now has the accepted-open state
the review-side field was built to make visible.

**The count is already emitted; what is missing is the breakdown.** `role_streaks[].open` is that
count and it is **severity-blind**: `engine.RoleStreak`'s `Open` is documented in
`internal/engine/rolestreaks.go` as the role's *non-PASS row count in that latest round*, with no
reference to severity. It reaches both `tp audit --status` and `tp audit --record`, and `tp run
--status` on an audit-phase stop, through `auditSignalFields` in `internal/cli/audit_record.go` —
anchor on that function, because the row cited `audit_record.go:152` and that line is a `next_action`
comment at `HEAD`.

**Four places pin the key's absence by name, and all four are present at `HEAD`:**

1. `spec/0.31.0.md` §8.4 — *"`nonblocking_open` is review-only (audit convergence has no non-blocking
   notion, §4.4) and is never an audit field"*;
2. that spec's test 18, which repeats it as an acceptance row;
3. `skills/tp/REFERENCE.md`'s `--compact` disposition paragraph, which marks the field *"(review-only,
   emitted only on an accepted-open clean round)"* — cited by that phrase, because the row's
   `REFERENCE.md:641` has moved;
4. `TestReviewNonBlockingOpen_AuditUnaffected` in `internal/cli/reviewconvergeon_clean_test.go`, which
   asserts that neither `tp audit --record` nor `tp audit --status` emits the key.

A fifth statement exists and is a comment rather than a guard: `internal/engine/reviewclean.go`'s
*"Review-only: no audit payload carries nonblocking_open."* No release has decided to invert any of
them. **The `tp audit --merge` breakdown is a different surface and shipped separately** — it buckets
`by_severity` through `engine.AuditSeverityBucket` in `internal/cli/audit_merge.go`, on the merged
payload rather than on the round's convergence signal.

## 6. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it. The last column says which are **red at HEAD** — rows 1, 1b, 3b, 7, 8 and 9, plus the
write half of row 3. The rest are green today and cannot be watched failing before implementation;
they exist to stop that implementation over-generalising, and row 1 is the acceptance.

| # | from | assertion | the mutant that must fail it | at HEAD |
|---|---|---|---|---|
| 1 | §2 | with `audit_converge_on` **pinned to `all` in the fixture**, one `FAIL` row dispositioned `wontfix` with evidence records `clean: true` | the shipped behaviour, which records `false` | **red** — watch it fail first |
| 1b | §2.1 | the same assertion with the row's `severity: "error"` and `audit_converge_on: blocking` | grade the acceptance from severity, which reaches an advisory row and not this one | **red** |
| 2 | §2 *fixed* | the same row with `resolved.status: "fixed"` still blocks | treat every disposition alike, converging on a round whose spec has moved underneath it | green |
| 3 | §2.3 *evidence* | `wontfix` with an empty or missing `evidence` still blocks **at record time**, and `tp audit --resolve <n> wontfix ""` is refused **at the write** | accept the status alone, making an acceptance indistinguishable from a deletion | record-time half green; write half **red** (exits 0 today) |
| 3b | §2.3 *fence* | `TP_UNATTENDED=1 tp audit r.ndjson --resolve 0 wontfix "reason"` exits 2 | leave `--resolve` unfenced, which is HEAD — and which makes §2 an agent-writable escape hatch | **red** (exits 0 today) |
| 4 | §2 *visible* | the accepted row still appears in `role_streaks`' open count and in `tp audit --merge` output | drop it from the round, which is the record destruction this release exists to remove | green |
| 5 | §3 | a round with 14 `PASS` rows carrying non-empty `notes` reports 14, per role | count `notes` across all rows, so a `FAIL`'s note inflates the `PASS` figure; or read `note`, singular, which no row carries | green trivially (no such counter exists) |
| 6 | §3 *not gating* | `clean`, `consecutive_clean` and `--check` are identical with and without those notes | let the count gate, which gates on prose length | green |
| 7 | §4.1 | with `audit_converge_roles: ["spec-coverage"]`, a round where only a non-listed role holds non-PASS rows converges — **replayed on v0.36.0 r12–13 and v0.37.0 r6–7, both of which exit 1 today under `all`** | ignore the list, reproducing the two releases that shipped at exit 1 | **red** |
| 7b | §4 *default* | with `audit_converge_roles` unset, a row with an unrecognised role **and** a row with no `role` key both still gate | default to `expected_roles`, which relaxes both cases before anyone narrows anything | green — and it is the guard that keeps it so |
| 8 | §4.2 *fence* | the task layer, `tp import` and `tp config --extract` refuse only a write that changes what resolves | apply a value rule at `tp import`, which deadlocks it by refusing a block it merely carries forward | **red** |
| 8b | §4.2 *open* | the `--project` rule, once decided between §4.2's two candidates | a change rule at `--project`, which cannot see the bases it needs | **the decision is open; this row cannot be written until §4.2 is settled in review** |
| 9 | §4.3 *unknown* | a listed role absent from `expected_roles` is an error | treat it as a no-op, converging on an empty gating set | **red** |

**Row 1 is the acceptance; rows 3b and 8b are the ones to argue about.** 3b is the safety property
§2.3 discovered missing — without it this release ships an escape hatch. 8b is the shape that cost the
fenced release four audit rounds, and it is deliberately left open rather than guessed at: a release
that unifies the fence across sinks will pass every single-sink test while deadlocking `tp import`.
