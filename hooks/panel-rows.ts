import type { Lang, RunSlot, RunView } from '../types'
import { capital, WORDS } from './panel-words'

// The pane's views as rows of words, kept apart from the drawing so each
// view is testable in each language without a surface.

// A PanelRow is one line of the pane: its words, how loud they are, and the
// task a press on it opens (with the key that addresses it).
export type PanelRow = { text: string; tone?: 'head' | 'quiet'; task?: string; key?: string }

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
