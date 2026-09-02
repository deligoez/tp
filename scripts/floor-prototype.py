#!/usr/bin/env python3
"""Prototype §2.1 of spec/1.0.0.md — the claim floor — and measure it.

This is a SPIKE, not the release implementation. It exists because three review
rounds specified the floor in prose and the fourth round's worst defects were
all invisible to reading: a list-marker leak that puts the bare strings "2."
through "9." in the floor, a `text_sha` that two conforming readings of step 4
compute differently, and a payload bound stated without an encoding. Each took
one run to find and two rounds of five-role review had missed them.

It implements §2.1's five steps LITERALLY, then measures the three things the
prose could not settle, then tests one candidate repair. Its output is evidence
for the spec; nothing here is a proposed API.

Usage:  python3 scripts/floor-prototype.py [--json] [glob]
Exit 0 always — this measures, it does not gate.
"""

import hashlib
import json
import re
import sys
from collections import Counter
from glob import glob as _glob

# §2.1's third arm, verbatim from the spec's table.
VERBS = (
    "measured", "ran", "counted", "derived", "reproduced", "observed",
    "verified", "asserted", "recorded", "fired", "held", "refuted",
)

FENCE = re.compile(r"^\s*(```|~~~)")
TABLE_ROW = re.compile(r"^\s*\|")
HEADING = re.compile(r"^\s*#")
HRULE = re.compile(r"^\s*([-*_])\1{2,}\s*$")
BLOCKQUOTE = re.compile(r"^\s*>\s?")
LIST_MARKER = re.compile(r"^\s*(?:[-*+]\s+|\d+\.\s+)")
SENTENCE_END = re.compile(r"(?<=[.!?])\s+")
# §2.1's second arm is a backtick-delimited SPAN, so both delimiters must be in
# the same unit.
CODE_SPAN = re.compile(r"`[^`]*`")


TABLE_SEP = re.compile(r"^\s*\|[\s:|-]+\|\s*$")
# A sentinel no markdown line begins with, so a table row survives blocking and
# is recognisable at canonicalise/split time. A row is ONE unit even when its
# cells hold full stops, so it must not reach the sentence splitter.
TABLE_MARK = "﻿TBL"


def table_row_unit(line):
    """A table DATA row, canonicalised into one unit.

    Measured, and this is why the rule exists: table rows are **22% of this
    corpus's candidate content and up to 60% of an individual spec** (36% of
    spec/1.0.0.md itself). Dropping them made §8's coverage a ratio over two
    thirds of a document reported as complete, and the first end-to-end run of
    the protocol found a FAIL in a table that produced no floor unit at all —
    "a floor that cannot see a table cannot hold anyone to it".

    Cells are joined with an em dash so the row reads as one sentence; the three
    arms then apply unchanged, which is what cuts header rows (labels carry no
    digit, no code span and no measurement verb) without a special case.
    """
    # Split on an UNESCAPED pipe. `\|` inside a cell is markdown's escape and is
    # content: splitting on it turned `string \| null` into two cells and the
    # unit read "`unit_id` — string \ — null — yes", which is a mangling rather
    # than a sentence. Found by the second end-to-end run, on this spec's own
    # §7.2 field table.
    body = line.strip()
    body = re.sub(r"^\|", "", body)
    # The trailing pipe is stripped only when it is NOT itself escaped. `\|$`
    # ate the pipe and left the bare backslash on a row ending in an escaped
    # pipe — `| a | b\|` gave `a — b\` where the Go port gives `a — b|`. Found
    # by the port, which is the direction that check is supposed to run in.
    body = re.sub(r"(?<!\\)\|$", "", body)
    cells = [c.strip().replace(r"\|", "|") for c in re.split(r"(?<!\\)\|", body)]
    return re.sub(r"\s+", " ", " — ".join(c for c in cells if c)).strip()


def blocks(text):
    """Steps 1 and 2: drop, then split into blank-line-separated blocks."""
    kept, fence = [], False
    for line in text.split("\n"):
        if FENCE.match(line):
            fence = not fence
            continue
        if fence:
            continue
        if not line.strip():
            kept.append(None)          # block boundary
            continue
        if TABLE_ROW.match(line):
            if TABLE_SEP.match(line):
                continue
            kept.append(None)          # each data row is its own block
            kept.append(TABLE_MARK + line)
            kept.append(None)
            continue
        if HEADING.match(line) or HRULE.match(line):
            continue
        kept.append(line)
    out, cur = [], []
    for line in kept:
        if line is None:
            if cur:
                out.append(cur)
                cur = []
        else:
            cur.append(line)
    if cur:
        out.append(cur)
    return out


def canonicalise(block, strip_markers="every"):
    """Step 3. `strip_markers` selects the rule.

    "every" is §2.1 as it now reads and is the DEFAULT. "first" is the
    superseded reading, kept so the repair's effect stays measurable — leaving
    it as the default meant the headline table reported the pre-repair floor,
    which is how three figures in §2.1/§2.2 came from a branch the spec no
    longer specifies.
    """
    if block and block[0].startswith(TABLE_MARK):
        return table_row_unit(block[0][len(TABLE_MARK):])
    lines = [BLOCKQUOTE.sub("", ln) for ln in block]
    if strip_markers == "first":
        lines[0] = LIST_MARKER.sub("", lines[0])
    elif strip_markers == "every":
        # ONLY when the block is a list — that is, its first line opens one.
        #
        # Stripping every line unconditionally DELETES TEXT, and it had already
        # corrupted this repository's own spec when the second end-to-end run
        # found it. Hard-wrapped prose puts an ordinal at the start of a
        # continuation line — "…pinned to a named input by test\n14. The mapping
        # is…" — and the rule ate the `14. `, so the unit read "…by test The
        # mapping is…". Same sentence, different text, different text_sha,
        # depending only on where the author's line breaks fell.
        #
        # A genuine list opens with a marker, and this repository indents its
        # continuation lines, so no marker survives to be missed. Two review
        # rounds and one full grounding pass read over the corrupted unit; the
        # run that executed the derivation found it.
        if LIST_MARKER.match(lines[0]):
            lines = [LIST_MARKER.sub("", ln) for ln in lines]
    joined = " ".join(ln.strip() for ln in lines)
    return re.sub(r"\s+", " ", joined).strip()


def units_from_block(block, strip_markers="every", keep_terminator=True):
    """Steps 3, 4 and 5 for one block — the ONE place a block becomes units.

    A table data row is a single unit however many full stops its cells hold, so
    it must not reach the sentence splitter; every other block does. Both call
    sites go through here so the two cannot diverge.
    """
    if block and block[0].startswith(TABLE_MARK):
        u = canonicalise(block, strip_markers)
        return [u] if u else []
    return split_units(canonicalise(block, strip_markers), keep_terminator)


def split_units(joined, keep_terminator=True):
    """Steps 4 and 5. `keep_terminator` is the second variable under test.

    §2.1 says "split at each `.`, `!` or `?` followed by whitespace" and does
    not say whether the terminator stays with the unit. Both readings conform
    and they produce the SAME segmentation with DIFFERENT strings, so they
    agree on the floor and disagree on every `text_sha`.
    """
    if keep_terminator:
        parts = SENTENCE_END.split(joined)
    else:
        parts = [p.rstrip(".!?") for p in SENTENCE_END.split(joined)]
    return [p.strip() for p in parts if p.strip()]


def in_floor(unit):
    """The three arms of §2.1's table."""
    if re.search(r"[0-9]", unit):
        return "digit"
    # A SPAN, not a lone backtick. `"`" in unit` was this script's reading and
    # the Go port refuted it against §2.1's wording, which is the direction that
    # check is supposed to run in. The two disagree on 21 of this corpus's 7,368
    # units — 16 of them in the floor under one reading and cut under the other —
    # and every one is the same shape: a code span whose two delimiters landed in
    # different units because step 4 split between them. A lone delimiter is the
    # wreckage of a span, not a span.
    if CODE_SPAN.search(unit):
        return "identifier"
    low = unit.lower()
    if any(re.search(rf"\b{v}\b", low) for v in VERBS):
        return "verb"
    return None


def floor_of(text, strip_markers="every", keep_terminator=True):
    units = []
    for b in blocks(text):
        units.extend(units_from_block(b, strip_markers, keep_terminator))
    return units, [u for u in units if in_floor(u)]


def sha(u):
    return hashlib.sha256(u.encode("utf-8")).hexdigest()[:12]


def payload_bytes(floor, shape):
    """§11 row 4's bound is `units × 96 + 512`, stated with no encoding.

    Three encodings a conforming implementation could pick. §2.2 names the
    fields but not the wire format, and the three differ by 70%.
    """
    total = 0
    for i, u in enumerate(floor, 1):
        head, line, digest, prefix = f"u{i}", 123, sha(u), u[:60]
        if shape == "json":
            total += len(json.dumps(
                {"unit_id": head, "anchor": "§7.2", "line": line,
                 "text_sha": digest, "text": prefix}, ensure_ascii=False)) + 1
        elif shape == "labelled":     # the review.go:1054 shape §2.2 cites
            total += len(f"{head} §7.2 (line {line}, {digest}): {prefix}\n".encode())
        elif shape == "tsv":
            total += len(f"{head}\t§7.2\t{line}\t{digest}\t{prefix}\n".encode())
    return total


def measure(path):
    text = open(path, encoding="utf-8").read()
    units, floor = floor_of(text)                                   # §2.1 as it reads
    _, floor_noterm = floor_of(text, keep_terminator=False)
    _, floor_first = floor_of(text, strip_markers="first")           # superseded rule

    hashes = [sha(u) for u in floor]
    dup = {h: c for h, c in Counter(hashes).items() if c > 1}
    frag = [u for u in floor if len(u) <= 3]
    hashes_first = [sha(u) for u in floor_first]
    dup_first = {h: c for h, c in Counter(hashes_first).items() if c > 1}
    frag_first = [u for u in floor_first if len(u) <= 3]

    # step-4 canonicality: same segmentation, do the hashes agree?
    agree = sum(1 for a, b in zip(hashes, [sha(u) for u in floor_noterm]) if a == b)

    bound = len(floor) * 96 + 512
    return {
        "path": path,
        "units": len(units),
        "floor": len(floor),
        "arms": dict(Counter(in_floor(u) for u in floor)),
        "fragments": len(frag),
        "fragment_examples": sorted(set(frag))[:10],
        # NAMED, because the spec quoted this line and read a group count as a
        # multiplicity: a row asserted "no text_sha collides more than twice"
        # from a `3` that meant three groups. Both are reported now.
        "colliding_hash_groups": len(dup),
        "max_hash_multiplicity": max(dup.values()) if dup else 1,
        "units_in_collisions": sum(dup.values()),
        "step4_hash_agreement": f"{agree}/{len(floor)}",
        "floor_superseded_rule": len(floor_first),
        "fragments_superseded_rule": len(frag_first),
        "colliding_groups_superseded_rule": len(dup_first),
        "bound": bound,
        "bytes_json": payload_bytes(floor, "json"),
        "bytes_labelled": payload_bytes(floor, "labelled"),
        "bytes_tsv": payload_bytes(floor, "tsv"),
    }


def truncate_bytes(s, limit=60):
    """§2.2: first `limit` BYTES, cut on a rune boundary."""
    b = s.encode("utf-8")
    if len(b) <= limit:
        return s
    cut = b[:limit]
    while cut and (cut[-1] & 0xC0) == 0x80:
        cut = cut[:-1]
    if cut and cut[-1] >= 0x80:
        cut = cut[:-1]
    return cut.decode("utf-8", "ignore")


def tail_bytes(s, limit=30):
    """The unit's LAST `limit` bytes, cut on a rune boundary."""
    b = s.encode("utf-8")
    if len(b) <= limit:
        return ""
    cut = b[-limit:]
    while cut and (cut[0] & 0xC0) == 0x80:
        cut = cut[1:]
    return cut.decode("utf-8", "ignore")


def emit(path):
    """§2.2's anchor payload, in §11 row 4's labelled-prose shape.

    Emits `(unit_id, anchor, line, text_sha, ordinal, first 60 bytes)` per floor
    unit and nothing else — the unit reads the spec file itself. Whether that is
    enough to locate and disposition a unit is the release's central untested
    bet, and this mode exists to run it rather than argue about its cost.
    """
    text = open(path, encoding="utf-8").read()
    lines = text.split("\n")

    # anchor = the last §n(.n)* heading at or above the unit; §0 before the first
    heads = []
    for i, l in enumerate(lines, 1):
        m = re.match(r"^#{2,3}\s+(\d+(?:\.\d+)*)", l)
        if m:
            heads.append((i, "§" + m.group(1)))

    def anchor_for(lineno):
        cur = "§0"
        for i, a in heads:
            if i <= lineno:
                cur = a
            else:
                break
        return cur

    # locate each unit by its first words, so `line` is the file line it starts on
    units = []
    for b in blocks(text):
        for u in units_from_block(b):
            units.append((u, b))
    seen = Counter()
    out = []
    cut = 0
    for n, (u, b) in enumerate(units, 1):
        # A table block's first entry is TABLE_MARK + the source line, which
        # matches no line in the file. Left unhandled, EVERY table row anchored
        # to §0 — 91 of 243 units on this repository's own spec, while the test
        # asserting the §0 case passed. Strip the sentinel before locating.
        b0 = b[0][len(TABLE_MARK):] if b and b[0].startswith(TABLE_MARK) else (b[0] if b else "")
        head = " ".join(u.split()[:4])
        lineno = next((i for i, l in enumerate(lines, 1)
                       if head and head.split()[0] in l and b0.strip()[:20] in l), 0) or \
                 next((i for i, l in enumerate(lines, 1) if l.strip() == b0.strip()), 0)
        if not in_floor(u):
            # Named but not carried. The first end-to-end run found three of the
            # graded spec's worst defects in units the arms had cut, so the cut
            # set is announced — an id and an anchor, no text. The obligation
            # stays on the floor; the reader is told where to look past it.
            cut += 1
            out.append(f"u{n} {anchor_for(lineno)} (cut)")
            continue
        d = sha(u)
        seen[d] += 1
        # NO TEXT IN THE INDEX. The first end-to-end run settled this. The unit
        # reads the spec file anyway, and the one anchor defect it hit was
        # EXTENT: a 60-byte head stopped 90 bytes short of the defect and the
        # unit graded the wrong sentence. Carrying a tail too costs more than it
        # buys — measured over spec/*.md, inlining the floor is 99,633B, a
        # 60-byte head 46,337B (0.47x), head-50 plus tail-30 52,326B (0.53x). A
        # 2x saving on a payload the reader does not need, because it has the
        # file. This index is ~0.05x, and `--units` prints every unit's full text
        # in ONE call, which is what the run ended up doing by hand.
        out.append(f"u{n} {anchor_for(lineno)} {d} #{seen[d]} {len(u.encode())}B")
    tbl = len([l for l in lines
               if l.lstrip().startswith("|") and not TABLE_SEP.match(l)])
    # sum(seen.values()), NOT len(seen): the counter is keyed by hash, so a
    # distinct-key count under-reports by exactly the collisions §8 introduces
    # `ordinal` to handle — four units short on spec/0.1.0.md, in the direction
    # that makes coverage look higher than it is.
    out.append(f"# {sum(seen.values())} in floor, {cut} cut; {tbl} table data rows "
               f"are segmented into units (§2.1 step 1) and are counted above")
    return out


def units_of(path):
    """Every floor unit's FULL text, one per line, keyed by the index's id.

    The companion to `emit`: the index says what you owe a disposition for, this
    says what each unit is. One call, so the reader never guesses an extent.
    """
    text = open(path, encoding="utf-8").read()
    out = []
    for n, u in enumerate(
            [u for b in blocks(text) for u in units_from_block(b)], 1):
        if in_floor(u):
            out.append(f"u{n}\t{sha(u)}\t{u}")
    return out


def main():
    if "--emit" in sys.argv:
        i = sys.argv.index("--emit")
        for row in emit(sys.argv[i + 1]):
            print(row)
        return 0
    if "--units" in sys.argv:
        i = sys.argv.index("--units")
        for row in units_of(sys.argv[i + 1]):
            print(row)
        return 0
    args = [a for a in sys.argv[1:] if a != "--json"]
    as_json = "--json" in sys.argv
    paths = sorted(_glob(args[0] if args else "spec/*.md"))
    rows = [measure(p) for p in paths]

    if as_json:
        print(json.dumps(rows, indent=2, ensure_ascii=False))
        return 0

    print(f"{'spec':34s} {'units':>6s} {'floor':>6s} {'frag':>5s} {'grps':>5s} "
          f"{'maxmul':>7s} {'old→frag':>9s} {'bound':>7s} {'json':>7s} {'lbl':>7s} {'tsv':>7s}")
    tot = Counter()
    over = {"json": 0, "labelled": 0, "tsv": 0}
    worst_ratio, worst_path, max_mult = 0.0, "", 1
    for r in rows:
        print(f"{r['path']:34s} {r['units']:6d} {r['floor']:6d} {r['fragments']:5d} "
              f"{r['colliding_hash_groups']:5d} {r['max_hash_multiplicity']:7d} "
              f"{r['fragments_superseded_rule']:9d} {r['bound']:7d} {r['bytes_json']:7d} "
              f"{r['bytes_labelled']:7d} {r['bytes_tsv']:7d}")
        for k in ("units", "floor", "fragments", "colliding_hash_groups",
                  "fragments_superseded_rule", "colliding_groups_superseded_rule"):
            tot[k] += r[k]
        max_mult = max(max_mult, r["max_hash_multiplicity"])
        for enc in over:
            if r[f"bytes_{enc}"] > r["bound"]:
                over[enc] += 1
        ratio = r["bytes_labelled"] / r["bound"] if r["bound"] else 0
        if ratio > worst_ratio:
            worst_ratio, worst_path = ratio, r["path"]

    n = len(rows)
    print()
    print(f"{n} specs. units {tot['units']}, floor {tot['floor']}.")
    print(f"§2.1 as it reads:  fragments (<=3 chars) {tot['fragments']}, "
          f"colliding-hash GROUPS {tot['colliding_hash_groups']}, "
          f"max multiplicity of one hash in a file {max_mult}")
    print(f"superseded rule:   fragments {tot['fragments_superseded_rule']}, "
          f"colliding-hash groups {tot['colliding_groups_superseded_rule']}")
    print(f"§11 row 4 bound exceeded by: json {over['json']}/{n}, "
          f"labelled {over['labelled']}/{n}, tsv {over['tsv']}/{n}")
    print(f"  worst labelled/bound ratio: {worst_ratio:.3f} ({worst_path}) — "
          f"headroom is why row 4 asserts over a constructed input, not this corpus")
    agree_all = [r["step4_hash_agreement"] for r in rows]
    a = sum(int(x.split("/")[0]) for x in agree_all)
    b = sum(int(x.split("/")[1]) for x in agree_all)
    print(f"step-4 terminator readings agree on {a}/{b} text_sha values "
          f"({a / b * 100:.1f}%) — same segmentation, different strings")
    return 0


if __name__ == "__main__":
    sys.exit(main())
