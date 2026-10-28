// Whether the browser can play a file, before it tries.
//
// The player shows a black rectangle when the browser cannot decode the file,
// and nothing else: no message, no clue. The reason is that the downloader
// produces what the source offers, and what YouTube offers is AV1 in a WebM
// or Matroska container. Every browser can play H.264 and AAC in MP4; only
// some can play AV1, and only on hardware that has an AV1 decoder — which
// means an iPhone 12 cannot, and an iPhone 17 can.
//
// So the codecs are read from the file by the server, and the browser is
// asked what it can decode before the file is handed to a <video> element.
// canPlayType is the browser's own answer, so this is not a list of guesses:
// it asks the same engine that will do the decoding.

// The codecs, as the browser spells them in a MIME type, keyed by ffprobe's
// names.
//
// Every codec the downloader can produce, as the browser spells it in a MIME
// type.
//
// H.264 and AAC are here even though every browser can decode them, because
// a codec missing from this table was treated as "cannot play" — and AAC was
// missing, so every file with an AAC track was refused. A file that plays
// perfectly showed the message saying it could not be played.
const VIDEO_CODECS: Record<string, string> = {
  av1: 'av01.0.05M.08',
  vp9: 'vp09.00.10.08',
  vp8: 'vp8',
  hevc: 'hvc1.1.6.L93.B0',
  avc1: 'avc1.42E01E',
  h264: 'avc1.42E01E',
}

const AUDIO_CODECS: Record<string, string> = {
  opus: 'opus',
  vorbis: 'vorbis',
  flac: 'flac',
  mp3: 'mp4a.40.34',
  aac: 'mp4a.40.2',
  mp4a: 'mp4a.40.2',
}

// The containers, as MIME types.
//
// The container is as much a part of the question as the codec: Safari plays
// AV1 in MP4 but not in WebM, and refuses Matroska outright. A file with a
// container nobody knows is not playable, and a codec-only check would miss
// that entirely.
const CONTAINERS: Record<string, string> = {
  mp4: 'video/mp4',
  m4v: 'video/mp4',
  webm: 'video/webm',
  mkv: 'video/x-matroska',
  mov: 'video/quicktime',
}

// The container of a file, from its name.
export function containerOf(name: string): string {
  const dot = name.lastIndexOf('.')
  if (dot <= 0) return ''
  return name.slice(dot + 1).toLowerCase()
}

// Whether the browser can play a file with these codecs.
//
// The check is the browser's own canPlayType, which is the same engine that
// will do the decoding — so it accounts for the hardware, the OS version and
// the container, none of which a table in this file would get right.
export function canPlay(
  name: string,
  videoCodec?: string,
  audioCodec?: string
): boolean {
  const box = containerOf(name)
  const mime = CONTAINERS[box]

  // An unknown container means the MIME type is unknown, and no codec will
  // play in a container the browser does not know. Refused rather than
  // guessed.
  //
  // This is checked before the browser is asked, so it holds even where there
  // is no browser to ask.
  if (!mime) return false

  // The codecs are looked up before the browser is asked, because a codec
  // this table does not know is a refusal on its own: there is nothing to ask
  // about. The lookup happens either way, so the answer does not depend on
  // whether there is a browser to consult.
  const video = videoCodec ? VIDEO_CODECS[videoCodec] : undefined
  const audio = audioCodec ? AUDIO_CODECS[audioCodec] : undefined

  if (videoCodec && !video) return false
  if (audioCodec && !audio) return false

  if (typeof document === 'undefined') return true

  // Both codecs are asked of the same element, with the same MIME type, in
  // one question.
  //
  // Asking them separately was wrong twice over. The audio question used
  // "audio/mp4" while the file is in whatever container it is in, so an Opus
  // track in an MP4 was judged by a different rule than the video next to
  // it. And asking one codec at a time answers a question nobody asked: the
  // browser has to be able to play the file as a whole, video and audio
  // together, and a file whose two halves it can each play separately is not
  // necessarily one it can play.
  const codecs = [video, audio].filter(Boolean).join(',')

  if (!codecs) return true

  const probe = document.createElement('video')
  if (probe.canPlayType(`${mime}; codecs="${codecs}"`) === '') return false

  return true
}

// The codec names, in words the user can act on.
//
// The names are the ones ffprobe reports, because those are the names the
// user will meet in the download options — "av1" is a word they can look for,
// "AV01.0.05M.08" is not.
export function describeCodecs(videoCodec?: string, audioCodec?: string): string {
  const bits: string[] = []

  if (videoCodec) {
    bits.push(videoCodec === 'av1' ? 'AV1' : videoCodec.toUpperCase())
  }

  if (audioCodec) {
    bits.push(audioCodec.toUpperCase())
  }

  return bits.join(' + ')
}
