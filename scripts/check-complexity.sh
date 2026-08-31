#!/bin/sh
# Fail when a function's cognitive complexity exceeds the threshold, unless it
# was already over it when the baseline was taken.
#
# WHY A BASELINE RATCHET AND NOT A LINTER SETTING
#
# Enabling gocognit in .golangci.yml at a meaningful threshold turns the gate
# red on 54 existing functions across 38 files — too dispersed for an exclusion
# list, and refactoring them is a release of its own, not a prerequisite for the
# next commit. golangci-lint's `new-from-rev` would scope the check to changed
# code, but it applies to EVERY linter, so buying this one would quietly stop
# `unused`, `dupl` and the rest from reporting on existing code. That trade is
# not worth making.
#
# So the debt is written down instead. baseline-complexity.txt lists every
# function that was already over the threshold; this script fails only on a
# function that is not in it. New code meets the real bar, existing code is
# visible and countable, and no other check is weakened. Shrinking the file is
# the work; it is meant to shrink.
#
# WHY COGNITIVE AND NOT CYCLOMATIC
#
# Measured on this repository at the same threshold: cyclomatic > 22 catches 22
# functions, cognitive > 22 catches 62 — six times as many, and the ones it adds
# are the nested-branching functions that are actually hard to hold in your head.
# Cyclomatic counts paths; cognitive counts the nesting that makes paths hard to
# follow. Adding gocyclo alongside would report a strict subset.
#
# WHY TEST FILES ARE EXCLUDED
#
# 8 of the 62 violations are in _test.go files, and on the clone side the same
# measurement put 83% of hits in test code. Table-driven and property tests are
# deliberately flat and repetitive; holding them to a complexity budget pushes
# them toward abstraction, which is the opposite of what makes a test readable
# when it fails. gocognit is given only non-test files.
#
#   go install github.com/uudashr/gocognit/cmd/gocognit@latest

set -eu

threshold=${TP_COGNIT_THRESHOLD:-22}
baseline=$(dirname "$0")/baseline-complexity.txt

if ! command -v gocognit >/dev/null 2>&1; then
	echo "gocognit not on PATH: go install github.com/uudashr/gocognit/cmd/gocognit@latest" >&2
	exit 1
fi

# gocognit prints "<score> <package> <func> <file>:<line>:<col>". The baseline
# key is package+function, deliberately without the line number: moving a
# function down a file is not a new violation, and keying on the line would
# make every edit above it look like one.
current=$(gocognit -over "$threshold" ./cmd ./internal 2>/dev/null |
	grep -v '_test\.go' |
	awk '{print $2 " " $3}' |
	sort -u)

known=$(grep -v '^#' "$baseline" 2>/dev/null | grep -v '^[[:space:]]*$' | sort -u || true)

new=$(printf '%s\n' "$current" | grep -vxF "$known" 2>/dev/null || true)

if [ -n "$new" ]; then
	echo "cognitive complexity over $threshold in code that was not already over it:" >&2
	printf '%s\n' "$new" >&2
	echo >&2
	echo "Split the function, or — if it genuinely has to be this shape — add it to" >&2
	echo "$baseline with a comment saying why." >&2
	exit 1
fi

# A function that left the baseline is a function that got better; keeping it
# listed hides the improvement and lets a future regression slip back in under
# its name.
stale=$(printf '%s\n' "$known" | grep -vxF "$current" 2>/dev/null || true)
if [ -n "$stale" ]; then
	echo "these are in $baseline but no longer over $threshold — delete their lines:" >&2
	printf '%s\n' "$stale" >&2
	exit 1
fi
