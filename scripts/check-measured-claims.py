#!/usr/bin/env python3
"""Re-run the derivations behind measured claims and fail when a claim is stale.

Mechanizes the `measured-claim-not-reproducible` review class, which survived
three prose corrections in the v0.34.0 review loop.

Every property below was added because a review round broke the version before
it. The tester role broke this check three times in a clone; the escapes it
found are named here so the next author does not reopen them:

1. A registered claim MUST be present. A claim reworded past its pattern fails.
2. The derivation RULE is read from the document, not held here -- both the tag
   window and the match rule. Holding either let the document state one rule
   while the check re-derived another and agreed with itself.
3. A claimed list is compared as a SET, not a count. Cardinality alone passed a
   body of 100 identical fabricated entries.
4. The unpopulated state is declared by a placeholder, and entries alongside
   that placeholder are a contradiction rather than an exemption. Appending a
   list without deleting the placeholder line used to disable the body check
   permanently -- over an artifact no reviewer ever sees, because it is produced
   after review closes.
5. The tree is read through `git ls-files`. Walking the working tree let an
   untracked scratch file fail the check, whose obvious repair would have been
   to edit the spec's number: a guard that teaches the wrong fix.

COVERAGE is stated by LOCATION, not by kind, because "re-derivable counts" reads
as a promise over the whole spec and this checks two sentence patterns in two
files. Everything else stays the reviewers' business. It prints on every run,
because the registration tells every reviewer to stop reporting the whole class.

Usage: check-measured-claims.py [--quiet]
Exit 0 when every registered claim reproduces, 1 otherwise.
"""

import re
import subprocess
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent

GUARD_DOC = "spec/0.34.0-guard-tests.md"
SPEC_DOC = "spec/0.34.0.md"

COVERAGE = (
    f"guards one claim only -- the guard-test window and list in {GUARD_DOC}, "
    f"against the window {SPEC_DOC} names. Every other number in either document "
    f"is unchecked and remains reportable. The repository test-function count was "
    f"guarded and is not: this release removes tests, so that number is falsified "
    f"by the work it describes and the guard taught editing it as the repair."
)

# The shape the documented derivation must keep. Pinning the flags is what makes
# dropping --no-ext-diff a failure: without it this repository's external diff
# driver emits a rendered view and the derivation silently returns zero.
DOCUMENTED_DIFF = re.compile(r"^git diff --no-ext-diff -U0 (\S+) (\S+)$", re.M)

# The window the SPEC names. Moving authority for the rule into the guard
# document was only half a fix: the check then verified that document's internal
# consistency and never its conformance to the spec, so a narrower window written
# into the artifact -- by the very task this check polices, after review closes --
# passed with a matching short list. The spec is the authority; the document must
# agree with it.
SPEC_WINDOW = re.compile(r"between the `(\S+)` and `(\S+)` tags")

# The match rule the document states. Pinned so that rewording the rule while
# leaving the command and the numbers intact fails.
DOCUMENTED_RULE = re.compile(r"`\^\\\+func Test`.{0,40}`_test\.go`")

# One file:function per line, bare or as a markdown bullet.
LIST_ENTRY = re.compile(r"^\s*(?:[-*]\s+)?`?([\w./-]+_test\.go):(Test\w+)`?\s*$", re.M)

# Entries count only under this heading. Reading the whole file let a written
# list of 20 pass while 80 more sat in a comment or a superseded draft: the
# document had no region the phrase "that list" referred to.
LIST_HEADING = re.compile(r"^## Guard tests\s*$", re.M)

# Anchored to the start of a line, not searched as a phrase. A bare substring
# test matched the sentence documenting the placeholder rule as well as the
# declaration itself, so deleting the declaration -- the instruction this file
# and the spec both give -- left the check at exit 1, and the empty-list branch
# could never fire. The guard was defeated by its own documentation.
PLACEHOLDER = "Populated by the first task"
PLACEHOLDER_LINE = re.compile(r"^Populated by the first task", re.M)

# Non-fatal notices: states the check cannot call verified but must not fail.
PENDING = []

COUNT_CLAIM = re.compile(
    r"(?P<functions>\d+)\s+test functions across\s+(?P<files>\d+)\s+files"
)


def added_test_functions(from_ref: str, to_ref: str) -> set:
    out = subprocess.run(
        ["git", "diff", "--no-ext-diff", "-U0", from_ref, to_ref],
        cwd=REPO, capture_output=True, text=True, check=True,
    ).stdout

    added = set()
    current = None
    for line in out.splitlines():
        if line.startswith("+++ b/"):
            current = line[6:].strip()
        if current and current.endswith("_test.go"):
            m = re.match(r"\+func (Test\w+)", line)
            if m:
                added.add((current, m.group(1)))
    return added




def check_guard_doc(text: str, spec_text: str, fail) -> None:
    window = DOCUMENTED_DIFF.search(text)
    if not window:
        fail(
            f"{GUARD_DOC}: the documented derivation no longer matches "
            f"`git diff --no-ext-diff -U0 <from> <to>`. Either the window moved or "
            f"--no-ext-diff was dropped; both change what the numbers below mean."
        )
        return

    spec_window = SPEC_WINDOW.search(spec_text)
    if not spec_window:
        fail(
            f"{SPEC_DOC}: the guard-test window is no longer stated as "
            f"“between the `<from>` and `<to>` tags”, so the document's window "
            f"cannot be checked against it."
        )
        return
    if (window.group(1), window.group(2)) != (spec_window.group(1), spec_window.group(2)):
        fail(
            f"{GUARD_DOC}: derives over "
            f"{window.group(1)}..{window.group(2)} while {SPEC_DOC} names "
            f"{spec_window.group(1)}..{spec_window.group(2)}. The spec is the "
            f"authority for the sweep's scope."
        )
        return
    if not DOCUMENTED_RULE.search(text):
        fail(
            f"{GUARD_DOC}: the documented match rule no longer states "
            f"`^\\+func Test` over `_test.go`. The numbers below are derived under "
            f"that rule; restating the rule without re-deriving them is the failure "
            f"this guards."
        )
        return

    from_ref, to_ref = window.group(1), window.group(2)
    derived = added_test_functions(from_ref, to_ref)
    if not derived:
        fail(
            f"guard-test window {from_ref}..{to_ref}: derivation produced nothing, "
            f"which is a broken derivation rather than a result"
        )

    counts = {"functions": len(derived), "files": len({f for f, _ in derived})}
    claims = list(COUNT_CLAIM.finditer(text))
    if not claims:
        fail(
            f"{GUARD_DOC}: the guard-test count is no longer stated in a matchable "
            f"form. A registered claim must stay stated; derivation gives {counts}"
        )
    for m in claims:
        line_no = text[: m.start()].count("\n") + 1
        stated = {k: int(v) for k, v in m.groupdict().items()}
        if stated != counts:
            fail(
                f"{GUARD_DOC}:{line_no}: states {stated} for window "
                f"{from_ref}..{to_ref}, derivation gives {counts}"
            )

    heading = LIST_HEADING.search(text)
    if not heading:
        fail(
            f"{GUARD_DOC}: no `## Guard tests` heading. Entries are read from that "
            f"section only, so without it the list has no determinate extent."
        )
        return
    body = text[heading.end():]
    entries = {(f, fn) for f, fn in LIST_ENTRY.findall(body)}
    placeholder = PLACEHOLDER_LINE.search(text) is not None

    if entries and placeholder:
        fail(
            f"{GUARD_DOC}: {len(entries)} list entries alongside the declaration "
            f"line starting {PLACEHOLDER!r}. Delete that line when the list is "
            f"written -- leaving it suppresses every check below. Mentions of the "
            f"phrase inside a sentence do not count; only a line that starts with it."
        )
        return
    if not entries:
        if not placeholder:
            fail(
                f"{GUARD_DOC}: no list entries and no {PLACEHOLDER!r} declaration. "
                f"An empty guard list is a failed derivation, never a result."
            )
        else:
            # Legitimate while the first task has not run: the list is produced
            # after review closes. It must never read as verified, though --
            # this check cannot tell "declared pending" from "derivation failed",
            # so it says so out loud and the task's acceptance owns the rest.
            PENDING.append(
                f"{GUARD_DOC}: list PENDING -- {len(derived)} entries derived, 0 "
                f"written. Nothing below the summary is verified until the first "
                f"task writes the list and deletes the {PLACEHOLDER!r} line."
            )
        return

    missing = sorted(f"{f}:{fn}" for f, fn in derived - entries)
    extra = sorted(f"{f}:{fn}" for f, fn in entries - derived)
    if missing:
        fail(f"{GUARD_DOC}: derived but not listed ({len(missing)}): {missing[:5]}")
    if extra:
        fail(f"{GUARD_DOC}: listed but not derived ({len(extra)}): {extra[:5]}")


def main() -> int:
    quiet = "--quiet" in sys.argv
    failures = []

    guard = REPO / GUARD_DOC
    if not guard.exists():
        failures.append(f"{GUARD_DOC}: no such file")
    else:
        check_guard_doc(
            guard.read_text(encoding="utf-8"),
            (REPO / SPEC_DOC).read_text(encoding="utf-8"),
            failures.append,
        )

    for p in PENDING:
        print(f"PENDING: {p}", file=sys.stderr)
    for f in failures:
        print(f, file=sys.stderr)
    if failures:
        print(f"{len(failures)} stale or missing measured claim(s)", file=sys.stderr)
    if not quiet:
        print(f"coverage: {COVERAGE}", file=sys.stderr if failures else sys.stdout)
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
