export type RPCMethods =
  | "Service.Exec"
  | "Service.Kill"
  | "Service.Pause"
  | "Service.Resume"
  | "Service.Clear"
  | "Service.Retry"
  | "Service.Running"
  | "Service.KillAll"
  | "Service.FreeSpace"
  | "Service.Formats"
  | "Service.ExecPlaylist"
  | "Service.DirectoryTree"
  | "Service.UpdateExecutable"
  | "Service.CheckYtdlpUpdate"
  | "Service.UpdateYtdlp"
  | "Service.GetVersion"
  | "Service.ExecLivestream"
  | "Service.ProgressLivestream"
  | "Service.KillLivestream"
  | "Service.KillAllLivestream"
  | "Service.ClearCompleted"

export type RPCRequest = {
  method: RPCMethods
  params?: any[]
  id?: string
}

export type RPCResponse<T> = Readonly<{
  result: T
  // net/rpc/jsonrpc reports errors as an object with a message, not a number.
  error: { code: number, message: string } | null
  id?: string
}>

type DownloadInfo = {
  url: string
  filesize_approx?: number
  resolution?: string
  thumbnail: string
  title: string
  vcodec?: string
  acodec?: string
  ext?: string
  created_at: string
}

export enum ProcessStatus {
  PENDING = 0,
  DOWNLOADING,
  COMPLETED,
  ERRORED,
  LIVESTREAM,
  // A download that was stopped on purpose and is waiting to be resumed. It
  // stays in the queue: it has not produced a file, so it is not in the file
  // browser, and it can be resumed where it left off.
  PAUSED,
}

type DownloadProgress = {
  speed: number
  eta: number
  percentage: string
  process_status: ProcessStatus
}

export type RPCResult = Readonly<{
  id: string
  progress: DownloadProgress
  info: DownloadInfo
  /** seconds; how long the video is, known before a byte is transferred */
  duration?: number
  output: {
    savedFilePath: string
    /** actual bytes on disk; 0 when the file is gone */
    fileSize?: number
    /** whether the saved file still exists */
    fileExists?: boolean
    /** dimensions actually written, captured at postprocess */
    width?: number
    height?: number
  }
}>

export type RPCParams = {
  URL: string
  Params?: string
}

export type DLMetadata = {
  formats: Array<DLFormat>
  _type: string
  best: DLFormat
  thumbnail: string
  title: string
  entries: Array<DLMetadata>
  /** seconds; drives the audio-only suggestion and the size estimate */
  duration?: number
  /** manually authored caption tracks, keyed by language code */
  subtitles?: Record<string, unknown>
  /** generated caption tracks, keyed by language code (yt-dlp field name) */
  automatic_captions?: Record<string, unknown>
  /**
   * Whether the source is a livestream that is running now.
   *
   * yt-dlp works it out per site, so this does not depend on a list of hosts
   * kept in the application. It is absent when the extractor does not say —
   * most do not have the field — and absent means "not known to be live".
   */
  is_live?: boolean
}

export type DLFormat = {
  format_id: string
  format_note: string
  fps: number
  resolution: string
  vcodec: string
  acodec: string
  filesize_approx: number
  language: string
  /** container, e.g. mp4 / webm / m4a; decides the merge target */
  ext?: string
}

export type CaptionTrack = {
  language: string
  name?: string
  automatic?: boolean
}

export type SubtitleFile = {
  path: string
  language: string
  label: string
}

export type DirectoryEntry = {
  name: string
  path: string
  size: number
  modTime: string
  isVideo: boolean
  isDirectory: boolean
  // Length in seconds, for media files. Absent when it cannot be determined.
  duration?: number
  // The codecs of the file, as the server read them with ffprobe. The browser
  // cannot decode a file it does not understand, and it cannot tell from the
  // name: a ".mp4" can hold AV1, which is what the downloader produces by
  // default and what a phone without an AV1 decoder cannot play.
  videoCodec?: string
  audioCodec?: string
  // Where the file came from: the site name and the page it was on.
  sourceSite?: string
  sourceURL?: string
  // For a folder: how many files it holds, and when its newest file was
  // written. The figures shown for a folder are about its contents.
  fileCount?: number
  folderModTime?: string
  // The caption files sitting next to this one.
  subtitles?: SubtitleFile[]
}

export type DeleteRequest = Pick<DirectoryEntry, 'path'>

export type PlayRequest = DeleteRequest

export type CustomTemplate = {
  id: string
  name: string
  content: string
}

export enum LiveStreamStatus {
  WAITING,
  IN_PROGRESS,
  COMPLETED,
  ERRORED
}

export type LiveStreamProgress = Record<string, {
  status: LiveStreamStatus
  waitTime: string
  liveDate: string
}>

export type RPCVersion = {
  rpcVersion: string
  rpcBuiltAt: string
  ytdlpVersion: string
  // Where yt-dlp was found. The version alone does not say which binary
  // produced a file.
  ytdlpPath?: string
}

export type ArchiveEntry = {
  id: string
  title: string
  path: string
  thumbnail: string
  source: string
  metadata: string
  created_at: string
}

export type PaginatedResponse<T> = {
  first: number
  next: number
  data: T
}