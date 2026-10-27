import { describe, expect, it } from 'vitest'
import { canPlay, containerOf, describeCodecs } from './playback'

// The check that stops a black rectangle.
//
// The verdict comes from the browser's own canPlayType, which is the same
// engine that would do the decoding — so it knows about the hardware, not
// just the codec. That makes it hard to test in a headless environment
// without stubbing the answer, so the tests below check the parts that are
// ours: the container lookup, the codec names, and the refusal to play a
// file whose container nobody knows.

const withVideo = (codec: string) => ({
  canPlayType: (type: string) => (type.includes('avc1') ? 'probably' : ''),
})

const withAudio = (codec: string) => ({
  canPlayType: (type: string) => (type.includes('mp4a') ? 'probably' : ''),
})

describe('containerOf', () => {
  it('takes the extension from the name', () => {
    expect(containerOf('Video.mp4')).toBe('mp4')
    expect(containerOf('A.B.webm')).toBe('webm')
  })

  it('is empty for a name with no extension', () => {
    expect(containerOf('Video')).toBe('')
    expect(containerOf('.hidden')).toBe('')
  })
})

describe('canPlay', () => {
  it('refuses a file whose container is unknown', () => {
    // The MIME type is unknown, so no codec will play in it — and a
    // codec-only check would have said yes.
    expect(canPlay('Video.xyz', 'h264', 'aac')).toBe(false)
  })

  it('refuses a codec the browser does not know', () => {
    expect(canPlay('Video.mp4', 'theora', undefined)).toBe(false)
  })
})

describe('describeCodecs', () => {
  it('names the codecs in a way a user can act on', () => {
    expect(describeCodecs('av1', 'opus')).toBe('AV1 + OPUS')
    expect(describeCodecs('h264', undefined)).toBe('H264')
    expect(describeCodecs(undefined, 'aac')).toBe('AAC')
    expect(describeCodecs()).toBe('')
  })
})
