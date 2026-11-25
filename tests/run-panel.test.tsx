import type { On } from 'claude-code'
import { expect, mock, test } from 'claude-code/testing'
import type { Engine } from 'claude-code/testing'

import { toNow, tpVersion, toView } from '../hooks/run-panel'
import { resolveLanguage, WORDS } from '../hooks/panel-words'
import { detailRows, epicsRows, glanceRows, nowRows, runRows, tasksRows } from '../hooks/panel-rows'

const PANE = {
  plugin: 'tp',
  component: 'Pane',
  requestId: 'tp-run',
  props: {
    title: 'tp run',
    isFocused: false,
    bodyColumns: 60,
    placement: 'dock',
    scroll: { offset: 0, bodyRows: 20 },
    view: {},
  },
} as const

const STATUS = {
  run_id: '01RUN',
  phase: 'implement',
  run_state: 'in_flight',
  stop_reason: null,
  units_done: 3,
  wall_clock_seconds: 754,
  spend_usd: 1.2,
  caps: { max_units: 100, max_wall_clock_seconds: 28800, max_budget_usd: 0 },
  running: [{ seq: 4, kind: 'implement', id: 'add-cmd', attempt: 2 }],
  last_unit: { kind: 'implement', id: 'fix-x', attempt: 1, exit_code: 0, duration_seconds: 65 },
}

// A realistic `tp now --json` payload (the contract in the panel's decision
// note): a live run holding one task, an agent holding another, one land
// request and one thing waiting for the operator.
const NOW = {
  project: 'orders-api',
  task_file: 'spec/1.4.0.tasks.json',
  phase: 'implement',
  next_action: 'Run tp next.',
  needs_you: [{ kind: 'escalation', text: 'The run stopped on a decision only you can make.', id: 'auth-refresh' }],
  tasks: { total: 31, done: 14, wip: 2, open: 15, ready: 3, blocked: 12 },
  held: [
    { id: 'auth-refresh', title: 'Refresh tokens before expiry', holder: 'run', since: '2026-11-25T10:00:00Z', worktree: '/w/auth-refresh', attempt: 2 },
    { id: 'rate-limit', title: 'Limit requests per key', holder: 'agent', since: null, worktree: null, attempt: 1 },
  ],
  landing: [{ id: 'cache-key', title: 'Key the cache by tenant', since: '2026-11-25T10:02:00Z' }],
  ready: [
    { id: 'webhook-retry', title: 'Retry failed webhooks' },
    { id: 'audit-log', title: 'Log every admin change' },
  ],
  run: {
    run_id: '01RUN',
    run_state: 'in_flight',
    phase: 'implement',
    started_at: '2026-11-25T10:00:00Z',
    stop_reason: null,
    units_done: 3,
    wall_clock_seconds: 55,
    spend_usd: 0.42,
    caps: { max_units: 100, max_wall_clock_seconds: 28800, max_budget_usd: 0 },
    running: [{ seq: 4, kind: 'implement', id: 'auth-refresh', attempt: 2, started_at: '2026-11-25T10:00:00Z' }],
    last_finished: { kind: 'implement', id: 'alpha', attempt: 1, exit_code: 0, duration_seconds: 45 },
  },
  epics: [
    { id: 'payments', name: 'Payment flow', done: 8, total: 10, state: 'open' },
    { id: 'search', name: 'Search', done: 4, total: 4, state: 'done' },
  ],
}

test('a duration reads in seconds, minutes and hours', async () => {
  const duration = WORDS.en.duration
  expect(duration(42)).toBe('42s')
  expect(duration(754)).toBe('12m 34s')
  expect(duration(28800)).toBe('8h 0m')
})

test('the tasks view lists the counts, the held work, the land requests and the ready queue', async () => {
  const read = toNow(NOW, 0)!
  expect(tasksRows(read.now, 'en', AT).map(row => row.text)).toEqual([
    'Tasks: 14 of 31 done, 2 in progress, 3 ready, 12 blocked',
    'In progress',
    'auth-refresh  Refresh tokens before expiry · run, attempt 2, 4m 5s',
    'rate-limit  Limit requests per key · agent',
    'Waiting to land',
    'cache-key  Key the cache by tenant · 2m 5s',
    'Ready next',
    'webhook-retry  Retry failed webhooks',
    'audit-log  Log every admin change',
  ])
})

test('the brand reads the version tp prints', async () => {
  expect(tpVersion('tp version v1.32.0\n')).toBe('v1.32.0')
  expect(tpVersion('tp version (devel)')).toBe('(devel)')
  expect(tpVersion('')).toBe('')
  // A local build's pseudo-version reads as the next release, marked dev.
  expect(tpVersion('tp version v1.32.2-0.20261125141042-4b4eaa22f3c5+dirty')).toBe('v1.32.2-dev')
})

// The fixture's clock: the run started at 10:00:00, the pane draws 4m 5s on.
const AT = Date.parse('2026-11-25T10:00:00Z') + 245_000

test('the now view answers what runs, what needs you and how far along, in English', async () => {
  const read = toNow(NOW, 0)!
  const rows = nowRows(read.now, read.run, 'en', AT, 40)
  expect(rows.map(row => row.text)).toEqual([
    'Implement phase · run in progress (4m 5s)',
    'Needs you',
    'Escalation: The run stopped on a decision only you can make.',
    'In progress',
    'auth-refresh  Refresh tokens before expiry · run, attempt 2, 4m 5s',
    'rate-limit  Limit requests per key · agent',
    'Waiting to land',
    'cache-key  Key the cache by tenant · 2m 5s',
    'Tasks: 14 of 31 done, 2 in progress, 3 ready, 12 blocked',
    'Epics',
    'Payment flow  ████████░░ 8/10',
    'Ready next',
    'webhook-retry  Retry failed webhooks',
    'audit-log  Log every admin change',
  ])
  // A row naming a task opens it, each under its own key.
  expect(rows.filter(row => row.task).map(row => row.key)).toEqual([
    'held:auth-refresh',
    'held:rate-limit',
    'landing:cache-key',
    'ready:webhook-retry',
    'ready:audit-log',
  ])
})

test('the epics view draws one bar per epic, its numbers beside it', async () => {
  const read = toNow(NOW, 0)!
  expect(epicsRows(read.now, 'tr', 60).map(row => row.text)).toEqual([
    'Payment flow  ████████████████░░░░ 8/10',
    'Search        ████████████████████ 4/4 · bitti',
  ])
  expect(epicsRows({ ...read.now, epics: null }, 'en', 60).map(row => row.text)).toEqual([
    'This project has no epics.',
  ])
})

test('a payload without running still draws', async () => {
  const { running: _drop, ...older } = STATUS
  const view = toView(older, 0)
  expect(view?.running.length).toBe(0)
  expect(toView({ error: 'no run state' }, 0)).toBe(null)
})

test('the rows name the phase, the caps and each running slot', async () => {
  const startedAt = Date.parse('2026-11-25T10:00:00Z')
  const view = toView(
    { ...STATUS, running: [{ ...STATUS.running[0], started_at: '2026-11-25T10:00:00Z' }] },
    0,
  )
  const rows = runRows(view!, 'en', startedAt + 245_000).map(row => row.text)
  expect(rows).toEqual([
    'Implement phase, running',
    'Units 3 of 100 (2 finished, 1 running)',
    'Time 12m 34s of 8h 0m',
    'Spend $1.20 (estimate)',
    'Running now',
    'add-cmd implement, attempt 2, 4m 5s',
    'Last: fix-x implement, exit 0 after 1m 5s',
  ])
})

test('the now view reads in Turkish, titles as written', async () => {
  const read = toNow(NOW, 0)!
  expect(nowRows(read.now, read.run, 'tr', AT, 40).map(row => row.text)).toEqual([
    'Uygulama aşaması · koşu sürüyor (4dk 5sn)',
    'Senden bekleyen',
    'Sana iletildi: The run stopped on a decision only you can make.',
    'Devam eden',
    'auth-refresh  Refresh tokens before expiry · koşu, 2. deneme, 4dk 5sn',
    'rate-limit  Limit requests per key · ajan',
    'Birleştirilmeyi bekleyen',
    'cache-key  Key the cache by tenant · 2dk 5sn',
    'Görevler: 14/31 bitti, 2 devam ediyor, 3 hazır, 12 engelli',
    'Epikler',
    'Payment flow  ████████░░ 8/10',
    'Sırada',
    'webhook-retry  Retry failed webhooks',
    'audit-log  Log every admin change',
  ])
})

// What `tp show auth-refresh --json` adds once the detail opens.
const SHOWN = {
  id: 'auth-refresh',
  title: 'Refresh tokens before expiry',
  status: 'wip',
  acceptance: ['Tokens refresh 60s before expiry', 'A failed refresh logs the user out'],
  touches: ['internal/auth/**'],
  failed: false,
}

test('a task opened from a list shows who holds it, since when, where, and what it must do', async () => {
  const read = toNow(NOW, 0)!
  expect(detailRows('auth-refresh', read.now, SHOWN, 'en', AT).map(row => row.text)).toEqual([
    'Refresh tokens before expiry',
    'auth-refresh',
    'Held by: run, attempt 2',
    'Since: 4m 5s ago',
    'Worktree: /w/auth-refresh',
    'Acceptance',
    'Tokens refresh 60s before expiry',
    'A failed refresh logs the user out',
    'Touches',
    'internal/auth/**',
  ])
  // Until tp show answers, the rows say it is being read.
  expect(detailRows('cache-key', read.now, null, 'en', AT).map(row => row.text)).toEqual([
    'Key the cache by tenant',
    'cache-key',
    'Waiting to land: 2m 5s',
    'Reading the task…',
  ])
})

test('run units that hold no task are in progress too', async () => {
  const slot = (id: string, seq: number) => ({ seq, kind: 'review-role', id, attempt: 1, started_at: '2026-11-25T10:00:00Z' })
  const review = toNow(
    {
      ...NOW,
      phase: 'review',
      task_file: null,
      tasks: null,
      held: [],
      landing: [],
      ready: [],
      needs_you: [],
      epics: null,
      run: { ...NOW.run, phase: 'review', running: [slot('clarity', 1), slot('scope', 2)] },
    },
    0,
  )!
  expect(nowRows(review.now, review.run, 'en', AT, 40).map(row => row.text)).toEqual([
    'Review phase · run in progress (4m 5s)',
    'In progress',
    'clarity review role, attempt 1, 4m 5s',
    'scope review role, attempt 1, 4m 5s',
    'No task file yet',
  ])
})

test('a stopped run names why it stopped', async () => {
  const view = toView({ ...STATUS, run_state: 'stopped', stop_reason: 'converged', running: [] }, 0)
  const rows = runRows(view!, 'en', 0).map(row => row.text)
  expect(rows[0]).toBe('Implement phase, stopped')
  expect(rows.at(-1)).toBe('Stopped: converged')
})

test('a task detail reads in Turkish', async () => {
  const read = toNow(NOW, 0)!
  expect(detailRows('auth-refresh', read.now, SHOWN, 'tr', AT).map(row => row.text)).toEqual([
    'Refresh tokens before expiry',
    'auth-refresh',
    'Tutan: koşu, 2. deneme',
    'Başlangıç: 4dk 5sn önce',
    'Çalışma ağacı: /w/auth-refresh',
    'Kabul ölçütleri',
    'Tokens refresh 60s before expiry',
    'A failed refresh logs the user out',
    'Dokunduğu dosyalar',
    'internal/auth/**',
  ])
  expect(detailRows('webhook-retry', read.now, { ...SHOWN, id: 'webhook-retry', failed: true }, 'tr', AT).map(row => row.text)).toEqual([
    'Retry failed webhooks',
    'webhook-retry',
    'Başlamaya hazır',
    'Bu görev okunamadı.',
  ])
})

// Seen live: two units 31s in read "Time 0s", "Units 2 of 100" with none
// finished, and a Last row repeating a running slot.
test('a live run counts its time from the start and its units as they finish', async () => {
  const start = Date.parse('2026-11-25T10:00:00Z')
  const slot = (id: string, seq: number) => ({ seq, kind: 'implement', id, attempt: 1, started_at: '2026-11-25T10:00:00Z' })
  const view = toView(
    {
      ...STATUS,
      started_at: '2026-11-25T10:00:00Z',
      units_done: 2,
      wall_clock_seconds: 0,
      running: [slot('alpha', 1), slot('beta', 2)],
      last_unit: { kind: 'implement', id: 'beta', attempt: 1, exit_code: null, duration_seconds: null },
    },
    0,
  )
  const rows = runRows(view!, 'en', start + 31_000).map(row => row.text)
  expect(rows).toEqual([
    'Implement phase, running',
    'Units 2 of 100 (0 finished, 2 running)',
    'Time 31s of 8h 0m',
    'Spend $1.20 (estimate)',
    'Running now',
    'alpha implement, attempt 1, 31s',
    'beta implement, attempt 1, 31s',
  ])
})

test('the widget glance says the phase, how much is in progress and the first thing that needs you', async () => {
  const read = toNow(NOW, 0)!
  expect(glanceRows(read.now, read.run, 'en').map(row => row.text)).toEqual([
    'Implement phase',
    '2 in progress',
    'Escalation: The run stopped on a decision only you can make.',
  ])
  expect(glanceRows(read.now, read.run, 'tr').map(row => row.text)).toEqual([
    'Uygulama aşaması',
    '2 iş devam ediyor',
    'Sana iletildi: The run stopped on a decision only you can make.',
  ])
  // An older tp gives the run alone: its slots are what is in progress.
  expect(glanceRows(null, read.run, 'en').map(row => row.text)).toEqual(['Implement phase', '1 in progress'])
  expect(glanceRows(null, null, 'en').map(row => row.text)).toEqual(['Nothing in progress'])
})

test('a narrow pane shortens a title, never the id or who holds it, and wraps a sentence whole', async ($, on) => {
  mock.clock(on, { now: AT })
  world(on, { 'tp --version': VERSION, 'tp now --json': { exitCode: 0, stdout: JSON.stringify(NOW) } })
  await $.session.start({ cwd: '/p', surface: 'terminal', isInteractive: true })
  await runCommand($)
  const ui = await $.ui.mount({ ...PANE, props: { ...PANE.props, bodyColumns: 50 }, surface: 'terminal' })
  expect((await ui.find({ key: 'held:auth-refresh' }))?.text).toBe('auth-refresh  Refresh tok… · run, attempt 2, 4m 5s')
  expect(await ui.find({ type: 'Text', text: 'Escalation: The run stopped on a decision only you can make.' })).toBeTruthy()
  await ui.unmount()
})

test('a project with no run says so', async $ => {
  const ui = await $.ui.mount({ ...PANE, surface: 'terminal' })
  expect((await ui.find({ type: 'Text' }))?.text).toBe('tp')
  expect(await ui.find({ type: 'Text', text: 'No run in this project yet.' })).toBeTruthy()
  await ui.unmount()
})

type Answer = { exitCode: number; stdout: string }

// world answers what the pane calls beneath it: a tp project, each tp command
// by its arguments (any other exits 1), and records every command it ran.
function world(on: On, answers: Record<string, Answer>): string[] {
  const ran: string[] = []
  on('command.register', () => ({ value: { command: 'tp-run' } }))
  on('ui.open', () => ({ value: { isPlaced: true } }))
  on('session.start', (_, e) => ({ cwd: e.cwd }))
  on('fs.stat', () => ({ value: { kind: 'dir', size: 0, mtimeMs: 0, isLink: false } }))
  on('process.run', (_, e) => {
    const line = e.argv.join(' ')
    ran.push(line)
    const answer = answers[line] ?? { exitCode: 1, stdout: '' }
    return { value: { ...answer, stderr: '', isStdoutTruncated: false, isStderrTruncated: false } }
  })
  return ran
}

async function runCommand($: Engine): Promise<string> {
  const { text } = await $.command.run({
    command: 'tp-run',
    args: '',
    origin: { kind: 'composer' },
    presentation: { isFullscreen: false, columns: 120 },
  })
  return text ?? ''
}

const VERSION = { exitCode: 0, stdout: 'tp version v1.32.1\n' }

test('an older tp without tp now falls back to the run status and draws the run view alone', async ($, on) => {
  mock.clock(on, { now: AT })
  const ran = world(on, {
    'tp --version': VERSION,
    'tp run --status': { exitCode: 0, stdout: JSON.stringify({ ...NOW.run, last_finished: undefined, last_unit: NOW.run.last_finished }) },
  })
  await $.session.start({ cwd: '/p', surface: 'terminal', isInteractive: true })
  await runCommand($)
  const ui = await $.ui.mount({ ...PANE, surface: 'terminal' })
  expect(await ui.find({ type: 'Text', text: 'Implement phase, running' })).toBeTruthy()
  expect(await ui.find({ type: 'Text', text: 'Last: alpha implement, exit 0 after 45s' })).toBeTruthy()
  // No views to switch to: the run is all an older tp tells.
  expect(await ui.findAll({ type: 'Button' })).toHaveLength(0)
  // Read-only: nothing but the version, tp now and the run status was run.
  expect([...new Set(ran)].sort()).toEqual(['tp --version', 'tp now --json', 'tp run --status'])
  await ui.unmount()
})

test('a tp now that failed once is asked again when the minute comes round', async ($, on) => {
  const clock = mock.clock(on, { now: AT })
  const answers: Record<string, Answer> = {
    'tp --version': VERSION,
    'tp now --json': { exitCode: 1, stdout: '' },
    'tp run --status': { exitCode: 0, stdout: JSON.stringify(NOW.run) },
  }
  const ran = world(on, answers)
  const reads = () => ran.filter(line => line === 'tp now --json').length
  await $.session.start({ cwd: '/p', surface: 'terminal', isInteractive: true })
  await clock.advance(10_000)
  // The live run is read every 5 seconds through the status alone.
  expect(reads()).toBe(1)
  answers['tp now --json'] = { exitCode: 0, stdout: JSON.stringify(NOW) }
  await clock.advance(50_000)
  expect(reads()).toBe(1)
  await clock.advance(5_000)
  expect(reads()).toBe(2)
  const ui = await $.ui.mount({ ...PANE, surface: 'terminal' })
  expect(await ui.find({ key: 'view-now' })).toBeTruthy()
  await ui.unmount()
})

test('the command outside a tp project says so and reads no status', async ($, on) => {
  let ran = false
  on('command.register', () => ({ value: { command: 'tp-run' } }))
  on('session.start', (_, e) => ({ cwd: e.cwd }))
  on('fs.stat', () => {
    throw new Error('ENOENT: no such file or directory')
  })
  on('process.run', () => {
    ran = true
    return { value: { exitCode: 1, stdout: '', stderr: '', isStdoutTruncated: false, isStderrTruncated: false } }
  })

  await $.session.start({ cwd: '/not-tp', surface: 'terminal', isInteractive: true })
  const { text } = await $.command.run({
    command: 'tp-run',
    args: '',
    origin: { kind: 'composer' },
    presentation: { isFullscreen: false, columns: 120 },
  })
  expect(text).toBe('This folder is not a tp project.')
  expect(ran).toBe(false)
})

test('the pane opens on now, switches views by its buttons and opens a task from a row', async ($, on) => {
  mock.clock(on, { now: AT })
  const ran = world(on, {
    'tp --version': VERSION,
    'tp now --json': { exitCode: 0, stdout: JSON.stringify(NOW) },
    'tp show auth-refresh --json': {
      exitCode: 0,
      stdout: JSON.stringify({ ...SHOWN, acceptance: 'Tokens refresh 60s before expiry\n- A failed refresh logs the user out' }),
    },
  })
  await $.session.start({ cwd: '/p', surface: 'terminal', isInteractive: true })
  await runCommand($)
  const ui = await $.ui.mount({ ...PANE, surface: 'terminal' })
  expect(await ui.find({ type: 'Text', text: 'Implement phase · run in progress (4m 5s)' })).toBeTruthy()
  expect((await ui.findAll({ type: 'Button' })).map(b => b.key).slice(0, 4)).toEqual([
    'view-now',
    'view-tasks',
    'view-epics',
    'view-run',
  ])

  await ui.press({ key: 'view-epics' })
  expect(await ui.find({ type: 'Text', text: /^Search +█+ 4\/4 · done$/ })).toBeTruthy()

  await ui.press({ key: 'view-now' })
  await ui.press({ key: 'held:auth-refresh' })
  expect(await ui.find({ type: 'Text', text: 'Held by: run, attempt 2' })).toBeTruthy()
  expect(await ui.find({ type: 'Text', text: 'A failed refresh logs the user out' })).toBeTruthy()

  await ui.press({ key: 'back' })
  expect(await ui.find({ type: 'Text', text: 'Needs you' })).toBeTruthy()
  expect([...new Set(ran)].sort()).toEqual(['tp --version', 'tp now --json', 'tp show auth-refresh --json'])
  await ui.unmount()
})

test('held work is read every 5 seconds, an idle project once a minute', async ($, on) => {
  const clock = mock.clock(on, { now: AT })
  // An agent holds a task outside any run: held work alone keeps it live.
  const answers = { 'tp --version': VERSION, 'tp now --json': { exitCode: 0, stdout: JSON.stringify({ ...NOW, run: null }) } }
  const ran = world(on, answers)
  const reads = () => ran.filter(line => line === 'tp now --json').length
  await $.session.start({ cwd: '/p', surface: 'terminal', isInteractive: true })
  await clock.advance(10_000)
  expect(reads()).toBe(2)
  answers['tp now --json'] = { exitCode: 0, stdout: JSON.stringify({ ...NOW, run: null, held: [] }) }
  await clock.advance(5_000)
  expect(reads()).toBe(3)
  // Idle now: nothing until the minute comes round.
  await clock.advance(45_000)
  expect(reads()).toBe(3)
  await clock.advance(5_000)
  expect(reads()).toBe(4)
})

test('the language comes from the project, then Claude Code, then the locale, then English', async () => {
  expect(resolveLanguage({ project: 'tr', setting: 'english', locale: 'en_US.UTF-8' })).toBe('tr')
  expect(resolveLanguage({ project: 'en', setting: 'turkish', locale: 'tr_TR.UTF-8' })).toBe('en')
  expect(resolveLanguage({ setting: 'Türkçe', locale: 'en_US.UTF-8' })).toBe('tr')
  // A language the pane has no words for passes the choice on.
  expect(resolveLanguage({ setting: 'japanese', locale: 'tr_TR.UTF-8' })).toBe('tr')
  expect(resolveLanguage({ locale: 'C' })).toBe('en')
  expect(resolveLanguage({})).toBe('en')
})

test('the widget seat draws the glance and no buttons', async ($, on) => {
  mock.clock(on, { now: AT })
  world(on, { 'tp --version': VERSION, 'tp now --json': { exitCode: 0, stdout: JSON.stringify(NOW) } })
  await $.session.start({ cwd: '/p', surface: 'desktop', isInteractive: true })
  await runCommand($)
  const ui = await $.ui.mount({ ...PANE, props: { ...PANE.props, placement: 'widget' }, surface: 'desktop' })
  expect((await ui.findAll({ type: 'Text' })).map(t => t.text)).toEqual([
    'Implement phase',
    '2 in progress',
    'Escalation: The run stopped on a decision only you can make.',
  ])
  expect(await ui.findAll({ type: 'Button' })).toHaveLength(0)
  await ui.unmount()
})

test('the tp now payload keeps what the pane draws, its run included', async () => {
  const read = toNow(NOW, 7)
  expect(read?.now.project).toBe('orders-api')
  expect(read?.now.held.map(task => task.holder)).toEqual(['run', 'agent'])
  expect(read?.now.read_at).toBe(7)
  expect(read?.run?.last_finished?.id).toBe('alpha')
  expect(read?.run?.running[0]?.id).toBe('auth-refresh')
  // Every list may come empty and run, epics and tasks null.
  const idle = toNow({ ...NOW, run: null, epics: null, tasks: null, held: [], landing: [], ready: [], needs_you: [] }, 0)
  expect(idle?.run).toBe(null)
  expect(idle?.now.epics).toBe(null)
  expect(toNow({ error: 'not a tp project' }, 0)).toBe(null)
})

test('the project setting chooses Turkish over an English locale', async ($, on) => {
  mock.clock(on, { now: AT })
  mock.env(on, { LANG: 'en_US.UTF-8' })
  world(on, { 'tp --version': VERSION, 'tp now --json': { exitCode: 0, stdout: JSON.stringify(NOW) } })
  on('fs.read', (_, e) => {
    if (e.path !== '/p/.tp/config.json') throw new Error('ENOENT: no such file or directory')
    return { value: JSON.stringify({ workflow: {}, panel: { language: 'tr' } }) }
  })
  await $.session.start({ cwd: '/p', surface: 'terminal', isInteractive: true })
  await runCommand($)
  const ui = await $.ui.mount({ ...PANE, surface: 'terminal' })
  expect(await ui.find({ type: 'Text', text: 'Uygulama aşaması · koşu sürüyor (4dk 5sn)' })).toBeTruthy()
  expect(await ui.find({ type: 'Button', key: 'view-now', text: 'Şimdi' })).toBeTruthy()
  await ui.unmount()
})

test('with no project setting Claude Code\'s language setting chooses over the locale', async ($, on) => {
  mock.clock(on, { now: AT })
  mock.env(on, { LANG: 'en_US.UTF-8' })
  world(on, { 'tp --version': VERSION, 'tp now --json': { exitCode: 0, stdout: JSON.stringify(NOW) } })
  on('fs.read', () => {
    throw new Error('ENOENT: no such file or directory')
  })
  on('settings.read', () => ({ value: { language: 'Türkçe' } }))
  await $.session.start({ cwd: '/p', surface: 'terminal', isInteractive: true })
  await runCommand($)
  const ui = await $.ui.mount({ ...PANE, surface: 'terminal' })
  expect(await ui.find({ type: 'Text', text: 'Senden bekleyen' })).toBeTruthy()
  await ui.unmount()
})

test('the run view reads in Turkish', async () => {
  const start = Date.parse('2026-11-25T10:00:00Z')
  const read = toNow(NOW, 0)
  const rows = runRows(read!.run!, 'tr', start + 245_000).map(row => row.text)
  expect(rows).toEqual([
    'Uygulama aşaması, çalışıyor',
    'Birimler 3 / 100 (2 bitti, 1 çalışıyor)',
    'Süre 4dk 5sn / 8sa 0dk',
    'Harcama $0.42 (tahmini)',
    'Şu an çalışan',
    'auth-refresh uygulama, 2. deneme, 4dk 5sn',
    'Son: alpha uygulama, çıkış 0, 45sn sürdü',
  ])
})

test('with no project or Claude Code setting the system locale chooses', async ($, on) => {
  mock.clock(on, { now: AT })
  mock.env(on, { LANG: 'tr_TR.UTF-8' })
  world(on, { 'tp --version': VERSION, 'tp now --json': { exitCode: 0, stdout: JSON.stringify(NOW) } })
  on('fs.read', () => {
    throw new Error('ENOENT: no such file or directory')
  })
  on('settings.read', () => ({ value: {} }))
  await $.session.start({ cwd: '/p', surface: 'terminal', isInteractive: true })
  await runCommand($)
  const ui = await $.ui.mount({ ...PANE, surface: 'terminal' })
  expect(await ui.find({ type: 'Text', text: 'Senden bekleyen' })).toBeTruthy()
  await ui.unmount()
})

test('a short pane keeps what needs you and the held work, and counts what it leaves out', async () => {
  const read = toNow(NOW, 0)!
  // A section's heading takes a row and the gap above it another.
  expect(nowRows(read.now, read.run, 'en', AT, 10).map(row => row.text)).toEqual([
    'Implement phase · run in progress (4m 5s)',
    'Needs you',
    'Escalation: The run stopped on a decision only you can make.',
    'In progress',
    'auth-refresh  Refresh tokens before expiry · run, attempt 2, 4m 5s',
    'rate-limit  Limit requests per key · agent',
    'Tasks: 14 of 31 done, 2 in progress, 3 ready, 12 blocked',
  ])
  expect(nowRows(read.now, read.run, 'en', AT, 7).map(row => row.text)).toEqual([
    'Implement phase · run in progress (4m 5s)',
    'Needs you',
    'Escalation: The run stopped on a decision only you can make.',
    'In progress',
    '2 more',
  ])
})
