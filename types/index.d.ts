// The panel's state contract (spec/backlog/run-panel.md,
// spec/backlog/panel-dashboard.md): what the hooks module keeps in $.state
// under the plugin's name.

// Lang is a language the panel has words for.
export type Lang = 'en' | 'tr'

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

// FinishedUnit is the latest unit of the run that has an exit code.
export type FinishedUnit = {
  kind: string
  id: string
  attempt: number
  exit_code: number | null
  duration_seconds: number | null
}

// RunView is the part of a run the panel draws, read from `tp now --json`'s
// `run` or from `tp run --status`, plus when it was read.
export type RunView = {
  run_id: string
  phase: string
  run_state: string
  stop_reason: string | null
  started_at: string | null
  units_done: number
  wall_clock_seconds: number
  spend_usd: number
  caps: { max_units: number; max_wall_clock_seconds: number; max_budget_usd: number }
  running: RunSlot[]
  last_finished: FinishedUnit | null
  read_at: number
}

// What waits for a person: kind is blocker, escalation, decision, cap, land
// or run; code is a word that does not change with tp's English text (a stop
// reason, a decision's name, a blocker's code), absent from an older tp.
export type NeedsYou = { kind: string; text: string; id?: string; code?: string }

// A task in wip or claimed; holder is run, agent or you.
export type HeldTask = {
  id: string
  title: string
  holder: string
  since: string | null
  worktree: string | null
  // null for a task no run attempt holds (an agent's or the person's own).
  attempt: number | null
}

export type LandingTask = { id: string; title: string; since: string | null }

export type ReadyTask = { id: string; title: string }

export type TaskCounts = { total: number; done: number; wip: number; open: number; ready: number; blocked: number }

export type EpicProgress = { id: string; name: string; done: number; total: number; state: string }

// NowView is `tp now --json` less its run (kept as RunView), plus when it
// was read.
export type NowView = {
  project: string
  task_file: string | null
  phase: string
  needs_you: NeedsYou[]
  tasks: TaskCounts | null
  held: HeldTask[]
  landing: LandingTask[]
  ready: ReadyTask[]
  epics: EpicProgress[] | null
  read_at: number
}

// A view of the pane; detail is one task opened from a list.
export type ViewName = 'now' | 'tasks' | 'epics' | 'run'

export type Screen = { view: ViewName | 'detail'; task: string | null; back: ViewName }

// What `tp show <id> --json` adds to a task the pane opened: read once when
// it opens. failed says the read did not answer.
export type TaskDetail = {
  id: string
  title: string
  status: string
  acceptance: string[]
  touches: string[]
  failed: boolean
}

declare module 'claude-code' {
  interface PluginState {
    tp: {
      run: Shaped<RunView | null>
      now: Shaped<NowView | null>
      screen: Shaped<Screen>
      detail: Shaped<TaskDetail | null>
      lang: Shaped<Lang>
      // The run id the pane last opened itself for.
      opened: Shaped<string>
    }
  }
}
