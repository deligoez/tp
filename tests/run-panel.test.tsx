import { expect, test } from 'claude-code/testing'

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

test('a stopped run names why it stopped', async () => {
  const view = toView({ ...STATUS, run_state: 'stopped', stop_reason: 'converged', running: [] }, 0)
  const rows = runRows(view!, 'en', 0).map(row => row.text)
  expect(rows[0]).toBe('Implement phase, stopped')
  expect(rows.at(-1)).toBe('Stopped: converged')
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

test('a project with no run says so', async $ => {
  const ui = await $.ui.mount({ ...PANE, surface: 'terminal' })
  expect((await ui.find({ type: 'Text' }))?.text).toBe('tp')
  expect(await ui.find({ type: 'Text', text: 'No run in this project yet.' })).toBeTruthy()
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

test('the language comes from the project, then Claude Code, then the locale, then English', async () => {
  expect(resolveLanguage({ project: 'tr', setting: 'english', locale: 'en_US.UTF-8' })).toBe('tr')
  expect(resolveLanguage({ project: 'en', setting: 'turkish', locale: 'tr_TR.UTF-8' })).toBe('en')
  expect(resolveLanguage({ setting: 'Türkçe', locale: 'en_US.UTF-8' })).toBe('tr')
  // A language the pane has no words for passes the choice on.
  expect(resolveLanguage({ setting: 'japanese', locale: 'tr_TR.UTF-8' })).toBe('tr')
  expect(resolveLanguage({ locale: 'C' })).toBe('en')
  expect(resolveLanguage({})).toBe('en')
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
