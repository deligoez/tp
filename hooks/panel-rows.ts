import type { Lang, NowView, RunSlot, RunView } from '../types'
import { capital, WORDS } from './panel-words'

// The pane's views as rows of words, kept apart from the drawing so each
// view is testable in each language without a surface.

// A PanelRow is one line of the pane: its words, how loud they are, and the
// task a press on it opens (with the key that addresses it).
// gap asks for an empty row above it, between groups.
export type PanelRow = { text: string; tone?: 'head' | 'quiet'; task?: string; key?: string; gap?: boolean }

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
    { text: capital(`${w.phase(view.phase)}, ${w.state(view.run_state)}`, lang), tone: 'head' },
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
// drawn with a gap above it, so it costs two rows of the pane.
type Section = { heading?: PanelRow; items: PanelRow[]; keep?: boolean }

const HEADING_ROWS = 2

function heading(text: string): PanelRow {
  return { text, tone: 'head', gap: true }
}

function taskRow(section: string, id: string, title: string, tail: string): PanelRow {
  return { text: `${id}  ${title}${tail ? ` · ${tail}` : ''}`, task: id, key: `${section}:${id}` }
}

// bar draws done of total as a row of filled and empty cells; the numbers
// always follow it, so the bar is never the only way to read it.
export function bar(done: number, total: number, width: number): string {
  const filled = total > 0 ? Math.round((Math.min(done, total) / total) * width) : 0
  return '█'.repeat(filled) + '░'.repeat(width - filled)
}

// fitSection gives a section the rows it can have: all of it, or as many
// items as fit with a last row counting the rest; none when not even its
// heading and one row fit.
function fitSection(section: Section, room: number, lang: Lang): { rows: PanelRow[]; cost: number } {
  const head = section.heading ? [section.heading] : []
  const headCost = section.heading ? HEADING_ROWS : 0
  const avail = room - headCost
  if (avail < (section.items.length > 0 ? 1 : 0) || (head.length === 0 && section.items.length === 0)) {
    return { rows: [], cost: 0 }
  }
  if (section.items.length <= avail) return { rows: [...head, ...section.items], cost: headCost + section.items.length }
  const kept = section.items.slice(0, avail - 1)
  const more: PanelRow = { text: WORDS[lang].more(section.items.length - kept.length), tone: 'quiet' }
  return { rows: [...head, ...kept, more], cost: room }
}

// nowRows answers the three questions in one screen: what is happening now,
// what needs the person, how far along it is. Trimmed to budget rows, the
// first answers kept longest: the phase, what needs you, the held work, the
// land requests, the task counts, the epics, the ready queue.
export function nowRows(now: NowView, run: RunView | null, lang: Lang, at: number, budget: number): PanelRow[] {
  const w = WORDS[lang]
  const live = run?.run_state === 'in_flight'
  const head = capital(w.phase(now.phase), lang) + (live ? ` · ${w.runLive(ageOf(run?.started_at, at, lang))}` : '')
  const held: Section = {
    heading: heading(w.inProgress),
    items: now.held.map(t => taskRow('held', t.id, t.title, w.held(w.holder(t.holder), t.attempt, ageOf(t.since, at, lang)))),
  }
  if (held.items.length === 0) held.items.push({ text: w.nothingHeld, tone: 'quiet' })
  const openEpics = (now.epics ?? []).filter(e => e.state !== 'done')
  // In display order, which is also the order a short pane keeps them in.
  const sections: Section[] = [
    { items: [{ text: head, tone: 'head' }] },
    { heading: now.needs_you.length > 0 ? heading(w.needsYou) : undefined, items: now.needs_you.map(n => ({ text: `${w.need(n.kind)}: ${n.text}` })) },
    held,
    { heading: heading(w.landing), items: now.landing.map(t => taskRow('landing', t.id, t.title, ageOf(t.since, at, lang))) },
    { heading: now.tasks ? heading(w.counts(now.tasks)) : undefined, items: [], keep: true },
    { heading: heading(w.epics), items: openEpics.map(e => ({ text: `${e.name}  ${bar(e.done, e.total, 10)} ${e.done}/${e.total}` })) },
    { heading: heading(w.readyNext), items: now.ready.map(t => taskRow('ready', t.id, t.title, '')) },
  ]
  return fitSections(sections, budget, lang)
}

// fitSections lays sections out in order within budget rows, an empty one
// (no items, not kept) drawing nothing, its heading included.
function fitSections(sections: Section[], budget: number, lang: Lang): PanelRow[] {
  const rows: PanelRow[] = []
  let room = budget
  for (const section of sections) {
    if (section.items.length === 0 && !section.keep) continue
    const fitted = fitSection(section, room, lang)
    rows.push(...fitted.rows)
    room -= fitted.cost
  }
  // The first row of the pane needs no gap above it.
  if (rows[0]?.gap) rows[0] = { ...rows[0], gap: false }
  return rows
}
