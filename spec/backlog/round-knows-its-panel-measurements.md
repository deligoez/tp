# round-knows-its-panel — measurements

Supplemental material for `round-knows-its-panel.md`; the spec stands without it. **This file is not a
spec and `tp ground` never grades it.** The last section is the derivation behind a counter the rows
spec proposed and this spec dropped.

---

## The too-strict direction, derived

v0.36.0 shipped with `--check` at exit 1 while `spec-coverage` was 74/74 PASS in each of rounds
10–13, because a non-conformance role's rows override a clean conformance role. Every non-PASS row in
those four rounds belongs to `maintainability-conventions`, and the round v0.36.0 actually shipped on
carries two of them at `status: FAIL`, `severity: error`, both unresolved — so this is **not** a case
where nobody ever FAILed. Derivation:
`python3 -c "import json;[print(n,[(r.get('role'),r.get('status'),r.get('severity')) for r in (json.loads(l) for l in open('spec/.tp-review/0.36.0/audit-round-%d.ndjson'%n) if l.strip()) if r.get('status')!='PASS']) for n in (10,11,12,13)]"`.
`CLAUDE.md` records the divergence, but its v0.36.0 sentence says *zero open spec-scoped findings*
while *no role held a FAIL* is its **v0.37.0** sentence. An earlier draft of the spec's overview merged
the two, and the merged sentence is false; the source sentence in `CLAUDE.md` is not this file's to fix.

## Why the panel cannot come from the corpus

**§2 changed shape after grounding: the panel is now written at emission, which is a new write on the
emission path.** An earlier draft wrote it at record time and offered a fallback that derived it from
`roles_hash`; both were refuted by running, below. The release's subject, its gate and the field it
puts on the round entry are unchanged.

`roles_hash` hashes the role *files* (`ComputeRolesHash`, `internal/engine/rolehash.go`); the emitted
panel is that set minus frontmatter deactivations, domain mismatches, `no-checklist-items` skips and
`--role` narrowing. Two narrowings were built on a three-role fixture and **both leave `roles_hash`
byte-identical to the unnarrowed corpus's**:

```
spec frontmatter  tp: {audit_roles: {go-safety: {enabled: false}}}
  -> 2 prompts, skipped_roles [{"role":"go-safety","reason":"disabled-by-spec"}], roles_hash unchanged
tp audit <spec> --affected-files main.go --role ax-contract
  -> 1 prompt, skipped_roles [],                                                  roles_hash unchanged
```

`rolehash.go`'s own comment says why the first is invisible — *"a spec-frontmatter override is covered
by spec_hash, not here (no double-count)"*. **The second is invisible to everything**: under `--role`
the narrowed-away roles are named in no field of the emission and in no file on disk, so even the
reason-carrying `skipped_roles` hatch cannot see them. `--role` is v0.36.0's headline feature and this
project's per-role loop is built on it, so a rule deriving the panel from the corpus would judge every
`--role` round permanently un-clean, with nothing to appeal to.

## Three fields shipped erasable

**`ReviewRound` has gained a field three times already, each shipped under this defect; this release
is the first to *fix* it.** `roles_hash` landed in v0.25.0, `id_scheme` in v0.30.0, `harness_note` in
v0.31.0 — derivation: `git log --oneline -S'<the field declaration>' -- internal/engine/reviewstate.go`
for each, then `git tag --contains <sha>`. All three are optional keys on the round entry and all
three have been erasable ever since.

## The trigger is a stray `--record`, and what the fix does not buy

**The trigger is a stray `--record`, and only that.** `SaveReviewState` has exactly two production
callers, `internal/cli/audit_record.go` and `internal/cli/review_record.go`. Measured with the keys
injected: a plain emission, `tp audit --status` and `tp review` all leave them intact; only
`tp <phase> --record` erases them. `CLAUDE.md` warns that the PATH-installed tp lags the repository,
so the live hazard is a stray *recording* invocation from that binary — a narrower and more avoidable
event than "one stray invocation", which is what an earlier draft of this paragraph claimed.

**This fix does not make a field durable against an *older* binary, and claiming it did would be the
section's worst sentence.** A binary predating the fix still marshals the typed struct and drops the
key. Measured: a tp built from tag `v0.37.0` was run over a `state.json` carrying an injected
`expected_roles`, and one `--record` dropped it — the same result as `HEAD`. What the fix buys is
durability against binaries **at or after** this release. That is less than the paragraph above might
suggest and is exactly what the two dependents need: the PATH hazard stays open for as long as any
pre-fix tp is installed anywhere.

**Those two dependents are the emit-time hash release and the reconcile release, and each states the
dependency in its own text.** The emit-time hash's `spec_moved_mid_round` counter and the reconcile
release's `reconciliations` rows are both keys a pre-fix tp erases; both specs carry that measurement
themselves, and the reconcile release additionally names the shipped test that asserts the loss
today. They are not the next two releases in roadmap order, and this release does not spare them the
restatement — it only has to ship first.

## Two rows that gate while appearing in no signal

At HEAD, in two fresh fixtures:

- a round holding one `FAIL` row with `role: "not-a-real-role"` records `clean: false`, `findings: 1`,
  `role_streaks: [{role: not-a-real-role, consecutive_clean: 0, open: 1}]`;
- a round holding one `FAIL` row with **no `role` key** records `clean: false`, `findings: 1`,
  `role_streaks: []` — it gates while appearing in no per-role signal, announced only by a stderr
  line (*"1 row(s) … are missing the role field"*) that `--quiet` erases.

A default phrased as *"every expected role"* would therefore have shipped two silent relaxations
before anyone narrowed anything: a row whose role is not in `expected_roles` would stop gating, and a
row with no role at all would stop gating with nothing left to notice it. The second is the reverse of
the fail-closed rule `auditclean.go` states for the sibling field — *a row tp cannot grade is a row tp
must not stop counting* — in a release whose whole subject is that narrowing must be fenced. And a
round holding one `FAIL` row with role `not-a-real-role` records at HEAD with no complaint, which is
why a listed-but-unexpected role is an error.

## What the two shipped releases actually measure

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
release and refuted on the other, and is therefore not made in the spec.** What both releases do share
is the role axis: in all four rounds the masking rows belong to a non-conformance role and
`spec-coverage` holds none. **The missing axis is role** — on the evidence of these two replays and no
more.

## The shipped fence at four sinks

The shipped analogue, `audit_converge_on`, measured at all four sinks in a fresh project:

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
it moves include ones tp cannot enumerate."* Two candidates, neither taken:

1. **Refuse every non-empty list at `--project`.** Sound and blunt: it also refuses a list that
   narrows nothing under any base, and there is no way to express the policy repo-wide.
2. **Refuse at `--project` only when the list is not a superset of every base's `expected_roles`.**
   Precise, and it requires the sink to enumerate bases it cannot see.

The analogy to `audit_converge_on` is exact at the other three sinks and under-determined at this one.
**The fenced release's own history is that an under-determined `--project` rule cost it four audit
rounds**, so this is named as an open decision for the review round rather than settled by assertion.

## The separation is the point

A sibling discipline outside this repository runs a two-axis review — *does the code follow the
standards* and *does it match the spec* — in parallel agents whose contexts never touch, then reports
them **side by side with an explicit rule never to merge or rerank them**, and never to name a single
worst finding across axes, because *"reporting them separately stops one axis from masking the
other."*

**That is precisely the failure §4a exists to fix, arrived at independently.** tp's `--check` collapses
every role into one `clean` bit — `AuditRowsClean` iterates the round's rows with no role dimension at
all — and the measured consequence is one role's rows masking a conformance role that was clean four
rounds running.

**tp merges in a second place, and it is not the one an earlier draft named.** The `(location, class)`
clustering is `tp review --merge`, in `clusterMergeFindings`. `tp audit --merge` does something
different: it dedups on **`(role, item_id)`** — role *inside* the key, so it cannot cluster across
roles at all — and its `overlap_report` clusters by `(item_id, category)`. Since the review phase is
outside the release, the audit-side merge is what is in scope, and it does not collapse the role axis
in the first place. **Nothing in the merge surface is changed**, and the sibling's rule is about the
*verdict* rather than the display in any case.

## PASS rows carrying a note (dropped counter)

The rows spec proposed that `--status` report how many `PASS` rows carry a non-empty `notes` string,
per role, so that *"55/55 PASS, 55 carrying notes"* stops reading as *"55/55 PASS"*. Its motivation:
`CLAUDE.md` records a peer cycle where a role scored **55/55 PASS in three consecutive rounds while
its prose named a real spec defect each time** — all three fixed by commit. This repository has a
weaker instance of its own: `spec/.tp-review/0.36.0/audit-round-13.ndjson` holds a `spec-coverage`
PASS row (`item_id: task-role-mode-test`) whose notes say a doc comment *"claims more than its body
delivers"*, inside a round counted toward the `spec_coverage_clean_rounds` v0.36.0 shipped on. The
field is `notes`, plural — `internal/cli/audit_schema.go` asks for it as *"always; short string, max
500 chars; `\"\"` if no notes"*.

tp cannot distinguish a note naming a defect from a note recording what was read, and the second kind
is almost all of them:

```
python3 -c 'import json,glob,collections;R=[json.loads(l) for f in glob.glob("spec/.tp-review/*/audit-round-*.ndjson") for l in open(f) if l.strip()];P=[r for r in R if r.get("status")=="PASS"];N=lambda r:bool((r.get("notes") or "").strip());print(len(P),sum(map(N,P)),collections.Counter((r.get("role"),N(r)) for r in P))'
```

**12,113 of 12,447 PASS rows carry a note (97.3%) — and per role it is 100.0% on every one**:
`spec-coverage` 8,304, `maintainability-conventions` 818, `go-safety` 777, `ax-contract` 769, each
`True` in every row. The whole 334-row shortfall keys on `role: None` — pre-role rounds, which a
per-role counter cannot bucket in the first place. A gate here would be a gate on prose length.

**Why it was dropped.** On the population the field would actually count, the number is not merely
near-constant but identically equal to the role's PASS count in every round tp has recorded. A field
that asserts only that prose exists to be read, and equals a number `--status` already emits, is a
constant with a second name. (Corpus check on the phase: counting `status == "PASS"` rows over
`spec/.tp-review/*/review-round-*.ndjson` returns **0** — PASS rows exist only in audit files.)

**A second shape of the same defect, recorded so it is not lost with the counter.** `spec/1.1.0.md`'s
ground round 1 recorded a `PASS` whose note described a disposition the spec cut two rounds later. The
verdict was right when it was written and the note was not, and because the sentence it grades never
changed, the carry brought the stale reading forward unexamined into rounds 2 and 3. The class is
therefore wider than a note naming a defect its verdict does not: **a note whose justification cites
text the spec no longer contains** is equally invisible to every count a clean streak is read from.
Both shapes are prose a reader has to open the row to see; the release that returns a
`PASS`-with-note row to its own author next round closes the gap from the prompt's end.
