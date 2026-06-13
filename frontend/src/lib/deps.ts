export type DependencyKey = 'python' | 'ytdlp' | 'extras' | 'ffmpeg'

export const dependencyKeys: DependencyKey[] = ['python', 'ytdlp', 'extras', 'ffmpeg']

export type DependencyInfo = {
  name: string
  description: string
  size: string
  source: string
  details: string
  optional?: boolean
}

export const dependencyInfo: Record<DependencyKey, DependencyInfo> = {
  python: {
    name: 'Python runtime',
    description: 'Standalone bundled copy of Python to run yt-dlp (used instead of yt-dlp binary to improve startup speed)',
    size: '~45 MB',
    source: 'github.com/astral-sh/python-build-standalone',
    details:
      "yt-dlp is a Python program, so the app needs a Python interpreter to run it. " +
      "This downloads a standalone Python build into the app's own folder — it " +
      "won't touch or conflict with any Python already on your system. It comes from " +
      "the official GitHub releases of python-build-standalone, a project maintained by Astral.",
  },
  ytdlp: {
    name: 'yt-dlp',
    description: 'The actual open-source tool from its official GitHub repository',
    size: '~3 MB',
    source: 'github.com/yt-dlp/yt-dlp',
    details:
      "yt-dlp is the open-source program that does the actual video downloading, " +
      "with support for thousands of sites. This downloads the official release " +
      "directly from the yt-dlp project's GitHub releases page.",
  },
  extras: {
    name: 'Site-support Python libraries',
    description: 'Needed by many sites (browser impersonation, certificates)',
    size: '~10 MB',
    source: 'pypi.org (official Python package index)',
    details:
      "A set of optional libraries that yt-dlp recommends: curl_cffi (lets yt-dlp " +
      "identify itself like a normal browser, which sites like Vimeo require), plus " +
      "certificates, websocket, and decryption helpers. Installed from PyPI, the " +
      "official Python package index, using the standalone Python runtime you already downloaded above.",
  },
  ffmpeg: {
    name: 'ffmpeg',
    description: 'Optional — only needed to download a section of a video or extract audio',
    size: '~30–90 MB',
    source: 'ffmpeg.martin-riedl.de (macOS/Linux) · github.com/yt-dlp/FFmpeg-Builds (Windows)',
    details:
      'ffmpeg is the open-source tool yt-dlp uses to cut and convert media. ' +
      'You only need it for the "download only a section" feature and for ' +
      "audio-only extraction — full-video downloads work fine without it. " +
      "On macOS and Linux this downloads signed static builds from Martin " +
      "Riedl's ffmpeg build server; on Windows it uses the builds the " +
      "yt-dlp project itself maintains. You can skip this and install it later.",
    optional: true,
  },
}
