# 1.0.1 — measurements

Forensics for `spec/backlog/00-a-finding-can-leave-a-round.md`. **This file is not a spec and `tp ground` never grades it.** It exists
because the grading of that spec measured what the forensics cost: 149 floor units over 376 lines, one
unit per 2.5 lines, and a round that spent 718k tokens across three graders. Counting the graded rows
by `kind` says where the cost went and where the value did:

| `kind` | units | findings | rate |
|---|---|---|---|
| `mechanism` | 16 | 8 | **50%** |
| `corpus` | 22 | 8 | 36% |
| `behaviour` | 39 | 10 | 26% |
| `document` | 29 | 7 | 24% |
| `code-structure` | 16 | 2 | 12% |

Counting rule: the 125 rows recorded in `spec/.tp-review/1.0.1/ground-round-2.ndjson`; a *finding* is a
row whose verdict is neither `PASS` nor `NOT-A-CLAIM`.

`mechanism` — a design or universal claim — is 13% of the graded rows and 22% of the findings.
`corpus` — a figure re-derived from the tree — matches it on rate and not on worth: *"four days later"
was two days*, *"eight of ten" does not reach eight by its stated derivation*. Both are correct
findings about sentences that, unwritten, would have offered nothing to find. **So the expense is not
the apparatus; it is the number inside the apparatus,** because every figure obliges a grader to run a
command. That is why the material below sits here and the spec keeps decisions.

---

## 1. Why a patch rather than a reordering

tp has shipped ten patch releases —
`git tag -l 'v*' | sort -V | grep -E '^v[0-9]+\.[0-9]+\.[1-9][0-9]*$'` — and eight of the ten are
headed as defect repairs, read with `gh release view <tag> --json body`: v0.3.1 (`### Bug Fix`),
v0.12.2 (`## v0.12.2 — Bugfix`), v0.31.1, v0.31.2, v0.34.1, v0.34.2, v0.35.1 and v0.35.2. The other two
are v0.28.1 (docs) and v0.1.1 (a bare changelog).

**Ground round 2 corrected the derivation and the conclusion survived it.** Reading *heading lines*
alone reaches five, or seven if v0.31.1/v0.31.2 are counted loosely; **v0.35.2's headings are
`## How it was found` / `## The two halves` / `## What is deliberately still allowed`, none of which
names a defect** — only its opening paragraph does. The honest form is "heading lines or opening
paragraph", and the count is eight on the bodies.

**The argument this section used to make is void.** It said reordering the pending minors would be
another renumbering, and that two held-back candidates "stay in their minors". Commit `3aa992fd` moved
every pending spec to `spec/backlog/` **without version numbers**, so there are no minors to move; the
two candidates are now `spec/backlog/01-checklist-covers-what-changed.md` and
`spec/backlog/02a-round-knows-its-panel.md`. What survives is the narrow bar, and §4.1 of the spec
records the one place it moved.

## 2. `unresolved_findings`

`roundPayload` (`internal/engine/resumepayload.go`) counts every row in the last round whose
`resolved.status` is not `"wontfix"`. On the audit phase every checklist item is a row, so a clean
round of 106 items reports 106 minus its dispositions.

### 2.1 The v0.37.0 round-7 table, and the tree state it needs

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

### 2.2 The complement, by set identity

```
python3 -c 'import json;rows=[json.loads(l) for l in open("spec/.tp-review/0.37.0/audit-round-7.ndjson") if l.strip()];counted={id(r) for r in rows if ((r.get("resolved") or {}).get("status") if isinstance(r.get("resolved"),dict) else None)!="wontfix"};print(counted=={id(r) for r in rows if r.get("status")=="PASS"})'
```

returns `True` at `HEAD`: every row that was a finding was excluded and every row that was not was
included.

### 2.3 Why `fixed` must close a row

`fixed` is roughly thirteen of every fourteen recorded dispositions. Derive it — do not read a figure
here, both counts move with every recorded round and the share is what holds:

```
python3 -c 'import json,glob,collections;c=collections.Counter(json.loads(l)["resolved"]["status"] for p in ("spec/.tp-review/*/*.ndjson","spec/backlog/.tp-review/*/*.ndjson") for f in glob.glob(p) for l in open(f) if l.strip() and isinstance(json.loads(l).get("resolved"),dict));print(c)'
```

**Both globs are required**; the backlog move put twenty-one specs' round directories outside the
first. `CLAUDE.md` carries the table of which figures that actually changes.

### 2.4 Why three counters and not two

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

## 3. The refused `--role` invocation

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
those instead, so it is no fixed string.

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

## 4. The third design, found twice

§4.1 of the spec once claimed *"there is no design that both keeps §2's reason and adds no field."*
**Two independent designs refuted it**, and recording both is the point: one came from a peer session
asked to try, the other from the grading unit whose brief told it to construct a counter-example. n=2.

**Design A — re-stamp at `--resolve`.** After writing the disposition into the recorded round file,
`--resolve` re-stamps that round's `state.json` entry under one rule: if every non-`PASS` row is
disposed (`wontfix`/`duplicate` with evidence) then `Clean=true`, otherwise the stamp stands. No reader
changes, no policy is read, no field is added. `spec/0.37.0.md` §2 already anticipates an operator
write changing the stamp — *"An operator who wants it undone re-records the round."*

**Design B — recover the policy from the stamp.** Because `all` implies `blocking`, exactly three
cases arise over the recorded rows: no non-`PASS` row (clean under both); the two policies disagreeing
(so the stored `clean` identifies which was in force, uniquely); both false (unrecoverable, so grade
survivors under `all`, the stricter candidate). Built and run: on §4's own case the probe reports
`consecutive_clean 1`, `open 0` where the `HEAD` binary reports zeros and `open 1`; a round recorded
under `all` and then read with `blocking` adopted stays at 0, and one recorded under `blocking` then
relaxed stays at 1.

**Both are conservative in one enumerable case**, which is what the spec now states instead of the
universal. Design B's: a round stamped not-clean where both policies said not-clean, whose disposals
remove every blocking row but leave a non-blocking one, reports not-clean where `blocking` would say
clean. Unreachable under the built-in default, since `audit_converge_on` defaults to `all` and
recovery is exact there. Design A's: under `blocking`, a partial acceptance does not clear; the exit
is `--resolve-all` with a shared justification.

**And §4 reverses a stated refusal rather than filling a gap.** `spec/0.37.0.md` §2 says outright:
*"Surviving is deliberately not the word: it would imply a disposition that removes a row from the
count, which §5 declines to create."* The spec says so in its own words now.

**A third exit exists that the spec once denied.** "The only exits are repairing it or destroying the
record" is false: recording the **same rows** again under a different `audit_converge_on` moves the
verdict. Measured — round 1 `clean False`, set `blocking`, re-record → round 2 `clean True`, round 1
intact. Three limits make it sharper rather than weaker: it reaches only non-`error` severities,
`blocking` is human-only under `TP_UNATTENDED`, and it costs a round of budget. **It also moved
`clean` while leaving `role_streaks` at `{0, open 1}`** — §5's two-predicate split, visible on a live
tree.

## 5. What the emitted instruction actually says

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

Cause 3 also contradicts the spec's own §7 usage line, which offered `fixed` among the dispositions.

## 6. Also shipped — the narrative

The acceptance rows for these live in the spec; what they replaced is here.

Both write fences named `mcp__codedbpro__replace` in their matcher and extracted only
`file_path`/`notebook_path`/`file`; replace names its target `path` or `paths`. Measured on a fenced
path against the pre-fix hooks (`dc80e766^`, `a05947a1^`): `Write` and `create` exit 2, `replace`
exits **0** under both spellings; the role allowlist exits 0 on a spec write. Post-fix all exit 2 with
an unfenced control at 0.

`mcp__codedbpro__batch` reached neither matcher, so a batched write was never routed to either hook.
The scripts were already correct — handed a batch payload directly, the deny script exits 2 — so only
the matchers were short, which made it a wider hole than the first: `replace` was at least called.

**The list is not everything that shipped since `v1.0.0`.** Also in the tree:
`scripts/clean-emissions.sh`, `abf69d4f` (`configresolve.go`), `fa68051b` (test parallelisation across
~200 files), four README diagram commits and five `SKILL.md` commits. The spec's §8 covers the fence
repairs because the release notes must name them; it does not claim to be a changelog.

## 7. Suite timings

The figures once quoted in the spec's test section predate `fa68051b` (test parallelisation,
2026-09-04, four hours after ground round 1 ran). At `HEAD` in a clean clone: `internal/cli`
16.6–18.1s, `internal/engine` 36.2–36.7s, full suite green. `CLAUDE.md` records this suite as
load-sensitive, which explains the spread between runs but not the gap a reader would now see against
a stale figure.
