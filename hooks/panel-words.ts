import type { Lang, NeedsYou, TaskCounts, ViewName } from '../types'

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
  // needText words a need from its code, or nothing for a code it has no
  // words for (tp's own text is shown then).
  needText: (need: NeedsYou) => string | undefined
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
  paneOpen: string
  noRunYet: string
  notProject: string
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
  crashed: 'beklenmedik biçimde durdu (sürücü çöktü)',
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
  converged: 'the work is done',
  'cap-units': 'the unit cap was reached',
  'cap-wall-clock': 'the time cap was reached',
  'cap-budget': 'the budget cap was reached',
  escalation: 'a decision waits for you',
  'unit-failure': 'a unit failed',
  'no-units': 'no work was left to start',
  interrupted: 'the run was interrupted',
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

// needs_you kinds (blocker, escalation, decision, cap, land, run): the label a
// person reads before tp's own sentence.
const NEED_EN: Record<string, string> = {
  blocker: 'Blocked',
  escalation: 'Escalation',
  decision: 'Decision',
  cap: 'Cap reached',
  land: 'Ready to land',
  run: 'Run',
}

const NEED_TR: Record<string, string> = {
  blocker: 'Engel',
  escalation: 'Sana iletildi',
  decision: 'Karar',
  cap: 'Sınıra ulaşıldı',
  land: 'Birleştirmeye hazır',
  run: 'Koşu',
}

// A need's sentence by its code, said after the kind's label. A key is
// `kind:code` where the kind changes the sentence (a run's stop reason, an
// epic blocker that only you can clear), else the bare code: a decision's
// name arrives as kind decision or escalation, a blocker's code as blocker or
// decision. Each takes the need's id, empty when it has none. tp's
// runStopTexts, escalationTexts and blockerNeed (internal/cli/now_needs.go)
// say the same in English.
type Say = (id: string) => string

const NEED_TEXT_EN: Record<string, Say> = {
  'run:crashed': () => 'Stopped without finishing: its driver is gone.',
  'run:cap-units': () => 'Stopped at its limit on units.',
  'run:cap-wall-clock': () => 'Stopped at its time limit.',
  'run:cap-budget': () => 'Stopped at its spending limit.',
  'run:escalation': () => 'Stopped on a unit\'s question, and the question cannot be read.',
  'run:unit-failure': id => `Stopped: ${id || 'a unit'} failed every attempt.`,
  'run:interrupted': () => 'Interrupted before it finished.',
  'run:driver-error': () => 'Stopped on an error of its own.',
  'run:worktree-setup-failed': id => `Stopped: setting up a worktree for ${id || 'a unit'} failed.`,
  'skip-gate': id => `${id || 'A unit'} asks to skip the quality gate.`,
  'raise-review-cap': id => `${id || 'A unit'} asks to raise the spec review's round limit.`,
  'raise-audit-cap': id => `${id || 'A unit'} asks to raise the audit's round limit.`,
  'raise-found-cap': id => `${id || 'A unit'} asks to raise the limit on found tasks.`,
  'import-force': id => `${id || 'A unit'} asks to overwrite the task list.`,
  'audit-converge-on': id => `${id || 'A unit'} asks to let the audit finish on blocking findings only.`,
  'accept-finding': id => `${id || 'A unit'} asks to accept a finding without a change.`,
  'discard-emission': id => `${id || 'A unit'} asks to discard a round nobody recorded.`,
  'epic-done': id => `${id || 'A unit'} asks to mark an epic as done.`,
  'gate-met': id => `${id || 'A unit'} asks to mark a milestone gate as met.`,
  'epic-binding': id => `${id || 'A unit'} asks to bind the task file to an epic.`,
  'grant-round': id => `${id || 'A unit'} asks for one more epic review round.`,
  'rewrite-past-blocking': id => `${id || 'A unit'} asks to start the spec over past an unresolved blocking finding.`,
  other: id => `${id || 'A unit'} stopped the run and needs your decision.`,
  'no-ready-task': () => 'No task is ready: the open tasks wait on unfinished dependencies or a hold.',
  'blocker:epic-blocked': id => `No task can start: ${id ? `epic ${id}` : 'an epic'} waits on other epics to finish.`,
  'decision:epic-blocked': id =>
    `No task can start: ${id ? `epic ${id}` : 'an epic'} waits on a condition only you can mark as met.`,
  'review-carried-blocking': () => 'A blocking review finding from an earlier round is still unresolved.',
  'blocker:spec-stale': () => 'The spec changed after its last review and needs a new review round.',
  'decision:spec-stale': () => 'The spec changed after its last review, and the next review round needs your decision.',
  'cap:review_max_rounds': () => 'The spec review reached its round limit with findings still open.',
  'cap:audit_max_rounds': () => 'The audit reached its round limit with findings still open.',
  'land:land-request': id => `${id || 'A finished task'} waits to be landed onto this branch.`,
}

const NEED_TEXT_TR: Record<string, Say> = {
  'run:crashed': () => 'Bitmeden durdu, sürücüsü artık çalışmıyor.',
  'run:cap-units': () => 'Birim sınırına ulaşınca durdu.',
  'run:cap-wall-clock': () => 'Süre sınırına ulaşınca durdu.',
  'run:cap-budget': () => 'Harcama sınırına ulaşınca durdu.',
  'run:escalation': () => 'Bir birimin sorusu yüzünden durdu, ama soru okunamıyor.',
  'run:unit-failure': id => `${id || 'Bir birim'} her denemede başarısız olunca durdu.`,
  'run:interrupted': () => 'Bitmeden yarıda kesildi.',
  'run:driver-error': () => 'Kendi hatası yüzünden durdu.',
  'run:worktree-setup-failed': id => `${id || 'Bir birim'} için çalışma ağacı hazırlanamayınca durdu.`,
  'skip-gate': id => `${id || 'Bir birim'} kalite kapısını atlamak istiyor.`,
  'raise-review-cap': id => `${id || 'Bir birim'} spec incelemesinin tur sınırını yükseltmek istiyor.`,
  'raise-audit-cap': id => `${id || 'Bir birim'} denetimin tur sınırını yükseltmek istiyor.`,
  'raise-found-cap': id => `${id || 'Bir birim'} bulunan görevlerin sınırını yükseltmek istiyor.`,
  'import-force': id => `${id || 'Bir birim'} görev listesinin üzerine yazmak istiyor.`,
  'audit-converge-on': id => `${id || 'Bir birim'} denetimin yalnız engelleyici bulgulara bakarak bitmesini istiyor.`,
  'accept-finding': id => `${id || 'Bir birim'} bir bulguyu değişiklik yapmadan kabul etmek istiyor.`,
  'discard-emission': id => `${id || 'Bir birim'} kimsenin kaydetmediği bir turu silmek istiyor.`,
  'epic-done': id => `${id || 'Bir birim'} bir epiği bitti olarak işaretlemek istiyor.`,
  'gate-met': id => `${id || 'Bir birim'} bir kilometre taşı kapısını karşılandı olarak işaretlemek istiyor.`,
  'epic-binding': id => `${id || 'Bir birim'} görev dosyasını bir epiğe bağlamak istiyor.`,
  'grant-round': id => `${id || 'Bir birim'} bir epik inceleme turu daha istiyor.`,
  'rewrite-past-blocking': id => `${id || 'Bir birim'} çözülmemiş engelleyici bir bulguya rağmen spec'i baştan başlatmak istiyor.`,
  other: id => `${id || 'Bir birim'} koşuyu durdurdu, kararını bekliyor.`,
  'no-ready-task': () => 'Hazır görev yok: açık görevler bitmemiş bağımlılıkları ya da bir bekletmenin kalkmasını bekliyor.',
  'blocker:epic-blocked': id => `Hiçbir görev başlayamıyor: ${id ? `${id} epiği` : 'bir epik'} başka epiklerin bitmesini bekliyor.`,
  'decision:epic-blocked': id =>
    `Hiçbir görev başlayamıyor: ${id ? `${id} epiği` : 'bir epik'} yalnız senin karşılandı diyebileceğin bir koşulu bekliyor.`,
  'review-carried-blocking': () => 'Önceki bir turdan kalan engelleyici bir inceleme bulgusu hâlâ çözülmedi.',
  'blocker:spec-stale': () => 'Spec son incelemeden sonra değişti, yeni bir inceleme turu gerekiyor.',
  'decision:spec-stale': () => 'Spec son incelemeden sonra değişti, sıradaki inceleme turu kararını bekliyor.',
  'cap:review_max_rounds': () => 'Spec incelemesi tur sınırına ulaştı, açık bulgular var.',
  'cap:audit_max_rounds': () => 'Denetim tur sınırına ulaştı, açık bulgular var.',
  'land:land-request': id => `${id || 'Biten bir görev'} bu dala birleştirilmeyi bekliyor.`,
}

// sayNeed finds a need's sentence in table: by its kind and code, then by
// the code alone; nothing without a code or for one the table lacks.
function sayNeed(table: Record<string, Say>, need: NeedsYou): string | undefined {
  if (!need.code) return undefined
  const say = table[`${need.kind}:${need.code}`] ?? table[need.code]
  return say?.(need.id ?? '')
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
  needText: need => sayNeed(NEED_TEXT_EN, need),
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
  paneOpen: 'The tp pane is open.',
  noRunYet: 'No tp run in this project yet.',
  notProject: 'This folder is not a tp project.',
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
  last: (id, kind, exit, took) => `Son: ${id} ${kind}, çıkış kodu ${exit}${took ? `, ${took} sürdü` : ''}`,
  stopped: reason => `Durdu: ${reason}`,
  noRun: 'Bu projede henüz koşu yok.',
  need: kind => NEED_TR[kind] ?? kind,
  needText: need => sayNeed(NEED_TEXT_TR, need),
  holder: holder => HOLDER_TR[holder] ?? holder,
  runLive: age => `koşu sürüyor${age ? ` (${age})` : ''}`,
  needsYou: 'Seni bekleyen',
  inProgress: 'Devam eden',
  nothingHeld: 'Devam eden iş yok',
  landing: 'Birleştirilmeyi bekleyen',
  readyNext: 'Sırada',
  epics: 'Epikler',
  counts: c => `Görevler: ${c.done}/${c.total} bitti, ${c.wip} devam ediyor, ${c.ready} hazır, ${c.blocked} bloke`,
  more: n => `${n} tane daha`,
  held: (holder, attempt, age) => [holder, (attempt ?? 1) > 1 ? `${attempt}. deneme` : '', age].filter(Boolean).join(', '),
  noTaskFile: 'Henüz görev dosyası yok',
  noEpics: 'Bu projede epik yok.',
  epicDone: 'bitti',
  heldBy: holder => `Üzerinde çalışan: ${holder}`,
  since: age => `${age} önce başladı`,
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
  paneOpen: 'tp paneli açık.',
  noRunYet: 'Bu projede henüz tp koşusu yok.',
  notProject: 'Bu klasör bir tp projesi değil.',
}

export const WORDS: Record<Lang, Words> = { en: EN, tr: TR }

// capital starts a row with a capital letter as its language writes one
// ("inceleme" is "İnceleme" in Turkish).
export function capital(text: string, lang: Lang): string {
  return text.charAt(0).toLocaleUpperCase(lang) + text.slice(1)
}

// needLine is a need as the pane draws it: the kind's label, then the
// sentence its code says, or tp's own text for a code without words.
export function needLine(need: NeedsYou, lang: Lang): string {
  const w = WORDS[lang]
  return `${w.need(need.kind)}: ${w.needText(need) ?? need.text}`
}
