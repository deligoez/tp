# a-rewrite-names-what-it-dropped — measurements

Supplemental material for `a-rewrite-names-what-it-dropped.md`; the spec stands without it. **This
file is not a spec and `tp ground` never grades it.** Everything below was run on 2026-09-11 against
`tp v1.1.2-0.20260910212136-18032abe405f` (the `HEAD` `18032abe` build), always on copies outside the
repository, because `tp review` emission writes round state.

---

## Field report WB-3155, verified 2026-09-11

**#19 — "neither ground nor review can see what disappeared."** The claim: `ground` grades what
exists, so a sentence that is gone has no verdict; `review` takes the spec as authoritative, so
*"this used to say something it no longer says"* is not its question; `lint` checks form. So a rule
dropped silently in a rewrite passes all three. The reporter built a fidelity check by hand (the old
version through `git show`, compared with the new body plus its sidecar) and found **two real
losses**, both touching live behaviour:
- a rule that a shipped commit turned the previous year's admin export into a 409, and that the PR
  description had to *say* so. No task owned it and it was not a claim about code.
- a rule that event discovery's glob stays limited to `Campaigns/*`, with two named failure modes: a
  double registration of eight subscribers, and waking a listener that sends live SMS.

The proposal: `tp fidelity <spec.md> --against <git-ref>`, listing normative sentences present in
the old version with no counterpart in the new, and taking the decisions sidecar as input too,
*"because part of what disappears has moved there (2 of our 4 findings were)"*. The same report
measured its rewrite at 1,601 → 1,322 lines and a floor of 899 → 705 units, with 89 units moved into
the decisions trail.

- **Verdict: CONFIRMED** for the mechanism. **Not reproducible here** for the two losses and the
  counts, which are the reporter's own measurements on a repository this one cannot read; they are
  carried as relayed.
- **Reproduction of the mechanism.** A scratch directory holds `v1.md` with a section `## 1. Export`
  (two sentences: *"The export returns 409 for a year-old request. The PR description must state
  this change."*) and a section `## 2. Discovery` (*"Event discovery is limited to
  `Campaigns/*`."*). `spec.md` keeps only the first Export sentence and renames the second section
  to `## 2. Event discovery` with different text. Then `tp init spec.md` and
  `tp review spec.md --diff-from v1.md --json`, exit 0. In every role prompt, the changed-sections
  block shows `## 1. Export (MODIFIED)` with the current text only, `## 2. Event discovery (ADDED)`,
  and *"- "2. Discovery" was removed from the spec."*. Neither *"PR description"* nor *"Campaigns"*
  appears anywhere in the emitted prompts.
- **Source.** `DiffSections` in `internal/engine/diff.go:41-109` matches base and current sections
  by exact heading (`baseMap`/`currMap`). A section present in both is `MODIFIED` and carries the
  **current** content. A section only in the base is `REMOVED`, and its `Content` is documented as
  *"empty for unchanged/removed"* (`:21`).
- **Refuted:** nothing. The report's proposed command name is not taken, for the reason given in the
  spec's Alternatives.

## The prototype

**What it computes.** For a commit `C` that rewrote a spec, the old spec is `C^:<old path>`, the new
one is `C:<new path>`, and the counterpart is `C:<sidecar>`. Each is run through
`tp ground <file> --units` in a scratch directory, which prints one floor unit per line as
`unit_id`, a 12-hex `text_sha` and the text, and writes no state (checked: no `.tp-review/` appears).
An old unit is **listed** when its `text_sha` is in neither the new floor nor the counterpart's
floor. Each listed unit gets the highest Jaccard overlap of its lowercased `[a-z0-9_]+` words with
any sentence of the new text plus the counterpart, splitting on sentence ends, blank lines and table
rows. The listing sorts on that score, ascending. The script:

```bash
bash -c 'cat > /tmp/listing.py <<"PY"
import os, re, subprocess, sys, tempfile
tp = os.environ.get("TP", "tp")
c, old, new = sys.argv[1:4]; side = sys.argv[4] if len(sys.argv) > 4 else None
show = lambda rev, p: subprocess.run(["git", "show", f"{rev}:{p}"], capture_output=True, text=True, check=True).stdout
def units(text):
    d = tempfile.mkdtemp(); open(os.path.join(d, "s.md"), "w").write(text)
    out = subprocess.run([tp, "ground", "s.md", "--units"], cwd=d, capture_output=True, text=True, check=True).stdout
    return [tuple(l.split("\t", 2)) for l in out.splitlines() if l.strip()]
cur = show(c, new) + "\n" + (show(c, side) if side else "")
have = {u[1] for u in units(show(c, new))} | ({u[1] for u in units(show(c, side))} if side else set())
listed = [u for u in units(show(c + "^", old)) if u[1] not in have]
words = lambda s: set(re.findall(r"[a-z0-9_]+", s.lower()))
sents = [words(s) for s in re.split(r"(?<=[.!?])\s+|\n\s*\n|\n\|", cur) if s.strip()]
def overlap(u):
    w = words(u[2]); return max((len(w & s) / len(w | s) for s in sents if w | s), default=0.0)
for score, u in sorted((overlap(u), u) for u in listed):
    print(f"{score:.2f}\t{u[0]}\t{u[2][:160]}")
print(f"listed {len(listed)}", file=sys.stderr)
PY
python3 /tmp/listing.py 1ea05d64 spec/backlog/a-finding-can-leave-an-audit-round.md spec/backlog/a-finding-can-leave-an-audit-round.md spec/backlog/a-finding-can-leave-an-audit-round-measurements.md | head -3'
```

**Four rewrites, picked by hand** from the commits since 2026-09-01 that inserted and deleted more
than sixty lines each under `spec/` (`git log --shortstat`). They were picked for one old spec file
mapping to one new one, and for covering the four shapes below:

| commit | what the rewrite was | old floor | kept verbatim | only in the counterpart | listed | listed without the counterpart |
|---|---|---|---|---|---|---|
| `1ea05d64` | a backlog body rewritten under Step 0.5, same scope | 49 | 4 | 0 | 45 | 45 |
| `a21e3572` | a backlog body cut to its seams, forensics moved to a new sidecar — **the field's shape** | 114 | 40 | 47 | 27 | 74 |
| `ceb28de4` | the hotfix spec cut to its audit half, renamed by slug | 116 | 12 | 1 | 103 | 104 |
| `3c4d2a5d` | `spec/1.0.1.md` rescoped to *report, not judge* | 111 | 0 | 0 | 111 | 111 |

**How the listed units were judged.** `1ea05d64` and `a21e3572` were read one by one, every listed
unit beside its best-overlap sentence, and each was marked *dropped* (no counterpart in either file),
*reworded* or *moved*. `ceb28de4` and `3c4d2a5d` were counted and not read. Their rewrites changed
scope — part of the first went to another spec in a different commit, and the second was rescoped
outright. Most of their listing is therefore expected to be text that is really gone, and judging it
would measure the rescope, not the listing. That expectation was not checked.

- **`1ea05d64`, 45 listed: 2 dropped, 43 reworded.** The two dropped units are `u47` (*"No judgement
  of `resolved.evidence`'s content, and no fence on the write."*) and `u48` (*"Refusing an empty
  evidence string at the write, and refusing `--resolve` under `TP_UNATTENDED=1`, is
  `a-findings-exits-agree.md` §4 — and that fence is what keeps §2 from being an escape hatch an agent
  can write for itself…"*). Together they recorded that the spec's central change depends on a
  sibling's fence, and the rewrite dropped both with nothing in their place. The next commit on the
  file, `6f8e8564` four minutes later, moved that fence into the spec as its §3. **The listing sorts
  them 1st and 3rd of 45.** The interloper at 2nd is `u39`, a Non-Goal whose history clause (*"the
  bar the hotfix set"*) was dropped while its rule survived. `u9`, a sentence whose implementation
  clause Step 0.5 removed, sorts 21st, because its first half survives.
- **`a21e3572`, 27 listed: 0 dropped, 27 reworded or moved.** The three lowest (all at or below
  0.30) are rewordings of withdrawn-row and fixture-property sentences. The rest are sentences whose
  `file:line` citations or quoted figures were trimmed. The counterpart does the narrowing here: 47
  of the 114 old units are verbatim in the new sidecar, so without it the listing is 74 units.

**What that does and does not show.** It shows the exact-hash listing on its own is not narrow: a
rewording pass lists 45 of 49 units. It shows the counterpart is what narrows a rewrite in the
field's shape, and the order is what narrows a rewording pass. It does **not** show the kill
condition passing. It is one rewrite with dropped units; the prototype computes the listing from
`--units` output rather than running the built flag; and the order is sensitive to tokenization. An
earlier run of the same comparison, keeping `.`, `-` and backticks inside words, sorted `u47` and
`u48` 1st and 2nd instead of 1st and 3rd.

**Comparing against the whole text instead of the floor changed nothing here.** A second pass counted
an old unit as kept when its whitespace-normalized text was a substring of the whole new text plus
counterpart, cut spans included. It listed exactly as many units as the floor-hash comparison on all
four rewrites. The decision's *"no unit of the current spec"* is therefore not load-bearing on this
evidence, and the implementing task may compare against either.

## Why the decision is a flag on `tp ground`

`tp ground --units` already prints each floor unit with its `text_sha` and writes nothing. The
listing is a set difference over that output, plus a sort. `tp ground <spec> --units --against HEAD`
at `18032abe` exits 2 with `{"error":"unknown flag: --against","code":2,"hint":"run 'tp ground
--help' for usage"}`. That is the value the spec's test rows name for `HEAD`.
