/**
 * Result of the instant, offline syntax check. `null` means "nothing obviously
 * wrong" — which is not the same as "this is a video": only the backend probe
 * (CheckURL) can say that.
 */
export type UrlSyntaxIssue = {
  /** 'scheme' when the protocol is wrong, 'malformed' for everything else. */
  kind: 'scheme' | 'malformed'
  message: string
}

/** Matches "scheme:" at the start of a string, per RFC 3986. */
const SCHEME_RE = /^([a-z][a-z0-9+.-]*):/i

/** A hostname we'd accept: at least one dot and a 2+ letter final label. */
const HOSTNAME_RE = /^[a-z0-9-]+(\.[a-z0-9-]+)*\.[a-z]{2,}$/i

/** Bare IPv4, which URL() happily parses but HOSTNAME_RE would reject. */
const IPV4_RE = /^\d{1,3}(\.\d{1,3}){3}$/

const GENERIC_MESSAGE =
  "That doesn't look like a web address. Paste a full link starting with https://"

/**
 * Checks whether a string is even shaped like a web address, without touching
 * the network. Catches the cases that aren't worth a yt-dlp round trip: search
 * terms typed into the field, half-pasted links, and non-web protocols.
 *
 * Deliberately lenient about a missing scheme — "youtube.com/watch?v=…" is a
 * perfectly normal thing to paste, and yt-dlp accepts it — so we only complain
 * when the host itself doesn't look like a domain.
 *
 * Returns null for empty input: an untouched field isn't an error.
 */
export function checkUrlSyntax(raw: string): UrlSyntaxIssue | null {
  const trimmed = raw.trim()
  if (trimmed === '') return null

  if (/\s/.test(trimmed)) {
    return { kind: 'malformed', message: "Web addresses can't contain spaces." }
  }

  // Reject non-web protocols by name so the message can be specific — a user
  // who pasted a magnet or ftp link gets told why, not just "that looks wrong".
  const schemeMatch = SCHEME_RE.exec(trimmed)
  let candidate = trimmed
  if (schemeMatch) {
    const scheme = schemeMatch[1].toLowerCase()
    if (scheme !== 'http' && scheme !== 'https') {
      return {
        kind: 'scheme',
        message: `${scheme}: links aren't supported — paste an http or https address.`,
      }
    }
  } else {
    // No scheme: assume https so URL() can parse it, matching what yt-dlp does.
    candidate = `https://${trimmed}`
  }

  let parsed: URL
  try {
    parsed = new URL(candidate)
  } catch {
    return { kind: 'malformed', message: GENERIC_MESSAGE }
  }

  const host = parsed.hostname
  if (!host) return { kind: 'malformed', message: GENERIC_MESSAGE }
  // Loopback and bare IPs are valid targets even though they have no TLD.
  if (host === 'localhost' || IPV4_RE.test(host) || host.startsWith('[')) return null
  if (!HOSTNAME_RE.test(host)) {
    return { kind: 'malformed', message: GENERIC_MESSAGE }
  }

  return null
}

/**
 * Parses a URL the way the rest of the app treats user input: a missing scheme
 * is assumed to be https, because "youtube.com/watch?v=…" is a perfectly normal
 * thing to paste (and yt-dlp accepts it). Returns null if it still won't parse.
 */
function parseLoose(raw: string): URL | null {
  const trimmed = raw.trim()
  if (trimmed === '') return null
  const candidate = SCHEME_RE.test(trimmed) ? trimmed : `https://${trimmed}`
  try {
    return new URL(candidate)
  } catch {
    return null
  }
}

/** Hostname with the interchangeable subdomains stripped. */
function bareHost(parsed: URL): string {
  return parsed.hostname.replace(/^(www\.|m\.|music\.)/, '')
}

/** The YouTube domains that use `?list=` to mark a playlist. */
const YOUTUBE_HOSTS = new Set(['youtube.com', 'youtube-nocookie.com', 'youtu.be'])

/**
 * Extracts a YouTube video ID from a URL string.
 * Returns null for non-YouTube URLs or malformed input —
 * the caller can then fall back to a generic preview lookup.
 */
export function extractVideoId(raw: string): string | null {
  const parsed = parseLoose(raw)
  if (!parsed) return null
  const host = bareHost(parsed)
  if (host === 'youtu.be') {
    const id = parsed.pathname.split('/')[1] ?? null
    return id && /^[\w-]{11}$/.test(id) ? id : null
  }
  if (host === 'youtube.com' || host === 'youtube-nocookie.com') {
    let id: string | null
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

/**
 * Detects whether a URL points into a playlist, returning an identifier for it
 * or null.
 *
 * Offline and therefore necessarily approximate: yt-dlp supports playlists on
 * hundreds of sites and only it knows for sure, but asking would cost a second
 * network probe on every keystroke. The trade is a good one because the stakes
 * are low in both directions — a false positive shows an extra checkbox nobody
 * has to tick, and a false negative just leaves the current single-video
 * behaviour in place.
 *
 * The returned string is only used as an identity (to notice when the user has
 * moved to a *different* playlist), so its exact shape doesn't matter.
 */
export function extractPlaylistId(raw: string): string | null {
  const parsed = parseLoose(raw)
  if (!parsed) return null

  // YouTube marks a playlist with ?list=…, on both /playlist and /watch URLs —
  // the latter being what you get by copying a video's address while watching
  // it inside a playlist.
  if (YOUTUBE_HOSTS.has(bareHost(parsed))) {
    const list = parsed.searchParams.get('list')
    return list && /^[\w-]+$/.test(list) ? list : null
  }

  // Everywhere else, a /playlist path segment is the nearest thing to a shared
  // convention (Vimeo, SoundCloud, Bandcamp-likes).
  const segments = parsed.pathname.split('/').filter(Boolean)
  if (segments.some((s) => s === 'playlist' || s === 'playlists')) {
    return parsed.pathname + parsed.search
  }

  return null
}
