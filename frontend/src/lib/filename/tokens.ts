// Registry of yt-dlp metadata fields the builder can place as tokens.
//
// This array is the *only* place to add a new field: append an entry and it
// appears in the palette, the chip popover, and the mock preview automatically.
// No UI or compile changes are needed.

export type TokenCategory = 'video' | 'channel' | 'date' | 'quality' | 'playlist'

export type TokenDef = {
  /** Stable id (kept equal to `field` for simplicity). */
  id: string
  /** Human label shown in the palette and on the chip. */
  label: string
  /** yt-dlp metadata field name used in the output template. */
  field: string
  category: TokenCategory
  /** Tabler icon name (without the `ti-` prefix) for the chip. */
  icon: string
  /** Example value used to render the offline preview. */
  sample: string
  /** Date-like field: enables the strftime format option in the popover. */
  isDate?: boolean
  /** Whether the truncate option makes sense (long free-text fields). */
  supportsTruncate?: boolean
}

export const TOKENS: TokenDef[] = [
  // --- Video ---
  { id: 'title',          label: 'Title',        field: 'title',          category: 'video',    icon: 'file-text',  sample: 'Never Gonna Give You Up (Official Video)', supportsTruncate: true },
  { id: 'id',             label: 'Video ID',     field: 'id',             category: 'video',    icon: 'id',         sample: 'dQw4w9WgXcQ' },
  { id: 'duration_string',label: 'Duration',     field: 'duration_string',category: 'video',    icon: 'clock',      sample: '3:33' },
  { id: 'view_count',     label: 'Views',        field: 'view_count',     category: 'video',    icon: 'eye',        sample: '1600000000' },

  // --- Channel / uploader ---
  { id: 'uploader',       label: 'Uploader',     field: 'uploader',       category: 'channel',  icon: 'user',       sample: 'Rick Astley', supportsTruncate: true },
  { id: 'channel',        label: 'Channel',      field: 'channel',        category: 'channel',  icon: 'broadcast',  sample: 'Rick Astley', supportsTruncate: true },
  { id: 'channel_id',     label: 'Channel ID',   field: 'channel_id',     category: 'channel',  icon: 'hash',       sample: 'UCuAXFkgsw1L7xaCfnd5JJOw' },

  // --- Date ---
  { id: 'upload_date',    label: 'Upload date',  field: 'upload_date',    category: 'date',     icon: 'calendar',   sample: '20091025', isDate: true },
  { id: 'release_date',   label: 'Release date', field: 'release_date',   category: 'date',     icon: 'calendar-event', sample: '20091025', isDate: true },

  // --- Quality ---
  { id: 'resolution',     label: 'Resolution',   field: 'resolution',     category: 'quality',  icon: 'aspect-ratio', sample: '1920x1080' },
  { id: 'height',         label: 'Height',       field: 'height',         category: 'quality',  icon: 'arrows-vertical', sample: '1080' },
  { id: 'fps',            label: 'FPS',          field: 'fps',            category: 'quality',  icon: 'activity',   sample: '30' },

  // --- Playlist ---
  { id: 'playlist_title', label: 'Playlist',     field: 'playlist_title', category: 'playlist', icon: 'list',       sample: 'My Playlist', supportsTruncate: true },
  { id: 'playlist_index', label: 'Playlist #',   field: 'playlist_index', category: 'playlist', icon: 'list-numbers', sample: '1' },
]

const TOKEN_BY_FIELD = new Map(TOKENS.map((t) => [t.field, t]))

/** Look up a token definition by its yt-dlp field name. */
export function tokenByField(field: string): TokenDef | undefined {
  return TOKEN_BY_FIELD.get(field)
}

export const CATEGORY_LABELS: Record<TokenCategory, string> = {
  video: 'Video',
  channel: 'Channel',
  date: 'Date',
  quality: 'Quality',
  playlist: 'Playlist',
}
