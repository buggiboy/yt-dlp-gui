import {
  GetDepsStatus,
  InstallExtras,
  InstallFfmpeg,
  InstallPython,
  InstallYtDlpZip,
  SkipFfmpeg,
  YtDlpVersion,
} from '../../../wailsjs/go/main/App.js'
import { dependencyInfo, type DependencyKey } from '../deps'

export class DepsState {
  deps = $state({ python: false, ytdlp: false, extras: false, ffmpeg: false, ffmpegSkipped: false })
  version = $state('')
  confirmTarget = $state<DependencyKey | null>(null)
  busy = $state(false)
  status = $state('')

  // ffmpegAvailable is derived directly from deps.ffmpeg, eliminating the
  // redundant separate state variable that previously required manual syncing.
  installed = $derived(this.deps.python && this.deps.ytdlp && this.deps.extras)
  setupComplete = $derived(this.installed && (this.deps.ffmpeg || this.deps.ffmpegSkipped))
  ffmpegAvailable = $derived(this.deps.ffmpeg)

  /**
   * resetProgress is injected from App so that install operations can clear
   * the shared progress bar (which lives in DownloadState) at start and end,
   * without creating a circular dependency between the two state classes.
   */
  constructor(private readonly resetProgress: () => void) {}

  async refresh(): Promise<void> {
    this.deps = await GetDepsStatus()
    if (this.installed && !this.version) {
      try {
        this.version = await YtDlpVersion()
      } catch {
        this.version = ''
      }
    }
  }

  async install(target: DependencyKey): Promise<void> {
    this.confirmTarget = null
    this.busy = true
    this.resetProgress()
    this.status = `Downloading ${dependencyInfo[target].name}…`
    try {
      if (target === 'python') await InstallPython()
      else if (target === 'ytdlp') await InstallYtDlpZip()
      else if (target === 'ffmpeg') await InstallFfmpeg()
      else await InstallExtras()
      this.status = `${dependencyInfo[target].name}: installed.`
    } catch (err) {
      this.status = `Install failed: ${err}`
    } finally {
      this.busy = false
      this.resetProgress()
      await this.refresh()
    }
  }

  async skip(): Promise<void> {
    try {
      await SkipFfmpeg()
    } finally {
      await this.refresh()
    }
  }
}
