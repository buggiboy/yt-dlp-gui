// Tier-1 cleanup presets and the regex helpers shared by compile() and
// preview().
//
// Each preset is a friendly toggle that maps to a single Python regex used by
// yt-dlp's `--replace-in-metadata`. They target the `title` field (the field
// most likely to contain noise); power users retarget other fields via custom
// Tier-3 rules.
//
// IMPORTANT ordering note: the presets are listed here in the *canonical apply
// order*, not the order the user toggles them. compile() and preview() both
// iterate this array and skip the disabled ones, so any combination of toggles
// produces a sensible result:
//   1. strip promo tags   — while spaces still delimit them
//   2. collapse spaces    — tidy the gaps the strip left behind
//   3. trim separators    — remove leading/trailing junk
//   4. spaces→underscores — last, since earlier steps need real spaces

import type { CleanupPresetId } from './types'

export type CleanupPreset = {
  id: CleanupPresetId
  label: string
  description: string
  /** Metadata field the rule applies to. */
  field: string
  /** Python regex (as accepted by --replace-in-metadata). */
  pattern: string
  /** Replacement string. */
  replace: string
}

/** Canonical apply order — do not reorder casually. */
export const CLEANUP_PRESETS: CleanupPreset[] = [
  {
    id: 'stripTags',
    label: 'Remove tags like (Official Video), [HD]',
    description: 'Strips bracketed promo text such as (Official Video), (Lyrics), [HD], (4K).',
    field: 'title',
    pattern: '(?i)\\s*[\\(\\[][^\\)\\]]*\\b(official|lyrics?|audio|video|hd|4k|mv|visualizer|explicit|remaster(ed)?)\\b[^\\)\\]]*[\\)\\]]',
    replace: '',
  },
  {
    id: 'collapseSpace',
    label: 'Collapse repeated spaces',
    description: 'Replaces runs of whitespace with a single space.',
    field: 'title',
    pattern: '\\s{2,}',
    replace: ' ',
  },
  {
    id: 'trimSeparators',
    label: 'Trim leading/trailing spaces and dashes',
    description: 'Removes spaces, dashes, and underscores from the start and end.',
    field: 'title',
    pattern: '^[\\s_-]+|[\\s_-]+$',
    replace: '',
  },
  {
    id: 'spacesToUnderscores',
    label: 'Replace spaces with underscores',
    description: 'Swaps every space for an underscore (applied last).',
    field: 'title',
    pattern: '\\s+',
    replace: '_',
  },
]

const PRESET_BY_ID = new Map(CLEANUP_PRESETS.map((p) => [p.id, p]))

export function cleanupPreset(id: CleanupPresetId): CleanupPreset | undefined {
  return PRESET_BY_ID.get(id)
}

/** Returns the enabled presets in canonical apply order, regardless of the
 *  order they were toggled on. */
export function enabledPresetsInOrder(enabled: CleanupPresetId[]): CleanupPreset[] {
  const set = new Set(enabled)
  return CLEANUP_PRESETS.filter((p) => set.has(p.id))
}

/** Converts a Python-style pattern into a JavaScript RegExp for the offline
 *  preview. It only handles a leading inline-flag group like `(?i)` / `(?is)`,
 *  mapping those flags onto JS RegExp flags — which covers the patterns this
 *  app generates and the common case for user rules. The `g` flag is always
 *  added so replacements are global, matching yt-dlp's behaviour.
 *
 *  Throws (like `new RegExp`) on an invalid pattern, so callers can surface a
 *  validation error. Note this is an *approximation*: Python `re` and JS
 *  RegExp differ on lookbehind, named groups, and some escapes. */
export function toJsRegExp(pattern: string): RegExp {
  let flags = 'g'
  let body = pattern
  const m = /^\(\?([imsuxaL]+)\)/.exec(pattern)
  if (m) {
    body = pattern.slice(m[0].length)
    if (m[1].includes('i')) flags += 'i'
    if (m[1].includes('s')) flags += 's'
    if (m[1].includes('m')) flags += 'm'
    if (m[1].includes('u')) flags += 'u'
  }
  return new RegExp(body, flags)
}

/** Validates a user-entered pattern by attempting to build a JS RegExp from it.
 *  Returns an error message, or null if it parses. Approximate (see toJsRegExp). */
export function validatePattern(pattern: string): string | null {
  if (pattern.trim() === '') return 'Pattern is empty'
  try {
    toJsRegExp(pattern)
    return null
  } catch (e) {
    return e instanceof Error ? e.message : 'Invalid pattern'
  }
}
