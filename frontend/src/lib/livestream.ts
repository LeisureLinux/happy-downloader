// Whether a link is a livestream, and what to do about it.
//
// A livestream does not end. yt-dlp downloads it until it is killed, so a
// Twitch link that went through the normal download path sat in the queue
// forever, at "preparing", with no file and no way to tell what it was doing.
//
// The server already has a livestream monitor — it waits for the stream to
// start, records it, and stops it on request. The interface never called it,
// so every livestream went the wrong way.

// The hosts whose links are livestreams.
//
// This is a fallback, not the answer.
//
// yt-dlp works out whether a source is live for every site it has an
// extractor for, and says so in the metadata. That is the answer to use:
// it needs no list here, so a site that starts being live tomorrow is
// handled without a change to this file.
//
// The list is for the case where the metadata has not been fetched yet —
// the field is absent from most extractors — and for the sites whose live
// pages are known to be live by their address. Twitch is here because its
// channel page is a stream when it is live and a recording when it is not,
// and the interface has to offer the recording length before it has asked
// the server anything.
const livestreamHosts = [
  'twitch.tv',
]

// isLivestreamURL reports whether a link should go to the livestream
// monitor, given what the metadata says.
//
// The metadata wins: when it says the source is live, it is live, whatever
// the host is called. The host list is consulted only when the metadata has
// nothing to say.
export const isLivestreamURL = (
  raw: string,
  isLive?: boolean
): boolean => {
  // yt-dlp said the stream is running. That is the answer.
  if (isLive === true) return true

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
