import {
  ChooseDownloadFolder,
  DownloadVideo,
  LoadSettings,
  OpenDependenciesFolder,
  SaveSettings,
} from '../../../wailsjs/go/main/App.js'
import { isValidTime } from '../time'
import { extractVideoId } from '../url'
import type { VideoFormat, VideoPreviewData } from '../types'
import type { FilenameFormat } from '../filename/types'
import { compile } from '../filename/compile'
import { defaultFormat } from '../filename/presets'

const PRESET_FORMATS: VideoFormat[] = [2160, 1440, 1080, 720, 480].map((height) => ({
  height,
  ext: '',
}))

export class DownloadState {
  // --- URL ---
  url = $state('')

  // --- Quality ---
  quality = $state('best')
  audioFormat = $state('')
  availableFormats = $state<VideoFormat[] | null>(null)
  audioExtension = $state('')
  loadingFormats = $state(false)

  // --- Clip ---
  clipStart = $state('')
  clipEnd = $state('')

  // --- Download folder ---
  folder = $state('')

  // --- Advanced options ---
  subtitles = $state(false)
  subLangs = $state('en')
  embedMeta = $state(false)
  sponsorBlock = $state(false)
  sponsorBlockCategories = $state<string[]>(['sponsor'])
  // Raw extra yt-dlp flags — an escape hatch for power users who need a flag the
  // UI doesn't surface. Passed through verbatim and parsed shell-style by the Go
  // backend.
  extraArgs = $state('')
  // Max download rate for --limit-rate, in yt-dlp's number+unit form (e.g. "5M").
  // Empty means unlimited. Configured in the Settings → Downloads tab and
  // persisted via the Go settings layer.
  rateLimit = $state('')
  // Number of fragments to download in parallel (--concurrent-fragments N).
  // 0 leaves yt-dlp at its default of 1 (sequential); 4/8/16 are the offered
  // presets. A big speedup on sites that serve fragmented media. Configured in
  // Settings → Downloads and persisted via the Go settings layer.
  concurrentFragments = $state(0)
  // Structured output-filename configuration built in the Filename format modal.
  // Compiled to an -o template + naming flags at download time, and persisted
  // via the Go settings layer.
  filenameFormat = $state<FilenameFormat>(defaultFormat())

  // --- Preview ---
  preview = $state<VideoPreviewData | null>(null)
  loadingPreview = $state(false)

  // --- Progress & status ---
  busy = $state(false)
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

  async download(opts: { ffmpegAvailable: boolean; saveFolder: string }): Promise<void> {
    this.busy = true
    this.resetProgress()
    this.status = ''
    const { outtmpl, nameArgs } = compile(this.filenameFormat)
    try {
      await DownloadVideo({
        url: this.url,
        start: opts.ffmpegAvailable ? this.clipStart.trim() : '',
        end: opts.ffmpegAvailable ? this.clipEnd.trim() : '',
        quality: this.quality,
        audioFormat: this.quality === 'audio' && opts.ffmpegAvailable ? this.audioFormat : '',
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
      })
      this.status = `Done! Saved to ${opts.saveFolder || 'your Downloads folder'}.`
    } catch (err) {
      this.status = `${err}`
    } finally {
      this.busy = false
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

  /** Loads persisted settings (currently just the filename format) from disk.
   *  Best-effort: any parse/IO error leaves the default in place. Called once
   *  at startup. */
  async loadSettings(): Promise<void> {
    try {
      const raw = await LoadSettings()
      if (!raw) return
      const parsed = JSON.parse(raw)
      if (parsed && parsed.filenameFormat) {
        this.filenameFormat = parsed.filenameFormat as FilenameFormat
      }
      if (parsed && typeof parsed.rateLimit === 'string') {
        this.rateLimit = parsed.rateLimit
      }
      if (parsed && typeof parsed.concurrentFragments === 'number') {
        this.concurrentFragments = parsed.concurrentFragments
      } else if (parsed && parsed.fasterDownloads === true) {
        // Migrate the old boolean toggle: "on" mapped to 4 parallel fragments.
        this.concurrentFragments = 4
      }
    } catch {
      // Corrupt or missing settings — keep the defaults.
    }
  }

  /** Writes all persisted settings to disk as a single JSON blob. Best-effort:
   *  failures are swallowed so persistence never blocks the UI. */
  private async persistSettings(): Promise<void> {
    try {
      await SaveSettings(
        JSON.stringify({
          filenameFormat: this.filenameFormat,
          rateLimit: this.rateLimit,
          concurrentFragments: this.concurrentFragments,
        })
      )
    } catch {
      // Non-fatal: the in-memory values still apply for this session.
    }
  }

  /** Updates the active filename format and persists it. Called when the modal
   *  is applied. */
  async setFilenameFormat(fmt: FilenameFormat): Promise<void> {
    this.filenameFormat = fmt
    await this.persistSettings()
  }

  /** Updates the download speed limit (yt-dlp number+unit form, "" = unlimited)
   *  and persists it. Called from the Settings → Downloads tab. */
  async setRateLimit(value: string): Promise<void> {
    this.rateLimit = value
    await this.persistSettings()
  }

  /** Sets the number of parallel fragment downloads (0 = off) and persists it.
   *  Called from the Settings → Downloads tab. */
  async setConcurrentFragments(value: number): Promise<void> {
    this.concurrentFragments = value
    await this.persistSettings()
  }
}
