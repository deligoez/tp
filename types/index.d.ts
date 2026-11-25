// The run panel's state contract (spec/backlog/run-panel.md): what the
// hooks module keeps in $.state under the plugin's name.

// RunSlot is one unit attempt the run holds now, as `tp run --status`
// reports it under `running`.
export type RunSlot = {
  seq: number
  kind: string
  id: string
  attempt: number
  worktree?: string
  started_at?: string
}

// RunView is the part of `tp run --status` the panel draws, plus when it
// was read.
export type RunView = {
  run_id: string
  phase: string
  run_state: string
  stop_reason: string | null
  units_done: number
  wall_clock_seconds: number
  spend_usd: number
  caps: { max_units: number; max_wall_clock_seconds: number; max_budget_usd: number }
  running: RunSlot[]
  last_unit: {
    kind: string
    id: string
    attempt: number
    exit_code: number | null
    duration_seconds: number | null
  } | null
  read_at: number
}

declare module 'claude-code' {
  interface PluginState {
    tp: { run: Shaped<RunView | null> }
  }
}
