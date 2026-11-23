#!/bin/sh
# stop-fill-pool.sh - the tp plugin's second Stop hook
# (spec/backlog/filled-pool.md, Decision 2).
#
# A session that claimed tasks through Agent spawns is a coordinator. When
# its turn ends with parallel slots free and ready tasks (or claims with no
# agent) to fill them, `tp pool --stop-hook` prints a block decision naming
# them, and this hook passes it on. tp reads the payload itself: the
# background_tasks array and the agents' meta.json are JSON this shell does
# not parse. A second stop (stop_hook_active), a session that never claimed,
# a stop inside a tp run unit, a tp without `pool`, or any failure prints
# nothing and exits 0, so the hook can only nudge, never wedge a session.

set -u

[ -n "${TP_UNIT_KIND:-}" ] && exit 0
command -v tp >/dev/null 2>&1 || exit 0

out=$(tp pool --stop-hook 2>/dev/null) || exit 0
case $out in
'{"decision":"block"'*) printf '%s\n' "$out" ;;
esac
exit 0
