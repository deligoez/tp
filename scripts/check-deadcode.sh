#!/bin/sh
# Fail when any function is unreachable from a main package or a test.
#
# golangci-lint's `unused` skips exported identifiers by design, so an exported
# function with zero callers passes the gate. This closes that hole: deadcode
# builds a call graph by rapid type analysis and reports what nothing reaches.
#
# -test is deliberate. Without it, a function only tests call reads as dead,
# which is a different signal worth looking at by hand but not worth failing a
# build over. With it, a finding means nothing in the tool or its tests reaches
# the function at all.
#
# deadcode exits 0 whether or not it finds anything, so the exit code is ours.
#
#   go install golang.org/x/tools/cmd/deadcode@latest

set -eu

out=$(deadcode -test ./...)

if [ -n "$out" ]; then
	echo "$out" >&2
	echo >&2
	echo "unreachable from any main package or test: delete it, or wire it up." >&2
	exit 1
fi
