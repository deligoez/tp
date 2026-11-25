import type { Lang, TaskCounts, ViewName } from '../types'

// The panel's words in its two languages (spec/backlog/panel-dashboard.md):
// every word the pane draws comes from here, so a language is one table.

// langOf reads a language from a setting's free-form value ("tr", "Türkçe",
// "turkish", "tr_TR.UTF-8"), or nothing when it names neither of the two.
export function langOf(value: unknown): Lang | undefined {
  if (typeof value !== 'string') return undefined
  const v = value.trim().toLowerCase()
  if (v === 'tr' || v.startsWith('tr_') || v.startsWith('tr-') || v.startsWith('turk') || v.startsWith('türk')) {
    return 'tr'
  }
  if (v === 'en' || v.startsWith('en_') || v.startsWith('en-') || v.startsWith('english')) return 'en'
  return undefined
}

// The places a language is chosen from, first wins: the project's
// `.tp/config.json` (`panel.language`), Claude Code's `language` setting, the
// system locale (LC_ALL, LC_MESSAGES, LANG). One the pane has no words for
// passes the choice on; with none, English.
export type LanguageSources = { project?: unknown; setting?: unknown; locale?: unknown }

export function resolveLanguage(from: LanguageSources): Lang {
  return langOf(from.project) ?? langOf(from.setting) ?? langOf(from.locale) ?? 'en'
}

// Words is one language's table. tp's own terms (phases, unit kinds, run
// states, stop reasons) are translated from tp's closed sets
// (internal/engine/phase.go, unitkind.go, stopreason.go); a value outside them
// is shown as tp wrote it. Sentences are whole templates, never joined
// words, as Turkish orders them differently.
export type Words = {
  phase: (phase: string) => string
  state: (state: string) => string
  kind: (kind: string) => string
  stop: (reason: string) => string
  duration: (seconds: number) => string
  units: (count: string, finished: number, running: number) => string
  of: (value: string, cap: string) => string
  time: (value: string) => string
  spend: (value: string) => string
  runningNow: string
  slot: (id: string, kind: string, attempt: number, age: string) => string
  last: (id: string, kind: string, exit: number | null, took: string) => string
  stopped: (reason: string) => string
  noRun: string
  need: (kind: string) => string
  holder: (holder: string) => string
  runLive: (age: string) => string
  needsYou: string
  inProgress: string
  nothingHeld: string
  landing: string
  readyNext: string
  epics: string
  counts: (c: TaskCounts) => string
  more: (n: number) => string
  held: (holder: string, attempt: number | null, age: string) => string
  noTaskFile: string
  noEpics: string
  epicDone: string
  heldBy: (holder: string) => string
  since: (age: string) => string
  worktree: (path: string) => string
  waitingToLand: (age: string) => string
  readyToStart: string
  acceptance: string
  touches: string
  reading: string
  unreadable: string
  inProgressCount: (n: number) => string
  tabs: Record<ViewName, string>
  back: string
}

function clock(seconds: number, s: string, m: string, h: string): string {
  const t = Math.max(0, Math.round(seconds))
  if (t < 60) return `${t}${s}`
  const min = Math.floor(t / 60)
  if (min < 60) return `${min}${m} ${t % 60}${s}`
  return `${Math.floor(min / 60)}${h} ${min % 60}${m}`
}

const PHASE_TR: Record<string, string> = {
  review: 'inceleme aşaması',
  decompose: 'ayrıştırma aşaması',
  implement: 'uygulama aşaması',
  audit: 'denetim aşaması',
  release: 'yayın aşaması',
}

const STATE_EN: Record<string, string> = {
  in_flight: 'running',
  stopped: 'stopped',
  crashed: 'stopped without a reason (the driver died)',
}

const STATE_TR: Record<string, string> = {
  in_flight: 'çalışıyor',
  stopped: 'durdu',
  crashed: 'nedensiz durdu (sürücü kapandı)',
}

const KIND_TR: Record<string, string> = {
  implement: 'uygulama',
  decompose: 'ayrıştırma',
  decision: 'karar',
  'review-role': 'inceleme rolü',
  'review-record': 'inceleme kaydı',
  'review-resolve': 'inceleme çözümü',
  'audit-role': 'denetim rolü',
  'audit-record': 'denetim kaydı',
  'audit-fix': 'denetim düzeltmesi',
  'task-audit-role': 'görev denetimi rolü',
  'task-audit-record': 'görev denetimi kaydı',
}

const STOP_EN: Record<string, string> = {
  'cap-units': 'the unit cap was reached',
  'cap-wall-clock': 'the time cap was reached',
  'cap-budget': 'the budget cap was reached',
  escalation: 'a decision waits for you',
  'unit-failure': 'a unit failed',
  'no-units': 'no work was left to start',
  'driver-error': 'the driver failed',
  'worktree-setup-failed': 'a worktree could not be set up',
}

const STOP_TR: Record<string, string> = {
  converged: 'tamamlandı',
  'cap-units': 'birim sınırına ulaşıldı',
  'cap-wall-clock': 'süre sınırına ulaşıldı',
  'cap-budget': 'bütçe sınırına ulaşıldı',
  escalation: 'bir karar seni bekliyor',
  'unit-failure': 'bir birim başarısız oldu',
  'no-units': 'başlatılacak iş kalmadı',
  interrupted: 'yarıda kesildi',
  'driver-error': 'sürücü hata verdi',
  'worktree-setup-failed': 'çalışma ağacı hazırlanamadı',
}

// needs_you kinds (blocker, escalation, decision, cap, land): the label a
// person reads before tp's own sentence.
const NEED_EN: Record<string, string> = {
  blocker: 'Blocked',
  escalation: 'Escalation',
  decision: 'Decision',
  cap: 'Cap reached',
  land: 'Ready to land',
}

const NEED_TR: Record<string, string> = {
  blocker: 'Engel',
  escalation: 'Sana iletildi',
  decision: 'Karar',
  cap: 'Sınıra ulaşıldı',
  land: 'Birleştirmeye hazır',
}

// holder: who holds a task in wip, a live run, a spawned agent, or the person.
const HOLDER_TR: Record<string, string> = { run: 'koşu', agent: 'ajan', you: 'sen' }

const EN: Words = {
  phase: phase => `${phase} phase`,
  state: state => STATE_EN[state] ?? state,
  kind: kind => kind.replaceAll('-', ' '),
  stop: reason => STOP_EN[reason] ?? reason,
  duration: seconds => clock(seconds, 's', 'm', 'h'),
  units: (count, finished, running) =>
    running > 0 ? `Units ${count} (${finished} finished, ${running} running)` : `Units ${count}`,
  of: (value, cap) => `${value} of ${cap}`,
  time: value => `Time ${value}`,
  spend: value => `Spend ${value} (estimate)`,
  runningNow: 'Running now',
  slot: (id, kind, attempt, age) => `${id} ${kind}, attempt ${attempt}${age ? `, ${age}` : ''}`,
  last: (id, kind, exit, took) => `Last: ${id} ${kind}, exit ${exit}${took ? ` after ${took}` : ''}`,
  stopped: reason => `Stopped: ${reason}`,
  noRun: 'No run in this project yet.',
  need: kind => NEED_EN[kind] ?? kind,
  holder: holder => holder,
  runLive: age => `run in progress${age ? ` (${age})` : ''}`,
  needsYou: 'Needs you',
  inProgress: 'In progress',
  nothingHeld: 'Nothing in progress',
  landing: 'Waiting to land',
  readyNext: 'Ready next',
  epics: 'Epics',
  counts: c => `Tasks: ${c.done} of ${c.total} done, ${c.wip} in progress, ${c.ready} ready, ${c.blocked} blocked`,
  more: n => `${n} more`,
  held: (holder, attempt, age) => [holder, (attempt ?? 1) > 1 ? `attempt ${attempt}` : '', age].filter(Boolean).join(', '),
  noTaskFile: 'No task file yet',
  noEpics: 'This project has no epics.',
  epicDone: 'done',
  heldBy: holder => `Held by: ${holder}`,
  since: age => `Since: ${age} ago`,
  worktree: path => `Worktree: ${path}`,
  waitingToLand: age => `Waiting to land${age ? `: ${age}` : ''}`,
  readyToStart: 'Ready to start',
  acceptance: 'Acceptance',
  touches: 'Touches',
  reading: 'Reading the task…',
  unreadable: 'This task could not be read.',
  inProgressCount: n => `${n} in progress`,
  tabs: { now: 'Now', tasks: 'Tasks', epics: 'Epics', run: 'Run' },
  back: 'Back',
}

const TR: Words = {
  phase: phase => PHASE_TR[phase] ?? `${phase} aşaması`,
  state: state => STATE_TR[state] ?? state,
  kind: kind => KIND_TR[kind] ?? kind,
  stop: reason => STOP_TR[reason] ?? reason,
  duration: seconds => clock(seconds, 'sn', 'dk', 'sa'),
  units: (count, finished, running) =>
    running > 0 ? `Birimler ${count} (${finished} bitti, ${running} çalışıyor)` : `Birimler ${count}`,
  of: (value, cap) => `${value} / ${cap}`,
  time: value => `Süre ${value}`,
  spend: value => `Harcama ${value} (tahmini)`,
  runningNow: 'Şu an çalışan',
  slot: (id, kind, attempt, age) => `${id} ${kind}, ${attempt}. deneme${age ? `, ${age}` : ''}`,
  last: (id, kind, exit, took) => `Son: ${id} ${kind}, çıkış ${exit}${took ? `, ${took} sürdü` : ''}`,
  stopped: reason => `Durdu: ${reason}`,
  noRun: 'Bu projede henüz koşu yok.',
  need: kind => NEED_TR[kind] ?? kind,
  holder: holder => HOLDER_TR[holder] ?? holder,
  runLive: age => `koşu sürüyor${age ? ` (${age})` : ''}`,
  needsYou: 'Senden bekleyen',
  inProgress: 'Devam eden',
  nothingHeld: 'Devam eden iş yok',
  landing: 'Birleştirilmeyi bekleyen',
  readyNext: 'Sırada',
  epics: 'Epikler',
  counts: c => `Görevler: ${c.done}/${c.total} bitti, ${c.wip} devam ediyor, ${c.ready} hazır, ${c.blocked} engelli`,
  more: n => `${n} tane daha`,
  held: (holder, attempt, age) => [holder, (attempt ?? 1) > 1 ? `${attempt}. deneme` : '', age].filter(Boolean).join(', '),
  noTaskFile: 'Henüz görev dosyası yok',
  noEpics: 'Bu projede epik yok.',
  epicDone: 'bitti',
  heldBy: holder => `Tutan: ${holder}`,
  since: age => `Başlangıç: ${age} önce`,
  worktree: path => `Çalışma ağacı: ${path}`,
  waitingToLand: age => `Birleştirilmeyi bekliyor${age ? `: ${age}` : ''}`,
  readyToStart: 'Başlamaya hazır',
  acceptance: 'Kabul ölçütleri',
  touches: 'Dokunduğu dosyalar',
  reading: 'Görev okunuyor…',
  unreadable: 'Bu görev okunamadı.',
  inProgressCount: n => `${n} iş devam ediyor`,
  tabs: { now: 'Şimdi', tasks: 'Görevler', epics: 'Epikler', run: 'Koşu' },
  back: 'Geri',
}

export const WORDS: Record<Lang, Words> = { en: EN, tr: TR }

// capital starts a row with a capital letter as its language writes one
// ("inceleme" is "İnceleme" in Turkish).
export function capital(text: string, lang: Lang): string {
  return text.charAt(0).toLocaleUpperCase(lang) + text.slice(1)
}
