#!/bin/sh
# stop-role-incomplete.sh - the tp plugin's Stop hook (v0.35.0 sections 6.2 and 6.4).
#
# A role unit that stops without writing its findings file has produced nothing
# the driver can record, and the driver only learns that after the child is
# gone. This hook is the one place the unit itself can still be told, while it
# is still running.
#
# It is deliberately scoped to `review-role` and `audit-role`, the two kinds
# whose durable write is a file at a path the environment already names. It
# fires when TP_UNIT_KIND ends in `-role` and applies section 3.3's predicate to
# $TP_ROUND_DIR/role-$TP_UNIT_ID.ndjson.part - present and wholly parseable -
# alongside the escalation record at $TP_RUN_DIR/$TP_UNIT_SEQ-escalation.json.
# The `.part` name is the one the hook can see: it runs inside the child, before
# the driver's rename to the final name. Every other kind's predicate needs tp's
# own state readers, which is the driver's job, not a hook's, so this hook never
# reads the run state and never fires for them.
#
# Claude Code sends the Stop payload on stdin as JSON and reads exit 2 as a
# refusal to stop, with stderr as the reason the agent is given. It also reports
# `stop_hook_active` once a stop has already been blocked, and this hook stands
# down on it: a hook that always blocks produces an agent that can never finish,
# which is the loop this whole version exists to make terminating.

set -u

payload=$(cat)

# Only the two role kinds. An unset or non-role TP_UNIT_KIND is not this hook's
# business, and a session outside a run has no unit at all.
case ${TP_UNIT_KIND:-} in
*-role) ;;
*) exit 0 ;;
esac

# Block at most once per unit. The Stop payload carries no tool arguments and no
# file content, so the field can be read with grep rather than a JSON parser.
if printf '%s' "$payload" | grep -Eq '"stop_hook_active"[[:space:]]*:[[:space:]]*true'; then
	exit 0
fi

# ndjson_parses applies section 3.3's parse condition to one file: every content
# line must be a JSON object (or `null`, which tp's own reader accepts into an
# empty row). Blank and whitespace-only lines are skipped rather than counted,
# so a trailing newline never fails a role and a role with nothing to report
# writes an empty file and passes.
#
# The scan is a full JSON validator rather than a bracket count, because the
# driver and the oracle decide the same predicate with a real parser and the
# three have to agree (test 52). awk is the only dependency: a hook that needed
# python or jq would decide the predicate differently on the machines that lack
# them, which is the divergence the shared predicate exists to prevent.
ndjson_parses() {
	awk '
	function skipws(   c) {
		while (pos <= len) {
			c = substr(str, pos, 1)
			if (c == " " || c == "\t" || c == "\r" || c == "\n") pos++
			else return
		}
	}

	function lit(word) {
		if (substr(str, pos, length(word)) == word) {
			pos += length(word)
			return 1
		}
		return 0
	}

	function digits(   n) {
		n = 0
		while (pos <= len && substr(str, pos, 1) ~ /^[0-9]$/) {
			pos++
			n++
		}
		return n > 0
	}

	# JSON has no leading zeros, so 01 is rejected here exactly as the parsers
	# on the other two sides of the predicate reject it.
	function number(   c) {
		if (substr(str, pos, 1) == "-") pos++
		c = substr(str, pos, 1)
		if (c == "0") pos++
		else if (c ~ /^[1-9]$/) { if (!digits()) return 0 }
		else return 0
		if (substr(str, pos, 1) == ".") {
			pos++
			if (!digits()) return 0
		}
		c = substr(str, pos, 1)
		if (c == "e" || c == "E") {
			pos++
			c = substr(str, pos, 1)
			if (c == "+" || c == "-") pos++
			if (!digits()) return 0
		}
		return 1
	}

	function string(   c, i, h) {
		pos++
		while (pos <= len) {
			c = substr(str, pos, 1)
			if (c == "\\") {
				pos++
				c = substr(str, pos, 1)
				if (c == "\"" || c == "\\" || c == "/" || c == "b" || c == "f" || c == "n" || c == "r" || c == "t") {
					pos++
					continue
				}
				if (c == "u") {
					pos++
					for (i = 0; i < 4; i++) {
						h = substr(str, pos, 1)
						if (h !~ /^[0-9A-Fa-f]$/) return 0
						pos++
					}
					continue
				}
				return 0
			}
			if (c == "\"") {
				pos++
				return 1
			}
			pos++
		}
		return 0
	}

	function array(   c) {
		pos++
		skipws()
		if (substr(str, pos, 1) == "]") {
			pos++
			return 1
		}
		for (;;) {
			if (!value()) return 0
			skipws()
			c = substr(str, pos, 1)
			if (c == ",") { pos++; continue }
			if (c == "]") { pos++; return 1 }
			return 0
		}
	}

	function object(   c) {
		pos++
		skipws()
		if (substr(str, pos, 1) == "}") {
			pos++
			return 1
		}
		for (;;) {
			skipws()
			if (substr(str, pos, 1) != "\"") return 0
			if (!string()) return 0
			skipws()
			if (substr(str, pos, 1) != ":") return 0
			pos++
			if (!value()) return 0
			skipws()
			c = substr(str, pos, 1)
			if (c == ",") { pos++; continue }
			if (c == "}") { pos++; return 1 }
			return 0
		}
	}

	function value(   c) {
		skipws()
		if (pos > len) return 0
		c = substr(str, pos, 1)
		if (c == "{") return object()
		if (c == "[") return array()
		if (c == "\"") return string()
		if (c == "t") return lit("true")
		if (c == "f") return lit("false")
		if (c == "n") return lit("null")
		return number()
	}

	{
		str = $0
		sub(/^[ \t\r]+/, "", str)
		sub(/[ \t\r]+$/, "", str)
		if (str == "") next
		len = length(str)
		pos = 1
		if (!value()) { bad = 1; exit }
		skipws()
		if (pos <= len) { bad = 1; exit }
		# A row is an object; tp reads each line into one. `null` is the single
		# other value its reader accepts, into an empty row.
		if (substr(str, 1, 1) != "{" && str != "null") { bad = 1; exit }
	}

	END { exit (bad ? 1 : 0) }
	' "$1"
}

# The two paths the environment names (section 3.1). An empty value means the
# environment does not say which file is this unit's own, which is not a reason
# to report the unit finished: the block below is capped at one per unit, so
# failing closed here costs one message rather than a loop.
findings=''
if [ -n "${TP_ROUND_DIR:-}" ] && [ -n "${TP_UNIT_ID:-}" ]; then
	findings="$TP_ROUND_DIR/role-$TP_UNIT_ID.ndjson.part"
fi
escalation=''
if [ -n "${TP_RUN_DIR:-}" ] && [ -n "${TP_UNIT_SEQ:-}" ]; then
	escalation="$TP_RUN_DIR/$TP_UNIT_SEQ-escalation.json"
fi

# Either condition ends the unit legitimately. An escalation is a normal,
# expected outcome (section 5.2), not a crash, so it is checked first and on its
# own: a unit that asked for an operator decision owes no findings file.
if [ -n "$escalation" ] && [ -f "$escalation" ]; then
	exit 0
fi

if [ -n "$findings" ] && [ -f "$findings" ] && ndjson_parses "$findings"; then
	exit 0
fi

if [ -z "$findings" ]; then
	reason='the environment names no findings file for this unit (TP_ROUND_DIR and TP_UNIT_ID are what name it)'
elif [ -f "$findings" ]; then
	reason="it has a line that is not a JSON object: $findings"
else
	reason="it does not exist: $findings"
fi

printf '%s\n' \
	"tp unit not finished: a $TP_UNIT_KIND unit ends in its findings file, and $reason (v0.35.0 §6.2)." \
	'Write one JSON object per line to that file and stop again. A role with nothing to report writes an empty file, which passes.' \
	'If what you need is a decision only the operator can make, run `tp escalate --decision <name> --evidence <text>` instead; that ends the unit legitimately.' >&2
exit 2
