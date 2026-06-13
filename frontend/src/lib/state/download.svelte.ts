import {
  ChooseDownloadFolder,
  DownloadVideo,
  OpenDependenciesFolder,
} from '../../../wailsjs/go/main/App.js'
import { isValidTime } from '../time'
import { extractVideoId } from '../url'
import type { VideoFormat, VideoPreviewData } from '../types'

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
}
