// Offline preview: simulate the filename yt-dlp would produce, using the mock
// sample values from the token registry. No network, instant feedback.
//
// This mirrors yt-dlp's pipeline order: --replace-in-metadata edits the *field
// values* first, then the output template is expanded. Regex is applied with
// JS RegExp, so anything regex touches is only approximate (see toJsRegExp).

import type { FilenameFormat, Segment, TokenOpts } from './types'
import { TOKENS, tokenByField } from './tokens'
import { enabledPresetsInOrder, toJsRegExp } from './cleanup'

/** Builds a fresh mock metadata record from the token registry samples. */
function mockMetadata(): Record<string, string> {
  const meta: Record<string, string> = {}
  for (const t of TOKENS) meta[t.field] = t.sample
  return meta
}

/** Minimal strftime over a yt-dlp YYYYMMDD date string, covering the codes the
 *  date popover offers. */
function formatDate(yyyymmdd: string, fmt: string): string {
  const m = /^(\d{4})(\d{2})(\d{2})$/.exec(yyyymmdd)
  if (!m) return yyyymmdd
  const [, Y, Mo, D] = m
  return fmt
    .replace(/%Y/g, Y)
    .replace(/%y/g, Y.slice(2))
    .replace(/%m/g, Mo)
    .replace(/%d/g, D)
    .replace(/%H/g, '00')
    .replace(/%M/g, '00')
    .replace(/%S/g, '00')
}

function expandToken(
  field: string,
  opts: TokenOpts,
  meta: Record<string, string>
): string {
  const def = tokenByField(field)
  let val = meta[field] ?? ''
  if (def?.isDate && opts.dateFormat && val) {
    val = formatDate(val, opts.dateFormat)
  }
  if ((val === '' || val == null) && opts.default) {
    val = opts.default
  }
  if (opts.truncate && opts.truncate > 0) {
    val = val.slice(0, Math.floor(opts.truncate))
  }
  return val
}

export type PreviewResult = {
  /** The example filename. */
  name: string
  /** True when a regex transform was applied, so the UI can flag it as
   *  approximate (JS RegExp ≠ Python re). */
  approximate: boolean
  /** Non-fatal note (e.g. an invalid custom pattern was skipped). */
  warning: string | null
}

export type PreviewOptions = {
  /** File extension to append (without the dot). Defaults to "mp4". Lets the
   *  caller reflect the current quality choice (e.g. an audio-only download). */
  ext?: string
  /** Real metadata values to substitute for the registry samples — e.g. the
   *  actual video title once a preview lookup has resolved it. Empty/blank
   *  values are ignored so the sample still fills in. */
  meta?: Partial<Record<string, string>>
}

/** Produces an example filename for the given format. Never throws — a bad
 *  custom pattern is skipped and reported via `warning`. */
export function preview(format: FilenameFormat, options: PreviewOptions = {}): PreviewResult {
  const meta = mockMetadata()
  // Overlay any real metadata the caller supplied (blank values ignored).
  for (const [field, value] of Object.entries(options.meta ?? {})) {
    if (value != null && value !== '') meta[field] = value
  }
  let approximate = false
  let warning: string | null = null

  // 1. Apply Tier-1 presets to the title field, in canonical order.
  for (const preset of enabledPresetsInOrder(format.cleanupPresets)) {
    try {
      meta[preset.field] = (meta[preset.field] ?? '').replace(
        toJsRegExp(preset.pattern),
        preset.replace
      )
      approximate = true
    } catch {
      // Built-in patterns are known-good; ignore defensively.
    }
  }

  // 2. Apply Tier-3 custom rules in list order.
  for (const rule of format.customRules) {
    if (rule.pattern.trim() === '') continue
    const field = rule.field.trim() || 'title'
    try {
      meta[field] = (meta[field] ?? '').replace(toJsRegExp(rule.pattern), rule.replace)
      approximate = true
    } catch {
      warning = `Custom rule for "${field}" has an invalid pattern and was skipped`
    }
  }

  // 3. Expand the template against the (possibly edited) metadata. yt-dlp
  //    sanitizes path separators out of *field values* (so a title can't create
  //    a folder), but literal separators the user typed are kept — that's how
  //    subfolders are made intentionally. Mirror both here.
  let name = format.segments
    .map((seg: Segment) =>
      seg.kind === 'token'
        ? expandToken(seg.field, seg.opts, meta).replace(/[\\/]+/g, '_')
        : seg.text
    )
    .join('')

  // 4. Global filename options (approximate simulations of yt-dlp's sanitizer).
  if (format.restrictFilenames) {
    name = name.replace(/\s+/g, '_').replace(/[^A-Za-z0-9_.\-/]/g, '')
  }
  if (format.maxLength && format.maxLength > 0) {
    name = name.slice(0, Math.floor(format.maxLength))
  }

  const ext = (options.ext ?? 'mp4').replace(/^\./, '') || 'mp4'
  return { name: `${name}.${ext}`, approximate, warning }
}
