import type { Lang } from '../types'

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
