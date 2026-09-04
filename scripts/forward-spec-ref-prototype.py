#!/usr/bin/env python3
"""Prototype for the `forward-spec-ref` lint rule.

Reports a `spec/<version>.md` reference whose target is numbered ABOVE the
referrer AND above the shipped boundary. Both conditions are required: a
shipped spec naming a later spec that has since shipped is frozen at both ends
and cannot rot, and flagging it is the measured false positive.

References inside fenced blocks are skipped. The prototype's first version did
not skip them and flagged a shell transcript — a fixture spec named in example
output, not a reference to anything.

A file whose name is not a version yields no self-version and is skipped
entirely; a caller that cannot determine the shipped boundary should report
nothing rather than guess.

    scripts/forward-spec-ref-prototype.py spec 1.0.0
    scripts/forward-spec-ref-prototype.py spec 0.0.0 --no-boundary
    scripts/forward-spec-ref-prototype.py spec 1.0.0 --no-fence-skip

The counts this prints move with the corpus and with the boundary. Do not copy
one into a comment; run the command.
"""
import os
import re
import sys

REF = re.compile(r'spec/(\d+\.\d+\.\d+)\.md')
SELF = re.compile(r'^(\d+\.\d+\.\d+)\.md$')


def ver(s):
    return tuple(int(p) for p in s.split('.'))


def refs(path, skip_fences=True):
    """Yield (lineno, target) for every spec reference in one file."""
    with open(path, encoding='utf-8') as fh:
        lines = fh.read().split('\n')
    fenced = False
    for lineno, line in enumerate(lines, 1):
        if line.lstrip().startswith('```'):
            fenced = not fenced
            continue
        if fenced and skip_fences:
            continue
        for m in REF.finditer(line):
            yield lineno, m.group(1)


def main():
    if len(sys.argv) < 3:
        sys.exit(__doc__)
    specdir, boundary = sys.argv[1], ver(sys.argv[2])
    skip_fences = '--no-fence-skip' not in sys.argv
    use_boundary = '--no-boundary' not in sys.argv

    findings, skipped = [], []
    for name in sorted(os.listdir(specdir)):
        if not name.endswith('.md'):
            continue
        m = SELF.match(name)
        if not m:
            skipped.append(name)
            continue
        self_v = ver(m.group(1))
        for lineno, target in refs(os.path.join(specdir, name), skip_fences):
            t = ver(target)
            if t > self_v and (not use_boundary or t > boundary):
                findings.append((name, lineno, target))

    for f in findings:
        print('%s:%d -> spec/%s.md' % f)
    print('--- %d findings; %d non-version files skipped'
          % (len(findings), len(skipped)))


if __name__ == '__main__':
    main()
