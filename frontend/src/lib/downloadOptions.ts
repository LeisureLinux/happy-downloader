// The download dialog's choices, and the yt-dlp arguments they produce.
//
// The dialog shows the resolved command line as the operator changes options,
// so the mapping from choice to argument has to live in one place and be a
// pure function. Anything computed here can be asserted in a test, and the
// preview the user reads is the same string the downloader will run.

// MediaChoice is what the operator wants out of the video.
export type MediaChoice =
  // video with its audio track merged in
  | 'video'
  // audio only, video stream discarded
  | 'audio'
  // video only, audio stream discarded
  | 'videoOnly'

export type SubtitleMode = 'none' | 'manual' | 'auto'

export type DownloadOptions = {
  media: MediaChoice
  // resolution cap in pixels of height; 0 means "best available"
  maxHeight: number
  subtitleMode: SubtitleMode
  // language codes to fetch, e.g. ['en', 'zh-Hans']
  subtitleLangs: string[]
  // burn the chosen subtitles into the video stream
  embedSubtitles: boolean
  // container for the merged output; empty lets yt-dlp decide
  container: string
  // thumbnail and metadata written next to the file
  writeMetadata: boolean
  // ask for H.264 and AAC instead of whatever the source offers
  //
  // Not the default: AV1 is what YouTube serves and it is smaller and
  // better. But a device without an AV1 decoder — an iPhone 12, anything
  // older — cannot play it at all, and the player says so rather than
  // showing a black rectangle. This is the way to get a file that plays
  // there.
  //
  // The cost is size, and the amount varies: YouTube encodes the two
  // versions separately, and on some videos the H.264 one is 2.5 times
  // bigger, on others it is smaller.
  compatible: boolean
}

export const defaultOptions: DownloadOptions = {
  media: 'video',
  maxHeight: 0,
  subtitleMode: 'none',
  subtitleLangs: [],
  embedSubtitles: false,
  container: '',
  writeMetadata: true,
  compatible: false,
}

// Resolution presets, in pixels of height. 0 is the "no cap" entry.
export const heightPresets: Array<{ label: string; value: number }> = [
  { label: 'Best', value: 0 },
  { label: '2160p (4K)', value: 2160 },
  { label: '1440p', value: 1440 },
  { label: '1080p', value: 1080 },
  { label: '720p', value: 720 },
  { label: '480p', value: 480 },
  { label: '360p', value: 360 },
]

// Containers yt-dlp can produce with --merge-output-format. The list is
// deliberately short: these are the ones a merged video reliably lands in.
export const videoContainers = ['', 'mp4', 'mkv', 'webm', 'mov']

// Audio targets for --audio-format. Separate from the video list on purpose:
// asking for an mp4 container on an audio-only download produced an m4a named
// .mp4, and mp3 was simply unreachable.
export const audioContainers = ['', 'mp3', 'm4a', 'opus', 'flac', 'wav', 'vorbis']

// Kept for the existing tests and for callers that do not know the media kind.
export const containers = videoContainers

/** The container list that applies to the current media choice. */
export function containersFor(media: DownloadOptions['media']): string[] {
  return media === 'audio' ? audioContainers : videoContainers
}

export const subtitleLangs = [
  'en', 'zh-Hans', 'zh-Hant', 'ja', 'ko', 'es', 'fr', 'de', 'ru', 'pt'
]

/**
 * Build the yt-dlp arguments for the chosen options.
 *
 * The format selector is the interesting part: yt-dlp's `-f` is a small
 * expression language, and the three media choices map onto it as
 *
 *   video      bestvideo[height<=N]+bestaudio   (merge)
 *   audio      bestaudio                        (drop video)
 *   videoOnly  bestvideo[height<=N]             (drop audio)
 *
 * A height cap is folded into the selector rather than passed as a separate
 * flag, because --format-sort would still pick a taller stream when a shorter
 * one is the only one that fits the cap.
 */
export function buildDownloadArgs(o: DownloadOptions): string[] {
  const args: string[] = []

  const cap = o.maxHeight > 0 ? `[height<=${o.maxHeight}]` : ''

  // The compatible mode asks for H.264 and AAC by name.
  //
  // It has to be in the selector rather than a separate flag: yt-dlp picks
  // the best format the source offers, and that is AV1 on YouTube. Asking
  // for the container alone does not help — a merge of AV1 and Opus into an
  // MP4 is still AV1, and still unplayable on a device without an AV1
  // decoder.
  //
  // The fallback matters: a source with no H.264 at all would otherwise
  // produce nothing, and a smaller file is better than no file.
  const compatVideo = o.compatible ? '[vcodec^=avc1]' : ''
  const compatAudio = o.compatible ? '[acodec^=mp4a]' : ''

  switch (o.media) {
    case 'audio':
      args.push('-f', `bestaudio${compatAudio}/bestaudio/best`)
      args.push('--extract-audio')
      break
    case 'videoOnly':
      args.push('-f', `bestvideo${cap}${compatVideo}/bestvideo${cap}`)
      break
    case 'video':
    default:
      args.push('-f', `bestvideo${cap}${compatVideo}+bestaudio${compatAudio}/best`)
      break
  }

  // Subtitles. yt-dlp fetches manual and generated tracks from two different
  // endpoints, so the mode picks the flag pair.
  if (o.subtitleMode !== 'none' && o.subtitleLangs.length > 0) {
    const langs = o.subtitleLangs.join(',')

    if (o.subtitleMode === 'manual') {
      args.push('--write-subs')
    } else {
      args.push('--write-auto-subs')
    }

    args.push('--sub-langs', langs)
    args.push('--convert-subs', 'srt')

    if (o.embedSubtitles) {
      // Only mp4 and mov can carry a soft subtitle track; mkv can too but
      // needs the track marked default, which is what --embed-subs does.
      args.push('--embed-subs')
    }
  }

  // --merge-output-format only applies to a video merge; for audio-only the
  // container is the audio codec target and belongs to --audio-format.
  if (o.container) {
    if (o.media === 'audio') {
      args.push('--audio-format', o.container)
    } else {
      args.push('--merge-output-format', o.container)
    }
  }

  if (o.writeMetadata) {
    args.push('--embed-metadata', '--embed-thumbnail')
  }

  return args
}

/**
 * A one-line rendering of the arguments, for the preview box.
 *
 * Values are quoted when they contain whitespace, so the line can be pasted
 * into a shell and behave the same way the downloader will.
 */
export function renderArgs(args: string[]): string {
  return args
    .map(a => (a.includes(' ') || a.includes(',') ? `'${a.replace(/'/g, "'\\''")}'` : a))
    .join(' ')
}

// The list of subtitle language codes a video actually offers, manual first
// then generated, de-duplicated.
export function availableSubtitleLangs(meta?: {
  subtitles?: Record<string, unknown>
  automatic_captions?: Record<string, unknown>
}): { manual: string[]; auto: string[] } {
  return {
    manual: Object.keys(meta?.subtitles ?? {}),
    auto: Object.keys(meta?.automatic_captions ?? {}),
  }
}

// Whether the video has any audio stream at all. A source without one makes
// the audio-only choice meaningless, so the dialog hides it.
export function hasAudioStream(meta?: { formats?: Array<{ acodec?: string }> }): boolean {
  return (meta?.formats ?? []).some(f => f.acodec && f.acodec !== 'none')
}

export function hasVideoStream(meta?: { formats?: Array<{ vcodec?: string }> }): boolean {
  return (meta?.formats ?? []).some(f => f.vcodec && f.vcodec !== 'none')
}
