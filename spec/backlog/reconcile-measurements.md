# reconcile — measurements

Supplemental material for `reconcile.md`; the spec stands without it.

## The rows the invariant does not cover

The round's recorded findings file is rewritten in place by `tp review --resolve` / `--resolve-all`,
whose `--force` overwrites a disposition already there. 1,566 rows across 85 recorded round files
carried a post-hoc `resolved` object when this was written:

```
python3 -c "import json,glob;g=glob.glob('spec/.tp-review/*/review-round-*.ndjson')+glob.glob('spec/.tp-review/*/audit-round-*.ndjson');r=[(f,l) for f in g for l in open(f) if l.strip() and isinstance(json.loads(l).get('resolved'),dict)];print(len(r),len({f for f,_ in r}))"
```

## Why that cycle proceeded to audit

**That cycle proceeded to audit with the blocker standing, and the reasoning is worth keeping**: after
implementation is complete the audit subsumes what a re-review could ask, because `spec-coverage`
derives its checklist from the repaired text and tests the code against it. That the checklist came from
the *repaired* text is checkable — `shasum -a 256 spec/.tp-review/0.35.0/snapshot-audit-round-1.md`
equals the `spec_hash` that round recorded. The cost avoided was real, and it is structural rather than
anecdotal: `review_clean_rounds` resolves to 2 (`tp config --resolved`), a `fixed` disposition implies a
spec change and forces a re-review (`skills/tp/SKILL.md:19`), and the panel is five —
`python3 -c "import json,glob;print(sorted({json.loads(l)['role'] for f in glob.glob('spec/.tp-review/0.37.0/review-round-*.ndjson') for l in open(f) if l.strip() and json.loads(l).get('role')}))"`
returns architect, ax-economist, implementer, regression and tester. So settling a two-section repair
costs **at least ten role-rounds over the whole spec**. It is stated as a floor rather than a cycle's
round count because no review cycle in this repository has eleven rounds
(`python3 -c "import glob,collections;print(sorted(collections.Counter(f.split('/')[2] for f in glob.glob('spec/.tp-review/*/review-round-*.ndjson')).values()))"`)
— every `eleven rounds` in the tree names an **audit**, a different panel with a scope-blind
convergence cause rather than a localised spec repair.

## The audit side's instance

v0.35.0's own audit ran nine rounds over **seven distinct** `spec_hash` values:

```
python3 -c "import json;print([r['spec_hash'][:13] for r in json.load(open('spec/.tp-review/0.35.0/state.json'))['audit_rounds']])"
```

## Where the storage decision came from

An earlier draft made the unknown-key round-trip a hard prerequisite: `ReviewState` is a typed struct
and `SaveReviewState` marshals it, so a `reconciliations` array *injected* into `state.json` is gone
after the next `--record` — `go test ./internal/cli -count=1 -run
TestAGroundRoundLeavesAnExistingStateJSONByteIdentical` seeds a top-level key `ReviewState` does not
know, runs a record over it, and asserts the key is gone. That measurement is correct and the
conclusion drawn from it was not: a list that is a *typed field* of `ReviewState` survives every save
the current binary makes, and only a binary that predates the field drops it. The spec now records
that as a downgrade hazard.
