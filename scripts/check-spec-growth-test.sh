#!/usr/bin/env bash
# Fixtures for check-spec-growth.py: one throwaway git repository whose spec
# is recorded at round 1 with a forensics sidecar beside it, then grown,
# shrunk and restored one way at a time.
#
# The main fixture commits a non-empty sidecar with the round-1 snapshot, so
# "same size exits 0" only passes when the sidecar's baseline is read from that
# commit: a script that took the baseline sidecar as 0 would see growth equal to
# the sidecar's size and exit 1. The grown-sidecar case commits the growth, so
# a script reading the sidecar at HEAD rather than at the snapshot's commit
# would see no growth and exit 0. Each case is built so the wrong reading fails
# it, which is the only way a passing case says anything.
#
# An untracked round-1 snapshot (its state between emission and --record) and
# a missing git take the two untracked arms: at round 1 the sidecar is measured
# against itself, and from round 2 on it is not checked and the check says so.
# The no-git fixture's snapshot is committed and its sidecar grown, so the same
# input fails with git on PATH — the no-git verdicts are git's absence.
#
# Exit codes are tp's `workflow.checks` contract (engine.CheckRan): 0 clean,
# 1 grew, anything else could not run — so every usage error must be 2, never 1.
#
# Everything happens in a throwaway repository under mktemp. The git variables
# a caller's own repository can export (GIT_DIR, GIT_INDEX_FILE, ... — set, for
# one, inside a git hook) are cleared first, or the fixture's `git add` and
# `git commit` would land in that repository instead.
#
# Run: scripts/check-spec-growth-test.sh
set -u
# shellcheck disable=SC2046
unset $(git rev-parse --local-env-vars)

here=$(cd "$(dirname "$0")" && pwd)
script="$here/check-spec-growth.py"
failures=0

pass() { printf 'ok   %s\n' "$1"; }
fail() {
	printf 'FAIL %s\n' "$1"
	shift
	[ $# -gt 0 ] && printf '%s\n' "$*" | sed 's/^/       /'
	failures=$((failures + 1))
}

check_eq() {
	# check_eq <label> <want> <got>
	if [ "$2" = "$3" ]; then
		pass "$1 = $3"
	else
		fail "$1" "want [$2], got [$3]"
	fi
}

check_has() {
	# check_has <label> <needle> <file>
	case "$(cat "$3")" in
	*"$2"*) pass "$1" ;;
	*) fail "$1" "$(cat "$3")" ;;
	esac
}

check_lacks() {
	# check_lacks <label> <needle> <file>
	case "$(cat "$3")" in
	*"$2"*) fail "$1" "$(cat "$3")" ;;
	*) pass "$1" ;;
	esac
}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
repo="$tmp/repo"
mkdir -p "$repo/spec/.tp-review/x"

cd "$repo" || exit 1
git init -q .
git config user.email tester@example.invalid
git config user.name Tester
git config commit.gpgsign false

# growth <spec> — run the script, leaving its exit code in $code and its
# streams in $tmp/out.txt and $tmp/err.txt.
growth() {
	python3 "$script" "$@" >"$tmp/out.txt" 2>"$tmp/err.txt"
	code=$?
}

# 1. No round-1 snapshot yet: nothing to compare, and stderr says so.
printf '# x\n\n## 1. Thing\n\nA thing.\n' >spec/x.md
printf 'derivation one\nderivation two\n' >spec/x-measurements.md
growth spec/x.md
check_eq "no snapshot exit code" 0 "$code"
check_has "no snapshot is said on stderr" "no round-1 snapshot" "$tmp/err.txt"

# Round 1 is recorded: the snapshot and the sidecar land in one commit.
# 5 body lines + 2 sidecar lines = a baseline of 7.
cp spec/x.md spec/.tp-review/x/snapshot-round-1.md
git add -A
git commit -qm "record review round 1 of x"

# 2. Same size: the sidecar's 2 lines are read from the record commit.
growth spec/x.md
check_eq "same size exit code" 0 "$code"
check_has "same size reports a zero delta" "baseline 7, now 7, delta +0" "$tmp/out.txt"

# 3. Shrunk: a repair that deletes a line.
printf '# x\n\n## 1. Thing\n\n' >spec/x.md
growth spec/x.md
check_eq "shrunk exit code" 0 "$code"
check_has "shrunk reports a negative delta" "delta -1" "$tmp/out.txt"

# Text moved from the body into the sidecar is neither growth nor shrinkage.
printf 'derivation one\nderivation two\nA thing.\n' >spec/x-measurements.md
growth spec/x.md
check_eq "body-to-sidecar move exit code" 0 "$code"
git checkout -q -- spec/x.md spec/x-measurements.md

# 4. Grown body: one added requirement line, uncommitted.
printf 'A second requirement.\n' >>spec/x.md
growth spec/x.md
check_eq "grown body exit code" 1 "$code"
check_eq "grown body prints one line" 1 "$(wc -l <"$tmp/out.txt" | tr -d ' ')"
check_has "grown body names spec, baseline, now and delta" \
	"spec/x.md grew past round 1: baseline 7, now 8, delta +1 (body 5->6, sidecar 2->2)" "$tmp/out.txt"
git checkout -q -- spec/x.md

# 5. Grown sidecar, committed: the baseline is the record commit, not HEAD.
printf 'derivation three\n' >>spec/x-measurements.md
git add -A
git commit -qm "grow the sidecar after round 1"
growth spec/x.md
check_eq "grown sidecar exit code" 1 "$code"
check_has "grown sidecar is attributed to the sidecar" "(body 5->5, sidecar 2->3)" "$tmp/out.txt"

# A snapshot deleted and added back has two adds; the oldest is round 1's.
git rm -q spec/.tp-review/x/snapshot-round-1.md
git commit -qm "drop the snapshot"
mkdir -p spec/.tp-review/x
cp spec/x.md spec/.tp-review/x/snapshot-round-1.md
git add -A
git commit -qm "restore the snapshot"
growth spec/x.md
check_eq "re-added snapshot exit code" 1 "$code"
check_has "re-added snapshot keeps the oldest add's sidecar" "sidecar 2->3" "$tmp/out.txt"

# A sidecar created after round 1 was absent from the record commit: its
# baseline is 0, which is git's answer rather than a fallback, so no note.
mkdir -p spec/.tp-review/y
printf '# y\n' >spec/y.md
cp spec/y.md spec/.tp-review/y/snapshot-round-1.md
git add -A
git commit -qm "record review round 1 of y"
printf 'a derivation written later\n' >spec/y-measurements.md
growth spec/y.md
check_eq "sidecar added after round 1 exit code" 1 "$code"
check_has "sidecar added after round 1 counts from 0" "(body 1->1, sidecar 0->1)" "$tmp/out.txt"
check_eq "an absent sidecar is not a fallback" "" "$(cat "$tmp/err.txt")"

# An untracked round-1 snapshot, round 1 the latest emitted: tp has just
# written it and --record has not committed it yet. The sidecar's baseline is
# its working-tree size, because growth against round 1 at round 1 is zero by
# definition — so a sidecar present at round 1 is not reported as growth.
mkdir -p spec/.tp-review/z
printf '# z\n' >spec/z.md
printf 'derivation\n' >spec/z-measurements.md
cp spec/z.md spec/.tp-review/z/snapshot-round-1.md
growth spec/z.md
check_eq "round 1, untracked snapshot, sidecar present exit code" 0 "$code"
check_has "round 1, untracked snapshot reports no growth" \
	"baseline 2, now 2, delta +0 (body 1->1, sidecar 1->1)" "$tmp/out.txt"
check_lacks "round 1, untracked snapshot is not a sidecar-not-checked notice" \
	"sidecar not checked" "$tmp/err.txt"

# Round 2 emitted and round 1's snapshot still untracked: the sidecar's round-1
# size cannot be known. It is left out of both sides and the check says so on
# stderr and in its line; the body alone decides the exit.
cp spec/z.md spec/.tp-review/z/snapshot-round-2.md
printf 'derivation two\nderivation three\n' >>spec/z-measurements.md
growth spec/z.md
check_eq "round 2, untracked round-1 snapshot exit code" 0 "$code"
check_has "round 2, untracked round-1 snapshot is said on stderr" \
	"sidecar not checked: the round-1 snapshot is not committed" "$tmp/err.txt"
check_has "round 2, untracked round-1 snapshot is said in the line" \
	"sidecar not checked: the round-1 snapshot is not committed" "$tmp/out.txt"
check_has "round 2, the sidecar is not counted" "baseline 1, now 1, delta +0" "$tmp/out.txt"
printf 'A second requirement.\n' >>spec/z.md
growth spec/z.md
check_eq "round 2, untracked round-1 snapshot, grown body exit code" 1 "$code"
check_has "round 2, the grown body is what is reported" "baseline 1, now 2, delta +1" "$tmp/out.txt"

# git unavailable reads as an untracked snapshot, under the same two arms. The
# fixture's round-1 snapshot IS committed and its sidecar has grown since, so
# with git the check fails: the no-git verdicts below come from the missing
# git, not from a fixture that could not fail.
mkdir -p spec/.tp-review/w "$tmp/nogit"
ln -s "$(command -v python3)" "$tmp/nogit/python3"
printf '# w\n' >spec/w.md
printf 'derivation\n' >spec/w-measurements.md
cp spec/w.md spec/.tp-review/w/snapshot-round-1.md
git add spec/w.md spec/w-measurements.md spec/.tp-review/w
git commit -qm "record review round 1 of w"
printf 'derivation two\n' >>spec/w-measurements.md
nogit() {
	PATH="$tmp/nogit" "$tmp/nogit/python3" "$script" "$@" >"$tmp/out.txt" 2>"$tmp/err.txt"
	code=$?
}
growth spec/w.md
check_eq "with git, the committed fixture's grown sidecar exit code" 1 "$code"
nogit spec/w.md
check_eq "no git, round 1 exit code" 0 "$code"
check_has "no git, round 1 takes the working-tree sidecar" "(body 1->1, sidecar 2->2)" "$tmp/out.txt"
check_has "no git is said on stderr" "git is not available" "$tmp/err.txt"
cp spec/w.md spec/.tp-review/w/snapshot-round-2.md
nogit spec/w.md
check_eq "no git, round 2 exit code" 0 "$code"
check_has "no git, round 2 is a sidecar-not-checked notice" \
	"sidecar not checked: the round-1 snapshot is not committed" "$tmp/err.txt"
printf 'A second requirement.\n' >>spec/w.md
nogit spec/w.md
check_eq "no git, round 2, grown body exit code" 1 "$code"

# A repository with no commit yet tracks nothing, so its round 1 is the
# untracked arm — not a git failure, although `git log` exits 128 there.
fresh="$tmp/fresh"
mkdir -p "$fresh/spec/.tp-review/f"
git -C "$fresh" init -q .
printf '# f\n' >"$fresh/spec/f.md"
printf 'derivation\n' >"$fresh/spec/f-measurements.md"
cp "$fresh/spec/f.md" "$fresh/spec/.tp-review/f/snapshot-round-1.md"
growth "$fresh/spec/f.md"
check_eq "round 1 in a repository with no commit exit code" 0 "$code"
check_has "round 1 in a repository with no commit counts the sidecar" "(body 1->1, sidecar 1->1)" "$tmp/out.txt"

# 6. Bad usage: every form exits 2, so tp reads it as "could not run".
growth
check_eq "no argument exit code" 2 "$code"
growth spec/x.md spec/y.md
check_eq "two arguments exit code" 2 "$code"
growth --stats
check_eq "unknown flag exit code" 2 "$code"
growth ""
check_eq "empty spec path exit code" 2 "$code"
check_has "empty spec path names the command upstream" "printed nothing" "$tmp/err.txt"
growth spec/missing.md
check_eq "missing spec exit code" 2 "$code"
check_has "missing spec is named" "no such spec: spec/missing.md" "$tmp/err.txt"

if [ "$failures" != 0 ]; then
	printf '%s assertion(s) failed\n' "$failures"
	exit 1
fi
echo "all assertions passed"
