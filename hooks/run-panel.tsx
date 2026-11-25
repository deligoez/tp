import { atom, read, update } from 'claude-code'
import type { EngineInterface, Register } from 'claude-code'

import type { FinishedUnit, Lang, NowView, RunSlot, RunView, Screen, TaskDetail, ViewName } from '../types'
import { detailRows, epicsRows, fitRow, glanceRows, nowRows, runRows, tasksRows } from './panel-rows'
import type { PanelRow } from './panel-rows'
import { resolveLanguage, WORDS } from './panel-words'

// The panel (spec/backlog/run-panel.md, spec/backlog/panel-dashboard.md): a
// small dashboard of the session's project, read from `tp now --json` (or
// `tp run --status` on a tp without it). It reads and never drives: its
// buttons switch views and open a task, nothing here starts, stops, lands,
// resolves or claims.

const PANE = 'tp-run'
const TITLE = 'tp'

// Live work is read every few seconds (isLive says what is live); an idle
// project only once a minute, so a quiet session costs one short tp call a
// minute.
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
const screenState = atom(
  { plugin: 'tp', key: 'screen' } as const,
  { view: 'now', task: null, back: 'now' } as Screen,
  { shape: 'screen-1' },
)
const detailState = atom({ plugin: 'tp', key: 'detail' } as const, null as TaskDetail | null, {
  shape: 'detail-1',
})
const langState = atom({ plugin: 'tp', key: 'lang' } as const, 'en' as Lang, { shape: 'lang-1' })
// The run the pane last opened itself for, kept past a reload so a pane the
// person closed does not come back for the same run.
const openedState = atom({ plugin: 'tp', key: 'opened' } as const, '', { shape: 'opened-1' })

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
  const version = match?.[1] ?? ''
  // Go stamps a local build `v1.32.2-0.<timestamp>-<sha>[+dirty]`.
  const pseudo = /^(v\d+\.\d+\.\d+)-0\.\d{14}-[0-9a-f]{12}/.exec(version)
  return pseudo ? `${pseudo[1]}-dev` : version
}

// toDetail keeps what the detail adds from `tp show <id> --json`: acceptance
// is a string of `- ` items or a list, touches a list of globs.
export function toDetail(id: string, payload: Record<string, unknown>): TaskDetail {
  const raw = payload.acceptance
  const acceptance = (Array.isArray(raw) ? raw.map(String) : typeof raw === 'string' ? raw.split('\n') : [])
    .map(line => line.replace(/^\s*-\s+/, '').trim())
    .filter(Boolean)
  return {
    id,
    title: typeof payload.title === 'string' ? payload.title : id,
    status: typeof payload.status === 'string' ? payload.status : '',
    acceptance,
    touches: Array.isArray(payload.touches) ? payload.touches.map(String) : [],
    failed: false,
  }
}

// runJSON runs a read-only tp command in the session's folder and parses
// what it printed; any failure (no tp on PATH, a non-zero exit, a payload it
// cannot parse) reads as nothing.
async function runJSON($: EngineInterface, argv: string[]): Promise<Record<string, unknown> | undefined> {
  const done = await $.process.run(argv, { timeoutMs: 10_000 }).catch(() => undefined)
  if (!done || done.exitCode !== 0) return undefined
  try {
    const parsed: unknown = JSON.parse(done.stdout)
    return parsed && typeof parsed === 'object' ? (parsed as Record<string, unknown>) : undefined
  } catch {
    return undefined
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

// Set while `tp now` has never answered and fails: a tp older than it, read
// through `tp run --status` and asked again once a minute.
let nowMissing = false

async function readVersion($: EngineInterface): Promise<string> {
  const done = await $.process.run(['tp', '--version'], { timeoutMs: 10_000 }).catch(() => undefined)
  return done && done.exitCode === 0 ? tpVersion(done.stdout) : ''
}

// readLanguage takes the first of: `.tp/config.json`'s `panel.language`,
// Claude Code's `language` setting, the locale (LC_ALL, LC_MESSAGES, LANG).
// A source that cannot be read is passed over.
async function readLanguage($: EngineInterface, folder: string): Promise<Lang> {
  const config = await $.fs
    .read(`${folder}/.tp/config.json`)
    .then(text => (JSON.parse(String(text)) as { panel?: { language?: unknown } }).panel?.language)
    .catch(() => undefined)
  const setting = await $.settings.read({ key: 'language' }).catch(() => undefined)
  const env = async (get: () => Promise<string | undefined>) => (await get().catch(() => undefined)) || undefined
  const locale =
    (await env(() => $.env.get('LC_ALL'))) ??
    (await env(() => $.env.get('LC_MESSAGES'))) ??
    (await env(() => $.env.get('LANG')))
  return resolveLanguage({ project: config, setting, locale })
}

// refresh reads the project once: `tp now --json`, or on a tp without it
// `tp run --status`, whose run is then all the pane draws. A tp is taken for
// one without `tp now` only while `tp now` has never answered: after it has,
// a failed or slow read keeps the last views and is asked again next tick.
// Once taken for older, `tp now` is asked again only when retry says so (the
// minute's read).
async function refresh($: EngineInterface, retry: boolean): Promise<RunView | null> {
  const at = await $.clock.now()
  const payload = nowMissing && !retry ? undefined : await runJSON($, ['tp', 'now', '--json'])
  const got = payload ? toNow(payload, at) : null
  if (got) {
    nowMissing = false
    await update($, nowState, () => got.now)
    await update($, run, () => got.run)
    return got.run
  }
  // A kept view (which outlives a reload) says `tp now` answered before.
  if (await read($, nowState)) return read($, run)
  nowMissing = true
  const status = await runJSON($, ['tp', 'run', '--status'])
  const view = status ? toView(status, at) : null
  await update($, nowState, () => null)
  await update($, run, () => view)
  return view
}

// isLive: a run in flight, or a task a run or an agent holds, is read every
// few seconds; a task the person holds only while the pane is drawn.
async function isLive($: EngineInterface): Promise<boolean> {
  const now = await read($, nowState)
  const view = await read($, run)
  if (view?.run_state === 'in_flight') return true
  const held = now?.held ?? []
  if (held.some(task => task.holder === 'run' || task.holder === 'agent')) return true
  if (held.length === 0) return false
  // Only the person's own work is held: worth the fast read only while the
  // pane is drawn. The engine's pane record answers that, and it follows a
  // reload, a close by hand and a pane that waited undrawn.
  const panes = await $.ui.panes().catch(() => [])
  return panes.some(pane => pane.id === PANE && pane.isPlaced)
}

// openTask shows one task and asks `tp show` for what the lists do not
// carry.
async function openTask($: EngineInterface, id: string): Promise<void> {
  // Marked before the screen changes, so a tick between the writes and the
  // read does not start a second one.
  detailReading = id
  const screen = await read($, screenState)
  const back: ViewName = screen.view === 'detail' ? screen.back : screen.view
  await update($, screenState, (): Screen => ({ view: 'detail', task: id, back }))
  await update($, detailState, () => null)
  await readDetail($, id)
}

// The task whose `tp show` is being read, so a tick does not ask twice.
let detailReading = ''

// readDetail asks `tp show` for one task; an answer for a task the person
// has since left is dropped.
async function readDetail($: EngineInterface, id: string): Promise<void> {
  detailReading = id
  // An id is tp's own, but never let one read as a flag.
  const payload = id.startsWith('-') ? undefined : await runJSON($, ['tp', 'show', id, '--json'])
  if (detailReading === id) detailReading = ''
  const detail = payload ? toDetail(id, payload) : { ...toDetail(id, {}), failed: true }
  const current = await read($, screenState)
  if (current.view === 'detail' && current.task === id) await update($, detailState, () => detail)
}

// resumeDetail reads the detail the screen shows when nothing holds it: a
// reload cut its read off and kept the screen. Called from the timer and the
// command, never while drawing.
async function resumeDetail($: EngineInterface): Promise<void> {
  const screen = await read($, screenState)
  if (screen.view !== 'detail' || !screen.task || detailReading === screen.task) return
  const detail = await read($, detailState)
  if (detail?.id !== screen.task) await readDetail($, screen.task)
}

// The folder's last path segment: the project the pane is about.
function projectName(folder: string): string {
  return folder.split('/').filter(Boolean).pop() ?? ''
}

const VIEWS: readonly ViewName[] = ['now', 'tasks', 'epics', 'run']

// The rows the pane spends on itself above a view: the brand line and its
// gap, the view buttons and theirs.
const CHROME_ROWS = 4

// Set once the timer runs, so session.start and the command start it once.
// A reload starts the module over, and the flag with it.
let polling = false

// startPolling reads the project on a timer: every few seconds while work
// is live, else once a minute.
function startPolling($: EngineInterface): void {
  if (polling) return
  polling = true
  let tick = 0
  $.clock.every(LIVE_MS, async () => {
    tick++
    // Ahead of the idle skip, so an open detail never waits for the minute.
    await resumeDetail($)
    const minute = tick % IDLE_EVERY === 1
    if (!minute && !(await isLive($))) return
    const view = await refresh($, minute)
    // A run that starts while the session is open shows itself once.
    if (view?.run_state === 'in_flight' && (await read($, openedState)) !== view.run_id) {
      await update($, openedState, () => view.run_id)
      void $.ui.open({ id: PANE, title: TITLE })
    }
  })
}

export const register: Register = on => {
  on('session.start', async ($, e, next) => {
    const started = await next(e)
    await $.command.register({
      name: 'tp-run',
      description: 'Show what tp is doing in this project in a pane',
    })
    sessionCwd = e.cwd
    // Only a tp project gets the timer; a folder that becomes one later gets
    // it from the command.
    if (!(await isProject($, e.cwd))) return started
    version = await readVersion($)
    const lang = await readLanguage($, e.cwd)
    await update($, langState, () => lang)
    startPolling($)
    return started
  })

  on('command.run', { command: 'tp-run' }, async ($, e) => {
    const cwd = 'cwd' in e && typeof e.cwd === 'string' ? e.cwd : sessionCwd
    if (!(await isProject($, cwd))) return { text: 'This folder is not a tp project.' }
    if (!version) version = await readVersion($)
    const lang = await readLanguage($, cwd)
    await update($, langState, () => lang)
    const view = await refresh($, true)
    await resumeDetail($)
    const now = await read($, nowState)
    await $.ui.open({ id: PANE, title: TITLE })
    startPolling($)
    return { text: now || view ? 'The tp pane is open.' : 'No tp run in this project yet.' }
  })

  on('ui.render', { component: 'Pane', requestId: PANE }, async ($, e) => {
    const { Box, Text, Button } = $.ui.resolve(e)
    const lang = await read($, langState)
    const w = WORDS[lang]
    const now = await read($, nowState)
    const view = await read($, run)
    const columns = e.props.bodyColumns
    // A task row is one line a press opens, its title shortened to fit; any
    // other row wraps whole, so a sentence is never cut.
    const line = (row: PanelRow) =>
      row.task ? (
        <Box key={`row-${row.key}`} marginTop={row.gap ? 1 : 0}>
          <Button key={row.key} plain onPress={() => openTask($, row.task!)}>
            {fitRow(row, columns)}
          </Button>
        </Box>
      ) : (
        <Box marginTop={row.gap ? 1 : 0}>
          <Text bold={row.tone === 'head'} dimColor={row.tone === 'quiet'}>
            {row.text}
          </Text>
        </Box>
      )

    // The widget seat is a glance: clicks may not reach it, so no buttons.
    if (e.props.placement === 'widget') {
      return <Box flexDirection="column">{glanceRows(now, view, lang).map(line)}</Box>
    }

    // The brand line: tp and its version in the theme's accent, then the
    // project, quiet.
    const brand = (
      <Box flexDirection="row" marginBottom={1}>
        <Text bold color="suggestion">
          {version ? `tp ${version}` : 'tp'}
        </Text>
        <Text dimColor>{`  ${now?.project ?? projectName(sessionCwd)}`}</Text>
      </Box>
    )

    // A tp without `tp now`: the run view alone, as the panel was.
    if (!now) {
      return (
        <Box flexDirection="column">
          {brand}
          {view ? runRows(view, lang, await $.clock.now()).map(line) : <Text dimColor>{w.noRun}</Text>}
        </Box>
      )
    }

    const at = await $.clock.now()
    const screen = await read($, screenState)
    const shown: ViewName = screen.view === 'detail' ? screen.back : screen.view
    // The view buttons sit first, so the window never scrolls them out of
    // reach of their hotkeys; the current one is drawn at full strength.
    const tabs = (
      <Box flexDirection="row" gap={2} marginBottom={1}>
        {VIEWS.map((name, index) => (
          <Button
            key={`view-${name}`}
            label={w.tabs[name]}
            hotkey={String(index + 1)}
            plain
            dimColor={shown !== name}
            onPress={() => update($, screenState, () => ({ view: name, task: null, back: name }))}
          >
            {/* The view shown is bold as well as at full strength, so it is
                not told by brightness alone. */}
            <Text bold={shown === name}>{w.tabs[name]}</Text>
          </Button>
        ))}
      </Box>
    )
    let rows: PanelRow[]
    let back = null
    if (screen.view === 'detail' && screen.task) {
      const detail = await read($, detailState)
      rows = detailRows(screen.task, now, detail?.id === screen.task ? detail : null, lang, at)
      back = (
        <Box marginBottom={1}>
          <Button key="back" label={w.back} hotkey="b" onPress={() => update($, screenState, () => ({ view: shown, task: null, back: shown }))}>
            {w.back}
          </Button>
        </Box>
      )
    } else if (screen.view === 'tasks') rows = tasksRows(now, lang, at)
    else if (screen.view === 'epics') rows = epicsRows(now, lang, columns)
    else if (screen.view === 'run') rows = view ? runRows(view, lang, at) : [{ text: w.noRun, tone: 'quiet' }]
    else rows = nowRows(now, view, lang, at, Math.max(1, e.props.scroll.bodyRows - CHROME_ROWS), columns)
    return (
      <Box flexDirection="column">
        {brand}
        {tabs}
        {back}
        {rows.map(line)}
      </Box>
    )
  })
}
