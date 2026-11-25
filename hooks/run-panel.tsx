import { atom, read, update } from 'claude-code'
import type { EngineInterface, Register } from 'claude-code'

import type { FinishedUnit, NowView, RunSlot, RunView } from '../types'
import { runRows } from './panel-rows'

// The run panel (spec/backlog/run-panel.md): a pane that shows the tp run of
// the session's project, read from `tp run --status`. It reads and never
// drives: no control here stops, retries or starts a unit.

const PANE = 'tp-run'
const TITLE = 'tp run'

// A live run is read every few seconds; an idle project only once a minute,
// so a session in a project with no run costs one short tp call a minute.
const LIVE_MS = 5_000
const IDLE_EVERY = 12

// Change a tag when its value's fields change: it declines what an earlier
// version stored, which outlives a reload.
const run = atom({ plugin: 'tp', key: 'run' } as const, null as RunView | null, {
  shape: 'run-3',
})
const nowState = atom({ plugin: 'tp', key: 'now' } as const, null as NowView | null, {
  shape: 'now-1',
})

// lastFinished is the latest unit with an exit code: `tp now`'s
// last_finished, or `tp run --status`'s last_unit once it has ended.
function lastFinished(payload: Record<string, unknown>): FinishedUnit | null {
  const last = (payload.last_finished ?? payload.last_unit ?? null) as FinishedUnit | null
  if (!last || typeof last !== 'object' || last.exit_code === null || last.exit_code === undefined) return null
  return last
}

// toView keeps the fields the panel draws from a run's payload (`tp now`'s
// run or `tp run --status`), tolerating a tp that predates `running`.
export function toView(payload: Record<string, unknown>, now: number): RunView | null {
  if (typeof payload.run_id !== 'string') return null
  const caps = (payload.caps ?? {}) as RunView['caps']
  return {
    run_id: payload.run_id,
    phase: String(payload.phase ?? ''),
    run_state: String(payload.run_state ?? ''),
    stop_reason: typeof payload.stop_reason === 'string' ? payload.stop_reason : null,
    started_at: typeof payload.started_at === 'string' ? payload.started_at : null,
    units_done: Number(payload.units_done ?? 0),
    wall_clock_seconds: Number(payload.wall_clock_seconds ?? 0),
    spend_usd: Number(payload.spend_usd ?? 0),
    caps: {
      max_units: Number(caps.max_units ?? 0),
      max_wall_clock_seconds: Number(caps.max_wall_clock_seconds ?? 0),
      max_budget_usd: Number(caps.max_budget_usd ?? 0),
    },
    running: Array.isArray(payload.running) ? (payload.running as RunSlot[]) : [],
    last_finished: lastFinished(payload),
    read_at: now,
  }
}

function list<T>(value: unknown): T[] {
  return Array.isArray(value) ? (value as T[]) : []
}

// toNow keeps what the pane draws from `tp now --json`, its run apart as a
// RunView; a payload that is not one (no project) reads as nothing.
export function toNow(payload: Record<string, unknown>, at: number): { now: NowView; run: RunView | null } | null {
  if (typeof payload.project !== 'string') return null
  const runPayload = payload.run && typeof payload.run === 'object' ? (payload.run as Record<string, unknown>) : null
  return {
    now: {
      project: payload.project,
      task_file: typeof payload.task_file === 'string' ? payload.task_file : null,
      phase: String(payload.phase ?? ''),
      needs_you: list(payload.needs_you),
      tasks: payload.tasks && typeof payload.tasks === 'object' ? (payload.tasks as NowView['tasks']) : null,
      held: list(payload.held),
      landing: list(payload.landing),
      ready: list(payload.ready),
      epics: Array.isArray(payload.epics) ? (payload.epics as NowView['epics']) : null,
      read_at: at,
    },
    run: runPayload ? toView(runPayload, at) : null,
  }
}

// tpVersion takes the version out of `tp --version`'s line ("tp version
// v1.32.0"), or nothing when tp printed something else.
export function tpVersion(printed: string): string {
  const match = /^tp version (\S+)/.exec(printed.trim())
  return match?.[1] ?? ''
}

// readStatus runs `tp run --status` in the session's folder; any failure (no
// tp on PATH, no run yet, a payload it cannot parse) reads as no run.
async function readStatus($: EngineInterface): Promise<RunView | null> {
  const done = await $.process
    .run(['tp', 'run', '--status'], { timeoutMs: 10_000 })
    .catch(() => undefined)
  if (!done || done.exitCode !== 0) return null
  try {
    return toView(JSON.parse(done.stdout) as Record<string, unknown>, await $.clock.now())
  } catch {
    return null
  }
}

// isProject reports whether folder holds a tp project (its .tp directory).
async function isProject($: EngineInterface, folder: string): Promise<boolean> {
  if (!folder) return false
  return (await $.fs.stat(`${folder}/.tp`).catch(() => undefined)) !== undefined
}

// The session's folder, kept from session.start: command.run carries no cwd
// of its own on this engine, and a reload runs session.start again.
let sessionCwd = ''

// The tp binary's version, read once per session for the pane's brand line.
let version = ''

async function readVersion($: EngineInterface): Promise<string> {
  const done = await $.process.run(['tp', '--version'], { timeoutMs: 10_000 }).catch(() => undefined)
  return done && done.exitCode === 0 ? tpVersion(done.stdout) : ''
}

// The folder's last path segment: the project the pane is about.
function projectName(folder: string): string {
  return folder.split('/').filter(Boolean).pop() ?? ''
}

export const register: Register = on => {
  let autoOpened = ''

  on('session.start', async ($, e, next) => {
    const started = await next(e)
    await $.command.register({
      name: 'tp-run',
      description: 'Show the tp run of this project in a pane',
    })
    sessionCwd = e.cwd
    // Only a tp project gets the timer.
    if (!(await isProject($, e.cwd))) return started
    version = await readVersion($)

    let tick = 0
    $.clock.every(LIVE_MS, async () => {
      tick++
      const prev = await read($, run)
      if (prev?.run_state !== 'in_flight' && tick % IDLE_EVERY !== 1) return
      const view = await readStatus($)
      await update($, run, () => view)
      // A run that starts while the session is open shows itself once.
      if (view?.run_state === 'in_flight' && autoOpened !== view.run_id) {
        autoOpened = view.run_id
        void $.ui.open({ id: PANE, title: TITLE })
      }
    })
    return started
  })

  on('command.run', { command: 'tp-run' }, async ($, e) => {
    const cwd = 'cwd' in e && typeof e.cwd === 'string' ? e.cwd : sessionCwd
    if (!(await isProject($, cwd))) return { text: 'This folder is not a tp project.' }
    if (!version) version = await readVersion($)
    await update($, run, () => null)
    const view = await readStatus($)
    await update($, run, () => view)
    await $.ui.open({ id: PANE, title: TITLE })
    return { text: view ? 'The tp run pane is open.' : 'No tp run in this project yet.' }
  })

  on('ui.render', { component: 'Pane', requestId: PANE }, async ($, e) => {
    const { Box, Text } = $.ui.resolve(e)
    const view = await read($, run)
    // The brand line: tp and its version in the theme's accent, then the
    // project, quiet.
    const brand = (
      <Box flexDirection="row" marginBottom={1}>
        <Text bold color="suggestion">
          {version ? `tp ${version}` : 'tp'}
        </Text>
        <Text dimColor>{`  ${projectName(sessionCwd)}`}</Text>
      </Box>
    )
    if (!view) {
      return (
        <Box flexDirection="column">
          {brand}
          <Text dimColor>No run in this project yet.</Text>
        </Box>
      )
    }
    const rows = runRows(view, 'en', await $.clock.now())
    return (
      <Box flexDirection="column">
        {brand}
        {rows.map(row => (
          <Text bold={row.tone === 'head'} dimColor={row.tone === 'quiet'}>
            {row.text}
          </Text>
        ))}
      </Box>
    )
  })
}
