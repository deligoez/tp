#!/usr/bin/env python3
"""Refuse a Tests-row whose fixture or mutant is only stated in the sidecar.

Registered as the mechanical check for the `method-only-in-ungraded-sidecar`
finding class. That class was returned by `tp review --record` on
`spec/1.1.0.md` with `rounds_seen: 3, total: 3`, and CLAUDE.md's rule is that a
class surviving three prose repairs gets a check rather than a fourth prose fix.

Why the rule is worth mechanizing: a `<base>-measurements.md` sidecar is not
graded — `tp ground` and the review panel read the spec, not the sidecar — so a
row that names its fixture or its mutant by pointing at the sidecar has put the
half a reader must check into the half nobody checks. The row must carry its own
fixture and mutant; the sidecar keeps the derivation behind them.

The counting rule, stated so a disagreement is about the rule and not the code:

* Only headings whose text is a number followed by `Tests` open a scanned
  section (`## 5. Tests`, `### 5. Tests`); the section ends at the next heading
  of the same or shallower level.
* Inside it, a scanned row is a table line — first non-space character `|` —
  that is neither the header row nor a `|---|` separator.
* The scanned cells are the columns with `fixture` or `mutant` among the
  lowercased alphabetic words of their header cell. Whole words, not
  substrings: `the mutant that must fail it` names a cell and `mutants` does
  not. A table with neither header names no cells and contributes no rows to
  the scanned count.
* A cell cites the sidecar when it contains `measurements file` or
  `-measurements.md`. The bare word `measurements` is deliberately NOT the
  trigger: it produces a false positive on prose that merely mentions
  measurement.
* Fenced code blocks are skipped, so a sample table inside a fence is prose.

Usage: check-test-rows-cite-sidecar.py <spec.md> [<spec.md> ...]
Exit 0 when rows were scanned and no scanned cell cites the sidecar, 1 when a
cell cites it, and 1 when nothing was scanned at all. That last case is the
point: the three ways this scan empties itself — a plural header, a Tests
heading carrying a suffix, an unclosed fence anywhere above the section — each
returned success over an input whose fixture cell did cite the sidecar, so a
zero-row run certifies nothing and must not pass.
`scripts/check-test-rows-cite-sidecar-test.sh` holds one fixture per way.
`--stats` adds the scanned-row and scanned-file counts to stdout, which is what
makes a corpus run reportable.
"""

from __future__ import annotations

import pathlib
import re
import sys

TESTS_HEADING = re.compile(r"^(#{2,6})\s+\d+(?:\.\d+)*\.?\s+Tests\s*$", re.I)
ANY_HEADING = re.compile(r"^(#{1,6})\s+")
CITES = ("measurements file", "-measurements.md")
SCANNED_HEADERS = ("fixture", "mutant")


def _cells(line: str) -> list[str]:
    """Split a markdown table line into its cells, dropping the outer pipes.

    Escaped pipes (`\\|`, which is how a table cell carries a literal `|`) are
    masked before the split so a cell is not torn in two by its own content.
    """
    body = line.strip()
    body = body[1:] if body.startswith("|") else body
    body = body[:-1] if body.endswith("|") else body
    return [c.replace("\x00", r"\|").strip() for c in body.replace(r"\|", "\x00").split("|")]


def _is_separator(cells: list[str]) -> bool:
    return bool(cells) and all(re.fullmatch(r":?-{2,}:?", c) for c in cells if c != "")


def _header_columns(cells: list[str]) -> dict[int, str]:
    out: dict[int, str] = {}
    for i, c in enumerate(cells):
        words = re.findall(r"[a-z]+", c.lower())
        for name in SCANNED_HEADERS:
            if name in words:
                out[i] = name
                break
    return out


def check_spec(spec: pathlib.Path) -> tuple[list[str], int]:
    problems: list[str] = []
    scanned_rows = 0
    lines = spec.read_text(encoding="utf-8").splitlines()

    in_tests = False
    tests_depth = 0
    fenced = False
    columns: dict[int, str] = {}
    in_table = False

    for lineno, raw in enumerate(lines, start=1):
        if raw.lstrip().startswith("```"):
            fenced = not fenced
            continue
        if fenced:
            continue

        heading = ANY_HEADING.match(raw)
        if heading:
            depth = len(heading.group(1))
            if TESTS_HEADING.match(raw):
                in_tests, tests_depth = True, depth
            elif in_tests and depth <= tests_depth:
                in_tests = False
            columns, in_table = {}, False
            continue

        if not in_tests:
            continue

        if not raw.lstrip().startswith("|"):
            columns, in_table = {}, False
            continue

        cells = _cells(raw)
        if _is_separator(cells):
            continue
        if not in_table:
            # The first table line of a run is its header row.
            columns, in_table = _header_columns(cells), True
            continue

        if not columns:
            continue
        scanned_rows += 1
        for idx, name in columns.items():
            if idx >= len(cells):
                continue
            cell = cells[idx]
            hit = next((c for c in CITES if c in cell), None)
            if hit:
                problems.append(
                    f"{spec.name}:{lineno}: {name} cell cites the measurements sidecar "
                    f"({hit!r}) — a Tests row must carry its own {name}"
                )
    return problems, scanned_rows


def main(argv: list[str]) -> int:
    args = [a for a in argv[1:] if a != "--stats"]
    stats = "--stats" in argv[1:]
    if not args:
        print("usage: check-test-rows-cite-sidecar.py [--stats] <spec.md> [...]", file=sys.stderr)
        return 2

    problems: list[str] = []
    rows = files = 0
    for arg in args:
        spec = pathlib.Path(arg)
        if not spec.is_file():
            print(f"no such spec: {arg}", file=sys.stderr)
            return 2
        found, n = check_spec(spec)
        problems.extend(found)
        rows += n
        files += 1 if n else 0

    for p in problems:
        print(p)
    if stats:
        print(f"scanned {rows} Tests-table row(s) across {files} file(s) with a Tests table")
    if problems:
        print(f"{len(problems)} Tests row cell(s) citing the measurements sidecar")
        return 1
    if rows == 0:
        print(
            f"scanned 0 Tests-table rows across {len(args)} file(s) — a clean exit here "
            "would certify nothing. Check the Tests heading (its text must end at `Tests`), "
            "the column headers (`fixture`/`mutant` as whole words, not `fixtures`/`mutants`), "
            "and every code fence above the section for one that is never closed."
        )
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
