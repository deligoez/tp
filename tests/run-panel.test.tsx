import { expect, test } from 'claude-code/testing'

import { duration, panelRows, toView } from '../hooks/run-panel'

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

test('a duration reads in seconds, minutes and hours', async () => {
  expect(duration(42)).toBe('42s')
  expect(duration(754)).toBe('12m 34s')
  expect(duration(28800)).toBe('8h 0m')
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
  const rows = panelRows(view!, startedAt + 245_000).map(row => row.text)
  expect(rows).toEqual([
    'implement phase, running',
    'Units 3 of 100',
    'Time 12m 34s of 8h 0m',
    'Spend $1.20 (estimate)',
    'Running now',
    'add-cmd implement, attempt 2, 4m 5s',
    'Last: fix-x implement, exit 0 after 1m 5s',
  ])
})

test('a stopped run names why it stopped', async () => {
  const view = toView({ ...STATUS, run_state: 'stopped', stop_reason: 'converged', running: [] }, 0)
  const rows = panelRows(view!, 0).map(row => row.text)
  expect(rows[0]).toBe('implement phase, stopped')
  expect(rows.at(-1)).toBe('Stopped: converged')
})

test('a project with no run says so', async $ => {
  const ui = await $.ui.mount({ ...PANE, surface: 'terminal' })
  expect((await ui.find({ type: 'Text' }))?.text).toBe('No run in this project yet.')
  await ui.unmount()
})
