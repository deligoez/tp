import type { Lang, NowView, RunSlot, RunView, TaskDetail } from '../types'
import { capital, WORDS } from './panel-words'

// The pane's views as rows of words, kept apart from the drawing so each
// view is testable in each language without a surface.

// A PanelRow is one line of the pane: its words, how loud they are, and the
// task a press on it opens (with the key that addresses it). gap asks for an
// empty row above it, between groups; parts are a task row's pieces, so a
// narrow pane can shorten its title alone.
export type PanelRow = {
  text: string
  tone?: 'head' | 'quiet'
  task?: string
  key?: string
  gap?: boolean
  parts?: { id: string; title: string; tail: string }
}

// ageOf says how long ago an RFC 3339 time was, or nothing without one.
export function ageOf(since: string | null | undefined, now: number, lang: Lang): string {
  if (!since) return ''
  const at = Date.parse(since)
  return Number.isNaN(at) ? '' : WORDS[lang].duration((now - at) / 1000)
}

function slotRow(slot: RunSlot, now: number, lang: Lang): PanelRow {
  const w = WORDS[lang]
  return { text: w.slot(slot.id, w.kind(slot.kind), slot.attempt, ageOf(slot.started_at, now, lang)) }
}

// runRows says a run: its phase and state, units, time and spend against
// their caps, each running slot, the last finished unit and why it stopped.
export function runRows(view: RunView, lang: Lang, now: number): PanelRow[] {
  const w = WORDS[lang]
  const caps = view.caps
  const live = view.run_state === 'in_flight'
  const ofCap = (value: string, cap: number, capText: string) => (cap > 0 ? w.of(value, capText) : value)
  // units_done counts started attempts, the same count run_max_units caps, so
  // a live run says how many of them have finished.
  const busy = view.running.length
  const units = ofCap(String(view.units_done), caps.max_units, String(caps.max_units))
  // tp writes wall_clock_seconds only when a unit starts or ends, so a live
  // run counts its own time from its start.
  const started = view.started_at ? Date.parse(view.started_at) : NaN
  const elapsed =
    live && !Number.isNaN(started) ? Math.max(view.wall_clock_seconds, (now - started) / 1000) : view.wall_clock_seconds
  const usd = (n: number) => `$${n.toFixed(2)}`
  const rows: PanelRow[] = [
    // A run with no phase (no task file yet) says its state alone.
    { text: capital(view.phase ? `${w.phase(view.phase)}, ${w.state(view.run_state)}` : w.state(view.run_state), lang), tone: 'head' },
    { text: w.units(units, view.units_done - busy, busy) },
    { text: w.time(ofCap(w.duration(elapsed), caps.max_wall_clock_seconds, w.duration(caps.max_wall_clock_seconds))) },
    { text: w.spend(ofCap(usd(view.spend_usd), caps.max_budget_usd, usd(caps.max_budget_usd))) },
  ]
  if (busy > 0) rows.push({ text: w.runningNow, tone: 'head' })
  for (const slot of view.running) rows.push(slotRow(slot, now, lang))
  const last = view.last_finished
  if (last) {
    const took = last.duration_seconds === null ? '' : w.duration(last.duration_seconds)
    rows.push({ text: w.last(last.id, w.kind(last.kind), last.exit_code, took), tone: 'quiet' })
  }
  if (view.stop_reason) rows.push({ text: w.stopped(w.stop(view.stop_reason)) })
  return rows
}

// A Section is a group of rows under an optional heading. The heading is
// drawn with a gap above it, a row of its own.
type Section = { heading?: PanelRow; items: PanelRow[]; keep?: boolean }

function heading(text: string): PanelRow {
  return { text, tone: 'head', gap: true }
}

function taskRow(section: string, id: string, title: string, tail: string): PanelRow {
  return { text: `${id}  ${title}${tail ? ` · ${tail}` : ''}`, task: id, key: `${section}:${id}`, parts: { id, title, tail } }
}

// fitRow says a row in at most columns cells. A task row is one line: its
// title gives way first, then is left out, so the id and who holds it stay;
// only then is its end cut. Any other row is returned whole, to wrap.
export function fitRow(row: PanelRow, columns: number): string {
  const chars = (s: string) => Array.from(s).length
  if (!row.parts || chars(row.text) <= columns) return row.text
  const { id, title, tail } = row.parts
  const tailText = tail ? ` · ${tail}` : ''
  const room = columns - chars(id) - 2 - chars(tailText)
  const text = room >= 4 ? `${id}  ${Array.from(title).slice(0, room - 1).join('')}…${tailText}` : `${id}${tailText}`
  return chars(text) <= columns ? text : `${Array.from(text).slice(0, columns - 1).join('')}…`
}

// rowsTall is how many lines of the pane a row takes at columns cells: a
// task row one, any other as many as it wraps to, its gap one more.
function rowsTall(row: PanelRow, columns: number): number {
  const lines = row.parts ? 1 : Math.max(1, Math.ceil(Array.from(row.text).length / Math.max(1, columns)))
  return lines + (row.gap ? 1 : 0)
}

// bar draws done of total as a row of filled and empty cells; the numbers
// always follow it, so the bar is never the only way to read it.
export function bar(done: number, total: number, width: number): string {
  const filled = total > 0 ? Math.round((Math.min(done, total) / total) * width) : 0
  return '█'.repeat(filled) + '░'.repeat(width - filled)
}

// fitSection gives a section the rows it can have in room lines: all of it,
// or as many items as fit with a last row counting the rest; none when not
// even its heading and one row fit.
function fitSection(section: Section, room: number, lang: Lang, columns: number): { rows: PanelRow[]; cost: number } {
  const head = section.heading ? [section.heading] : []
  const headCost = section.heading ? rowsTall(section.heading, columns) : 0
  const heights = section.items.map(row => rowsTall(row, columns))
  const total = headCost + heights.reduce((a, b) => a + b, 0)
  if (head.length === 0 && section.items.length === 0) return { rows: [], cost: 0 }
  if (total <= room) return { rows: [...head, ...section.items], cost: total }
  const avail = room - headCost
  if (avail < 1 || section.items.length === 0) return { rows: [], cost: 0 }
  let used = 0
  let kept = 0
  while (kept < heights.length && used + heights[kept]! + 1 <= avail) used += heights[kept++]!
  const more: PanelRow = { text: WORDS[lang].more(section.items.length - kept), tone: 'quiet' }
  return { rows: [...head, ...section.items.slice(0, kept), more], cost: headCost + used + 1 }
}

// nowRows answers the three questions in one screen: what is happening now,
// what needs the person, how far along it is. Trimmed to budget lines at
// columns cells, the first answers kept longest: the phase, what needs you,
// the work in progress, the land requests, the task counts, the epics, the
// ready queue.
export function nowRows(
  now: NowView,
  run: RunView | null,
  lang: Lang,
  at: number,
  budget: number,
  columns = Number.MAX_SAFE_INTEGER,
): PanelRow[] {
  const w = WORDS[lang]
  const live = run?.run_state === 'in_flight'
  // With no phase (no task file yet) the head is the live run alone, or
  // nothing: the counts row says there is no task file.
  const parts = [now.phase ? w.phase(now.phase) : '', live ? w.runLive(ageOf(run?.started_at, at, lang)) : ''].filter(Boolean)
  const head = capital(parts.join(' · '), lang)
  const openEpics = (now.epics ?? []).filter(e => e.state !== 'done')
  // In display order, which is also the order a short pane keeps them in.
  const sections: Section[] = [
    { items: head ? [{ text: head, tone: 'head' }] : [] },
    { heading: now.needs_you.length > 0 ? heading(w.needsYou) : undefined, items: now.needs_you.map(n => ({ text: `${w.need(n.kind)}: ${n.text}` })) },
    heldSection(now, run, lang, at),
    landingSection(now, lang, at),
    countsSection(now, lang),
    { heading: heading(w.epics), items: openEpics.map(e => ({ text: `${e.name}  ${bar(e.done, e.total, 10)} ${e.done}/${e.total}` })) },
    readySection(now, lang),
  ]
  return fitSections(sections, budget, lang, columns)
}

// heldSection is the work in progress: every held task, then each unit a
// live run holds that no held task names (a review or audit role).
function heldSection(now: NowView, run: RunView | null, lang: Lang, at: number): Section {
  const w = WORDS[lang]
  const held = now.held.map(t => taskRow('held', t.id, t.title, w.held(w.holder(t.holder), t.attempt, ageOf(t.since, at, lang))))
  const ids = new Set(now.held.map(t => t.id))
  const slots = run?.run_state === 'in_flight' ? run.running.filter(slot => !ids.has(slot.id)).map(slot => slotRow(slot, at, lang)) : []
  const items = [...held, ...slots]
  return { heading: heading(w.inProgress), items: items.length > 0 ? items : [{ text: w.nothingHeld, tone: 'quiet' }] }
}

function landingSection(now: NowView, lang: Lang, at: number): Section {
  return {
    heading: heading(WORDS[lang].landing),
    items: now.landing.map(t => taskRow('landing', t.id, t.title, ageOf(t.since, at, lang))),
  }
}

function readySection(now: NowView, lang: Lang): Section {
  return { heading: heading(WORDS[lang].readyNext), items: now.ready.map(t => taskRow('ready', t.id, t.title, '')) }
}

// countsSection is one row, drawn as a heading: no task file says so.
function countsSection(now: NowView, lang: Lang): Section {
  const w = WORDS[lang]
  return { heading: heading(now.tasks ? w.counts(now.tasks) : w.noTaskFile), items: [], keep: true }
}

// fitSections lays sections out in order within budget rows, an empty one
// (no items, not kept) drawing nothing, its heading included.
function fitSections(sections: Section[], budget: number, lang: Lang, columns: number): PanelRow[] {
  const rows: PanelRow[] = []
  let room = budget
  for (const section of sections) {
    if (section.items.length === 0 && !section.keep) continue
    const fitted = fitSection(section, room, lang, columns)
    rows.push(...fitted.rows)
    room -= fitted.cost
  }
  // The first row of the pane needs no gap above it.
  if (rows[0]?.gap) rows[0] = { ...rows[0], gap: false }
  return rows
}

// tasksRows is the work in the task file: its counts, what is held, what
// waits to land and what is ready, every row of each (the pane scrolls).
export function tasksRows(now: NowView, lang: Lang, at: number): PanelRow[] {
  const sections = [countsSection(now, lang), heldSection(now, null, lang, at), landingSection(now, lang, at), readySection(now, lang)]
  return fitSections(sections, Number.MAX_SAFE_INTEGER, lang, Number.MAX_SAFE_INTEGER)
}

// epicsRows draws one bar per epic, as wide as the pane spares, its numbers
// beside it and a done epic marked in words.
export function epicsRows(now: NowView, lang: Lang, columns: number): PanelRow[] {
  const w = WORDS[lang]
  const epics = now.epics ?? []
  if (epics.length === 0) return [{ text: w.noEpics, tone: 'quiet' }]
  const nameWidth = Math.max(...epics.map(e => e.name.length))
  const width = Math.max(5, Math.min(20, columns - nameWidth - 16))
  return epics.map(e => ({
    text: `${e.name.padEnd(nameWidth)}  ${bar(e.done, e.total, width)} ${e.done}/${e.total}${e.state === 'done' ? ` · ${w.epicDone}` : ''}`,
  }))
}

// detailRows is one task opened from a list: its title and id, who holds it
// and since when, its worktree, then what `tp show` adds (acceptance and
// touches) once it has answered.
export function detailRows(id: string, now: NowView | null, detail: TaskDetail | null, lang: Lang, at: number): PanelRow[] {
  const w = WORDS[lang]
  const held = now?.held.find(t => t.id === id)
  const landing = now?.landing.find(t => t.id === id)
  const ready = now?.ready.find(t => t.id === id)
  const title = held?.title ?? landing?.title ?? ready?.title ?? detail?.title ?? id
  const rows: PanelRow[] = [{ text: title, tone: 'head' }, { text: id, tone: 'quiet' }]
  if (held) {
    rows.push({ text: w.heldBy(w.held(w.holder(held.holder), held.attempt, '')) })
    const age = ageOf(held.since, at, lang)
    if (age) rows.push({ text: w.since(age) })
    if (held.worktree) rows.push({ text: w.worktree(held.worktree) })
  } else if (landing) {
    rows.push({ text: w.waitingToLand(ageOf(landing.since, at, lang)) })
  } else if (ready) {
    rows.push({ text: w.readyToStart })
  }
  if (!detail) return [...rows, { text: w.reading, tone: 'quiet' }]
  if (detail.failed) return [...rows, { text: w.unreadable }]
  const list = (head: string, items: string[]): PanelRow[] =>
    items.length > 0 ? [heading(head), ...items.map(text => ({ text }))] : []
  return [...rows, ...list(w.acceptance, detail.acceptance), ...list(w.touches, detail.touches)]
}

// glanceRows is the widget's glance: the phase, how much is in progress (held
// tasks and run slots no held task names), and the first thing that needs
// the person.
export function glanceRows(now: NowView | null, run: RunView | null, lang: Lang): PanelRow[] {
  const w = WORDS[lang]
  const phase = now?.phase ?? run?.phase
  const heldIds = new Set((now?.held ?? []).map(t => t.id))
  const live = run?.run_state === 'in_flight' ? run.running.filter(slot => !heldIds.has(slot.id)).length : 0
  const busy = heldIds.size + live
  const rows: PanelRow[] = []
  if (phase) rows.push({ text: capital(w.phase(phase), lang), tone: 'head' })
  rows.push({ text: busy > 0 ? w.inProgressCount(busy) : w.nothingHeld })
  const need = now?.needs_you[0]
  if (need) rows.push({ text: `${w.need(need.kind)}: ${need.text}` })
  return rows
}
