import {
  CheckYtDlpUpdate,
  GetDepsStatus,
  InstallExtras,
  InstallFfmpeg,
  InstallPython,
  InstallYtDlpZip,
  OpenExternalURL,
  SkipFfmpeg,
  UpdateYtDlp,
  YtDlpVersion,
} from '../../../wailsjs/go/main/App.js'
import { dependencyInfo, type DependencyKey } from '../deps'

export class DepsState {
  deps = $state({ python: false, ytdlp: false, extras: false, ffmpeg: false, ffmpegSkipped: false })
  version = $state('')
  confirmTarget = $state<DependencyKey | null>(null)
  busy = $state(false)
  status = $state('')

  // Update state for the Settings → Dependencies pane. Kept separate from
  // `status` so the result stays visible next to the button that produced it,
  // rather than in the main window behind the modal.
  checking = $state(false)
  updating = $state(false)
  updateStatus = $state('')
  // Set once a check finds a newer release, and cleared when the user takes it.
  // The pane keys its confirm button off this.
  updateAvailable = $state(false)
  latestVersion = $state('')
  // Where to read what changed. Deliberately survives the update itself, so the
  // link is still there after the user has taken it.
  changelogUrl = $state('')

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

  /**
   * Asks GitHub what the current yt-dlp release is, without downloading it.
   *
   * Cheap enough to run during a download (it's one API call and touches
   * nothing on disk), so unlike the update itself it isn't gated on `busy`.
   */
  async checkForUpdate(): Promise<void> {
    this.checking = true
    this.updateAvailable = false
    this.changelogUrl = ''
    this.updateStatus = 'Checking for updates…'
    try {
      const result = await CheckYtDlpUpdate()
      if (result.current) this.version = result.current
      this.latestVersion = result.latest
      this.updateAvailable = result.available
      this.changelogUrl = result.changelogURL
      this.updateStatus = result.available
        ? `Version ${result.latest} is available.`
        : `You're up to date (${result.current}).`
    } catch (err) {
      this.updateStatus = `Update check failed: ${err}`
    } finally {
      this.checking = false
    }
  }

  /**
   * Re-downloads yt-dlp, replacing the installed copy with the current release.
   * Confirms what `checkForUpdate` found.
   *
   * Worth doing regularly: yt-dlp ships fixes for site changes constantly, and
   * an old copy simply stops being able to extract from YouTube. Nothing else
   * in the app ever replaces it, so this is the only path off the build the
   * user first installed.
   */
  async updateYtDlp(): Promise<void> {
    this.updating = true
    // Also blocks the Download button while the zipapp is being swapped out.
    this.busy = true
    this.updateStatus = 'Downloading the latest yt-dlp…'
    this.resetProgress()
    try {
      const result = await UpdateYtDlp()
      this.version = result.version
      this.updateAvailable = false
      this.updateStatus = result.updated
        ? `Updated to ${result.version}.`
        : `Already up to date (${result.version}).`
    } catch (err) {
      this.updateStatus = `Update failed: ${err}`
    } finally {
      this.updating = false
      this.busy = false
      this.resetProgress()
      await this.refresh()
    }
  }

  /**
   * Opens the changelog in the user's browser. The backend only accepts yt-dlp
   * project URLs, and this one came from the backend in the first place.
   */
  async openChangelog(): Promise<void> {
    if (!this.changelogUrl) return
    try {
      await OpenExternalURL(this.changelogUrl)
    } catch (err) {
      this.updateStatus = `Could not open the changelog: ${err}`
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
