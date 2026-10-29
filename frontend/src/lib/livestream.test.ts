import { describe, expect, it } from 'vitest'
import { isLivestreamURL } from './livestream'

describe('isLivestreamURL', () => {
  it('believes yt-dlp when it says the source is live', () => {
    // The metadata is the answer. A site that is live is live, whatever
    // its address looks like, so no list of hosts is needed for it.
    expect(isLivestreamURL('https://live.bilibili.com/1741634790', true)).toBe(true)
    expect(isLivestreamURL('https://example.com/whatever', true)).toBe(true)
  })

  it('does not call a source live when yt-dlp says it is not', () => {
    // A YouTube video is not a livestream, even though the site also has
    // livestreams and used to be treated as one.
    expect(isLivestreamURL('https://www.youtube.com/watch?v=dQw4w9WgXcQ', false)).toBe(false)
    expect(isLivestreamURL('https://live.bilibili.com/1741634790', false)).toBe(false)
  })

  it('falls back to the host when the metadata says nothing', () => {
    // Most extractors have no is_live field at all, so the answer is
    // absent and the host list is consulted.
    expect(isLivestreamURL('https://www.twitch.tv/emiru')).toBe(true)
    expect(isLivestreamURL('https://twitch.tv/emiru')).toBe(true)
  })

  it('does not mistake an ordinary site for a livestream', () => {
    expect(isLivestreamURL('https://www.bilibili.com/video/BV1gjxu6CED1/')).toBe(false)
    expect(isLivestreamURL('https://vimeo.com/1223839596')).toBe(false)
    expect(isLivestreamURL('https://example.com/video.mp4')).toBe(false)
  })

  it('says no to what is not a link', () => {
    expect(isLivestreamURL('')).toBe(false)
    expect(isLivestreamURL('   ')).toBe(false)
    expect(isLivestreamURL('not a url')).toBe(false)
  })
})
