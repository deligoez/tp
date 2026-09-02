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
        if HEADING.match(line) or TABLE_ROW.match(line) or HRULE.match(line):
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


def canonicalise(block, strip_markers="first"):
    """Step 3. `strip_markers` is the variable under test.

    "first" is §2.1 as written: strip a leading list marker from the block's
    FIRST line only. "every" is the candidate repair.
    """
    lines = [BLOCKQUOTE.sub("", ln) for ln in block]
    if strip_markers == "first":
        lines[0] = LIST_MARKER.sub("", lines[0])
    elif strip_markers == "every":
        lines = [LIST_MARKER.sub("", ln) for ln in lines]
    joined = " ".join(ln.strip() for ln in lines)
    return re.sub(r"\s+", " ", joined).strip()


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
    if "`" in unit:
        return "identifier"
    low = unit.lower()
    if any(re.search(rf"\b{v}\b", low) for v in VERBS):
        return "verb"
    return None


def floor_of(text, strip_markers="first", keep_terminator=True):
    units = []
    for b in blocks(text):
        units.extend(split_units(canonicalise(b, strip_markers), keep_terminator))
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
    units, floor = floor_of(text)
    _, floor_notérm = floor_of(text, keep_terminator=False)
    _, floor_fixed = floor_of(text, strip_markers="every")

    hashes = [sha(u) for u in floor]
    dup = {h: c for h, c in Counter(hashes).items() if c > 1}
    frag = [u for u in floor if len(u) <= 3]
    hashes_fixed = [sha(u) for u in floor_fixed]
    dup_fixed = {h: c for h, c in Counter(hashes_fixed).items() if c > 1}
    frag_fixed = [u for u in floor_fixed if len(u) <= 3]

    # step-4 canonicality: same segmentation, do the hashes agree?
    agree = sum(1 for a, b in zip(hashes, [sha(u) for u in floor_notérm]) if a == b)

    bound = len(floor) * 96 + 512
    return {
        "path": path,
        "units": len(units),
        "floor": len(floor),
        "arms": dict(Counter(in_floor(u) for u in floor)),
        "fragments": len(frag),
        "fragment_examples": sorted(set(frag))[:10],
        "colliding_hashes": len(dup),
        "units_in_collisions": sum(dup.values()),
        "step4_hash_agreement": f"{agree}/{len(floor)}",
        "floor_marker_fix": len(floor_fixed),
        "fragments_after_fix": len(frag_fixed),
        "collisions_after_fix": len(dup_fixed),
        "bound": bound,
        "bytes_json": payload_bytes(floor, "json"),
        "bytes_labelled": payload_bytes(floor, "labelled"),
        "bytes_tsv": payload_bytes(floor, "tsv"),
    }


def main():
    args = [a for a in sys.argv[1:] if a != "--json"]
    as_json = "--json" in sys.argv
    paths = sorted(_glob(args[0] if args else "spec/*.md"))
    rows = [measure(p) for p in paths]

    if as_json:
        print(json.dumps(rows, indent=2, ensure_ascii=False))
        return 0

    print(f"{'spec':34s} {'units':>6s} {'floor':>6s} {'frag':>5s} {'coll':>5s} "
          f"{'fix→frag':>9s} {'fix→coll':>9s} {'bound':>7s} {'json':>7s} {'lbl':>7s} {'tsv':>7s}")
    tot = Counter()
    over = {"json": 0, "labelled": 0, "tsv": 0}
    for r in rows:
        print(f"{r['path']:34s} {r['units']:6d} {r['floor']:6d} {r['fragments']:5d} "
              f"{r['colliding_hashes']:5d} {r['fragments_after_fix']:9d} "
              f"{r['collisions_after_fix']:9d} {r['bound']:7d} {r['bytes_json']:7d} "
              f"{r['bytes_labelled']:7d} {r['bytes_tsv']:7d}")
        for k in ("units", "floor", "fragments", "colliding_hashes",
                  "fragments_after_fix", "collisions_after_fix"):
            tot[k] += r[k]
        for enc in over:
            if r[f"bytes_{enc}"] > r["bound"]:
                over[enc] += 1

    n = len(rows)
    print()
    print(f"{n} specs. units {tot['units']}, floor {tot['floor']}.")
    print(f"§2.1 AS WRITTEN      fragments (<=3 chars) {tot['fragments']}, "
          f"colliding text_sha {tot['colliding_hashes']}")
    print(f"§2.1 + marker repair fragments {tot['fragments_after_fix']}, "
          f"colliding text_sha {tot['collisions_after_fix']}")
    print(f"§11 row 4 bound exceeded by: json {over['json']}/{n}, "
          f"labelled {over['labelled']}/{n}, tsv {over['tsv']}/{n}")
    agree_all = [r["step4_hash_agreement"] for r in rows]
    a = sum(int(x.split("/")[0]) for x in agree_all)
    b = sum(int(x.split("/")[1]) for x in agree_all)
    print(f"step-4 terminator readings agree on {a}/{b} text_sha values "
          f"({a / b * 100:.1f}%) — same segmentation, different strings")
    return 0


if __name__ == "__main__":
    sys.exit(main())
