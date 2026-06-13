import { isValidTime } from './time'

/** Everything the command preview needs — a subset of DownloadState plus the
 *  cross-cutting ffmpeg availability and resolved save folder that live on the
 *  App component. */
export type CommandInputs = {
  url: string
  quality: string
  audioFormat: string
  clipStart: string
  clipEnd: string
  folder: string
  subtitles: boolean
  subLangs: string
  embedMeta: boolean
  sponsorBlock: boolean
  sponsorBlockCategories: string[]
  rateLimit: string
  concurrentFragments: number
  extraArgs: string
  ffmpegAvailable: boolean
  // Compiled from the filename-format builder (see lib/filename/compile.ts).
  // filenameTemplate is the name portion only (no folder); nameArgs are the
  // naming-related flags (--replace-in-metadata, --restrict-filenames, …).
  filenameTemplate: string
  nameArgs: string[]
}

/** Quotes an argument for display if it contains characters a shell would treat
 *  specially. Mirrors how a user would have to type the command themselves. */
function quote(arg: string): string {
  if (arg === '') return "''"
  if (/^[A-Za-z0-9_\-./:%@,=]+$/.test(arg)) return arg
  // Wrap in single quotes, escaping any embedded single quotes.
  return `'${arg.replace(/'/g, `'\\''`)}'`
}

/** Translates the quality choice into yt-dlp args. Mirrors App.qualityArgs in
 *  ytdlp.go. */
function qualityArgs(quality: string, audioFormat: string, ffmpegAvailable: boolean): string[] {
  const q = quality.trim()
  if (q === '' || q === 'best') return []
  if (q === 'audio') {
    const args = ['-f', 'bestaudio/best']
    if (ffmpegAvailable) {
      args.push('-x')
      const f = audioFormat.trim()
      if (f !== '') args.push('--audio-format', f, '--audio-quality', '0')
    }
    return args
  }
  const h = Number(q)
  if (!Number.isInteger(h) || h <= 0) return []
  return ['-S', `res:${h}`]
}

/** Builds the yt-dlp command string the app would run for the current options.
 *  Replicates the argument assembly in DownloadVideo (ytdlp.go), omitting purely
 *  internal plumbing (--newline, --progress-template, --ffmpeg-location) so the
 *  result reads as a command a user could run themselves. Returns "" when there
 *  is no URL yet. */
export function buildCommand(i: CommandInputs): string {
  const url = i.url.trim()
  if (url === '') return ''

  const args: string[] = []

  // Output template: the user's filename format, prefixed with the save folder.
  const folder = i.folder.trim()
  const name = i.filenameTemplate || '%(title)s.%(ext)s'
  args.push('-o', `${folder ? folder + '/' : ''}${name}`)

  args.push(...qualityArgs(i.quality, i.audioFormat, i.ffmpegAvailable))

  // Section download — only when ffmpeg is available and the times are valid,
  // matching what the frontend actually sends.
  const start = i.clipStart.trim()
  const end = i.clipEnd.trim()
  if (
    i.ffmpegAvailable &&
    (start !== '' || end !== '') &&
    isValidTime(start) &&
    isValidTime(end)
  ) {
    const from = start !== '' ? start : '0'
    const to = end !== '' ? end : 'inf'
    args.push('--download-sections', `*${from}-${to}`, '--force-keyframes-at-cuts')
  }

  if (i.subtitles) {
    const langs = i.subLangs.trim() || 'en'
    args.push('--write-subs', '--write-auto-subs', '--sub-langs', langs, '--embed-subs')
  }

  if (i.embedMeta) {
    args.push('--embed-metadata', '--embed-thumbnail', '--embed-chapters')
  }

  if (i.sponsorBlock && i.sponsorBlockCategories.length > 0) {
    args.push('--sponsorblock-remove', i.sponsorBlockCategories.join(','))
  }

  const rate = i.rateLimit.trim()
  if (rate !== '') {
    args.push('--limit-rate', rate)
  }

  if (i.concurrentFragments > 0) {
    args.push('--concurrent-fragments', String(i.concurrentFragments))
  }

  // Naming-related flags from the filename-format builder, before extra args so
  // a power-user flag in Extra arguments still wins.
  args.push(...i.nameArgs)

  // Echo extra arguments verbatim (the backend parses them shell-style), placed
  // last before the URL to mirror how DownloadVideo appends them. Already a
  // command fragment the user typed, so it isn't re-quoted.
  const extra = i.extraArgs.trim()

  return `yt-dlp ${[...args.map(quote), extra, quote(url)].filter(Boolean).join(' ')}`
}
