#!/bin/sh
# A scripted implement unit for scripts/measure-worktree-setup.sh, spawned
# through TP_RUNNER_SEAM (args: kind, id, log path, budget). It stands in for an
# agent: it waits AGENT_SLEEP seconds (default 1), commits one file, and closes
# through the real `tp done` ($MEASURE_TP), so the quality gate runs where the
# unit runs.
kind=$1
id=$2
log=$3
[ "$kind" = implement ] || exit 1
{
	sleep "${AGENT_SLEEP:-1}"
	echo "$id" > "$id.txt"
	git add "$id.txt" && git commit -q -m "unit $id"
	sha=$(git rev-parse HEAD)
	"$MEASURE_TP" done "$id" --commit "$sha" -- "implemented by the scripted measurement unit, gate run by tp done"
} >> "$log" 2>&1
