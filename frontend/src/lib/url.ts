/**
 * Extracts a YouTube video ID from a URL string.
 * Returns null for non-YouTube URLs or malformed input —
 * the caller can then fall back to a generic preview lookup.
 */
export function extractVideoId(raw: string): string | null {
  let parsed: URL
  try {
    parsed = new URL(raw.trim())
  } catch {
    return null
  }
  const host = parsed.hostname.replace(/^(www\.|m\.|music\.)/, '')
  if (host === 'youtu.be') {
    const id = parsed.pathname.split('/')[1] ?? null
    return id && /^[\w-]{11}$/.test(id) ? id : null
  }
  if (host === 'youtube.com' || host === 'youtube-nocookie.com') {
    let id: string | null = null
    if (parsed.pathname === '/watch') {
      id = parsed.searchParams.get('v')
    } else {
      const match = parsed.pathname.match(/^\/(embed|shorts|live|v)\/([^/?]+)/)
      id = match ? match[2] : null
    }
    return id && /^[\w-]{11}$/.test(id) ? id : null
  }
  return null
}
