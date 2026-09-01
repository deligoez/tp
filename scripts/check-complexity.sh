#!/bin/sh
# Fail when a function exceeds a size or complexity threshold, unless it was
# already over it when the baseline was taken. Two measures, two baselines:
# cognitive complexity via gocognit, and length/statement count via funlen.
#
# WHY A BASELINE RATCHET AND NOT A LINTER SETTING
#
# Enabling gocognit in .golangci.yml at a meaningful threshold turns the gate
# red on 54 existing functions across 38 files, and funlen on 71 more — too
# dispersed for an exclusion list, and refactoring them is a release of its own,
# not a prerequisite for the next commit. golangci-lint's `new-from-rev` would
# scope the check to changed code, but it applies to EVERY linter, so buying
# this one would quietly stop `unused`, `dupl` and the rest from reporting on
# existing code. That trade is not worth making.
#
# The rejected third option was a `//nolint:gocognit,funlen` comment on each
# offender plus `nolintlint`. It gives the same "the 126th violation fails
# immediately" property, and it puts the number at the top of the function where
# a reader sees it — but it means editing every currently-violating function to buy
# it, and every one of those edits is a chance to suppress more than intended.
# A ratchet keeps the same property with the debt in two files nobody has to
# read to understand the code.
#
# So the debt is written down instead. The baseline files list every function
# that was already over threshold; this script fails only on a function that is
# not in them. New code meets the real bar, existing code is visible and
# countable, and no other check is weakened. Shrinking the files is the work.
#
# WHY COGNITIVE AND NOT CYCLOMATIC
#
# Measured on this repository at the same threshold: cyclomatic > 22 catches 22
# functions, cognitive > 22 catches 62 — six times as many, and the ones it adds
# are the nested-branching functions that are actually hard to hold in your head.
# Cyclomatic counts paths; cognitive counts the nesting that makes paths hard to
# follow. Adding gocyclo alongside would report a strict subset.
#
# WHY funlen IS RUN ALONE, AND WHY uniq-by-line IS PASSED
#
# golangci-lint's `issues.uniq-by-line` defaults to TRUE: one issue per line.
# gocognit and funlen both report on the function's declaration line, so running
# them together silently drops every funlen issue on a function gocognit already
# flagged. This script sidesteps it by running funlen with
# --default=none so nothing else is enabled, and passes --uniq-by-line=false
# anyway so the invocation stays correct if a linter is ever added to it.
#
# funlen also emits TWO messages — "is too long" and "has too many statements".
# Matching only one would silently permit the other; the
# extraction below matches both and collapses them to one key per function.
#
# WHY TEST FILES ARE EXCLUDED
#
# A substantial minority of both tools' violations land in _test.go files.
# Table-driven and property tests are deliberately flat and repetitive; holding
# them to a complexity budget pushes them toward abstraction, which is the
# opposite of what makes a test readable when it fails.
#
#   go install github.com/uudashr/gocognit/cmd/gocognit@latest

set -eu

threshold=${TP_COGNIT_THRESHOLD:-22}
scripts=$(dirname "$0")

# compare <label> <baseline-file> <current-keys>
# Fails on a key not in the baseline, and equally on a baseline line that no
# longer violates: a function that got better must leave the list, or the
# improvement is hidden and a future regression slips back in under its name.
compare() {
	label=$1
	baseline=$2
	current=$3

	known=$(grep -v '^#' "$baseline" 2>/dev/null | grep -v '^[[:space:]]*$' | sort -u || true)

	new=$(printf '%s\n' "$current" | grep -vxF "$known" 2>/dev/null || true)
	if [ -n "$new" ]; then
		echo "$label in code that was not already over the threshold:" >&2
		printf '%s\n' "$new" >&2
		echo >&2
		echo "Split the function, or — if it genuinely has to be this shape — add it" >&2
		echo "to $baseline with a comment saying why." >&2
		exit 1
	fi

	stale=$(printf '%s\n' "$known" | grep -vxF "$current" 2>/dev/null || true)
	if [ -n "$stale" ]; then
		echo "these are in $baseline but no longer violate — delete their lines:" >&2
		printf '%s\n' "$stale" >&2
		exit 1
	fi
}

# --- cognitive complexity -------------------------------------------------
#
# gocognit prints "<score> <package> <func> <file>:<line>:<col>". The baseline
# key is package+function, deliberately without the line number: moving a
# function down a file is not a new violation, and keying on the line would
# make every edit above it look like one.

if ! command -v gocognit >/dev/null 2>&1; then
	echo "gocognit not on PATH: go install github.com/uudashr/gocognit/cmd/gocognit@latest" >&2
	exit 1
fi

# Run the tool and the pipeline separately. Inside $( … ) ending in `sort -u`,
# $? is sort's, so `set -e` never fires on the tool: audit round 1 measured
# golangci-lint dying with "go command required, not found" while the ratchet
# reported all 71 funlen entries as no-longer-violating and told the operator to
# delete them. Obeying that message disarms half the ratchet. A gate that fails
# open is worse than no gate, so each tool's exit status is checked on its own.
# gocognit's EXIT CODE cannot separate a result from a failure — measured:
# violations exit 1, a parse error exits 1, a missing directory exits 1. An
# earlier version of this guard claimed "127 when it cannot run"; that 127 was
# the SHELL's command-not-found, because the experiment removed gocognit from
# PATH rather than breaking gocognit. It measured the shell and was recorded as
# the tool's convention.
#
# What does discriminate, measured on all four cases: gocognit writes to stderr
# ONLY on failure. Violations go to stdout with stderr empty; a failure leaves
# stdout empty and stderr non-empty.
cognit_err=$(mktemp)
cognit_raw=$(gocognit -over "$threshold" ./cmd ./internal 2>"$cognit_err") || true
if [ -s "$cognit_err" ]; then
	echo "gocognit failed to run; the complexity ratchet cannot be evaluated:" >&2
	cat "$cognit_err" >&2
	rm -f "$cognit_err"
	exit 1
fi
rm -f "$cognit_err"

# `NF &&` is load-bearing: with zero violations gocognit prints nothing, and a
# bare `{print $2 " " $3}` turns that empty line into a single space, which the
# ratchet then reports as a new violation named " ". Reachable today through
# TP_COGNIT_THRESHOLD, and permanently the day the baseline empties — which this
# script's own header calls the work.
cognit=$(printf '%s\n' "$cognit_raw" |
	grep -v '_test\.go' |
	awk 'NF {print $2 " " $3}' |
	sort -u)

compare "cognitive complexity over $threshold" "$scripts/baseline-complexity.txt" "$cognit"

# --- function length ------------------------------------------------------
#
# funlen's key is the package DIRECTORY plus the function, because its output
# gives a path rather than a package name. Same stability property: it survives
# a function moving within its file.

if ! command -v golangci-lint >/dev/null 2>&1; then
	echo "golangci-lint not on PATH" >&2
	exit 1
fi

# Three steps rather than one expression: path+function, drop _test.go, then
# strip the file name to leave the directory.
#
# The two message forms get one -e each rather than a `\(a\|b\)` alternation:
# BSD sed has no alternation in a basic regex, so the combined expression
# matches nothing on macOS while working on GNU sed in CI. Caught here because
# the ratchet's stale half failed loudly on an empty result — a check that only
# looked for NEW violations would have passed vacuously and measured nothing.
# golangci-lint exits 1 when it REPORTS issues and 3+ when it fails to run, so
# only the latter is a tool failure — the same distinction the gocognit check
# above makes with its plain exit status.
funlen_raw=$(golangci-lint run --default=none --enable=funlen \
	--max-issues-per-linter=0 --max-same-issues=0 --uniq-by-line=false) || funlen_status=$?
if [ "${funlen_status:-0}" -gt 1 ]; then
	echo "golangci-lint failed to run (exit ${funlen_status}); the funlen ratchet cannot be evaluated" >&2
	exit 1
fi
funlen=$(printf '%s\n' "$funlen_raw" |
	sed -n \
		-e "s|^\(.*\)\.go:[0-9]*:[0-9]*: Function '\([^']*\)' is too long.*|\1 \2|p" \
		-e "s|^\(.*\)\.go:[0-9]*:[0-9]*: Function '\([^']*\)' has too many statements.*|\1 \2|p" |
	grep -v '^[^ ]*_test ' |
	sed 's|/[^/ ]* | |' |
	sort -u)

compare "function length or statement count over funlen's thresholds" "$scripts/baseline-funlen.txt" "$funlen"
