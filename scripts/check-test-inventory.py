#!/usr/bin/env python3
"""Mechanical check for the `test-inventory-drift` review class.

A spec section that claims to enumerate the committed tests a change invalidates
drifts silently: the searched literals move, a test is renamed, or a catch-all is
narrowed into a list that orphans an entry. Three consecutive v0.31.2 review
rounds surfaced this class, so it is mechanized rather than re-reviewed.

The check reads the literals the spec says it searched for, finds every committed
test function that references any of them, and compares that set against the test
names the spec's inventory section actually mentions.

Usage:  check-test-inventory.py <spec.md> <section-heading-substring>
Exit 0 when every referencing test is named, 1 on drift, 2 on usage/IO error.
"""

import os
import re
import sys

# Literals are read from the spec's own inventory preamble: every `backticked`
# token on the lines between the section heading and its first list item.
LITERAL_RE = re.compile(r"`([^`]+)`")
GO_TEST_RE = re.compile(r"^func (Test\w+)\(", re.MULTILINE)
TEST_NAME_RE = re.compile(r"\b(Test\w+)\b")


def die(msg, code=2):
    print(f"check-test-inventory: {msg}", file=sys.stderr)
    sys.exit(code)


def read(path):
    try:
        with open(path, encoding="utf-8") as fh:
            return fh.read()
    except OSError as err:
        die(f"cannot read {path}: {err}")


def section_of(spec_text, heading_substring):
    """Return (literals, body) for the section whose heading contains the substring.

    The searched literals come from the section's ```search-literals fenced block,
    one per line, so prose that *mentions* a token without searching it cannot leak
    into the set.
    """
    lines = spec_text.splitlines()
    start = None
    for i, line in enumerate(lines):
        if line.startswith("#") and heading_substring.lower() in line.lower():
            start = i
            break
    if start is None:
        die(f"no heading containing {heading_substring!r}")
    level = len(lines[start]) - len(lines[start].lstrip("#"))
    end = len(lines)
    fenced = False
    for i in range(start + 1, len(lines)):
        stripped = lines[i]
        # A fenced block may hold lines that look like headings (the literal set
        # itself contains "## Project Context"); never end the section inside one.
        if stripped.lstrip().startswith("```"):
            fenced = not fenced
            continue
        if not fenced and stripped.startswith("#"):
            here = len(stripped) - len(stripped.lstrip("#"))
            if here <= level:
                end = i
                break
    body = lines[start + 1 : end]

    literals = []
    inside = False
    for line in body:
        if line.strip() == "```search-literals":
            inside = True
            continue
        if inside:
            if line.strip() == "```":
                break
            if line.strip():
                literals.append(line.strip())
    if not inside:
        die("no ```search-literals fenced block in the inventory section")
    return literals, "\n".join(body)
def go_test_files(root):
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in (".git", "vendor", "node_modules")]
        for name in filenames:
            if name.endswith("_test.go"):
                yield os.path.join(dirpath, name)


def enclosing_test(text, index):
    """Name of the Test function containing the byte offset, or None."""
    found = None
    for match in GO_TEST_RE.finditer(text):
        if match.start() > index:
            break
        found = match.group(1)
    return found


def main():
    if len(sys.argv) != 3:
        die("usage: check-test-inventory.py <spec.md> <section-heading-substring>")
    spec_path, heading = sys.argv[1], sys.argv[2]
    root = os.path.dirname(os.path.abspath(spec_path))
    while root != "/" and not os.path.isdir(os.path.join(root, ".git")):
        root = os.path.dirname(root)
    if root == "/":
        die("no git repository root above the spec")

    spec_text = read(spec_path)
    literals, body = section_of(spec_text, heading)
    if not literals:
        die("the search-literals block is empty")

    named = set(TEST_NAME_RE.findall(body))

    referencing = {}
    for path in go_test_files(root):
        text = read(path)
        for literal in literals:
            for match in re.finditer(re.escape(literal), text):
                test = enclosing_test(text, match.start())
                if test:
                    referencing.setdefault(test, set()).add(literal)

    missing = {t: lits for t, lits in referencing.items() if t not in named}

    print(f"literals searched: {len(literals)}")
    print(f"tests referencing them: {len(referencing)}")
    print(f"tests named in the inventory: {len(named & set(referencing))}")

    if missing:
        print("\nDRIFT — referencing tests absent from the inventory:", file=sys.stderr)
        for test in sorted(missing):
            print(f"  {test}  ({', '.join(sorted(missing[test]))})", file=sys.stderr)
        sys.exit(1)

    print("no drift: every referencing test is named in the inventory")
if __name__ == "__main__":
    main()
