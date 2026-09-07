# tp v0.37.1 — Two defects with fixes already measured

> **This file is decisions, and it is a patch release.** Both defects below were reproduced by a test
> that failed against `HEAD`, fixed, and re-run green with the full suite passing. That this happened
> *before* the file was written is the author's own report and nothing in the tree can settle it — no
> test for either fix exists at `HEAD`, so that run left no committed trace. What is checkable is that
> it reproduces: ground round 1 (`spec/.tp-review/0.37.1/ground-round-1.ndjson`) built both fixes in an
> `rsync` copy, wrote the probes before touching the code, and watched five of the six §5 rows it
> covered red at `HEAD` and green after — with row 4 green in both states, exactly as §5 predicts.
> Nothing here is a behaviour change beyond the defect; the two minor releases that specified these
> items keep everything else they carry.

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

## 4. Non-Goals

1. **No third defect.** §1 names the two that were held back and why.
2. **No new recorded field, no new flag, no config.** §2's fix is internal. §2.1 is not: it adds three
   keys to `next_action.payload`, an agent-facing JSON contract documented at
   `skills/tp/REFERENCE.md:98`, and that file is owed an update (§2.1). What it adds nothing to is a
   *recorded* round file or `state.json` — which is the test §1's bar actually states.
3. **No convergence change.** `clean`, `consecutive_clean` and `--check` read neither field; §2 changes
   what a driver is told and §3 changes when a file is written.
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

## 5. Tests

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
