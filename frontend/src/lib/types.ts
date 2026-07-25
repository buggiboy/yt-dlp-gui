export type VideoFormat = {
  height: number
  ext: string
}

export type VideoPreviewData = {
  kind: string
  url: string
  thumbnail: string
  title: string
  duration: string
}

/**
 * A problem with the URL the user typed, ready to render. `kind` mirrors the
 * Go URLCheckKind plus the two verdicts the offline syntax check can reach
 * ('malformed', 'scheme'); the UI only branches on it to decide whether a
 * "supported sites" link is worth offering.
 *
 * `blocking` is what actually gates the Download button — a transient network
 * failure produces a message but leaves the button live.
 */
export type UrlProblem = {
  kind:
    | 'malformed'
    | 'scheme'
    | 'unsupported'
    | 'novideo'
    | 'unavailable'
    | 'auth'
    | 'network'
    | 'unknown'
  message: string
  blocking: boolean
}
