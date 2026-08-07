#!/usr/bin/env python3
"""Verify that a spec's code citations resolve against the tree.

Registered as the mechanical check for the `code-citation-drift` finding class.
Three rounds of v0.33.0 spec review produced findings of that class, so the
prose fix is replaced by a detector, per CLAUDE.md's mechanization rule.

What it checks, over every backticked span in the spec:

1. A repo-relative path (contains "/", ends in a known source extension) must
   exist in the tree.
2. A `path:N` or `path:A-B` citation must name a file with at least that many
   lines.
3. A `pkg.Identifier` reference whose `pkg` matches a directory under
   `internal/` must have `Identifier` declared somewhere in that package.

It deliberately does not try to judge whether the surrounding sentence is a
true statement about the code — that is the reviewer's job. It catches the
mechanical half: a moved file, a stale line number, a renamed symbol.

Usage: check-spec-code-citations.py <spec.md> [<spec.md> ...]
Exit 0 when every citation resolves, 1 otherwise (paths and reasons on stdout).
"""

from __future__ import annotations

import pathlib
import re
import sys

SOURCE_SUFFIXES = (".go", ".py", ".md", ".json", ".yml", ".yaml")

# `code span` — the only place this spec family puts citations.
CODE_SPAN = re.compile(r"`([^`\n]+)`")
# path, optionally followed by :N or :A-B
PATH_CITE = re.compile(r"^([\w./\-]+\.(?:go|py|md|json|yml|yaml))(?::(\d+)(?:-(\d+))?)?$")
# pkg.Identifier, exported or not, no further dots
PKG_SYMBOL = re.compile(r"^([a-z][a-z0-9]*)\.([A-Za-z_]\w*)$")
# Go declaration of a name at package scope
def _decl_patterns(name: str) -> tuple[re.Pattern[str], ...]:
    n = re.escape(name)
    return (
        re.compile(rf"^func\s+{n}\b", re.M),
        re.compile(rf"^func\s+\([^)]*\)\s+{n}\b", re.M),
        re.compile(rf"^(?:type|var|const)\s+{n}\b", re.M),
        re.compile(rf"^\s+{n}\s+[\w\[\]*.]+\s*=", re.M),  # grouped var/const
        re.compile(rf"^\s+{n}\s*=", re.M),
    )


def repo_root(start: pathlib.Path) -> pathlib.Path:
    for d in [start, *start.parents]:
        if (d / ".git").exists():
            return d
    # Falling back to the spec's own directory would resolve every citation
    # against the wrong tree and report the whole spec as broken, which reads
    # as a spec defect rather than a misinvocation. Refuse instead.
    raise SystemExit(f"no .git above {start}: cannot resolve repo-relative citations")


def line_count(path: pathlib.Path) -> int:
    with path.open("rb") as fh:
        return sum(1 for _ in fh)


def package_files(root: pathlib.Path, pkg: str) -> list[pathlib.Path]:
    base = root / "internal" / pkg
    if not base.is_dir():
        return []
    return [p for p in base.glob("*.go") if not p.name.endswith("_test.go")]


def check_spec(spec: pathlib.Path, root: pathlib.Path) -> list[str]:
    problems: list[str] = []
    text = spec.read_text(encoding="utf-8")
    # A fenced block is prose here (message templates, hints) — skip it so a
    # sample payload never reads as a citation.
    outside_fences: list[str] = []
    fenced = False
    for line in text.splitlines():
        if line.lstrip().startswith("```"):
            fenced = not fenced
            continue
        if not fenced:
            outside_fences.append(line)

    seen_paths: set[str] = set()
    seen_symbols: set[str] = set()
    for lineno, line in enumerate(outside_fences, start=1):
        for span in CODE_SPAN.findall(line):
            span = span.strip()

            m = PATH_CITE.match(span)
            if m and ("/" in m.group(1) or m.group(2)):
                rel, start_s, end_s = m.group(1), m.group(2), m.group(3)
                key = f"{rel}:{start_s or ''}:{end_s or ''}"
                if key in seen_paths:
                    continue
                seen_paths.add(key)
                target = root / rel
                if not target.is_file():
                    problems.append(f"{spec.name}:{lineno}: no such file: {rel}")
                    continue
                wanted = max(int(start_s or 0), int(end_s or 0))
                if wanted:
                    have = line_count(target)
                    if wanted > have:
                        problems.append(
                            f"{spec.name}:{lineno}: {rel} has {have} lines, citation names {wanted}"
                        )
                continue

            m = PKG_SYMBOL.match(span)
            if m:
                pkg, name = m.group(1), m.group(2)
                if span in seen_symbols:
                    continue
                files = package_files(root, pkg)
                if not files:
                    continue  # not an internal package; nothing to check
                seen_symbols.add(span)
                pats = _decl_patterns(name)
                found = any(
                    any(p.search(f.read_text(encoding="utf-8")) for p in pats) for f in files
                )
                if not found:
                    problems.append(
                        f"{spec.name}:{lineno}: internal/{pkg} declares no {name} (cited as {span})"
                    )
    return problems


def main(argv: list[str]) -> int:
    if len(argv) < 2:
        print("usage: check-spec-code-citations.py <spec.md> [...]", file=sys.stderr)
        return 2
    problems: list[str] = []
    for arg in argv[1:]:
        spec = pathlib.Path(arg).resolve()
        if not spec.is_file():
            print(f"no such spec: {arg}", file=sys.stderr)
            return 2
        problems.extend(check_spec(spec, repo_root(spec.parent)))
    for p in problems:
        print(p)
    if problems:
        print(f"{len(problems)} unresolved code citation(s)")
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
