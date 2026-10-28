// Whether a link is a livestream, and what to do about it.
//
// A livestream does not end. yt-dlp downloads it until it is killed, so a
// Twitch link that went through the normal download path sat in the queue
// forever, at "preparing", with no file and no way to tell what it was doing.
//
// The server already has a livestream monitor — it waits for the stream to
// start, records it, and stops it on request. The interface never called it,
// so every livestream went the wrong way.

// The sites whose links are livestreams.
//
// A channel's page is a livestream when it is live and a recording when it is
// not, and the server works that out for itself. What the interface needs to
// know is which path to send it down.
const livestreamHosts = [
  'twitch.tv',
  'youtube.com',
  'youtu.be',
]

// isLivestreamURL reports whether a link should go to the livestream
// monitor.
//
// It is only about the site: whether the stream is live is the server's
// business, and the monitor waits for it.
export const isLivestreamURL = (raw: string): boolean => {
  const url = raw.trim()

  if (!url) return false

  let host: string
  try {
    host = new URL(url).hostname.toLowerCase().replace(/^www\./, '')
  } catch {
    return false
  }

  return livestreamHosts.some(suffix =>
    host === suffix || host.endsWith('.' + suffix)
  )
}
