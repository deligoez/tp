#!/usr/bin/env python3
"""Re-run the derivations behind measured claims and fail when a claim is stale.

Mechanizes the `measured-claim-not-reproducible` review class, which survived
three prose corrections in the v0.34.0 review loop: a count written into a
document goes stale the moment the tree moves, and prose review cannot see it.

Each entry in CLAIMS names a document, a regex whose named groups are the
claimed numbers, and a derivation that recomputes them. A claim absent from the
document is fine -- the point is that a stated number must be true, not that a
number must be stated.

Usage: check-measured-claims.py [--verbose]
Exit 0 when every stated claim reproduces, 1 otherwise.
"""

import re
import subprocess
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent


def guard_test_window(from_tag: str, to_tag: str) -> dict:
    """Test functions ADDED between two tags, per spec/0.34.0.md section 7.1.

    --no-ext-diff is mandatory: this repository configures difftastic as the
    external diff driver, so without it the output is a rendered view, no line
    matches `^\\+func Test`, and the derivation silently returns zero.
    """
    out = subprocess.run(
        ["git", "diff", "--no-ext-diff", "-U0", from_tag, to_tag],
        cwd=REPO, capture_output=True, text=True, check=True,
    ).stdout

    added, files = set(), set()
    current = None
    for line in out.splitlines():
        if line.startswith("+++ b/"):
            current = line[6:].strip()
        if current and current.endswith("_test.go"):
            m = re.match(r"\+func (Test\w+)", line)
            if m:
                added.add((current, m.group(1)))
                files.add(current)
    return {"functions": len(added), "files": len(files)}


CLAIMS = [
    {
        "name": "guard-test window (v0.32.0..v0.33.0)",
        "document": "spec/0.34.0-guard-tests.md",
        # Matches e.g. "100 test functions across 16 files".
        "pattern": r"(?P<functions>\d+)\s+test functions across\s+(?P<files>\d+)\s+files",
        "derive": lambda: guard_test_window("v0.32.0", "v0.33.0"),
        # A derivation that returns nothing is a broken derivation, never a
        # result: that is exactly how the --no-ext-diff trap closes green.
        "nonzero": ["functions", "files"],
    },
]


def main() -> int:
    verbose = "--verbose" in sys.argv
    failures = []

    for claim in CLAIMS:
        doc = REPO / claim["document"]
        if not doc.exists():
            failures.append(f"{claim['document']}: no such file (claim {claim['name']!r})")
            continue

        derived = claim["derive"]()

        for key in claim.get("nonzero", []):
            if derived[key] == 0:
                failures.append(
                    f"{claim['name']}: derivation produced {key}=0, which is a broken "
                    f"derivation rather than a result"
                )

        text = doc.read_text(encoding="utf-8")
        matches = list(re.finditer(claim["pattern"], text))
        if not matches:
            if verbose:
                print(f"ok (no claim stated): {claim['name']} -> {derived}")
            continue

        for m in matches:
            line_no = text[: m.start()].count("\n") + 1
            stated = {k: int(v) for k, v in m.groupdict().items()}
            if stated != derived:
                failures.append(
                    f"{claim['document']}:{line_no}: {claim['name']} states {stated}, "
                    f"derivation gives {derived}"
                )
            elif verbose:
                print(f"ok: {claim['document']}:{line_no} {stated}")

    for f in failures:
        print(f, file=sys.stderr)
    if failures:
        print(f"{len(failures)} stale measured claim(s)", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
