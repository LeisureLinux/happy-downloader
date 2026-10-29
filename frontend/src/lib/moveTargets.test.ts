import { describe, expect, it } from 'vitest'
import { moveTargets } from './moveTargets'

const root = '/home/axu/media/downloads'
const ai = root + '/AI'
const english = root + '/English'
const news = english + '/News'

const tree = [root, ai, english, news]

describe('moveTargets', () => {
  it('offers the parent of the folder a file is in', () => {
    // This is the case that was broken for a folder: with the test written
    // the other way round, the parent was treated as being inside the moving
    // folder, so a folder could never be moved up.
    expect(moveTargets(tree, english, [english + '/clip.mp4'])).toEqual([
      root,
      ai,
      news,
    ])
  })

  it('offers the grandparent when a folder inside one is being moved', () => {
    // Looking inside English and moving News: the root and the siblings are
    // the point. Written the other way round, every ancestor of News was
    // treated as being inside it, so nothing above English could be chosen.
    const got = moveTargets(tree, english, [news])

    expect(got).toContain(root)
    expect(got).toContain(ai)
    // The folder being looked at is where it already is, and News is what is
    // moving.
    expect(got).not.toContain(english)
    expect(got).not.toContain(news)
  })

  it('does not offer the folder itself when moving it from where it sits', () => {
    // Looking at the root and moving English into the root is a no-op.
    expect(moveTargets(tree, root, [english])).toEqual([ai])
  })

  it('does not offer the folder the selection is already in', () => {
    expect(moveTargets(tree, ai, [ai + '/clip.mp4'])).not.toContain(ai)
  })

  it('offers sibling folders', () => {
    const got = moveTargets(tree, ai, [ai + '/clip.mp4'])

    expect(got).toContain(english)
    expect(got).toContain(news)
  })

  it('keeps the root when files are moved out of a subfolder', () => {
    // The complaint that started this: a file inside a folder could go deeper
    // but not back out, because the root was not on the list.
    const got = moveTargets(tree, english, [english + '/clip.mp4'])

    expect(got[0]).toBe(root)
  })

  it('tolerates a trailing slash', () => {
    expect(moveTargets(tree, english + '/', [english + '/clip.mp4'])).toEqual([
      root,
      ai,
      news,
    ])
  })

  it('handles an empty selection', () => {
    expect(moveTargets(tree, ai, [])).toEqual([root, english, news])
  })
})
