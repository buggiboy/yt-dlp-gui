// The single source of truth: turn a FilenameFormat into the two things yt-dlp
// needs — an `-o` output template and a list of naming-related args. Used by
// both the live command preview (command.ts) and the real download
// (download.svelte.ts → backend).

import type { CompiledFormat, FilenameFormat, Segment } from './types'
import { tokenByField } from './tokens'
import { enabledPresetsInOrder } from './cleanup'

/** The implicit, non-removable extension suffix. */
export const EXT_SUFFIX = '.%(ext)s'

/** Renders one token segment as a yt-dlp field expression, e.g.
 *  `%(upload_date>%Y-%m-%d|Unknown).40s`. Modifier order matters:
 *    %(  FIELD  >DATEFMT  |DEFAULT  )  .PRECISION  s
 */
function tokenToTemplate(seg: Extract<Segment, { kind: 'token' }>): string {
  const def = tokenByField(seg.field)
  let inner = seg.field
  if (def?.isDate && seg.opts.dateFormat) {
    inner += '>' + seg.opts.dateFormat
  }
  if (seg.opts.default && seg.opts.default !== '') {
    inner += '|' + seg.opts.default
  }
  const prec =
    seg.opts.truncate && seg.opts.truncate > 0 ? '.' + Math.floor(seg.opts.truncate) : ''
  return `%(${inner})${prec}s`
}

/** Builds the filename portion of the `-o` template (no folder, ext appended). */
export function segmentsToTemplate(segments: Segment[]): string {
  const body = segments
    .map((seg) =>
      seg.kind === 'token'
        ? tokenToTemplate(seg)
        : // Escape literal percent signs so yt-dlp doesn't read them as fields.
          seg.text.replace(/%/g, '%%')
    )
    .join('')
  return (body || '%(title)s') + EXT_SUFFIX
}

/** Compiles a FilenameFormat into yt-dlp inputs. The nameArgs are discrete argv
 *  entries (no shell quoting) so the Go backend can append them as-is. */
export function compile(format: FilenameFormat): CompiledFormat {
  const nameArgs: string[] = []

  // Tier-1 presets, in canonical order, then Tier-3 custom rules in list order.
  for (const preset of enabledPresetsInOrder(format.cleanupPresets)) {
    nameArgs.push('--replace-in-metadata', preset.field, preset.pattern, preset.replace)
  }
  for (const rule of format.customRules) {
    const field = rule.field.trim() || 'title'
    if (rule.pattern.trim() === '') continue
    nameArgs.push('--replace-in-metadata', field, rule.pattern, rule.replace)
  }

  if (format.restrictFilenames) nameArgs.push('--restrict-filenames')
  if (format.maxLength && format.maxLength > 0) {
    nameArgs.push('--trim-filenames', String(Math.floor(format.maxLength)))
  }

  return { outtmpl: segmentsToTemplate(format.segments), nameArgs }
}
