#!/usr/bin/env python3
"""Re-run the derivations behind measured claims and fail when a claim is stale.

Mechanizes the `measured-claim-not-reproducible` review class, which survived
three prose corrections in the v0.34.0 review loop: a count written into a
document goes stale the moment the tree moves, and prose review cannot see it.

Three properties, each added because a review round broke the previous version:

1. A registered claim MUST be present. A claim reworded past its pattern fails,
   because the documents this guards are ones the release itself rewrites, and
   a guard that goes quiet when its subject is edited guards nothing.
2. The derivation rule is READ FROM THE DOCUMENT, not held here. Holding it here
   let the window be changed in the prose while the check re-derived the old
   rule and agreed with itself -- the numbers matched a rule nobody had asked
   for. Parsing the documented command also pins the flags it must carry.
3. An artifact whose count is claimed has its BODY counted, not just its
   summary. A file with a correct summary line and no entries used to pass.

COVERAGE is narrow and stated: only figures a command can recompute are
checked. Estimates ("roughly doubles wall time") and historical observations
("lost the add's task in 1 of 20 runs") are not derivable from the current tree
and remain the reviewers' business. It prints on every run, including clean
ones, because the registration tells every reviewer to stop reporting the whole
class.

Usage: check-measured-claims.py [--quiet]
Exit 0 when every registered claim reproduces, 1 otherwise.
"""

import re
import subprocess
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent

COVERAGE = (
    "re-derivable counts only; estimates and historical observations are not "
    "covered and remain reportable"
)

# The shape the documented derivation must keep. Pinning it here is what makes
# dropping --no-ext-diff a failure: without that flag this repository's external
# diff driver emits a rendered view, no line matches `^\+func Test`, and the
# derivation silently returns zero.
DOCUMENTED_DIFF = re.compile(r"^git diff --no-ext-diff -U0 (\S+) (\S+)$", re.M)

# One `file:function` per line, e.g. internal/cli/audit_test.go:TestAuditRecord
LIST_ENTRY = re.compile(r"^[\w./-]+_test\.go:Test\w+\s*$", re.M)

PLACEHOLDER = "Populated by the first task"


def read_documented_window(text: str) -> tuple:
    """Extract the tag pair from the command the document itself states."""
    m = DOCUMENTED_DIFF.search(text)
    if not m:
        return None
    return m.group(1), m.group(2)


def added_test_functions(from_ref: str, to_ref: str) -> dict:
    out = subprocess.run(
        ["git", "diff", "--no-ext-diff", "-U0", from_ref, to_ref],
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


def repo_test_functions() -> dict:
    total = 0
    for path in REPO.rglob("*_test.go"):
        if ".git" in path.parts:
            continue
        total += len(re.findall(r"^func Test\w+", path.read_text(encoding="utf-8"), re.M))
    return {"total": total}


def check_guard_tests(text: str, failures: list) -> None:
    doc = "spec/0.34.0-guard-tests.md"

    window = read_documented_window(text)
    if window is None:
        failures.append(
            f"{doc}: the documented derivation does not match "
            f"`git diff --no-ext-diff -U0 <from> <to>`. Either the window moved or "
            f"--no-ext-diff was dropped; both change what the numbers below mean."
        )
        return

    derived = added_test_functions(*window)
    for key, value in derived.items():
        if value == 0:
            failures.append(
                f"guard-test window {window[0]}..{window[1]}: derivation produced "
                f"{key}=0, which is a broken derivation rather than a result"
            )

    pattern = re.compile(
        r"(?P<functions>\d+)\s+test functions across\s+(?P<files>\d+)\s+files"
    )
    matches = list(pattern.finditer(text))
    if not matches:
        failures.append(
            f"{doc}: the guard-test count is no longer stated in a matchable form. "
            f"A registered claim must stay stated; derivation gives {derived}"
        )
    for m in matches:
        line_no = text[: m.start()].count("\n") + 1
        stated = {k: int(v) for k, v in m.groupdict().items()}
        if stated != derived:
            failures.append(
                f"{doc}:{line_no}: states {stated} for window "
                f"{window[0]}..{window[1]}, derivation gives {derived}"
            )

    entries = LIST_ENTRY.findall(text)
    if entries:
        if len(entries) != derived["functions"]:
            failures.append(
                f"{doc}: body lists {len(entries)} file:function entries but the "
                f"derivation gives {derived['functions']}"
            )
    elif PLACEHOLDER not in text:
        failures.append(
            f"{doc}: no list entries and no {PLACEHOLDER!r} declaration. An empty "
            f"guard list is a failed derivation, never a result."
        )


def check_repo_count(failures: list) -> None:
    doc = "spec/0.34.0.md"
    text = (REPO / doc).read_text(encoding="utf-8")
    derived = repo_test_functions()
    pattern = re.compile(r"repository holds\s+(?P<total>\d+)\s+test functions")
    matches = list(pattern.finditer(text))
    if not matches:
        failures.append(
            f"{doc}: the repository test-function count is no longer stated in a "
            f"matchable form; derivation gives {derived}"
        )
    for m in matches:
        line_no = text[: m.start()].count("\n") + 1
        stated = {k: int(v) for k, v in m.groupdict().items()}
        if stated != derived:
            failures.append(
                f"{doc}:{line_no}: states {stated}, derivation gives {derived}"
            )


def main() -> int:
    quiet = "--quiet" in sys.argv
    failures = []

    guard_doc = REPO / "spec/0.34.0-guard-tests.md"
    if not guard_doc.exists():
        failures.append("spec/0.34.0-guard-tests.md: no such file")
    else:
        check_guard_tests(guard_doc.read_text(encoding="utf-8"), failures)

    check_repo_count(failures)

    stream = sys.stderr if failures else sys.stdout
    for f in failures:
        print(f, file=sys.stderr)
    if failures:
        print(f"{len(failures)} stale or missing measured claim(s)", file=sys.stderr)
    if not quiet:
        print(f"coverage: {COVERAGE}", file=stream)
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
