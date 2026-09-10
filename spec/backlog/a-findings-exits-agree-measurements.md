# a-findings-exits-agree — measurements

Supplemental material for `a-findings-exits-agree.md`; the spec stands without it. **This file is not
a spec and `tp ground` never grades it.**

---

## `unresolved_findings` on v0.37.0's round 7

`roundPayload` (`internal/engine/resumepayload.go`) counts every row in the last round whose
`resolved.status` is not `"wontfix"`. On the audit phase every checklist item is a row, so a clean
round of 106 items reports 106 minus its dispositions.

### The round-7 table, and the tree state it needs

| | | derived from `audit-round-7.ndjson` |
|---|---|---|
| rows recorded | 106 | count of non-blank lines |
| rows dispositioned `wontfix` | 3 | `Counter(resolved.status)` |
| `unresolved_findings` reports | 103 | `tp resume spec/0.37.0.md --json` |
| rows actually non-`PASS` | 3 | `Counter(status)` → `{PASS: 103, PARTIAL: 3}` |

**The table mixes two tree states and the mixture is load-bearing** — ground round 2 caught this. The
round is named at its tag; the rows are read at `HEAD`. At `v0.37.0` the file carries
`{None: 106}` — **zero** dispositions. The three `wontfix` objects were written after the tag by
`9de9a7b4`:

```
git show v0.37.0:spec/.tp-review/0.37.0/audit-round-7.ndjson | shasum -a 256   # 7e6ccb5d…
shasum -a 256 spec/.tp-review/0.37.0/audit-round-7.ndjson                      # a263decb…
```

So **against the round as shipped, `unresolved_findings` would have reported 106, not 103**, and the
complement property came into being three commits after the tag. The defect is real either way; the
sentence must say *"the round as it stands at `HEAD`"*.

### The complement, by set identity

```
python3 -c 'import json;rows=[json.loads(l) for l in open("spec/.tp-review/0.37.0/audit-round-7.ndjson") if l.strip()];counted={id(r) for r in rows if ((r.get("resolved") or {}).get("status") if isinstance(r.get("resolved"),dict) else None)!="wontfix"};print(counted=={id(r) for r in rows if r.get("status")=="PASS"})'
```

returns `True` at `HEAD`: every row that was a finding was excluded and every row that was not was
included.

### Why `fixed` must close a row

`fixed` is roughly thirteen of every fourteen recorded dispositions. Derive it — do not read a figure
here, both counts move with every recorded round and the share is what holds:

```
python3 -c 'import json,glob,collections;c=collections.Counter(json.loads(l)["resolved"]["status"] for p in ("spec/.tp-review/*/*.ndjson","spec/backlog/.tp-review/*/*.ndjson") for f in glob.glob(p) for l in open(f) if l.strip() and isinstance(json.loads(l).get("resolved"),dict));print(c)'
```

**Both globs are required**; the backlog move put twenty-one specs' round directories outside the
first. `CLAUDE.md` carries the table of which figures that actually changes. `duplicate` is at zero in
the recorded corpus; **the glob is part of the claim** — a tree-wide walk over every `*.ndjson`
returns a larger `fixed` count, because it picks up untracked working files, and the same zero.

## Why three counters and not two

With only `rows_recorded` and `findings_closed`, a defective round and a correct one are
indistinguishable. Ground round 1 built the mechanism and emitted
`{unresolved_findings: 103, rows_recorded: 106, findings_closed: 3}` **legitimately**, from a 106-row
round whose rows carry no `status` — the shape of every recorded review row — with three dispositioned
`wontfix`. `103 + 3 = 106` is self-consistent, so a reader holding those three numbers learns nothing.

`findings_total` replaces the plausibility judgement with an identity:
`unresolved_findings == findings_total - findings_closed`. The shipped loop violates it on round 7
(103 against `3 - 3 = 0`) while the legitimate no-`status` round satisfies it (103 against
`106 - 3 = 103`).

**Ground round 2 found a defect in how this was tested, not in the argument.** Test row 8 was
*reworded after round 1*: round 1 built and watched **two** counters, and `findings_total` was never
emitted, asserted or watched in either colour. So a preamble claiming row 8 was watched red credits
the new row with a run performed on the row it replaces — on the very deficiency the rewrite repairs.
The spec's table now says which rows were watched and which were not.

## The refused `--role` invocation

Reproduced against a binary built from the tree, in a fresh repo holding only `spec/demo.md` and
`main.go`:

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

Role names come from the embedded default corpus; in a checkout with tp's own corpus the hint names
those instead, so it is no fixed string. Re-measured live at the head this file was split at:
`tp audit spec.md --role bogus` exits 2 and leaves `snapshot-audit-round-1.md`; the refusal is at
`internal/cli/audit.go:379`, the write at `internal/cli/audit.go:518` inside `loadAuditSpec`.

**An earlier guard is buildable and was built** (ground round 1, `u53`, rebuilt independently in round
2): placed before `loadAuditSpec` and resting on `engine.RoleIsRecognised`, it compiles,
`go test ./internal/cli ./internal/engine` is green, the unknown role exits 2 with no state written,
and a valid `--role` still exits 0. It is not what ships: an early guard trades the diagnostic hint
for a bare refusal and repairs the `--role` branch alone, while moving the write keeps the hint and
covers every refusal ahead of it.

**A gap this release does not close.** `--role spec-coverage` on a spec with no checklist items for
that role exits **0** with `prompts: []`, writes the snapshot, and `--status` then reports
`in_flight_round: 1` — a round in flight about nothing. `HEAD` does this too, so it is pre-existing.

**A measurement trap, recorded because it nearly produced a false result.** An `rsync` copy made with
`--exclude 'spec/.tp-review'` fails **11** tests in `./internal/cli`, all the standalone `regression`
perspective, which refuses without a state directory holding a recorded round. The copy must carry
`spec/.tp-review`.

## Two predicates disagree about `duplicate`

Measured at `5058fc99` on a one-finding round resolved `duplicate` with evidence: `tp review <spec>
--status` reports the round clean with `consecutive_clean: 1`, while `tp resume` in the same tree
reports `next_action.payload.unresolved_findings: 1` and the summary *"run review round 2 (1
unresolved from the previous round)"*. One round, two surfaces, opposite answers — and neither is
reading the other's predicate.

## What the emitted instruction actually says

```
For each prompt, spawn a sub-agent via the Agent tool. Merge findings (tp review --merge), verify and
resolve them, then record the round: tp review <spec> --record <findings.ndjson>. Repeat until
tp review <spec> --status --check exits 0.
```

plus a fourth sentence appended unconditionally unless `--spec-inline`:
*"Read the spec at <path> before processing each prompt."* (two further sentences under
`--perspective regression` and registered checks). Only `--spec-inline` returns exactly the three
sentences above.

**The "one bare verb" claim was false and is withdrawn.** The conjunct is *"verify and resolve them"*
— **two** bare verbs — and `--verify` is a flag too, listed beside `--resolve`/`--resolve-all` under
`Modes (mutually exclusive)` and documented in `skills/tp/SKILL.md`. Naming `--resolve` while leaving
`verify` bare reproduces the ambiguity for `--verify`, so the repair names both.

**The 584-findings/0-dispositions figures have no second source in this corpus.** No spec here has 8
review rounds or 584 findings; `skills/tp/SKILL.md` carries the same figures from the same field
report. Graded `UNVERIFIABLE`, and the spec cites the report rather than asserting the number.

**Four ranked causes for that cycle, of which the wording is only one.** Two carry committed evidence
and the spec must not claim the wording is the mechanism:

1. The emitted wording (this section).
2. **`skills/tp/SKILL.md` step 5 told the reader to resolve into `merged.ndjson`** until `b853fb46`,
   one commit before this spec was written — and a disposition written there records nothing.
3. **`spec/0.31.0.md` §3.5 makes `--record` reject a findings file carrying `fixed` rows, exit 1**, so
   a cycle disposing everything `fixed` records zero dispositions **by construction**.
4. Reader inattention.

Cause 3 also contradicts the spec's earlier §7 usage line, which offered `fixed` among the dispositions.

## Routed here at the 2026-09-08 re-verification

Two items from the candidates files land on this spec's subject. They are recorded in this sidecar;
the spec body is not edited.

- **`tp review --resolve-all --severity <sev>` does not exist.** `internal/cli/review.go` registers
  `--resolve-all` as a bare bool (`cmd.Flags().BoolVar(&resolveAllMode, "resolve-all", false, …)`)
  and rejects modifier flags in that mode, so the only way to accept a subset is one `--resolve` per
  index. This bears on the exit `a-finding-can-leave-an-audit-round.md` proposes: that exit rests on
  a shared-justification `--resolve-all`, and a severity filter is what makes it cheap enough to use
  — accept the advisory rows in one call, leave the blocking ones open. Source:
  `spec/0.33.0-candidates.md` item 6.
- **The review-side category enum is unvalidated at every sink.** The audit side validates through
  `internal/engine/audit_category.go`; the review side carries its enum only as a prompt literal in
  `internal/cli/review.go`, `findingFormat`, and nothing at `--merge`, `--record` or `--status`
  checks what comes back against it. The design question — whether one vocabulary should be shared
  across the two prompts — is registered in `spec/undecided.md`, *Cross-site key agreement in the
  review prompt*; what belongs to this spec is that two exits disagree about the same field. Source:
  `spec/0.35.0-candidates.md` item 13.

## Decided at the 2026-09-08 decision pass

Two entries of `spec/undecided.md` were decided onto this spec.

**From *Cross-site key agreement in the review prompt*.** Decided: the review prompt renders **one
set at all three key-naming sites, from one Go constant**. The set is the record-required four —
`severity`, `finding`, `location`, `evidence` — plus `role` and `class`, both **mandatory**: `class`
is the dedup key, so *Optional* was a fiction. `category` is kept, because `by_category` ships. The
review-side `category` enum is **validated at the record sink** the way the audit side already is, as
a **warning**-severity refusal that names the row. The audit phase keeps its own status-based
vocabulary, stated once — `outputContractInstruction` is shared between the phases, so the decision
states the two vocabularies separately rather than equalising them. This settles the second item under
*Routed here at the 2026-09-08 re-verification* above, whose design question was registered in
`spec/undecided.md`.

**From *A review-side `accepted_blocking`*.** Decided: **one counter**, on the payload §2 of this
spec already rewrites. §2 fixes the shape of `unresolved_findings` and gives it three siblings bound
by an identity; the accepted-blocking count joins them there rather than arriving on its own. The gap
it closes is measured in `spec/undecided-measurements.md` §From the rows spec — the three-tree probe
at `5058fc99`, where a round whose only finding is a `critical` resolved `wontfix` returns a payload
whose key set is identical to a round recorded from an empty findings file.

## Routed here from v1.1.0's audit round 3 (2026-09-08)

Three items land on this spec's subject — two exits, or two sites, disagreeing about one field. They
are recorded in this sidecar; the spec body is not edited.

- **`findingIdentityKey` slices bytes where its own comment promises characters.**
  `internal/cli/review.go:1262-1273`: the doc comment at :1265 says *"finding_prefix is the first 80
  characters of the finding field"*, and the body is `if len(prefix) > findingPrefixLen { prefix =
  prefix[:findingPrefixLen] }` — `len` and the slice are both **bytes**. So two findings that differ
  well inside the first 80 characters collapse to one whenever the differing text sits past byte 80,
  which any non-ASCII prefix reaches sooner than a reader expects. **Run at `e8477464`** in a scratch
  git repository, three two-row files differing only in the `finding` text, each recorded as round 1
  and the carried-forward count read from round 2's emission:

  | control | first 80 chars | first 80 bytes | `previous_findings` | |
  |---|---|---|---|---|
  | 30 `ş` + 20 `x`, then `ALPHA-tail` / `BETA-tail` | differ | identical | **1** | **wrong** |
  | `A`/`B` + 120 `z` (differs at byte 1) | differ | differ | 2 | correct |
  | 80 `q`, then `ALPHA` / `BETA` | identical | identical | 1 | correct |

  The middle row is what makes the first row evidence rather than a coincidence: an input that
  differs early still separates, so the collapse is the byte boundary and not a broken key. The
  fixture is built so that 30 `ş` (2 bytes each) + 20 `x` is exactly 50 characters and exactly 80
  bytes — the tail is inside the promised character window and outside the actual byte window.
  Pre-existing, not release-created.

  **Two corrections to how this was first written up, both found by running it.** (1) The key does
  **not** drive `--merge` dedup. `runReviewMerge` (`internal/cli/review_merge.go:42`) clusters through
  `clusterMergeFindings`, the `(location, class)` clustering; run against all three files above,
  `tp review --merge` returned `merged_count=1, duplicates_removed=1` for **every** one, including the
  ASCII pair differing at byte 1 — so `--merge` cannot discriminate any of these controls and is the
  wrong surface to measure the bug on. (2) `previous_findings` as reported by
  `--verify --findings` is a raw row count with no dedup at all (`readVerifyFindings` appends
  unconditionally); all three files returned **2** there. The surface that does exercise the key is
  `dedupFindings` (`internal/cli/review.go:1294-1306`), reached from `review.go:622`, `:1309` and
  `:2020` — the last being the carried-forward previous-findings set measured in the table — plus
  `review_report.go`'s cross-round tracking at `:229`, `:358`, `:444` and `:468`. Five call sites, one
  identity predicate, two surfaces; the "same key across merge and report" framing was wrong on its
  merge half.
- **`findings` is an `int` on the audit side and an array on the review side.**
  `internal/cli/audit_merge.go:93` is `"findings": findingsCount, // rows whose status is not PASS`,
  while `tp review --merge --json` emits `findings` as the array of merged finding objects. One key,
  two JSON types, across two commands a caller reaches through the same `--merge --json` spelling —
  and documented nowhere, though `skills/tp/REFERENCE.md:695` already documents the analogous
  `by_severity` collision, so the pattern of documenting such a clash exists and this one was missed.
  Read at `e8477464`.
- **Two output-format constants still promise an empty array where the third promises an empty file.**
  `internal/cli/review.go:1691` and `:1736` both end *"If no changes needed, respond with an empty
  array (just `[]`)"* / *"If no tests needed, respond with an empty array (just `[]`)"*, while `:1797`
  says *"If no issues found, write nothing at all — an empty file is how a role reports a clean
  result."* Harmless today because the guard accepts an empty array, so this is a consistency finding
  rather than a defect: three sibling sites telling a role two different things about the same clean
  result. Read at `e8477464`.

## Rewritten 2026-09-11: what changed in the body, and why

**`fixed` is no longer closed on any counting surface — this reverses *Why `fixed` must close a row*
above.** The earlier body defined *closed* three ways at once: §1 said `wontfix`/`duplicate` with
evidence, §2 added `fixed`, and §5 said every surface reads the sibling spec's surviving set, in which
`fixed` stays open. One definition had to win, and the survey that found the contradiction (2026-09-11)
showed which: the review side's convergence predicate (`reviewFindingResolvedAway`,
`internal/engine/reviewclean.go`) subtracts `wontfix` and `duplicate` with non-empty evidence and
nothing else, and `a-finding-can-leave-an-audit-round.md` grades the audit side the same way. Closing
`fixed` in `tp resume`'s count would make `tp resume` report nothing unresolved for a round `tp review
--status --check` still fails — the one-round-two-answers class the spec exists to remove. The share
argument above (most recorded dispositions are `fixed`) is answered by the new `dispositioned` count,
which includes `fixed`, rather than by the closure predicate.

**Moved out:** the audit `open`/`role_streaks`/`--check`/`next_action` half of the old §5 went to
`a-finding-can-leave-an-audit-round.md` §5, together with its test rows (old rows 9 and 11).
**Absorbed:** `loops-own-state-writes.md` §2 (atomic round-file writes, now §6.3; its probes and the
`tp audit --merge -o` symlink, mode and hardlink measurements stay in
`loops-own-state-writes-measurements.md`) and `round-knows-its-panel.md` §2.2 (unknown `state.json`
keys, now §6.2; its measurements stay in `round-knows-its-panel-measurements.md` under *Three fields
shipped erasable* and *The trigger is a stray `--record`, and what the fix does not buy*).
**Routed on:** the `findingIdentityKey` byte-slicing item above goes to `an-unreadable-file-is-named`,
with the other rune-boundary cut. The other routed extras above (`--resolve-all --severity`, the
review-side category enum, the key-set constant, `accepted_blocking`, the `[]` wording) stay recorded
here and out of the body.

**Test-row watch status.** The earlier table recorded rows 1, 2, 3 and 5 watched red against `HEAD`
and green after a built fix, with row 4 green in both. Renumbered, those are rows 1, 2, 6 and 4; the
old row 3 asserted the reverse of today's `fixed` decision, so its run no longer counts for the
reworded row.

## Field report WB-3155, verified 2026-09-11

A field report (WB-3155), from a project on tp v1.1.1, was checked claim by claim against a binary
built from `18032abe`. Fixtures were built in throwaway git repositories outside this tree; the shapes
are described here because the scratch paths will not survive.

### Surveyed at `HEAD`: the defects the body already carried

- **A clean audit round reports its checklist as unresolved — CONFIRMED.** An audit round recorded
  from three `PASS` rows: `tp resume` reports `next_action.payload.unresolved_findings: 3`. Source:
  `roundPayload`, `internal/engine/resumepayload.go:95-110`, which counts every row whose
  `resolved.status` is not `wontfix`.
- **`duplicate` disagrees between `tp review --status` and `tp resume` — CONFIRMED**, as in *Two
  predicates disagree about `duplicate`* above: `reviewclean.go:32-43` closes `duplicate`,
  `resumepayload.go:105` does not.
- **A refused `--role` still writes a snapshot — CONFIRMED.** `tp audit <spec> --role bogus` exits 2
  and `snapshot-audit-round-1.md` exists afterwards.
- **`--record` names no file — CONFIRMED.** A review `--record` payload's keys are `clean`,
  `consecutive_clean`, `converged`, `findings`, `harness_stale`, `mechanize_candidates`,
  `mechanized_classes`, `next_action`, `required_clean_rounds`, `round`, `stale` — no `file`. (The
  `--status` round entry does carry `file`, as a bare filename.) `tp set --workflow
  review_clean_rounds=3` prints `{"updated": {"review_clean_rounds": 3}}`.

### #16 — "review convergence is unreachable without dispositions, and nothing says so": PARTLY

**What holds.** `tp review --status` carries no disposition count, and the emitted instruction still
says *"Merge findings (tp review --merge), verify and resolve them, then record the round"*
(`internal/cli/review.go:2058`). The `skills/tp/SKILL.md` step that pointed at `merged.ndjson` was
already corrected in `b853fb46` (2026-09-07).

**What the trap actually is: timing.** Reproduced on a one-finding review round, two arms:

| arm | sequence | recorded row's `resolved` | `--status` |
|---|---|---|---|
| before | `--resolve 0 wontfix "<reason>"` on the merge file, then `--record` it | `{status: wontfix, evidence: …}` | `clean: true`, `consecutive_clean: 1` |
| after | `--record` the merge file, then `--resolve 0 wontfix "<reason>"` on it | absent | `clean: false`, `consecutive_clean: 0` |

In the *after* arm the resolve exits 0 and prints `{"file": "m.ndjson", "next_step": "tp review
--verify <spec> --findings m.ndjson", …}` — it names the file it wrote, which is not the recorded
round, and nothing downstream notices.

**Refuted part.** *"Convergence is unreachable"* is overstated: a disposition written into the merge
file before `--record` is carried into the recorded round and clears it. The reporter's cycle lost its
dispositions to the ordering, not to a missing mechanism. What the report is right about is that no
surface shows the difference, which is what `dispositioned` and the instruction's wording fix.

### #17 — "`wontfix` findings still enter the prompt under UNRESOLVED": header CONFIRMED, suppression INTENDED

`buildFindingsSummary` (`internal/cli/review.go:1314-1390`) writes *"UNRESOLVED findings from previous
rounds — DO NOT re-report:"* and lists `[WONTFIX]` rows beneath it. The header is false for those rows.
`duplicate` rows are not listed there — they are counted with `fixed` in the *"RESOLVED (fixed or
duplicate)"* line — so the lie is about `wontfix` alone.

**The ask to stop suppressing accepted rows is not taken**: `spec/undecided.md` *A prior-round section
for `tp review`* closed that question no, and the field report carries no measurement of that entry's
reopen condition. **The "60% of the prompt" figure is overstated as a general claim**: the list is
capped at 50 rows (`review.go:1358`) with an omitted-count line, so its share of a prompt is bounded;
the reporter's percentage came from comparing against a `--no-state` emission and was not re-measured
here.

### #20 — "the carry-forward tells a verification round not to look at fixed findings": REFUTED, with a residue

`fixed` rows are counted as resolved and, at high or critical severity, listed under *"Resolved
high/critical (DO NOT regress)"* (`review.go:1392-1415`); the regression prompt carries previously
fixed findings with their evidence (`internal/cli/review_regression.go:178-193`). The claim that
`fixed` rows sit under the UNRESOLVED header is false.

**The residue, CONFIRMED:** `tp review <spec> --role architect` on a round that would emit the
regression prompt drops it and reports `skipped_roles: []`. The filter keeps exactly the one prompt
whose role matches (`filterReviewPrompts`, `internal/cli/rolefilter.go:154-168`, called at
`review.go:620`) and `skipped_roles` is built before it runs, so the dropped regression prompt is
neither emitted nor listed.

### #21 — "`--record` and `--round` are mutually exclusive, learned only by trying": CONFIRMED, and narrower than the report

`validateModeFlags` (`internal/cli/review.go:463-465`) tests `round != 1` — the flag's value against
its default — not whether the flag was set. So `tp review <spec> --record f.ndjson --round 1` exits 0
and records the next round, while `--round 9` exits 2 with
`{"error":"--record is mutually exclusive with --round","hint":"see 'tp --help' or the subcommand's
'--help' for usage"}`. `tp review --help`'s *Modes (mutually exclusive)* list names `--merge`,
`--resolve`/`--resolve-all`, `--verify` and `--report`, and not `--record` or `--status`, and nowhere
says the recorded round's number is derived from state. The report's wider ask — split `tp review`
into subcommands — is not taken; the mode list is.

### Absorbed from the audit-side items

#29 (a resolve into a file that is not a recorded round) and #30 (`--status` reads no disposition)
are recorded in `a-finding-can-leave-an-audit-round-measurements.md` and in
`audit-records-what-was-graded`; this spec takes only the review-side `dispositioned` count and the
instruction wording they share a cause with.
