// What a file or folder can be moved into.

/**
 * moveTargets picks the folders a selection can be moved into.
 *
 * `destinations` is every folder under the download directory, as full paths;
 * `current` is the folder being looked at, and `moving` is what has been
 * selected.
 *
 * Two are removed, and the second is the one that was got wrong:
 *
 * - the folder the selection is already in, because moving into it does
 *   nothing;
 * - anything that is part of the selection, or inside it, because a folder
 *   cannot be put inside itself.
 *
 * The "inside it" test runs one way only — does the *destination* lie under
 * the thing being moved. Written the other way round it asks whether the
 * destination is an ancestor, which is the case that has to keep working:
 * moving a folder to its parent. Moving a file worked and moving a folder up
 * was impossible, and the only difference between them is which one has
 * children.
 */
export const moveTargets = (
  destinations: string[],
  current: string,
  moving: string[],
): string[] => {
  const normal = (p: string): string => p.replace(/\/+$/, '')
  const here = normal(current)

  const selected = moving.map(normal)

  return destinations.filter(dir => {
    const target = normal(dir)

    if (target === here) return false

    return !selected.some(moved => target === moved || target.startsWith(moved + '/'))
  })
}
