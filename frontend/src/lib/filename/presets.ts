// Named starting points for the filename builder, plus small model helpers.

import type { FilenameFormat, Segment, TokenOpts } from './types'
import { segmentsToTemplate } from './compile'

/** Stable-ish unique id for segments and rules. */
export function uid(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID()
  }
  return 'id-' + Math.random().toString(36).slice(2) + Date.now().toString(36)
}

export function tokenSeg(field: string, opts: TokenOpts = {}): Segment {
  return { id: uid(), kind: 'token', field, opts }
}

export function literalSeg(text: string): Segment {
  return { id: uid(), kind: 'literal', text }
}

/** Deep clone, refreshing ids so a cloned draft never shares object identity
 *  with the original (avoids accidental shared mutation through proxies). */
export function cloneFormat(f: FilenameFormat): FilenameFormat {
  return {
    segments: f.segments.map((s) =>
      s.kind === 'token'
        ? { id: uid(), kind: 'token', field: s.field, opts: { ...s.opts } }
        : { id: uid(), kind: 'literal', text: s.text }
    ),
    cleanupPresets: [...f.cleanupPresets],
    customRules: f.customRules.map((r) => ({ ...r, id: uid() })),
    restrictFilenames: f.restrictFilenames,
    maxLength: f.maxLength,
  }
}

function base(segments: Segment[]): FilenameFormat {
  return {
    segments,
    cleanupPresets: [],
    customRules: [],
    restrictFilenames: false,
    maxLength: 0,
  }
}

/** The out-of-the-box format: matches the app's previous hardcoded behaviour
 *  (`%(title)s.%(ext)s`). */
export function defaultFormat(): FilenameFormat {
  return base([tokenSeg('title')])
}

export type NamedPreset = { id: string; label: string; build: () => FilenameFormat }

export const NAMED_PRESETS: NamedPreset[] = [
  { id: 'title', label: 'Title', build: () => base([tokenSeg('title')]) },
  {
    id: 'title_id',
    label: 'Title [ID]',
    build: () => base([tokenSeg('title'), literalSeg(' ['), tokenSeg('id'), literalSeg(']')]),
  },
  {
    id: 'uploader_title',
    label: 'Uploader - Title',
    build: () => base([tokenSeg('uploader'), literalSeg(' - '), tokenSeg('title')]),
  },
  {
    id: 'date_title',
    label: 'Date - Title',
    build: () =>
      base([
        tokenSeg('upload_date', { dateFormat: '%Y-%m-%d' }),
        literalSeg(' - '),
        tokenSeg('title'),
      ]),
  },
  {
    id: 'channel_folder',
    label: 'Channel / Title (subfolder)',
    build: () => base([tokenSeg('channel'), literalSeg('/'), tokenSeg('title')]),
  },
]

/** Returns the id of the named preset whose *segment layout* matches the given
 *  format, or 'custom' if none do. Compares the compiled template so option
 *  differences (ids, advanced settings) don't matter. */
export function matchPreset(format: FilenameFormat): string {
  const target = segmentsToTemplate(format.segments)
  for (const p of NAMED_PRESETS) {
    if (segmentsToTemplate(p.build().segments) === target) return p.id
  }
  return 'custom'
}
