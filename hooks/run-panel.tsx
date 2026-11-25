import { atom, read, update } from 'claude-code'
import type { EngineInterface, Register } from 'claude-code'

import type { RunSlot, RunView } from '../types'

// The run panel (spec/backlog/run-panel.md): a pane that shows the tp run of
// the session's project, read from `tp run --status`. It reads and never
// drives: no control here stops, retries or starts a unit.

const PANE = 'tp-run'
const TITLE = 'tp run'

// A live run is read every few seconds; an idle project only once a minute,
// so a session in a project with no run costs one short tp call a minute.
const LIVE_MS = 5_000
const IDLE_EVERY = 12

// Change the tag when RunView's fields change: it declines what an earlier
// version stored, which outlives a reload.
const run = atom({ plugin: 'tp', key: 'run' } as const, null as RunView | null, {
  shape: 'run-1',
})

// toView keeps the fields the panel draws from `tp run --status`'s payload,
// tolerating a tp that predates `running`.
export function toView(payload: Record<string, unknown>, now: number): RunView | null {
  if (typeof payload.run_id !== 'string') return null
  const caps = (payload.caps ?? {}) as RunView['caps']
  const last = (payload.last_unit ?? null) as RunView['last_unit']
  return {
    run_id: payload.run_id,
    phase: String(payload.phase ?? ''),
    run_state: String(payload.run_state ?? ''),
    stop_reason: typeof payload.stop_reason === 'string' ? payload.stop_reason : null,
    units_done: Number(payload.units_done ?? 0),
    wall_clock_seconds: Number(payload.wall_clock_seconds ?? 0),
    spend_usd: Number(payload.spend_usd ?? 0),
    caps: {
      max_units: Number(caps.max_units ?? 0),
      max_wall_clock_seconds: Number(caps.max_wall_clock_seconds ?? 0),
      max_budget_usd: Number(caps.max_budget_usd ?? 0),
    },
    running: Array.isArray(payload.running) ? (payload.running as RunSlot[]) : [],
    last_unit: last,
    read_at: now,
  }
}

// duration says a number of seconds the way a person reads it.
export function duration(seconds: number): string {
  const s = Math.max(0, Math.round(seconds))
  if (s < 60) return `${s}s`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}m ${s % 60}s`
  return `${Math.floor(m / 60)}h ${m % 60}m`
}

// The words a person reads for run_state and the unit kinds: the panel never
// shows tp's own vocabulary where a plain phrase says it.
const STATE: Record<string, string> = {
  in_flight: 'running',
  stopped: 'stopped',
  crashed: 'stopped without a reason (the driver died)',
}

function slotAge(slot: RunSlot, now: number): string {
  if (!slot.started_at) return ''
  const at = Date.parse(slot.started_at)
  return Number.isNaN(at) ? '' : duration((now - at) / 1000)
}

// A PanelRow is one line of the pane: its words and how loud they are.
export type PanelRow = { text: string; tone?: 'head' | 'quiet' }

function ofCap(value: string, cap: number, capText: string): string {
  return cap > 0 ? `${value} of ${capText}` : value
}

// panelRows says a run in the lines the pane draws, kept apart from the
// drawing so the words are testable without a surface.
export function panelRows(view: RunView, now: number): PanelRow[] {
  const caps = view.caps
  const rows: PanelRow[] = [
    { text: `${view.phase} phase, ${STATE[view.run_state] ?? view.run_state}`, tone: 'head' },
    { text: `Units ${ofCap(String(view.units_done), caps.max_units, String(caps.max_units))}` },
    {
      text: `Time ${ofCap(duration(view.wall_clock_seconds), caps.max_wall_clock_seconds, duration(caps.max_wall_clock_seconds))}`,
    },
    {
      text: `Spend ${ofCap(`$${view.spend_usd.toFixed(2)}`, caps.max_budget_usd, `$${caps.max_budget_usd.toFixed(2)}`)} (estimate)`,
    },
  ]
  if (view.running.length > 0) rows.push({ text: 'Running now', tone: 'head' })
  for (const slot of view.running) {
    const age = slotAge(slot, now)
    rows.push({ text: `${slot.id} ${slot.kind}, attempt ${slot.attempt}${age ? `, ${age}` : ''}` })
  }
  const last = view.last_unit
  if (last) {
    const ended = last.exit_code === null ? 'still running' : `exit ${last.exit_code}`
    const took = last.duration_seconds === null ? '' : ` after ${duration(last.duration_seconds)}`
    rows.push({ text: `Last: ${last.id} ${last.kind}, ${ended}${took}`, tone: 'quiet' })
  }
  if (view.stop_reason) rows.push({ text: `Stopped: ${view.stop_reason}` })
  return rows
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

export const register: Register = on => {
  let autoOpened = ''

  on('session.start', async ($, e, next) => {
    const started = await next(e)
    await $.command.register({
      name: 'tp-run',
      description: 'Show the tp run of this project in a pane',
    })
    // Only a tp project gets the timer.
    const project = await $.fs.stat(`${e.cwd}/.tp`).catch(() => undefined)
    if (!project) return started

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

  on('command.run', { command: 'tp-run' }, async $ => {
    await update($, run, () => null)
    const view = await readStatus($)
    await update($, run, () => view)
    await $.ui.open({ id: PANE, title: TITLE })
    return { text: view ? 'The tp run pane is open.' : 'No tp run in this project yet.' }
  })

  on('ui.render', { component: 'Pane', requestId: PANE }, async ($, e) => {
    const { Box, Text } = $.ui.resolve(e)
    const view = await read($, run)
    if (!view) {
      return (
        <Box flexDirection="column">
          <Text dimColor>No run in this project yet.</Text>
        </Box>
      )
    }
    const rows = panelRows(view, await $.clock.now())
    return (
      <Box flexDirection="column">
        {rows.map(row => (
          <Text bold={row.tone === 'head'} dimColor={row.tone === 'quiet'}>
            {row.text}
          </Text>
        ))}
      </Box>
    )
  })
}
