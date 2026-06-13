// Structured model for the output-filename builder.
//
// This model is the single source of truth. It is compiled (see compile.ts)
// into the two yt-dlp mechanisms it maps onto:
//   - the `-o` output template  (field placeholders, modifiers)
//   - `--replace-in-metadata`   (regex cleanup of field values)
// and it is simulated (see preview.ts) against mock metadata so the UI can show
// an example filename without contacting the network.

/** Inline modifiers applied to a single field within the output template. */
export type TokenOpts = {
  /** Max characters to keep (yt-dlp printf precision, e.g. `%(title).40s`). */
  truncate?: number
  /** Fallback value when the field is missing/empty (`%(uploader|Unknown)s`). */
  default?: string
  /** strftime format for date-like fields (`%(upload_date>%Y-%m-%d)s`). */
  dateFormat?: string
}

/** One piece of the filename: either fixed text the user typed, or a field. */
export type Segment =
  | { id: string; kind: 'literal'; text: string }
  | { id: string; kind: 'token'; field: string; opts: TokenOpts }

/** A user-authored regex transform — the Tier-3 power-user escape hatch.
 *  Compiles to `--replace-in-metadata <field> <pattern> <replace>`. */
export type RegexRule = {
  id: string
  field: string
  pattern: string
  replace: string
}

/** Ids of the friendly Tier-1 cleanup toggles. See cleanup.ts for definitions. */
export type CleanupPresetId =
  | 'stripTags'
  | 'collapseSpace'
  | 'trimSeparators'
  | 'spacesToUnderscores'

/** The complete, serializable filename configuration. Persisted as JSON via the
 *  Go settings layer and round-tripped unchanged. */
export type FilenameFormat = {
  /** Ordered name parts. The `.%(ext)s` suffix is implicit and added at compile
   *  time, so it can never be removed or reordered by the user. */
  segments: Segment[]
  /** Which Tier-1 cleanup toggles are enabled (applied to the title field). */
  cleanupPresets: CleanupPresetId[]
  /** Tier-3 custom regex rules, applied in order after the presets. */
  customRules: RegexRule[]
  /** ASCII-only, space-free names (`--restrict-filenames`). */
  restrictFilenames: boolean
  /** Trim the final name to this many characters (`--trim-filenames`). 0 = off. */
  maxLength: number
}

/** Result of compiling a FilenameFormat into yt-dlp inputs. */
export type CompiledFormat = {
  /** The `-o` template, filename only (no folder, ext suffix included). */
  outtmpl: string
  /** Naming-related yt-dlp args, already tokenized as discrete argv entries so
   *  the Go backend can splice them in without shell parsing. */
  nameArgs: string[]
}
