import { describe, expect, it } from 'vitest'
import {
  buildDownloadArgs,
  containersFor,
  defaultOptions,
  renderArgs,
  type DownloadOptions
} from './downloadOptions'

const withOpts = (o: Partial<DownloadOptions>): DownloadOptions => ({
  ...defaultOptions,
  ...o
})

describe('buildDownloadArgs', () => {
  it('merges video and audio by default', () => {
    expect(buildDownloadArgs(defaultOptions)).toEqual([
      '-f', 'bestvideo+bestaudio/best',
      '--embed-metadata', '--embed-thumbnail'
    ])
  })

  it('folds the height cap into the format selector', () => {
    const args = buildDownloadArgs(withOpts({ maxHeight: 1080 }))
    expect(args).toContain('-f')
    expect(args).toContain('bestvideo[height<=1080]+bestaudio/best')
  })

  it('drops the video stream for audio-only', () => {
    const args = buildDownloadArgs(withOpts({ media: 'audio' }))
    expect(args).toContain('--extract-audio')
    expect(args.join(' ')).not.toContain('bestvideo')
  })

  it('drops the audio stream for video-only', () => {
    const args = buildDownloadArgs(withOpts({ media: 'videoOnly' }))
    expect(args).toContain('-f')
    // the selector is the value after -f, not its own argv slot
    expect(args[args.indexOf('-f') + 1]).toBe('bestvideo/bestvideo')
    expect(args.join(' ')).not.toContain('bestaudio')
  })

  it('uses --write-subs for manual subtitles and --write-auto-subs for generated', () => {
    const manual = buildDownloadArgs(withOpts({
      subtitleMode: 'manual', subtitleLangs: ['en', 'zh-Hans']
    }))
    expect(manual).toContain('--write-subs')
    expect(manual).not.toContain('--write-auto-subs')
    expect(manual).toContain('en,zh-Hans')

    const auto = buildDownloadArgs(withOpts({
      subtitleMode: 'auto', subtitleLangs: ['en']
    }))
    expect(auto).toContain('--write-auto-subs')
    expect(auto).not.toContain('--write-subs')
  })

  it('emits no subtitle flags when no language is selected', () => {
    const args = buildDownloadArgs(withOpts({ subtitleMode: 'manual', subtitleLangs: [] }))
    expect(args.join(' ')).not.toContain('subs')
  })

  it('embeds subtitles only when asked', () => {
    const off = buildDownloadArgs(withOpts({
      subtitleMode: 'manual', subtitleLangs: ['en'], embedSubtitles: false
    }))
    expect(off).not.toContain('--embed-subs')

    const on = buildDownloadArgs(withOpts({
      subtitleMode: 'manual', subtitleLangs: ['en'], embedSubtitles: true
    }))
    expect(on).toContain('--embed-subs')
  })

  it('passes the container through as --merge-output-format', () => {
    const args = buildDownloadArgs(withOpts({ container: 'mkv' }))
    expect(args).toContain('--merge-output-format')
    expect(args).toContain('mkv')
  })

  it('omits metadata flags when disabled', () => {
    const args = buildDownloadArgs(withOpts({ writeMetadata: false }))
    expect(args.join(' ')).not.toContain('embed-metadata')
    expect(args.join(' ')).not.toContain('embed-thumbnail')
  })

  it('keeps a no-cap selector free of the height predicate', () => {
    const args = buildDownloadArgs(withOpts({ maxHeight: 0 }))
    expect(args).toContain('bestvideo+bestaudio/best')
    expect(args.join(' ')).not.toContain('height<=')
  })
})

describe('renderArgs', () => {
  it('quotes values containing whitespace or commas', () => {
    expect(renderArgs(['--sub-langs', 'en,zh-Hans'])).toBe("--sub-langs 'en,zh-Hans'")
    expect(renderArgs(['--output', 'my file.mp4'])).toBe("--output 'my file.mp4'")
  })

  it('leaves plain tokens alone', () => {
    expect(renderArgs(['-f', 'bestaudio'])).toBe('-f bestaudio')
  })

  it('round-trips a full option set into a pasteable line', () => {
    const line = renderArgs(buildDownloadArgs(withOpts({
      media: 'video', maxHeight: 720, container: 'mp4'
    })))
    expect(line).toContain('bestvideo[height<=720]+bestaudio/best')
    expect(line).toContain('--merge-output-format mp4')
  })
})

describe('audio format targets', () => {
  it('asks for mp3 through --audio-format, not a video container', () => {
    const args = buildDownloadArgs({ ...defaultOptions, media: 'audio', container: 'mp3' })
    expect(args).toContain('--extract-audio')
    expect(args).toContain('--audio-format')
    expect(args[args.indexOf('--audio-format') + 1]).toBe('mp3')
    expect(args).not.toContain('--merge-output-format')
  })

  it('keeps --merge-output-format for video', () => {
    const args = buildDownloadArgs({ ...defaultOptions, media: 'video', container: 'mkv' })
    expect(args).toContain('--merge-output-format')
    expect(args).not.toContain('--audio-format')
  })

  it('offers audio targets only when the download is audio-only', () => {
    expect(containersFor('audio')).toContain('mp3')
    expect(containersFor('audio')).not.toContain('mp4')
    expect(containersFor('video')).toContain('mp4')
    expect(containersFor('video')).not.toContain('mp3')
  })
})
