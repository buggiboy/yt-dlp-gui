import {
  CancelDownload,
  ChooseDownloadFolder,
  DownloadVideo,
  LoadSettings,
  OpenDependenciesFolder,
  SaveSettings,
} from '../../../wailsjs/go/main/App.js'
import { isValidTime } from '../time'
import { checkUrlSyntax, extractPlaylistId, extractVideoId } from '../url'
import type { UrlProblem, VideoFormat, VideoPreviewData } from '../types'
import type { FilenameFormat } from '../filename/types'
import { compile } from '../filename/compile'
import { defaultFormat } from '../filename/presets'

const PRESET_FORMATS: VideoFormat[] = [2160, 1440, 1080, 720, 480].map((height) => ({
  height,
  ext: '',
}))

/** The options struct the backend's DownloadVideo/PreviewCommand accept.
 *  Derived from the generated binding rather than redeclared, so adding a field
 *  to Go's DownloadOptions surfaces here as a type error instead of silently
 *  going unsent. */
export type DownloadRequest = Parameters<typeof DownloadVideo>[0]

/** Matches yt-dlp's two cookie flags — `--cookies FILE` and
 *  `--cookies-from-browser BROWSER` — in either `--flag value` or `--flag=value`
 *  form. Deliberately loose: this only decides whether to trust the user over a
 *  "needs an account" verdict, so a false positive costs one failed attempt. */
const COOKIE_ARG_RE = /(^|\s)--cookies(-from-browser)?[\s=]/

/** Whether the user has supplied their own cookies in Extra arguments. */
export function hasCookieArgs(extraArgs: string): boolean {
  return COOKIE_ARG_RE.test(extraArgs)
}

/** How long to sit on a settings change before writing it to disk. Long enough
 *  that typing in a settings field doesn't rewrite the file per keystroke,
 *  short enough that closing the app right after a change still saves it. */
const PERSIST_DEBOUNCE_MS = 400

/** Bumped when the saved shape changes in a way `loadSettings` has to know
 *  about. Reads are defensive per-field regardless, so this is a hook for real
 *  migrations rather than a gate. */
const SETTINGS_VERSION = 2

/** Starting values for everything that persists. Declared once so the field
 *  initialisers below and `resetSettings` can't drift apart. Reference types get
 *  factory functions so no two owners ever share an object. */
type PersistedDefaults = {
  folder: string
  audioFormat: string
  subtitles: boolean
  subLangs: string
  embedMeta: boolean
  sponsorBlock: boolean
  extraArgs: string
  rateLimit: string
  concurrentFragments: number
}

// Explicitly typed rather than `as const` (or Object.freeze, which has the same
// effect for all-primitive objects): both would narrow each value to a literal
// type — '' instead of string — and every field initialised from it would
// inherit that narrowing and reject any real assignment.
const DEFAULTS: Readonly<PersistedDefaults> = {
  folder: '',
  audioFormat: '',
  subtitles: false,
  subLangs: 'en',
  embedMeta: false,
  sponsorBlock: false,
  extraArgs: '',
  rateLimit: '',
  concurrentFragments: 0,
}

const defaultSponsorCategories = (): string[] => ['sponsor']

// --- defensive readers -------------------------------------------------------
// Settings are user-editable JSON that may also predate the current schema, so
// every field is read through a guard: a value of the wrong type falls back to
// its default without taking the rest of the file down with it.

const readString = (v: unknown, fallback: string): string =>
  typeof v === 'string' ? v : fallback

const readBool = (v: unknown, fallback: boolean): boolean =>
  typeof v === 'boolean' ? v : fallback

const readNumber = (v: unknown, fallback: number): number =>
  typeof v === 'number' && Number.isFinite(v) ? v : fallback

const readStringArray = (v: unknown, fallback: string[]): string[] =>
  Array.isArray(v) && v.every((x) => typeof x === 'string') ? (v as string[]) : fallback

/** A stored filename format is the one setting that can break compilation
 *  outright, so it gets a shape check rather than a bare cast. Only the pieces
 *  `compile` walks are verified; anything malformed falls back to the default. */
function readFilenameFormat(v: unknown): FilenameFormat | null {
  if (!v || typeof v !== 'object') return null
  const f = v as Partial<FilenameFormat>
  if (!Array.isArray(f.segments) || !Array.isArray(f.customRules)) return null
  if (!Array.isArray(f.cleanupPresets)) return null
  const segmentsOk = f.segments.every(
    (s) =>
      s &&
      typeof s === 'object' &&
      (s.kind === 'literal' ? typeof s.text === 'string' : typeof s.field === 'string')
  )
  if (!segmentsOk) return null
  return v as FilenameFormat
}

export class DownloadState {
  // --- URL ---
  url = $state('')

  // --- URL validation ---
  // The verdict from the backend probe (yt-dlp actually looked), set by the
  // debounced effect in App.svelte. Null means "no objection" — either the URL
  // is fine or we haven't asked yet.
  remoteProblem = $state<UrlProblem | null>(null)
  // True while that probe is in flight, so the UI can say "Checking link…"
  // instead of flashing a stale verdict.
  checkingUrl = $state(false)
  // Flips true once the user has paused typing. Errors stay hidden until then,
  // so nobody gets scolded for a URL they're still halfway through pasting.
  urlSettled = $state(false)

  // --- Quality ---
  // Deliberately NOT persisted: applyFormats snaps this to the closest height a
  // given video actually offers, so a saved value would ratchet downwards every
  // time a low-resolution video came along. Resets to "best" each launch.
  quality = $state('best')
  audioFormat = $state(DEFAULTS.audioFormat)
  availableFormats = $state<VideoFormat[] | null>(null)
  audioExtension = $state('')
  loadingFormats = $state(false)

  // --- Playlist ---
  /** Opt-in to downloading every video in the playlist instead of just the one
   *  the URL names. Only meaningful (and only offered) when `isPlaylist` — see
   *  `playlistScope` for the value that actually reaches the backend. Reset
   *  whenever the URL moves to a different playlist, because grabbing a few
   *  hundred videos shouldn't be something a stale checkbox decides. */
  downloadPlaylist = $state(false)

  // --- Clip ---
  // Not persisted, on purpose: a clip range belongs to one specific video, and
  // a stale one would silently truncate the next download.
  clipStart = $state('')
  clipEnd = $state('')

  // --- Download folder ---
  folder = $state(DEFAULTS.folder)

  // --- Advanced options ---
  subtitles = $state(DEFAULTS.subtitles)
  subLangs = $state(DEFAULTS.subLangs)
  embedMeta = $state(DEFAULTS.embedMeta)
  sponsorBlock = $state(DEFAULTS.sponsorBlock)
  sponsorBlockCategories = $state<string[]>(defaultSponsorCategories())
  // Raw extra yt-dlp flags — an escape hatch for power users who need a flag the
  // UI doesn't surface. Passed through verbatim and parsed shell-style by the Go
  // backend. Persisted, since a cookie flag is exactly the sort of thing you
  // want every time — the Advanced options header flags when it's in use so a
  // saved flag can't sit there invisibly breaking downloads.
  extraArgs = $state(DEFAULTS.extraArgs)
  // Max download rate for --limit-rate, in yt-dlp's number+unit form (e.g. "5M").
  // Empty means unlimited. Configured in the Settings → Downloads tab.
  rateLimit = $state(DEFAULTS.rateLimit)
  // Number of fragments to download in parallel (--concurrent-fragments N).
  // 0 leaves yt-dlp at its default of 1 (sequential); 4/8/16 are the offered
  // presets. A big speedup on sites that serve fragmented media. Configured in
  // Settings → Downloads.
  concurrentFragments = $state(DEFAULTS.concurrentFragments)
  // Structured output-filename configuration built in the Filename format modal.
  // Compiled to an -o template + naming flags at download time.
  filenameFormat = $state<FilenameFormat>(defaultFormat())

  // --- Persistence ---
  /** False until loadSettings has run, so the autosave effect doesn't echo the
   *  file straight back to disk before we've read it. */
  hydrated = $state(false)

  // --- Preview ---
  preview = $state<VideoPreviewData | null>(null)
  loadingPreview = $state(false)
  // Real metadata for the current URL (title, uploader, upload date, …), keyed
  // by yt-dlp field name. Comes back from the same CheckURL probe that fills
  // the quality list, and is what turns the "Saves as" line from a placeholder
  // sample into the name this download will actually get. Null until a check
  // succeeds.
  videoMeta = $state<Record<string, string> | null>(null)

  // --- Progress & status ---
  busy = $state(false)
  // True between clicking Cancel and the backend call returning. Lets the UI
  // show "Cancelling…" and tells the error handler that the failure it's about
  // to see was intentional.
  cancelling = $state(false)
  percent = $state(0)
  speed = $state('')
  eta = $state('')
  step = $state('')
  status = $state('')

  // --- Derived ---
  clipValid = $derived(isValidTime(this.clipStart) && isValidTime(this.clipEnd))
  formatOptions = $derived<VideoFormat[]>(
    this.availableFormats && this.availableFormats.length > 0
      ? this.availableFormats
      : PRESET_FORMATS
  )
  videoId = $derived(extractVideoId(this.url))

  /** Everything that survives a restart, in one object.
   *
   *  This is deliberately both the thing that gets serialised *and* the thing
   *  the autosave effect watches: reading it subscribes to every persisted
   *  field at once, so the saved set and the change-detection set are the same
   *  list by construction. Adding a setting means adding one line here.
   *
   *  Notable absences, all intentional: `quality` (snapping would ratchet a
   *  saved value downwards), `clipStart`/`clipEnd` (belong to one video), and
   *  `downloadPlaylist` (belongs to one playlist). */
  persistedSettings = $derived({
    version: SETTINGS_VERSION,
    folder: this.folder,
    audioFormat: this.audioFormat,
    subtitles: this.subtitles,
    subLangs: this.subLangs,
    embedMeta: this.embedMeta,
    sponsorBlock: this.sponsorBlock,
    sponsorBlockCategories: this.sponsorBlockCategories,
    extraArgs: this.extraArgs,
    rateLimit: this.rateLimit,
    concurrentFragments: this.concurrentFragments,
    filenameFormat: this.filenameFormat,
  })

  /** Identifies the playlist this URL belongs to, or null when it isn't one. */
  playlistId = $derived(extractPlaylistId(this.url))
  isPlaylist = $derived(this.playlistId !== null)

  /** The value the backend actually gets. Gating on `isPlaylist` means a
   *  checkbox left ticked can never widen the scope of a URL that isn't a
   *  playlist at all. */
  playlistScope = $derived(this.isPlaylist && this.downloadPlaylist)

  /** The instant, offline verdict. Cheap enough to recompute on every
   *  keystroke, and certain enough to skip the backend probe entirely. */
  syntaxProblem = $derived<UrlProblem | null>(
    (() => {
      const issue = checkUrlSyntax(this.url)
      // A string that isn't a web address can't become one on the network, so
      // these always block.
      return issue ? { kind: issue.kind, message: issue.message, blocking: true } : null
    })()
  )

  /** The single problem to display. The syntax check wins when it fires — it's
   *  both faster and more specific than anything yt-dlp would say back. */
  urlProblem = $derived<UrlProblem | null>(
    this.urlSettled ? (this.syntaxProblem ?? this.remoteProblem) : null
  )

  /** True when the user has put cookie flags in Extra arguments. A
   *  "needs an account" verdict is exactly what cookies are for, so their
   *  presence means we shouldn't take yt-dlp's anonymous probe as the last
   *  word. */
  cookiesSupplied = $derived(hasCookieArgs(this.extraArgs))

  /** True when the displayed problem is an auth verdict the user has already
   *  answered with cookies — shown as a heads-up rather than an error. */
  authOverridden = $derived(this.urlProblem?.kind === 'auth' && this.cookiesSupplied)

  /** Whether the URL is bad enough to stop the download. Transient failures
   *  (network) surface a message but leave the button live, and so does an
   *  auth verdict once cookies are in play — the probe ran without them, so it
   *  can't know what the user's account can see. */
  urlBlocked = $derived(this.urlProblem?.blocking === true && !this.authOverridden)

  /** The metadata filename previews should render against: what the URL check
   *  resolved, falling back to the lighter preview lookup for the window before
   *  a check lands (and for sites where only the preview succeeds). `undefined`
   *  when nothing is known, which leaves the preview showing its sample values.
   *
   *  Fields the probe can't answer — resolution, height, fps — are deliberately
   *  absent: they depend on the format yt-dlp picks at download time, so a
   *  sample is more honest than a confident guess. */
  previewMeta = $derived<Record<string, string> | undefined>(
    (() => {
      const fromPreview: Record<string, string> = {}
      if (this.preview?.title) fromPreview.title = this.preview.title
      if (this.preview?.duration) fromPreview.duration_string = this.preview.duration
      // The check's metadata is the more authoritative of the two, so it wins.
      const merged = { ...fromPreview, ...(this.videoMeta ?? {}) }
      return Object.keys(merged).length > 0 ? merged : undefined
    })()
  )

  /** Adopts the format list from a successful URL check and snaps the quality
   *  choice to the closest height this video actually offers, so the picker
   *  never shows a resolution that would silently fall back to something else. */
  applyFormats(videos: VideoFormat[], audioExt: string): void {
    this.availableFormats = videos
    this.audioExtension = audioExt

    const heights = videos.map((v) => v.height)
    const selected = Number(this.quality)
    if (
      !heights.length ||
      Number.isNaN(selected) ||
      this.quality === 'best' ||
      this.quality === 'audio' ||
      heights.includes(selected)
    ) {
      return
    }
    const closest = heights.reduce((a, b) =>
      Math.abs(b - selected) < Math.abs(a - selected) ? b : a
    )
    this.quality = String(closest)
  }

  /** Clears progress bar state. Called at the start of any operation, and at
   *  the end of installs (so the bar disappears). Download intentionally does
   *  NOT call this at the end — the final percent stays visible until the next
   *  operation begins. */
  resetProgress(): void {
    this.percent = 0
    this.speed = ''
    this.eta = ''
    this.step = ''
  }

  /** The exact options the backend receives for this download.
   *
   *  Shared by `download()` and the command preview, and that sharing is the
   *  point: the preview is rendered by the backend (PreviewCommand) from this
   *  same struct, using the same argument builder the download itself uses. So
   *  the command shown to the user is the command that runs — there is no
   *  second implementation to keep in sync.
   *
   *  Options that ffmpeg gates are zeroed out here rather than in the backend,
   *  so an unusable clip range or audio format never leaves the frontend and
   *  the preview doesn't advertise a conversion that won't happen. */
  downloadOptions(env: { ffmpegAvailable: boolean }): DownloadRequest {
    const { outtmpl, nameArgs } = compile(this.filenameFormat)
    return {
      url: this.url,
      start: env.ffmpegAvailable ? this.clipStart.trim() : '',
      end: env.ffmpegAvailable ? this.clipEnd.trim() : '',
      playlist: this.playlistScope,
      quality: this.quality,
      audioFormat: this.quality === 'audio' && env.ffmpegAvailable ? this.audioFormat : '',
      folder: this.folder,
      subtitles: this.subtitles,
      subLangs: this.subtitles ? this.subLangs.trim() || 'en' : '',
      embedMeta: this.embedMeta,
      sponsorBlock: this.sponsorBlock ? this.sponsorBlockCategories : [],
      rateLimit: this.rateLimit.trim(),
      concurrentFragments: this.concurrentFragments,
      extraArgs: this.extraArgs.trim(),
      outtmpl,
      nameArgs,
    }
  }

  async download(opts: { ffmpegAvailable: boolean; saveFolder: string }): Promise<void> {
    // Backstop: the button is already disabled for a blocking problem, but a
    // keyboard activation could race the debounce that sets urlSettled. The
    // cookie exemption below mirrors urlBlocked.
    const problem = this.syntaxProblem ?? this.remoteProblem
    if (problem?.blocking && !(problem.kind === 'auth' && this.cookiesSupplied)) {
      this.urlSettled = true
      this.status = problem.message
      return
    }
    this.busy = true
    this.cancelling = false
    this.resetProgress()
    this.status = ''
    try {
      await DownloadVideo(this.downloadOptions({ ffmpegAvailable: opts.ffmpegAvailable }))
      this.status = `Done! Saved to ${opts.saveFolder || 'your Downloads folder'}.`
    } catch (err) {
      // A cancelled download fails by design — report it as the user's own
      // action rather than as something that went wrong.
      this.status = this.cancelling ? 'Download cancelled.' : `${err}`
    } finally {
      this.busy = false
      this.cancelling = false
    }
  }

  /** Stops the running download. The backend kills yt-dlp and any ffmpeg it
   *  spawned; the in-flight `download()` call then rejects and reports the
   *  cancellation. Partial `.part` files are left in place, so starting the
   *  same download again resumes rather than restarts. */
  async cancel(): Promise<void> {
    if (!this.busy || this.cancelling) return
    this.cancelling = true
    this.step = 'Cancelling…'
    try {
      await CancelDownload()
    } catch (err) {
      // Couldn't reach the backend — leave `cancelling` set so that if the
      // download does die we still describe it correctly.
      this.status = `${err}`
    }
  }

  async chooseFolder(): Promise<void> {
    try {
      const picked = await ChooseDownloadFolder()
      if (picked) this.folder = picked
    } catch (err) {
      this.status = `${err}`
    }
  }

  /** Reveals the app's managed dependencies folder (Python, yt-dlp, ffmpeg,
   *  etc.) in the OS file manager. */
  async openDepsFolder(): Promise<void> {
    try {
      await OpenDependenciesFolder()
    } catch (err) {
      this.status = `${err}`
    }
  }

  /** Loads persisted settings from disk. Called once at startup.
   *
   *  Every field is read defensively and independently, so a hand-edited or
   *  older settings file degrades one setting at a time rather than being
   *  thrown away wholesale. Sets `hydrated` on the way out — including on
   *  failure, since "we tried" is what the autosave effect is waiting for. */
  async loadSettings(): Promise<void> {
    try {
      const raw = await LoadSettings()
      if (!raw) return
      const parsed = JSON.parse(raw)
      if (!parsed || typeof parsed !== 'object') return

      this.folder = readString(parsed.folder, DEFAULTS.folder)
      this.audioFormat = readString(parsed.audioFormat, DEFAULTS.audioFormat)
      this.subtitles = readBool(parsed.subtitles, DEFAULTS.subtitles)
      this.subLangs = readString(parsed.subLangs, DEFAULTS.subLangs)
      this.embedMeta = readBool(parsed.embedMeta, DEFAULTS.embedMeta)
      this.sponsorBlock = readBool(parsed.sponsorBlock, DEFAULTS.sponsorBlock)
      this.sponsorBlockCategories = readStringArray(
        parsed.sponsorBlockCategories,
        defaultSponsorCategories()
      )
      this.extraArgs = readString(parsed.extraArgs, DEFAULTS.extraArgs)
      this.rateLimit = readString(parsed.rateLimit, DEFAULTS.rateLimit)

      const format = readFilenameFormat(parsed.filenameFormat)
      if (format) this.filenameFormat = format

      if (typeof parsed.concurrentFragments === 'number') {
        this.concurrentFragments = readNumber(
          parsed.concurrentFragments,
          DEFAULTS.concurrentFragments
        )
      } else if (parsed.fasterDownloads === true) {
        // Migrate the old boolean toggle: "on" mapped to 4 parallel fragments.
        this.concurrentFragments = 4
      }
    } catch {
      // Corrupt or unreadable settings — keep the defaults.
    } finally {
      this.hydrated = true
    }
  }

  /** Restores every persisted setting to its default and writes that out
   *  immediately. The escape hatch for a saved value that's causing trouble —
   *  most plausibly a bad flag in Extra arguments. Leaves per-download state
   *  (URL, quality, clip) alone, since none of it was saved in the first
   *  place. */
  async resetSettings(): Promise<void> {
    this.folder = DEFAULTS.folder
    this.audioFormat = DEFAULTS.audioFormat
    this.subtitles = DEFAULTS.subtitles
    this.subLangs = DEFAULTS.subLangs
    this.embedMeta = DEFAULTS.embedMeta
    this.sponsorBlock = DEFAULTS.sponsorBlock
    this.sponsorBlockCategories = defaultSponsorCategories()
    this.extraArgs = DEFAULTS.extraArgs
    this.rateLimit = DEFAULTS.rateLimit
    this.concurrentFragments = DEFAULTS.concurrentFragments
    this.filenameFormat = defaultFormat()

    // Skip the debounce: a reset should be on disk before the user can act on
    // the result.
    if (this.persistTimer !== null) {
      clearTimeout(this.persistTimer)
      this.persistTimer = null
    }
    await this.persistSettings()
  }

  /** Pending debounced write, if any. */
  private persistTimer: ReturnType<typeof setTimeout> | null = null

  /** Coalesces rapid settings changes into one disk write. Typing in a settings
   *  field fires on every keystroke; without this each one rewrote the whole
   *  settings file. Called by the autosave effect in App.svelte. */
  schedulePersist(): void {
    if (this.persistTimer !== null) clearTimeout(this.persistTimer)
    this.persistTimer = setTimeout(() => {
      this.persistTimer = null
      void this.persistSettings()
    }, PERSIST_DEBOUNCE_MS)
  }

  /** Writes the persisted settings to disk as a single JSON blob. Best-effort:
   *  failures are swallowed so persistence never blocks the UI. */
  private async persistSettings(): Promise<void> {
    try {
      await SaveSettings(JSON.stringify(this.persistedSettings))
    } catch {
      // Non-fatal: the in-memory values still apply for this session.
    }
  }

  // Setters below exist because the Settings and filename modals take callbacks
  // rather than binding directly. None of them save — the autosave effect
  // watching `persistedSettings` handles that for every field uniformly.

  /** Updates the active filename format. Called when the modal is applied. */
  setFilenameFormat(fmt: FilenameFormat): void {
    this.filenameFormat = fmt
  }

  /** Updates the download speed limit (yt-dlp number+unit form like "500K" or
   *  "5M"; "" = unlimited). */
  setRateLimit(value: string): void {
    this.rateLimit = value
  }

  /** Sets the number of parallel fragment downloads (0 = off). */
  setConcurrentFragments(value: number): void {
    this.concurrentFragments = value
  }
}
