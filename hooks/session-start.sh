#!/bin/sh
# session-start.sh - the tp plugin's SessionStart hook (v0.35.0 sections 6.1 and 6.2).
#
# It is the only hook that fires before a unit does any work, which makes it the
# one place a missing or stale tp can be reported rather than discovered halfway
# through an unattended run. The plugin deliberately ships no binary, so the
# remedy it can offer is the install command; the minimum it compares against is
# plugin.json's own version.
#
# On a current tp it prints `tp resume --compact` and nothing else. Claude Code
# adds a SessionStart hook's stdout to the session's context, so the orientation
# arrives without the agent having to ask for it. The hook does not branch on
# the matcher: `clear` is not reliably reported on every client, and an
# orientation that is occasionally redundant costs far less than one that is
# occasionally missing.

set -u

install_hint='Install tp with one of:
  brew tap deligoez/tap && brew install tp
  go install github.com/deligoez/tp/cmd/tp@latest'

# fail reports to the operator and stops the hook. Exit 2 on SessionStart shows
# stderr to the user and does not block the session: the point is that a stale
# or absent tp is stated rather than degrading quietly into confusing failures
# later on.
fail() {
	printf '%s\n%s\n' "$1" "$install_hint" >&2
	exit 2
}

plugin_root=${CLAUDE_PLUGIN_ROOT:-}
if [ -z "$plugin_root" ]; then
	# A client that does not export the variable still leaves this script at a
	# known place inside the plugin, and plugin.json is what carries the minimum.
	plugin_root=$(cd -- "$(dirname -- "$0")/.." && pwd) ||
		fail 'The tp plugin could not locate its own root.'
fi
manifest="$plugin_root/.claude-plugin/plugin.json"

# version_number reduces a reported version to its comparable dotted numbers: a
# release prints "v0.35.0" and a build from a working tree prints something like
# "v0.35.0-0.20260820093420-104822c4904b+dirty". Only the numbers are ordered,
# and a development build of the minimum still satisfies it. Note what that does
# and does not cover: a build made from a working tree AFTER the minimum's tag
# passes, but one made BEFORE it reports the previous release's number and is
# refused. tp's own pre-tag development window is therefore refused by its own
# preflight, and self-heals at the tag - correct behaviour for a version gate,
# and named here because an earlier draft of this comment claimed otherwise.
version_number() {
	printf '%s' "$1" | sed -e 's/^[vV]//' -e 's/[-+].*$//'
}

# version_part prints one dot-separated component as a number, or 0 when the
# version is shorter than the index or the component is not numeric.
version_part() {
	part=$(printf '%s' "$1" | cut -d. -f"$2")
	case $part in
	'' | *[!0-9]*) printf '0' ;;
	*) printf '%s' "$part" ;;
	esac
}

# version_below succeeds when the first version is strictly below the second.
version_below() {
	index=1
	while [ "$index" -le 3 ]; do
		left=$(version_part "$1" "$index")
		right=$(version_part "$2" "$index")
		if [ "$left" -lt "$right" ]; then
			return 0
		fi
		if [ "$left" -gt "$right" ]; then
			return 1
		fi
		index=$((index + 1))
	done
	return 1
}

tp_path=$(command -v tp 2>/dev/null) || tp_path=''
if [ -z "$tp_path" ]; then
	fail 'tp is not on PATH, and the tp plugin does not ship the binary.'
fi

minimum=$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$manifest" 2>/dev/null | head -n 1)
if [ -z "$minimum" ]; then
	fail "The tp plugin could not read its minimum version from $manifest."
fi

reported=$(tp --version 2>/dev/null | head -n 1)
reported=${reported##* }
if [ -z "$reported" ]; then
	fail "tp at $tp_path reported no version; the tp plugin needs $minimum or newer."
fi

if version_below "$(version_number "$reported")" "$(version_number "$minimum")"; then
	fail "tp $reported at $tp_path is older than the tp plugin's minimum $minimum."
fi

# Past the preflight the payload is one command's compact output, so the size of
# what this hook injects is bounded by a surface that already exists. A session
# that is not a tp cycle has nothing to orient with, which is not a preflight
# failure - the plugin has to stay usable in any repository.
orientation=$(tp resume --compact 2>/dev/null) || exit 0
[ -n "$orientation" ] || exit 0
printf '%s\n' "$orientation"
