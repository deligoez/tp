# tp v1.0.1 — A finding can leave a round

> **This file is decisions.** It carries two defects whose fixes were built and measured before it was
> written, three repairs to what tp *tells* its reader, and one change to what a round's rows mean. The
> theme is one sentence: **a finding leaves a round either as a spec change or as a `--resolve`
> disposition, there is no third way out, and `--status` says which happened.** Two field reports
> measured all three of the places that sentence is false today.
>
> **It was `spec/0.37.1.md` and is now `spec/1.0.1.md`, and the rename is forced rather than cosmetic.**
> `.claude-plugin/plugin.json` is read twice — Claude Code resolves plugin updates from it, and
> `hooks/session-start.sh` reads the same field as the **minimum tp version**. It says `1.0.1`. Tagging
> `v0.37.1` would tell everyone who installed that binary their tp is too old, so after `v1.0.0` there
> is no 0.x release. §4.1 records the one place this file's original bar had to move, and why.
>
> For the two pre-measured defects (§2, §3): both were reproduced by a test that failed against `HEAD`,
> fixed, and re-run green with the full suite passing. That this happened *before* the file was written
> is the author's own report and nothing in the tree can settle it — no test for either fix exists at
> `HEAD`, so that run left no committed trace. What is checkable is that it reproduces: ground round 1
> (`spec/.tp-review/1.0.1/ground-round-1.ndjson`) built both fixes in an `rsync` copy, wrote the probes
> before touching the code, and watched five of the six §9 rows it covered red at `HEAD` and green
> after — with row 4 green in both states, exactly as §9 predicts.

## 1. Why a patch and not a reordering

A hotfix does not move minor releases. tp has shipped ten patch releases —
`git tag -l 'v*' | sort -V | grep -E '^v[0-9]+\.[0-9]+\.[1-9][0-9]*$'` — and **eight of the ten are
headed as defect repairs in their own release notes**, derived by reading each tag's heading lines
with `gh release view <tag> --json body`: v0.3.1 (`### Bug Fix`), v0.12.2 (`## v0.12.2 — Bugfix`),
v0.31.1, v0.31.2, v0.34.1, v0.34.2, v0.35.1 and v0.35.2. The other two are v0.28.1 (docs) and v0.1.1
(a bare changelog). An earlier draft of this sentence enumerated six without counting them; the
conclusion is strengthened by the correction, not weakened.

Reordering the pending minors would be another renumbering, and `CLAUDE.md` records what the last one
cost: a sweep that examined **47 citations to renumbered specs, of which ~35 were stale**.

**The bar for this file is narrow, and two candidates were held back by it:** a defect qualifies only
if its fix is measured, carries no semantic change beyond the repair, and needs no new recorded field.
The `--check`-on-a-partial-round defect fails the third test (it needs the recorded panel and the
key-preservation fix), and the audit checklist's alphabetical ordering fails the second (churn ranking
is a behaviour change). Both stay in their minors.

## 2. `unresolved_findings` counts open findings

`roundPayload` (`internal/engine/resumepayload.go:95-110`) counts every row in the last round whose
`resolved.status` is not `"wontfix"`. On the audit phase every checklist item is a row, so a clean
round of 106 items reports 106.

**Measured on v0.37.0's own audit round 7** — the round this project shipped on, confirmed as the last
recorded audit round by `git show v0.37.0:spec/.tp-review/0.37.0/state.json`:

| | | derived from `spec/.tp-review/0.37.0/audit-round-7.ndjson` |
|---|---|---|
| rows recorded | 106 | count of non-blank lines |
| rows dispositioned `wontfix` | 3 | `Counter(resolved.status)` → `{None: 103, wontfix: 3}` |
| **`unresolved_findings` reports** | **103** | `tp resume spec/0.37.0.md --json` → `next_action.payload` |
| rows actually non-`PASS` | **3** | `Counter(status)` → `{PASS: 103, PARTIAL: 3}` |

**The 103 is exactly the set of `PASS` rows — by set identity, not cardinality.** Comparing the sets:

```
python3 -c 'import json;rows=[json.loads(l) for l in open("spec/.tp-review/0.37.0/audit-round-7.ndjson") if l.strip()];counted={id(r) for r in rows if ((r.get("resolved") or {}).get("status") if isinstance(r.get("resolved"),dict) else None)!="wontfix"};print(counted=={id(r) for r in rows if r.get("status")=="PASS"})'
```

returns `True`. All three real findings were dispositioned, so the field excluded every row that was a
finding and included every row that was not — the complement of the answer, not an approximation.

**Who it misleads was measured, and it is not the driver.** A grounding pass tested three hypotheses
and the shipped one was refuted: `unresolved_findings` is **written** at
`internal/engine/resumepayload.go:172` and `:190` into `next_action.payload` and **read nowhere in tp**. The driver
does not branch on it; `ReviewNextAction` branches on `blockingUnresolved`, computed separately from
the round's `clean` flag (`internal/cli/review_status.go:155`). So the field is for a **reader** — a human, or an
external driver that chooses to consume it — and the one documented instance of it misleading a
reader is **this repository's own orchestrator**, which reported the 103 to its operator and had to
retract it.

**A row counts as unresolved when it is a finding and is not closed.** A finding is a row whose
`status` is absent or not exactly `PASS` — tp's own documented rule (`skills/tp/SKILL.md`), so review
rows all remain findings and that phase's count is unchanged (Non-Goal 5 gives the corpus measurement
and its limit). Closed means `resolved.status` is `wontfix` **or** `fixed`, and `fixed` is thirteen of
every fourteen recorded dispositions, so omitting it left the overwhelming majority of closures inert:

```
python3 -c 'import json,glob,collections;c=collections.Counter(json.loads(l)["resolved"]["status"] for f in glob.glob("spec/.tp-review/*/*.ndjson") for l in open(f) if l.strip() and isinstance(json.loads(l).get("resolved"),dict));print(c)'
```

The counts move with every recorded round — 1,308 `fixed` / 98 `wontfix` when this file was written,
1,467 / 99 four days later — so the sentence rests on the share (93.0% then, 93.7% now), not on either
figure.

**An unrecognised `resolved.status` counts as open.** Fail-closed, matching `AuditRowsClean`'s
treatment of a severity it cannot grade.

### 2.1 The payload says what it counted

`unresolved_findings` gains three siblings in the same payload: **`rows_recorded`**,
**`findings_total`** and **`findings_closed`** — the round's total row count, how many of those rows
are findings, and how many finding rows carry a recognised disposition. All four integers fall out of
the single pass §2 already makes over the rows; nothing is read twice.

**A bare count cannot be sanity-checked, and this one went unchecked for every release since
`roundPayload` landed.** `git log --diff-filter=A -- internal/engine/resumepayload.go` gives abba7b76,
2026-07-23; `git tag --contains abba7b76 | wc -l` gives the number of releases that have shipped over
it — 18 as this is written, and rising with each tag. An earlier draft named a fixed endpoint
(v0.28.0 through v0.37.0, seventeen) and v1.0.0 overtook it two days later.

**Two counters would not be enough, and that was measured rather than argued.** With only
`rows_recorded` and `findings_closed`, a defective round and a correct one are indistinguishable:
ground round 1 (`u24`) built the mechanism and emitted `{unresolved_findings: 103, rows_recorded: 106,
findings_closed: 3}` *legitimately*, from a 106-row round whose rows carry no `status` — the shape of
every recorded review row — with three dispositioned `wontfix`. `103 + 3 = 106` is self-consistent, so
a reader holding those three numbers learns nothing.

**`findings_total` is what makes the count checkable, because it replaces a plausibility judgement
with an identity**: `unresolved_findings == findings_total - findings_closed`, exactly, by
construction. The shipped loop violates it on round 7 — 103 against `3 - 3 = 0` — while the legitimate
no-`status` round satisfies it — 103 against `106 - 3 = 103`. `rows_recorded` stays because it carries
what neither of the other two does: that round 7 was 106 items of which 3 were findings, the fact §2's
table rests on.

**They are reported, not gated**, and they are in this patch rather than a minor because they are the
same field's trustworthiness: the fix makes the number right, these make it checkable. Absorbing them
here retires the minor that would otherwise carry them alone.

**This release owes `skills/tp/REFERENCE.md` an update.** Its `next_action` row in the `tp resume`
table documents the payload as review/audit `{round, unresolved_findings}` (line 98 at the time of
writing); that row must name the three new keys, because `next_action.payload` is an agent-facing
contract and not an internal — see Non-Goal 2.

## 3. A refused invocation writes no state

`tp audit <spec> --role <unknown>` refuses with exit 2 and **creates a state directory anyway**.
Reproduced against a binary built from the tree, in a fresh git repo holding only `spec/demo.md` and
`main.go`, with no `.tp-review` anywhere and no `.tp/auditors/` corpus:

```
$ ls spec/.tp-review/demo/
ls: cannot access 'spec/.tp-review/demo': No such file or directory
$ tp audit spec/demo.md --role no-such-role --affected-files main.go
{"error":"unknown role: no-such-role","code":2,"hint":"this invocation emits: security, maintainability-conventions; skipped this round: spec-coverage (no-checklist-items)"}
$ echo $?
2
$ ls spec/.tp-review/demo/
snapshot-audit-round-1.md
```

The block is pasted from a run rather than abridged. Its role names come from the **embedded default
corpus**; in a checkout with tp's own corpus it names those instead, so the hint is no fixed string.

**"Validate earlier" is not structurally closed, and the reason first given here was wrong.** The
citations hold: `applyRoleFilter` (`internal/cli/rolefilter.go:136`) needs the emitted and skipped
role lists, which exist only after the prompts are generated (`internal/cli/audit.go:355`), so the
refusal at `:379` cannot precede the emission. But a *different*, earlier guard can, and ground round
1 (`u30`) built one: placed before `loadAuditSpec` and resting on `engine.RoleIsRecognised`, it
compiles, `go test ./internal/cli ./internal/engine` is green, the unknown role exits 2 with no state
written, and a valid `--role` still exits 0. The tree says so itself at
`internal/cli/audit.go:375-378` — the classification "is unaffected, since classifyRole falls through
to engine.RoleIsRecognised, which reads the corpus directly. The diagnostic is what the ordering
buys." That diagnostic is the real cost: an early guard trades the hint above for a bare refusal and
repairs the `--role` branch alone, while moving the write keeps the hint and covers every refusal
ahead of it.

**The snapshot write moves**, from `loadAuditSpec` (`internal/cli/audit.go:518`, which writes it while
merely *loading* the spec) to just past the role filter — the last point at which a refusal can still
occur before the payload is assembled. `loadAuditSpec` returns the bytes rather than writing them.

**That site is not the point at which tp knows it emits a round, and the difference is a gap this
release does not close.** Measured against the same binary: `--role spec-coverage` on a spec with no
checklist items for that role exits **0** with `prompts: []` and a `no-checklist-items` skip, writes
`snapshot-audit-round-1.md`, and `--status` then reports `in_flight_round: 1` — a round in flight
about nothing. The point tp knows a round was emitted is where `len(prompts) > 0` is known, one step
further on; `HEAD` does this too, so it is pre-existing rather than a regression, and closing it would
change what a zero-prompt emission does, which §1's bar excludes.

**The bytes must be the pre-blanking spec.** `engine.BlankFrontmatter` runs immediately after the
current write site, so they are copied before that call rather than re-read at the new one.

**Why it matters more than a stray file.** A round on disk because someone mistyped a flag is a round
about nothing, `--status` reports a directory's existence, and a typo is the likeliest cause.

## 4. An accepted audit finding can clear the round

**Measured at `HEAD`.** A `FAIL` row dispositioned `wontfix` with evidence, written into the round's
own **recorded** file — not into the merged input, which is the mistake `skills/tp/SKILL.md` used to
document — changes nothing:

```
$ tp audit .tp-review/alpha/audit-round-1.ndjson --resolve spec-coverage:i1 wontfix "accepted, out of scope"
resolved audit row spec-coverage:i1 as wontfix
$ tp audit alpha.md --status
{'consecutive_clean': 0, 'converged': False, 'spec_coverage_clean_rounds': 0}
role_streaks: [{'role': 'spec-coverage', 'consecutive_clean': 0, 'open': 1}]
rounds     : [(1, clean=False, findings=1)]
```

The same disposition on the review side takes `consecutive_clean` from 0 to **1**, because
`engine.ReviewRoundClean` recomputes a recorded round live from its rows and drops
`wontfix`/`duplicate` carrying evidence. Audit has no counterpart: `clean` is stamped at record and
`auditRoundOpenByRole` (`internal/engine/rolestreaks.go:128`) reads `AuditRowIsPass` alone, never
`resolved`.

**So on the audit side there is no way to accept a finding.** The only exits are repairing it or
destroying the record, and a cycle that has decided to ship over a known, justified finding cannot say
so. That is the mechanism behind the operator's complaint that cycles drag on, and it pairs with §7's
half: on the review side the channel works and is invisible; on the audit side it is visible and does
nothing.

**The surviving set is computed live; the policy is not.** `AuditRoundClean(specPath, entry)` reads the
round's recorded rows, drops every row closed `wontfix` or `duplicate` **with evidence**, and grades
what remains. `fixed` does not close a round — a repair is a claim about the code and the next round is
what tests it — which is the rule §7 states for review and is unchanged here.

**This preserves `spec/0.37.0.md` §2's reason and changes its letter, and the distinction is the whole
design.** §2 stamped `clean` so that a round could not be re-graded under a policy adopted after it was
recorded. That is about **which policy grades**, not about **which rows are alive**. Dispositions are
the second question, so the stamp keeps its job: the round entry records the `audit_converge_on` in
force when it was recorded, and `AuditRoundClean` grades the surviving rows under **that** value,
re-read never. A round recorded under `all` stays graded under `all` however the project is configured
later.

### 4.1 This adds a recorded field, and §1's bar is amended rather than stretched

§1 says a defect qualifies only if it "needs no new recorded field." **This one does**: the round entry
(`engine.ReviewRound`, shared by both phases) gains the `audit_converge_on` in force at record time.
There is no design that both keeps §2's reason and adds no field — grading the surviving rows requires
a policy, and the only policies available are the one in force now (which §2 forbids) and one that was
written down.

Two candidates were rejected rather than overlooked. **Grading live under the current policy** is what
review does and it needs no field, but it is exactly what §2 refused for audit, and "review does it"
is not an argument about audit. **Leaving `clean` alone and fixing only `open`/`role_streaks`** needs
no field either and was the first draft here; it is refused because it makes the two disagree — a round
would report `open: 0` and `clean: false`, and `--check` reads the second.

So the bar moves, explicitly: this release adds **one** field to a recorded entry, and nothing else.
Saying so here is cheaper than discovering it in review — `spec/0.37.0.md`'s §7 row 13 spent four audit
rounds on a requirement that was wrong rather than on an implementation that was.

## 5. One predicate, not two

`open`, `role_streaks`, `spec_coverage_clean_rounds` and `--check` must all be derived from the
function §4 introduces. `parseAuditRows` already requires this of its callers in prose; two predicates
are two things to drift, and the drift is invisible because both produce plausible integers.

`next_action` follows the same set: when every non-`PASS` row in the latest round carries a
disposition, it says the round is disposed and the next step is to re-audit — not "address the
findings, then re-audit", which is what a reader is told today after resolving everything.

## 6. tp names every file it writes

Three surfaces write a file and do not say which:

| surface | today | after |
|---|---|---|
| `tp review\|audit <spec> --record <f>` | `{round, findings, clean, …}` | the same plus **`file`**, the recorded round's path |
| `tp set --workflow <k>=<v>` | `{"updated": {…}}` | the same plus **`file`** |

The `--project` branch of `tp set --workflow` already prints both the path it wrote and a warning when
the value is shadowed; the task branch prints neither, and a field report traced a `checks` value
silently landing in **another spec's** task file to exactly that asymmetry.

`--record`'s `file` is what makes §7 actionable: the disposition belongs in the file `--record` wrote,
and without the key a reader has to know the path shape `spec/.tp-review/<base>/<phase>-round-<N>.ndjson`
by heart.

## 7. `--resolve` is a command, and the emitted instruction says so

A field cycle ran **8 review rounds and recorded 584 findings with 0 dispositions**. `--resolve` was
never called, in either spelling. The emitted `instruction` is the reason:

```
For each prompt, spawn a sub-agent via the Agent tool. Merge findings (tp review --merge), verify and
resolve them, then record the round: tp review <spec> --record <findings.ndjson>. Repeat until
tp review <spec> --status --check exits 0.
```

`--merge`, `--record` and `--status --check` are named as commands. **`resolve` is the one step in the
sentence that is a bare verb, and it is the one step that is also a flag.** Its author read it as
ordinary English for eight rounds, and the document grew 852 → 1547 lines because editing the spec was
the only exit a finding appeared to have.

The line names `tp <phase> <findings> --resolve <idx> <fixed|wontfix|duplicate> [evidence]` alongside
the other three. And `--help` gains the usage line: today the flag's description carries the selector
shape but the positional order appears only after you trigger the error.

## 8. Also shipped in this release

These are already committed and are listed so the release notes and the tree agree. They are fence
repairs, measured by running the hooks rather than reading them:

- **Both write fences read the path argument `mcp__codedbpro__replace` actually sends.** Both matchers
  name the tool; both extracted only `file_path`/`notebook_path`/`file`, and replace names its target
  `path` or `paths`. Measured on a fenced path: `Write` and `create` exit 2, `replace` exited **0**
  under both spellings. The role allowlist had the same hole, where failing open lets a role unit
  rewrite the spec or another role's round.
- **`mcp__codedbpro__batch` reaches both fences.** It carries a nested write while the tool name on the
  call is `batch`, and neither matcher named it — so a batched write was never routed to either hook at
  all. The scripts were already correct: handed a batch payload directly, the deny script exits 2.
  Only the matchers were short, which made this a wider hole than the first — `replace` was at least
  called.
- The plugin manifest is bumped so both reach installed users, and the session-start test's
  near-boundary version is derived from the minimum instead of pinned, after a literal `v1.0.0` in it
  went stale on the same bump.

## 9. Non-Goals

1. **No third pre-measured defect.** §1 names the two that were held back and why. §§4–7 are a
   different class: they close what two field reports measured, and each names the report line it
   closes.
2. **No new flag and no config.** §2's fix is internal. Two things are not: §2.1 adds three keys to
   `next_action.payload`, an agent-facing contract documented at `skills/tp/REFERENCE.md:98`; and §4
   adds one field to a recorded round entry. §4.1 states that the bar moved and why no design avoids
   it. Nothing here adds a flag, a config key or a command.
3. **One convergence change, scoped to one signal.** §4 changes which rows survive into the audit
   grade, and nothing else: not what a round records beyond the stamped policy, not the panel, not the
   `spec_hash`, not the review side. §2 changes what a driver is told and §3 changes when a file is
   written; neither touches convergence. **This makes the release loop-class rather than tool-class**,
   so budget it as such — the corpus median is ~23 rounds for a release that changes a convergence
   signal against ~11 for one that does not. The stopping rule is stated up front: if the finding count
   does not fall after a cut, the problem is the loop and not the document.
4. **Not the review side's discoverability, beyond the emitted string.** §7 fixes what tp *says*. The
   deeper half — that `--resolve` writes to whichever file it is handed, and a disposition in the
   merged input reaches nothing — was fixed in `skills/tp/SKILL.md` and needs no code here.
4. **No repair of past state.** `unresolved_findings` is computed on read, so every recorded round
   reports correctly the moment this ships. A state directory an earlier refusal created is
   indistinguishable from a legitimately emitted round that was never recorded, and is left alone.
5. **The review phase's count is unchanged, and the reason is the corpus rather than the code.** §2's
   finding clause is vacuous over every review row that exists — `0` of `4,552` rows in every
   `review-*.ndjson` under `spec/.tp-review/` carries a `status` key, from
   `python3 -c 'import json,glob;rows=[json.loads(l) for f in glob.glob("spec/.tp-review/*/review-*.ndjson") for l in open(f) if l.strip()];print(len(rows), sum(1 for r in rows if "status" in r))'`.
   It is **not** vacuous by construction: `internal/cli/review_record.go` validates no key set, so a
   review row carrying `status: "PASS"` would be accepted today and would stop counting under §2's
   clause. This release does not make the vacuity structural, and test row 4 measures the corpus
   behaviour rather than a guarantee.

## 10. Tests

Every row derives from a numbered decision and names a mutant that must fail it. Only row 1 rests on a
committed artifact — v0.37.0's round 7, read at test time with its own properties asserted rather than
assumed. Row 3's mutant cites the recorded disposition corpus, but its assertion is over a built row;
the rest are stated over fixtures the test builds, and row 7's "refusal set" names nothing enumerable
in the tree — see the note under the table.

**Rows 1, 2, 3, 5 and 8 have been watched red against `HEAD` and green after the fix, with row 4 green
in both states**, and `go test ./... -count=1` green throughout. That was run twice: by this file's
author (`internal/cli` 53.7s, `internal/engine` 30.5s) and independently in ground round 1 (47.4s /
27.7s). The difference is machine load — `CLAUDE.md` records this suite as load-sensitive.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | v0.37.0's real round 7 as a fixture — 106 rows, 3 non-`PASS`, all 3 dispositioned — reports **0** | the shipped `!= "wontfix"` loop, which returns 103 |
| 2 | §2 *finding* | an audit round of N `PASS` rows and zero findings reports 0 for every N | count rows rather than findings, making the number a function of checklist size |
| 3 | §2 *fixed* | a non-`PASS` row with `resolved.status: "fixed"` does not count | keep `wontfix` as the only closing value, leaving thirteen of every fourteen recorded dispositions inert — §2 carries the derivation |
| 4 | §2 *review* | a review round's count is byte-identical before and after | apply the `PASS` filter to review rows, which carry no `status` and would all be dropped |
| 5 | §3 | after a `--role` refusal in a repo with no state directory, none exists | the shipped order, which writes the snapshot first |
| 6 | §3 *scope* | a `--role` invocation that **emits at least one prompt** still writes its snapshot, and the bytes equal the spec **before** frontmatter blanking | drop the write instead of moving it, or move it after `BlankFrontmatter` |
| 7 | §3 *other refusals* | the same holds for every argument tp rejects before emitting — asserted over the refusal set | fix the `--role` branch alone, leaving every sibling refusal writing state |
| 8 | §2.1 | the payload carries `rows_recorded`, `findings_total` and `findings_closed`, and `unresolved_findings == findings_total - findings_closed` holds on every fixture in this table | emit `rows_recorded` and `findings_closed` alone — measured, that pair leaves the shipped loop's 103 arithmetically consistent with a 106-row round; or compute a counter from a second read of the round, which can disagree with the count it explains |
| 9 | §4 | a one-`FAIL` audit round, resolved `wontfix` with evidence in its **recorded** file, takes `consecutive_clean` 0 → 1 and `role_streaks[].open` 1 → 0 | the shipped stamp, which returns `clean: false` and `open: 1` — the transcript in §4 is this mutant's output |
| 10 | §4 *fixed* | the same row resolved `fixed` does **not** clear the round | close on any disposition, which would let a repair certify itself instead of leaving the next round to test it |
| 11 | §4 *evidence* | `wontfix` with an empty or absent evidence string does not clear the round | drop the evidence requirement, making acceptance free |
| 12 | §4 *stamped policy* | a round recorded under `audit_converge_on: all` still grades under `all` after the project is reconfigured to `blocking`, and the reverse | read the policy live at `--status` time — the mutant that looks correct on a project whose policy never changes, which is every fixture that does not deliberately change it |
| 13 | §5 | `open`, `role_streaks`, `spec_coverage_clean_rounds` and `--check` agree with `clean` on a round with one disposed and one open finding | give `open` its own predicate; measured over the same fixture the two disagree while both stay plausible |
| 14 | §5 *next_action* | with every non-`PASS` row disposed, `next_action` names re-auditing rather than addressing findings | leave the string static, which is what a reader is told today after resolving everything |
| 15 | §6 | both `--record` payloads and `tp set --workflow`'s task branch carry `file`, and the path each names is the file that changed on disk | assert the key's presence alone — a constant string passes that and names nothing |
| 16 | §7 | the emitted `instruction` names `tp <phase> <findings> --resolve <idx> <status>` as a command, and `--help`'s usage line carries the positional order | keep `resolve` as a bare verb; the field measurement (584 findings, 0 dispositions over 8 rounds) is what this row is worth |

**Row 7 is quantified over the refusal set deliberately, and it guards against regression rather than
sweeping extant siblings.** §3's transcript is one refusal, chosen because it is the one that was
reported; a fix scoped to it passes rows 5 and 6 and leaves the class open. Measured at `HEAD` the
class has exactly one open member — an unreadable `--findings` path (exit 3), a nonexistent
`--affected-files` path (exit 3) and an unsafe `--base` (exit 2) all abort before `loadAuditSpec` and
already write nothing. And "the refusal set" is not enumerable anywhere in tp: the exit sites are
spread across `audit.go`, `rolefilter.go` and `role_panel.go`, so the test hand-builds its list, which
is the shape `CLAUDE.md` calls a quantifier you did not count. Both facts belong in the row, because
together they say what it is worth — a later release adding a refusal ahead of the write cannot
silently reopen the class.
