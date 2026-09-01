#!/usr/bin/env bash
# check-suite-state.sh — run the test suite and prove it mutated no live review state.
#
# v0.36.0 §6.1 S1: `spec/.tp-review/` and `.tp/rounds/` must be byte-identical
# after the suite runs. The suite READS them — the release gate emits from every
# spec in the repository — so the forbidden thing is mutation, not access.
#
# Why this is a script and not a test. No test inside the suite can observe the
# suite: whatever it hashes, the tests scheduled after it have not run yet, and
# `go test` gives no after-everything hook that spans packages (§6.3 records the
# TestMain bracket as withdrawn for exactly that reason — TestMain is per-package
# and these tests span packages). So the observer has to sit outside the process
# that runs them, which is here.
#
# It WRAPS the suite rather than running beside it: hashing before and after one
# run is the only way to attribute a change to the suite, and running the suite
# twice to keep the gate's own `go test` step would double the slowest step of
# the gate for nothing.
#
# What a failure means: a test emitted against a spec still sitting in the
# repository instead of a relocated copy. `engine.ReviewStateDir` derives
# `<spec-dir>/.tp-review/<base>` from the spec's own path with no flag or env
# override, so moving the spec is what moves the state. `--no-state` is NOT the
# fix — it disables the reads the tests measure (the round number,
# skipped_roles, the consecutive-clean count, whether `regression` is appended),
# so a suite run under it passes while measuring a machine that was switched
# off.
set -euo pipefail

cd "$(dirname "$0")/.."

WATCHED=(spec/.tp-review .tp/rounds)

# Hash names and contents together: a file added or removed changes the digest
# even when every surviving file is untouched. Missing directories are legal —
# .tp/rounds/ exists only after a `tp run` — and hash to nothing.
#
# KNOWN BLIND SPOT, measured in audit round 3: an EMPTY watched directory hashes
# identically to a missing one, because BSD xargs runs nothing on empty input.
# So a run that creates a directory and leaves it empty is NOT caught, and
# engine.clearUnitArtifacts does exactly that MkdirAll. An earlier version of
# this comment claimed the opposite. The blindness is to directory-only changes:
# every file written under either path is still caught, and review state lives in
# files, so S1's actual hazard — a test advancing a round it does not own — is
# detected. Routed to a later release rather than repaired here; what could not
# stand was the claim.
state_digest() {
	local dir
	for dir in "${WATCHED[@]}"; do
		if [ -d "$dir" ]; then
			find "$dir" -type f -print0 | LC_ALL=C sort -z | xargs -0 shasum -a 256
		fi
	done | shasum -a 256 | cut -d' ' -f1
}

before=$(state_digest)

# The suite itself. "$@" lets a caller narrow the run (a package, a -run
# pattern) while keeping the measurement; with no arguments it is the gate's
# own test step.
if [ "$#" -gt 0 ]; then
	go test "$@"
else
	go test -race ./...
fi

after=$(state_digest)

if [ "$before" != "$after" ]; then
	echo "FAIL: the test suite mutated live review state" >&2
	echo "  watched: ${WATCHED[*]}" >&2
	echo "  before:  $before" >&2
	echo "  after:   $after" >&2
	echo "" >&2
	echo "  What changed (git):" >&2
	git status --short -- "${WATCHED[@]}" >&2 || true
	echo "" >&2
	echo "  A test emitted against a spec still in the repository. Copy the spec" >&2
	echo "  and its .tp-review/<base> into t.TempDir() first — relocatedSpec in" >&2
	echo "  internal/cli/sandbox_helpers_test.go does this. Do NOT reach for" >&2
	echo "  --no-state: it disables the state the tests measure." >&2
	exit 1
fi
