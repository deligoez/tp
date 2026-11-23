package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// StopPayload is the part of Claude Code's Stop hook input the pool reads.
type StopPayload struct {
	SessionID       string           `json:"session_id"`
	TranscriptPath  string           `json:"transcript_path"`
	Cwd             string           `json:"cwd"`
	StopHookActive  bool             `json:"stop_hook_active"`
	BackgroundTasks []BackgroundTask `json:"background_tasks"`
}

// BackgroundTask is one entry of the Stop input's background_tasks: a
// subagent's id is its agent id.
type BackgroundTask struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

// LiveSpawns maps the payload's running subagents to the spawn ids their
// claims carry. Claude Code keeps each agent's spawn as toolUseId in
// <transcript without .jsonl>/subagents/agent-<id>.meta.json; an agent whose
// file or id cannot be read is counted in unresolved instead.
func (s *StopPayload) LiveSpawns() (live []string, unresolved int) {
	dir := strings.TrimSuffix(s.TranscriptPath, ".jsonl")
	for _, bt := range s.BackgroundTasks {
		if bt.Type != "subagent" || bt.Status != "running" {
			continue
		}
		spawn := ""
		if id := strings.TrimPrefix(bt.ID, "agent-"); dir != s.TranscriptPath && ValidClaimToken(id) {
			spawn = metaSpawn(filepath.Join(dir, "subagents", "agent-"+id+".meta.json"))
		}
		if spawn == "" {
			unresolved++
			continue
		}
		live = append(live, spawn)
	}
	return live, unresolved
}

// metaSpawn is the toolUseId of one agent's meta.json, or "" when the file
// is unreadable or the id is not a claim token.
func metaSpawn(path string) string {
	raw, err := os.ReadFile(path) //nolint:gosec // a path under the session's own transcript directory, its agent id checked
	if err != nil {
		return ""
	}
	var meta struct {
		ToolUseID string `json:"toolUseId"`
	}
	if json.Unmarshal(raw, &meta) != nil || !ValidClaimToken(meta.ToolUseID) {
		return ""
	}
	return meta.ToolUseID
}
